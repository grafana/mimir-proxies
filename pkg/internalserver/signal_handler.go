package internalserver

import (
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

func newSignalHandler(shutdownDelay time.Duration, logger log.Logger) *serverSignalHandler {
	return &serverSignalHandler{
		quit:          make(chan struct{}),
		shutdownDelay: shutdownDelay,
		logger:        logger,
	}
}

type serverSignalHandler struct {
	quit          chan struct{}
	ready         atomic.Bool
	shutdownDelay time.Duration
	logger        log.Logger
}

func (dh *serverSignalHandler) Loop() {
	dh.ready.Store(true)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	for {
		select {
		case <-dh.quit:
			return

		case <-sigs:
			level.Info(dh.logger).Log("msg", "=== received SIGINT/SIGTERM ===", "sleep", dh.shutdownDelay)

			// Not ready anymore.
			dh.ready.Store(false)
			if dh.shutdownDelay > 0 {
				time.Sleep(dh.shutdownDelay)
			}

			level.Info(dh.logger).Log("msg", "shutting down")
			return
		}
	}
}

func (dh *serverSignalHandler) Stop() {
	close(dh.quit)
}

func (dh *serverSignalHandler) Ready() bool {
	return dh.ready.Load()
}
