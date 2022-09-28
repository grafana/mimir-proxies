package errorx

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

const (
	httpStatusCanceled = 499
)

// LogAndSetHTTPError logs the provided error and then translates the internal error into a http response.
// The error message set in the response is conservative in an attempt to prevent internal details (e.g. GCS bucket
// name) from leaking. If the error is from this errorx package, the top-level message is logged. Otherwise, hardcoded
// messages are returned
func LogAndSetHTTPError(ctx context.Context, w http.ResponseWriter, log log.Logger, err error) {
	code := http.StatusInternalServerError
	message := "unknown error"

	var errx Error
	if errors.Is(err, context.Canceled) {
		code = httpStatusCanceled
		_ = level.Error(log).Log("msg", "canceled", "response_code", code, "err", err)
		message = "request canceled"
	} else if errors.As(err, &errx) {
		switch code = errx.HTTPStatusCode(); code {
		case http.StatusBadRequest:
			_ = level.Warn(log).Log("msg", errx.Message(), "response_code", code, "err", tryUnwrap(errx))
		default:
			_ = level.Error(log).Log("msg", errx.Message(), "response_code", code, "err", tryUnwrap(errx))
		}
		message = errx.Message()
	} else {
		_ = level.Error(log).Log("msg", "unknown error", "response_code", code, "err", err)
	}

	http.Error(w, message, code)
}

func tryUnwrap(err error) error {
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return wrapped.Unwrap()
	}
	return err
}
