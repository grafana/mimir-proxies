package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/go-kit/log/level"
	"github.com/grafana/mimir/pkg/util/spanlogger"
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
		log, _ := spanlogger.New(r.Context(), "middleware.RequestLimits.Wrap")
		defer log.Span.Finish()

		reader := io.LimitReader(r.Body, int64(l.maxRequestBodySize)+1)
		body, err := io.ReadAll(reader)
		if err != nil {
			level.Warn(log).Log("msg", "failed to read request body", "err", err)
			http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusInternalServerError)
			return
		}
		if int64(len(body)) > l.maxRequestBodySize {
			msg := fmt.Sprintf("trying to send message larger than max (%d vs %d)", len(body), l.maxRequestBodySize)
			level.Warn(log).Log("msg", msg)
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))

		next.ServeHTTP(w, r)
	})
}
