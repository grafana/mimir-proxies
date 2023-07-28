package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
			middleware := NewRequestLimitsMiddleware(tc.maxRequestBodySize, log.NewNopLogger())
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

type errReader struct {
	mock.Mock
}

func (m errReader) Read(p []byte) (n int, err error) {
	args := m.Called(p)
	return args.Int(0), args.Error(1)
}

type TimeoutError struct {
	error
}

func (e TimeoutError) Timeout() bool {
	return true
}

func (e TimeoutError) Temporary() bool {
	return true
}

func (e TimeoutError) Error() string {
	return ""
}

func TestRequestLimitsMiddlewareReadError(t *testing.T) {
	for _, tc := range []struct {
		name           string
		readerErr      error
		expectedStatus int
	}{
		{
			name:           "in case of unexpected EOF should return 499",
			readerErr:      io.ErrUnexpectedEOF,
			expectedStatus: StatusClientClosedRequest,
		},
		{
			name:           "in case of timeout error should return 408",
			readerErr:      new(TimeoutError),
			expectedStatus: http.StatusRequestTimeout,
		},
		{
			name:           "in case other errors should return 500",
			readerErr:      errors.New("other error"),
			expectedStatus: http.StatusInternalServerError,
		},
	} {
		middleware := NewRequestLimitsMiddleware(1*mb, log.NewNopLogger())
		handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		reader := new(errReader)
		reader.Mock.On("Read", mock.Anything).Return(0, tc.readerErr)

		req := httptest.NewRequest(
			"GET",
			"https://example.com",
			reader,
		)
		resp := httptest.NewRecorder()

		handler.ServeHTTP(resp, req)

		assert.Equal(t, tc.expectedStatus, resp.Code)
	}
}
