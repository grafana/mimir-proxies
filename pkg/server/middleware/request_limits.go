package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type RequestLimits struct {
	maxRequestBodySize int64
	logger             log.Logger
}

func NewRequestLimitsMiddleware(maxRequestBodySize int64, logger log.Logger) *RequestLimits {
	return &RequestLimits{
		maxRequestBodySize: maxRequestBodySize,
		logger:             logger,
	}
}

func (l RequestLimits) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader := io.LimitReader(r.Body, int64(l.maxRequestBodySize)+1)
		body, err := io.ReadAll(reader)
		if err != nil {
			msg := fmt.Sprintf("failed to read request body: %v", err)
			_ = level.Warn(l.logger).Log("msg", msg)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}
		if int64(len(body)) > l.maxRequestBodySize {
			msg := fmt.Sprintf("trying to send message larger than max (%d vs %d)", len(body), l.maxRequestBodySize)
			_ = level.Warn(l.logger).Log("msg", msg, "error", err)
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))

		next.ServeHTTP(w, r)
	})
}
