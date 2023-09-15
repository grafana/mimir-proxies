package internalserver

import (
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type ReadinessProvider interface {
	Ready() bool
}

// NewReadinessHandler returns an endpoint that returns a simple 200 to denote
// the web server is active
func NewReadinessHandler(ready ReadinessProvider, logger log.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		if ready.Ready() {
			w.WriteHeader(http.StatusOK)
			_, err = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_, err = w.Write([]byte("Not ready"))
		}

		if err != nil {
			level.Error(logger).Log("msg", "ready endpoint error", "err", err)
		}
	}
}

type AlwaysReady struct{}

func (AlwaysReady) Ready() bool {
	return true
}
