package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/pkg/errors"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/mimir/pkg/util/spanlogger"
)

const (
	StatusClientClosedRequest = 499
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
		log, _ := spanlogger.New(r.Context(), "middleware.RequestLimits.Wrap")
		defer log.Span.Finish()

		reader := io.LimitReader(r.Body, int64(l.maxRequestBodySize)+1)
		body, err := io.ReadAll(reader)
		if err != nil {
			level.Warn(log).Log("msg", "failed to read request body", "err", err)
			_ = level.Warn(l.logger).Log("msg", "middleware.RequestLimits.Wrap failed to read request body", "err", err)

			switch {
			case isNetworkError(err):
				http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusRequestTimeout)
				return
			case errors.Is(err, context.Canceled) || errors.Is(err, io.ErrUnexpectedEOF):
				http.Error(w, fmt.Sprintf("failed to read request body: %v", err), StatusClientClosedRequest)
				return
			default:
				http.Error(w, fmt.Sprintf("failed to read request body: %v", err), http.StatusInternalServerError)
				return
			}
		}
		if int64(len(body)) > l.maxRequestBodySize {
			msg := fmt.Sprintf("trying to send message larger than max (%d vs %d)", len(body), l.maxRequestBodySize)
			level.Warn(log).Log("msg", msg)
			_ = level.Warn(l.logger).Log("msg", "middleware.RequestLimits.Wrap: "+msg)
			http.Error(w, msg, http.StatusRequestEntityTooLarge)
			return
		}

		r.Body = io.NopCloser(bytes.NewBuffer(body))

		next.ServeHTTP(w, r)
	})
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	netErr, ok := errors.Cause(err).(net.Error)
	return ok && netErr.Timeout()
}
