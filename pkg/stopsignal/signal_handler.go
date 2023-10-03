package stopsignal

import (
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func NewSignalHandler(shutdownDelay time.Duration, logger log.Logger) *SignalHandler {
	return &SignalHandler{
		quit:          make(chan struct{}),
		shutdownDelay: shutdownDelay,
		logger:        logger,
	}
}

type SignalHandler struct {
	quit          chan struct{}
	ready         atomic.Bool
	shutdownDelay time.Duration
	logger        log.Logger
}

func (sh *SignalHandler) Handler(signals ...os.Signal) (run func() error, stop func(error)) {
	sh.ready.Store(true)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, signals...)

	return func() error {
			level.Info(sh.logger).Log("msg", "Waiting for stop signal...")
			select {
			case <-sh.quit:
				return nil

			case <-sigs:
				level.Info(sh.logger).Log("msg", "=== received SIGINT/SIGTERM ===", "sleep", sh.shutdownDelay)

				// Not ready anymore.
				sh.ready.Store(false)
				if sh.shutdownDelay > 0 {
					time.Sleep(sh.shutdownDelay)
				}

				level.Info(sh.logger).Log("msg", "shutting down")
				return nil
			}
		},
		func(_ error) {
			sh.Stop()
		}
}

func (sh *SignalHandler) Stop() {
	close(sh.quit)
}

func (sh *SignalHandler) Ready() bool {
	return sh.ready.Load()
}
