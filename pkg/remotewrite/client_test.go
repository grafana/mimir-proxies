package remotewrite

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/mimir/pkg/distributor"
	"github.com/grafana/mimir/pkg/frontend/querymiddleware"
	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/grafana/mimir-graphite/v2/pkg/errorx"

	"github.com/grafana/dskit/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPRemoteWriteClient_Write(t *testing.T) {
	t.Run("handles out of order samples", func(t *testing.T) {
		assert, require := assert.New(t), require.New(t)

		mux := http.NewServeMux()
		mux.Handle("/api/prom/push", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(outOfOrderSampleResponseText))
		}))
		srv := httptest.NewServer(mux)
		defer srv.Close()

		cfg := Config{
			Endpoint: srv.URL + "/api/prom/push",
			Timeout:  time.Second,
		}

		recorderMock := &MockRecorder{}
		recorderMock.On("measureOutOfOrderSamples", 1).Once().Return(nil)
		defer recorderMock.AssertExpectations(t)

		client, err := NewClient(cfg, recorderMock, nil)
		require.NoError(err)

		ctx := user.InjectOrgID(context.Background(), "some-org-id")
		err = client.Write(ctx, &mimirpb.WriteRequest{})

		assert.ErrorAs(err, &errorx.BadRequest{}, err)
	})

	t.Run("client's transport can be decorated", func(t *testing.T) {
		assert, require := assert.New(t), require.New(t)

		customRoundTripperHeader := "X-Custom-Roundtripper"
		expectedCustomRoundTripperHeaderValue := "i-was-here"

		mux := http.NewServeMux()
		mux.Handle("/api/prom/push", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rtHeaderValue := req.Header.Get(customRoundTripperHeader)
			if rtHeaderValue == "" || rtHeaderValue != expectedCustomRoundTripperHeaderValue {
				rw.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(rw, "expected header %s with value %s, got %s", customRoundTripperHeader, expectedCustomRoundTripperHeaderValue, rtHeaderValue)
				return
			}
			rw.WriteHeader(http.StatusOK)

		}))
		srv := httptest.NewServer(mux)
		defer srv.Close()

		cfg := Config{
			Endpoint: srv.URL + "/api/prom/push",
			Timeout:  time.Second,
		}

		recorderMock := &MockRecorder{}

		tripperware := func(next http.RoundTripper) http.RoundTripper {
			return querymiddleware.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.Header.Set(customRoundTripperHeader, expectedCustomRoundTripperHeaderValue)
				return next.RoundTrip(r)
			})
		}

		client, err := NewClient(cfg, recorderMock, tripperware)
		require.NoError(err)

		ctx := user.InjectOrgID(context.Background(), "some-org-id")
		err = client.Write(ctx, &mimirpb.WriteRequest{})
		assert.NoError(err)
	})

	t.Run("forwards skip label validation header is config is set", func(t *testing.T) {
		assert, require := assert.New(t), require.New(t)

		mux := http.NewServeMux()
		mux.Handle("/api/prom/push", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			assert.Equal(req.Header.Get(distributor.SkipLabelNameValidationHeader), "true")
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte{})
		}))
		srv := httptest.NewServer(mux)
		defer srv.Close()

		cfg := Config{
			Endpoint:            srv.URL + "/api/prom/push",
			Timeout:             time.Second,
			SkipLabelValidation: true,
		}

		recorderMock := &MockRecorder{}
		defer recorderMock.AssertExpectations(t)

		client, err := NewClient(cfg, recorderMock, nil)
		require.NoError(err)

		ctx := user.InjectOrgID(context.Background(), "some-org-id")
		err = client.Write(ctx, &mimirpb.WriteRequest{})
		assert.NoError(err)
	})
}

const outOfOrderSampleResponseText = "user=41413: err: out of order sample. " +
	`timestamp=2021-02-16T10:07:30Z, series={__name__=\"my_proxy_dot_statsd_dot_client_dot_events\", ` +
	`client=\"go\", client__transport=\"udp\", client__version=\"4.2.0\", ` +
	`cluster__name=\"dev-cluster\", ` +
	`host=\"cluster.dev.internal\", ` +
	`kube__cluster__name=\"dev-cluster\"}`
