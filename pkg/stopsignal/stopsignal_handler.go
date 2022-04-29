package stopsignal

import (
	"os"
	"os/signal"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Handler provides the actor function and the stop functions to be used with run.Group.Add()
func Handler(logger log.Logger, signals ...os.Signal) (run func() error, stop func(error)) {
	sigC := make(chan os.Signal, 1)
	exitC := make(chan struct{})
	signal.Notify(sigC, signals...)

	return func() error {
			level.Info(logger).Log("msg", "Waiting for stop signal...")
			select {
			case s := <-sigC:
				level.Info(logger).Log("msg", "Received interrupt signal. Stopping...", "signal", s)
				return nil
			case <-exitC:
				return nil
			}
		},
		func(_ error) {
			close(exitC)
		}
}
