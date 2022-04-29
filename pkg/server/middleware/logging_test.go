package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/go-kit/log"
)

func TestLogRequest(t *testing.T) {
	t.Run("redacts the api_key param", func(t *testing.T) {
		const (
			secretAPIKey = "secret-api-key"
			path         = "/datadog/api/v1/validate"
		)
		buf := new(bytes.Buffer)
		logger := log.NewLogfmtLogger(buf)

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://grafana.net%s?api_key=%s", path, secretAPIKey), http.NoBody)
		require.NoError(t, err)

		logRequest(logger, req, http.StatusOK)

		line := buf.String()
		assert.Contains(t, line, path)
		assert.NotContains(t, line, secretAPIKey)
	})

	t.Run("keeps the api_key=grafana-labs", func(t *testing.T) {
		const (
			grafanaLabsAPIKey = "grafana-labs"
			path              = "/datadog/api/v1/validate"
		)
		buf := new(bytes.Buffer)
		logger := log.NewLogfmtLogger(buf)

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://grafana.net%s?api_key=%s", path, grafanaLabsAPIKey), http.NoBody)
		require.NoError(t, err)

		logRequest(logger, req, http.StatusOK)

		line := buf.String()
		fmt.Println(line)
		assert.Contains(t, line, path)
		assert.Contains(t, line, grafanaLabsAPIKey)
	})

	t.Run("does nothing when api_key is not provided", func(t *testing.T) {
		const (
			path = "/datadog/api/v1/validate"
		)
		buf := new(bytes.Buffer)
		logger := log.NewLogfmtLogger(buf)

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://grafana.net%s", path), http.NoBody)
		require.NoError(t, err)

		logRequest(logger, req, http.StatusOK)

		line := buf.String()
		assert.Contains(t, line, path)
		assert.NotContains(t, line, "api_key")
	})
}

func TestLoggingMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name         string
		innerHandler func(w http.ResponseWriter, r *http.Request)
		logContains  []string
	}{
		{
			name: "status code 200",
			innerHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			logContains: []string{"status=200", "level=info"},
		},
		{
			name: "status code 400",
			innerHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			logContains: []string{"status=400", "level=info"},
		},
		{
			name: "status code 500",
			innerHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			logContains: []string{"status=500", "level=warn"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			logger := log.NewLogfmtLogger(buf)

			middleware := NewLoggingMiddleware(logger)
			handler := middleware.Wrap(http.HandlerFunc(tc.innerHandler))

			req := httptest.NewRequest("GET", "https://example.com", nil)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			line := buf.String()
			for _, contains := range tc.logContains {
				assert.Contains(t, line, contains)
			}
		})
	}
}
