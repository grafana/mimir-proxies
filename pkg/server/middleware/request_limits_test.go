package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestLimitsMiddleware(t *testing.T) {
	for _, tc := range []struct {
		name               string
		maxRequestBodySize int64
		inputBody          []byte
		expectedStatus     int
	}{
		{
			name:               "requests with empty body should return 200",
			maxRequestBodySize: 0,
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "requests with body size below max should return 200",
			maxRequestBodySize: 1 * mb,
			inputBody:          []byte(strings.Repeat("a", 512*kb)),
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "requests with body size equal to max should return 200",
			maxRequestBodySize: 1 * mb,
			inputBody:          []byte(strings.Repeat("a", 1*mb)),
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "requests with body size greater than max should fail with 413",
			maxRequestBodySize: 0.5 * mb,
			inputBody:          []byte(strings.Repeat("a", 1*mb)),
			expectedStatus:     http.StatusRequestEntityTooLarge,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			middleware := NewRequestLimitsMiddleware(tc.maxRequestBodySize)
			handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(
				"GET",
				"https://example.com",
				bytes.NewReader(tc.inputBody),
			)
			resp := httptest.NewRecorder()

			handler.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedStatus, resp.Code)
		})
	}
}
