package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

type RequestLimits struct {
	maxRequestBodySize int64
}

func NewRequestLimitsMiddleware(maxRequestBodySize int64) *RequestLimits {
	return &RequestLimits{
		maxRequestBodySize: maxRequestBodySize,
	}
}

func (l RequestLimits) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader := io.LimitReader(r.Body, int64(l.maxRequestBodySize)+1)
		body, err := io.ReadAll(reader)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusInternalServerError)
			return
		}
		if int64(len(body)) > l.maxRequestBodySize {
			http.Error(w, fmt.Sprintf("trying to send message larger than max (%d vs %d)", len(body), l.maxRequestBodySize), http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))

		next.ServeHTTP(w, r)
	})
}
