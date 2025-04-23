package remoteread

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/prometheus/promql/promqltest"

	"github.com/grafana/mimir-proxies/pkg/errorx"

	"github.com/grafana/mimir/pkg/frontend/querymiddleware"
	"github.com/grafana/mimir/pkg/scheduler/queue"

	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/tsdb/chunkenc"
	"github.com/prometheus/prometheus/util/annotations"

	"github.com/gorilla/mux"
	"github.com/grafana/dskit/user"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageQueryable_Querier_Select(t *testing.T) {
	const tenantID = "42"
	ctx := user.InjectOrgID(context.Background(), tenantID)

	storage := promqltest.LoadedStorage(t, `
		load 1m
			test_metric1{foo="bar",baz="qux"} 1+1x5
	`)

	{
		// we can't use the promql syntax to load a label with a dash, so we use the storage appender instead
		metricWithDashInTag := labels.Labels{
			{Name: labels.MetricName, Value: "test_metric_with_dash"},
			{Name: "has-dash", Value: "true"},
		}
		app := storage.Appender(ctx)
		for ts := int64(0); ts < 5; ts++ {
			const unknownRef = 0
			_, err := app.Append(unknownRef, metricWithDashInTag, ts*60e3, float64(ts)+1)
			require.NoError(t, err)
		}
		require.NoError(t, app.Commit())
	}

	h := remote.NewReadHandler(nil, nil, storage, func() (_ config.Config) { return }, 1e6, 1, 0)
	router := mux.NewRouter()
	router.Handle("/path/api/v1/read", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if reqTenantID := request.Header.Get(user.OrgIDHeaderName); reqTenantID != tenantID {
			http.Error(writer, fmt.Sprintf("got wrong tenant %q", reqTenantID), http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(writer, request)
	}))
	srv := httptest.NewServer(router)
	defer srv.Close()

	cfg := StorageQueryableConfig{
		Address:      srv.URL + "/path",
		Timeout:      time.Second,
		KeepAlive:    time.Second,
		MaxIdleConns: 10,
		MaxConns:     10,

		ClientName: "test",
	}

	client, err := NewStorageQueryable(cfg, nil)
	require.NoError(t, err)

	querier, err := client.Querier(60e3, 120e3)
	require.NoError(t, err)
	defer func() {
		_ = querier.Close()
	}()

	t.Run("happy case", func(t *testing.T) {
		set := querier.Select(ctx, true, nil, labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"))
		require.NoError(t, set.Err())
		require.True(t, set.Next())
		series := set.At()
		require.Equal(t, labels.Labels{{Name: labels.MetricName, Value: "test_metric1"}, {Name: "baz", Value: "qux"}, {Name: "foo", Value: "bar"}}, series.Labels())

		it := series.Iterator(nil)
		require.Equal(t, chunkenc.ValFloat, it.Next())

		ts, val := it.At()
		require.Equal(t, int64(60e3), ts)
		require.Equal(t, float64(2), val)
		require.Equal(t, chunkenc.ValFloat, it.Next())

		ts, val = it.At()
		require.Equal(t, int64(120e3), ts)
		require.Equal(t, float64(3), val)
	})

	t.Run("label with a dash", func(t *testing.T) {
		set := querier.Select(ctx, true, nil, labels.MustNewMatcher(labels.MatchEqual, "has-dash", "true"))
		require.NoError(t, set.Err())
		require.True(t, set.Next(), "Response should have series")
		series := set.At()
		require.Equal(t, labels.Labels{{Name: labels.MetricName, Value: "test_metric_with_dash"}, {Name: "has-dash", Value: "true"}}, series.Labels())
	})
}

func TestStorageQueryable_Querier_TooManyRequests(t *testing.T) {
	const tenantID = "42"
	ctx := user.InjectOrgID(context.Background(), tenantID)

	router := mux.NewRouter()
	router.Handle("/path/api/v1/read", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Error(writer, queue.ErrTooManyRequests.Error(), http.StatusTooManyRequests)
	}))
	srv := httptest.NewServer(router)
	defer srv.Close()

	cfg := StorageQueryableConfig{
		Address:      srv.URL + "/path",
		Timeout:      time.Second,
		KeepAlive:    time.Second,
		MaxIdleConns: 10,
		MaxConns:     10,

		ClientName: "test",
	}

	client, err := NewStorageQueryable(cfg, nil)
	require.NoError(t, err)

	querier, err := client.Querier(60e3, 120e3)
	require.NoError(t, err)
	defer func() {
		_ = querier.Close()
	}()

	set := querier.Select(ctx, true, nil, labels.MustNewMatcher(labels.MatchEqual, "has-dash", "true"))
	require.Error(t, set.Err())
	require.ErrorAs(t, set.Err(), &errorx.TooManyRequests{})
}

func TestStorageQueryable_Querier_LabelValues(t *testing.T) {
	const tenantID = "42"

	for _, tc := range []struct {
		name                string
		doRequest           func(context.Context, storage.LabelQuerier) ([]string, annotations.Annotations, error)
		expectedQueryParams map[string]string
	}{
		{
			name: "with matchers",
			doRequest: func(ctx context.Context, querier storage.LabelQuerier) ([]string, annotations.Annotations, error) {
				return querier.LabelValues(ctx, "mylabelname", nil, labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"))
			},
			expectedQueryParams: map[string]string{
				"start":   "60",
				"end":     "120",
				"match[]": `{foo="bar"}`,
			},
		},
		{
			name: "without matchers",
			doRequest: func(ctx context.Context, querier storage.LabelQuerier) ([]string, annotations.Annotations, error) {
				return querier.LabelValues(ctx, "mylabelname", nil)
			},
			expectedQueryParams: map[string]string{
				"start":   "60",
				"end":     "120",
				"match[]": ``, // we check explicitly that it's not present
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expectedValues := []string{"foo", "bar"}

			router := mux.NewRouter()
			router.Handle("/path/api/v1/label/mylabelname/values", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if reqTenantID := request.Header.Get(user.OrgIDHeaderName); reqTenantID != tenantID {
					http.Error(writer, fmt.Sprintf("got wrong tenant %q", reqTenantID), http.StatusUnauthorized)
					return
				}
				for param, expectedValue := range tc.expectedQueryParams {
					if value := request.URL.Query().Get(param); value != expectedValue {
						http.Error(writer, fmt.Sprintf("param %q should be %q got %q", param, expectedValue, value), http.StatusBadRequest)
						return
					}
				}
				require.NoError(t, json.NewEncoder(writer).Encode(struct{ Data []string }{Data: expectedValues}))
			}))
			srv := httptest.NewServer(router)
			defer srv.Close()

			cfg := StorageQueryableConfig{
				Address:      srv.URL + "/path",
				Timeout:      time.Second,
				KeepAlive:    time.Second,
				MaxIdleConns: 10,
				MaxConns:     10,

				ClientName: "test",
			}

			client, err := NewStorageQueryable(cfg, nil)
			require.NoError(t, err)

			ctx := user.InjectOrgID(context.Background(), tenantID)

			querier, err := client.Querier(60e3, 120e3)
			require.NoError(t, err)
			defer func() {
				_ = querier.Close()
			}()

			values, _, err := tc.doRequest(ctx, querier)
			require.NoError(t, err)
			assert.Equal(t, expectedValues, values)
		})
	}

}

func TestStorageQueryable_Querier_LabelNames(t *testing.T) {
	const tenantID = "42"

	expectedNames := []string{"foo", "baz"}

	router := mux.NewRouter()
	router.Handle("/path/api/v1/labels", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if reqTenantID := request.Header.Get(user.OrgIDHeaderName); reqTenantID != tenantID {
			http.Error(writer, fmt.Sprintf("got wrong tenant %q", reqTenantID), http.StatusUnauthorized)
			return
		}
		for param, expectedValue := range map[string]string{
			"start":   "60",
			"end":     "120",
			"match[]": `{foo="bar"}`,
		} {
			if value := request.URL.Query().Get(param); value != expectedValue {
				http.Error(writer, fmt.Sprintf("param %q should be %q got %q", param, expectedValue, value), http.StatusBadRequest)
				return
			}
		}
		require.NoError(t, json.NewEncoder(writer).Encode(struct{ Data []string }{Data: expectedNames}))
	}))
	srv := httptest.NewServer(router)
	defer srv.Close()

	cfg := StorageQueryableConfig{
		Address:      srv.URL + "/path",
		Timeout:      time.Second,
		KeepAlive:    time.Second,
		MaxIdleConns: 10,
		MaxConns:     10,

		ClientName: "test",
	}

	client, err := NewStorageQueryable(cfg, nil)
	require.NoError(t, err)

	ctx := user.InjectOrgID(context.Background(), tenantID)

	querier, err := client.Querier(60e3, 120e3)
	require.NoError(t, err)
	defer func() {
		_ = querier.Close()
	}()

	values, _, err := querier.LabelNames(ctx, nil, labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"))
	require.NoError(t, err)
	assert.Equal(t, expectedNames, values)
}

func TestStorageQueryable_DecoratedRoundtripper(t *testing.T) {
	const tenantID = "42"
	ctx := user.InjectOrgID(context.Background(), tenantID)

	storage := promqltest.LoadedStorage(t, `
		load 1m
			test_metric1{foo="bar",baz="qux"} 1+1x5
	`)

	customRoundTripperHeader := "X-Custom-Roundtripper"
	expectedCustomRoundTripperHeaderValue := "i-was-here"

	h := remote.NewReadHandler(nil, nil, storage, func() (_ config.Config) { return }, 1e6, 1, 0)
	router := mux.NewRouter()
	router.Handle("/notripperware/api/v1/read", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if reqTenantID := request.Header.Get(user.OrgIDHeaderName); reqTenantID != tenantID {
			http.Error(writer, fmt.Sprintf("got wrong tenant %q", reqTenantID), http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(writer, request)
	}))
	router.Handle("/withtripperware/api/v1/read", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if reqTenantID := request.Header.Get(user.OrgIDHeaderName); reqTenantID != tenantID {
			http.Error(writer, fmt.Sprintf("got wrong tenant %q", reqTenantID), http.StatusUnauthorized)
			return
		}
		customRoundTripperValue := request.Header.Get(customRoundTripperHeader)
		if customRoundTripperValue != expectedCustomRoundTripperHeaderValue {
			http.Error(writer, fmt.Sprintf("got wrong '%s' header value '%s'", customRoundTripperHeader, customRoundTripperValue), http.StatusInternalServerError)
			return
		}
		h.ServeHTTP(writer, request)
	}))
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	t.Run("happy case - Passing a tripperware is not required", func(t *testing.T) {
		cfg := StorageQueryableConfig{
			Address:      srv.URL + "/notripperware",
			Timeout:      time.Second,
			KeepAlive:    time.Second,
			MaxIdleConns: 10,
			MaxConns:     10,

			ClientName: "test",
		}

		client, err := NewStorageQueryable(cfg, nil)
		require.NoError(t, err)

		querier, err := client.Querier(60e3, 120e3)
		require.NoError(t, err)
		defer func() {
			_ = querier.Close()
		}()
		set := querier.Select(ctx, true, nil, labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"))
		require.NoError(t, set.Err())
	})

	t.Run("happy case - Passing a tripperware works as expected", func(t *testing.T) {
		cfg := StorageQueryableConfig{
			Address:      srv.URL + "/withtripperware",
			Timeout:      time.Second,
			KeepAlive:    time.Second,
			MaxIdleConns: 10,
			MaxConns:     10,

			ClientName: "test",
		}
		tripperware := func(next http.RoundTripper) http.RoundTripper {
			return querymiddleware.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.Header.Set(customRoundTripperHeader, expectedCustomRoundTripperHeaderValue)
				return next.RoundTrip(r)
			})
		}
		client, err := NewStorageQueryable(cfg, tripperware)
		require.NoError(t, err)

		querier, err := client.Querier(60e3, 120e3)
		require.NoError(t, err)
		defer func() {
			_ = querier.Close()
		}()
		set := querier.Select(ctx, true, nil, labels.MustNewMatcher(labels.MatchEqual, "foo", "bar"))
		require.NoError(t, set.Err())
	})
}

func TestStorageQueryable_Series(t *testing.T) {
	testCases := []struct {
		name             string
		mint             int64
		maxt             int64
		labelMatchers    []string
		expectedPostForm url.Values
	}{
		{
			name:          "no start/end ts",
			mint:          -1,
			maxt:          -1,
			labelMatchers: []string{"graphite_untagged", "graphite_tagged"},
			expectedPostForm: map[string][]string{
				"match[]": {"graphite_untagged", "graphite_tagged"},
			},
		},
		{
			name:          "start ts",
			mint:          5000,
			maxt:          -1,
			labelMatchers: []string{"graphite_untagged", "graphite_tagged"},
			expectedPostForm: map[string][]string{
				"match[]": {"graphite_untagged", "graphite_tagged"},
				"start":   {strconv.Itoa(5)},
			},
		},
		{
			name:          "end ts",
			mint:          -1,
			maxt:          5000,
			labelMatchers: []string{"graphite_untagged", "graphite_tagged"},
			expectedPostForm: map[string][]string{
				"match[]": {"graphite_untagged", "graphite_tagged"},
				"end":     {strconv.Itoa(5)},
			},
		},
		{
			name:          "start/end ts",
			mint:          5000,
			maxt:          5000,
			labelMatchers: []string{"graphite_untagged", "graphite_tagged"},
			expectedPostForm: map[string][]string{
				"match[]": {"graphite_untagged", "graphite_tagged"},
				"start":   {strconv.Itoa(5)},
				"end":     {strconv.Itoa(5)},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := mux.NewRouter()
			router.Handle("/path/api/v1/series", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				require.Equal(t, "application/x-www-form-urlencoded", req.Header.Get("Content-Type"))
				require.NoError(t, req.ParseForm())
				require.Equal(t, tc.expectedPostForm, req.PostForm)

				// We're not testing for the response here so we just write back some minimal valid JSON.
				_, err := w.Write([]byte("{}"))
				require.NoError(t, err)
			}))
			srv := httptest.NewServer(router)
			defer srv.Close()

			queryable, err := NewStorageQueryable(StorageQueryableConfig{
				Address:      srv.URL + "/path",
				Timeout:      time.Second,
				KeepAlive:    time.Second,
				MaxIdleConns: 10,
				MaxConns:     10,

				ClientName: "test",
			}, nil)
			require.NoError(t, err)

			ctx := user.InjectOrgID(context.Background(), "42")

			q, err := queryable.Querier(tc.mint, tc.maxt)
			require.NoError(t, err)
			_, err = q.Series(ctx, tc.labelMatchers)
			require.NoError(t, err)
		})
	}
}
