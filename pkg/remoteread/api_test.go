package remoteread

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/dskit/user"
	"github.com/grafana/mimir/pkg/frontend/querymiddleware"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"

	"github.com/grafana/mimir-proxies/pkg/errorx"
)

func TestNewAPI(t *testing.T) {
	t.Run("maps 429 status code to errorx.TooManyRequests", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(http.StatusTooManyRequests)
		}))

		defer srv.Close()

		api, err := NewAPI(Config{
			Endpoint:     srv.URL,
			Timeout:      time.Minute,
			KeepAlive:    time.Minute,
			MaxIdleConns: 10,
			MaxConns:     10,
		})
		require.NoError(t, err)

		ctx := user.InjectOrgID(context.Background(), "foo")
		_, _, err = api.QueryRange(ctx, "foo", v1.Range{
			Start: time.Now().Add(-time.Hour),
			End:   time.Now(),
			Step:  15 * time.Second,
		})
		require.Error(t, err)
		require.ErrorAs(t, err, &errorx.TooManyRequests{})
	})

	t.Run("uses tripperware", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if header := request.Header.Get("X-Please-Be-A-Teapot"); header == "true" {
				writer.WriteHeader(http.StatusTeapot)
				return
			}
			writer.WriteHeader(http.StatusBadRequest)
		}))

		defer srv.Close()

		tripperware := func(next http.RoundTripper) http.RoundTripper {
			return querymiddleware.RoundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.Header.Set("X-Please-Be-A-Teapot", "true")
				return next.RoundTrip(r)
			})
		}

		api, err := NewAPI(Config{
			Endpoint:     srv.URL,
			Timeout:      time.Minute,
			KeepAlive:    time.Minute,
			MaxIdleConns: 10,
			MaxConns:     10,
		}, WithTripperware(tripperware))
		require.NoError(t, err)

		ctx := user.InjectOrgID(context.Background(), "foo")
		_, _, err = api.QueryRange(ctx, "foo", v1.Range{
			Start: time.Now().Add(-time.Hour),
			End:   time.Now(),
			Step:  15 * time.Second,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), fmt.Sprintf("%d", http.StatusTeapot), "If tripperware is used, server should respond with teapot status code 418, and that should be mentioned in the error message, however, status code was not foundin the error message.")
	})
}
