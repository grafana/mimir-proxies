package internalserver

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultGracefulShutdownTimeout = 5 * time.Second
	defaultListenPort              = 8081
)

// Config for the Internal Server
type Config struct {
	HTTPListenAddress             string        `yaml:"http_listen_address"`
	HTTPListenPort                int           `yaml:"http_listen_port"`
	ServerGracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout"`

	ReadinessProvider ReadinessProvider `yaml:"-"`
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *Config) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}

// RegisterFlagsWithPrefix registers flags, adding the provided prefix if
// needed. If the prefix is not blank and doesn't end with '.', a '.' is
// appended to it.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, flags *flag.FlagSet) {
	if prefix != "" && !strings.HasSuffix(prefix, ".") {
		prefix += "."
	}
	flags.StringVar(&cfg.HTTPListenAddress, prefix+"internalserver.http-listen-address", "", "Internal HTTP server listen address.")
	flags.IntVar(&cfg.HTTPListenPort, prefix+"internalserver.http-listen-port", defaultListenPort, "Internal HTTP server listen port.")
	flags.DurationVar(&cfg.ServerGracefulShutdownTimeout, prefix+"internalserver.graceful-shutdown-timeout", defaultGracefulShutdownTimeout, "Timeout for graceful shutdowns")
}

func Handler(logger log.Logger, cfg Config) (run func() error, stop func(error)) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	mux.Handle("/healthz", http.HandlerFunc(NewReadinessHandler(cfg.ReadinessProvider, logger)))

	// Pprof.
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	addr := fmt.Sprintf("%s:%d", cfg.HTTPListenAddress, cfg.HTTPListenPort)

	internalServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return func() error {
			_ = level.Info(logger).Log("msg", "Starting internal server", "addr", addr)
			return internalServer.ListenAndServe()
		},
		func(_ error) {
			ctx, cancel := context.WithTimeout(context.Background(), cfg.ServerGracefulShutdownTimeout)
			defer cancel()

			level.Info(logger).Log("msg", "Shutting down internal server")
			if err := internalServer.Shutdown(ctx); err != nil {
				_ = level.Error(logger).Log("msg", "Server shutdown error", "err", err)
			}
			level.Info(logger).Log("msg", "Server shut down correctly")
		}
}
