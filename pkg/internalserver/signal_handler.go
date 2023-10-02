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

func (sh *serverSignalHandler) Loop() {
	sh.ready.Store(true)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigs)

	for {
		select {
		case <-sh.quit:
			return

		case <-sigs:
			level.Info(sh.logger).Log("msg", "=== received SIGINT/SIGTERM ===", "sleep", sh.shutdownDelay)

			// Not ready anymore.
			sh.ready.Store(false)
			if sh.shutdownDelay > 0 {
				time.Sleep(sh.shutdownDelay)
			}

			level.Info(sh.logger).Log("msg", "shutting down")
			return
		}
	}
}

func (sh *serverSignalHandler) Stop() {
	close(sh.quit)
}

func (sh *serverSignalHandler) Ready() bool {
	return sh.ready.Load()
}
