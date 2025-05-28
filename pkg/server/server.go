package server

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/pkg/errors"

	"github.com/go-kit/log/level"

	"golang.org/x/net/netutil"
	"google.golang.org/grpc"

	"github.com/grafana/mimir-graphite/v2/pkg/server/middleware"

	"github.com/gorilla/mux"
)

const (
	mb                                          = 1024 * 1024
	defaultServerGracefulShutdown time.Duration = 5 * time.Second
	defaultHTTPReadTimeout        time.Duration = 30 * time.Second
	defaultHTTPWriteTimeout       time.Duration = 35 * time.Second
	defaultHTTPIdleTimeout        time.Duration = 30 * time.Second
	defaultHTTPRequestSizeLimit   int64         = 10 * mb

	defaultListenPort = 8000
	defaultGrpcPort   = 9095
)

type Config struct {
	HTTPListenAddress string `yaml:"http_listen_address"`
	// HTTPListenPort specifies the port to listen on. If the port is 0, a port
	// number is automatically chosen. The Addr method of Server can be used to
	// discover the chosen port. The value in Config will not be updated.
	HTTPListenPort int `yaml:"http_listen_port"`
	HTTPConnLimit  int `yaml:"http_conn_limit"`

	ServerGracefulShutdownTimeout time.Duration `yaml:"server_graceful_shutdown_timeout"`
	HTTPServerReadTimeout         time.Duration `yaml:"http_server_read_timeout"`
	HTTPServerWriteTimeout        time.Duration `yaml:"http_server_write_timeout"`
	HTTPServerIdleTimeout         time.Duration `yaml:"http_server_idle_timeout"`

	HTTPMaxRequestSizeLimit int64 `yaml:"http_max_request_size_limit"`

	GRPCListenPort int `yaml:"grpc_listen_port"`

	PathPrefix string `yaml:"path_prefix"`
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
	flags.StringVar(&cfg.HTTPListenAddress, prefix+"server.http-listen-address", "0.0.0.0", "Sets listen address for the http server")
	flags.IntVar(&cfg.HTTPListenPort, prefix+"server.http-listen-port", defaultListenPort, "Sets listen address port for the http server")
	flags.IntVar(&cfg.HTTPConnLimit, prefix+"server.http-listen-conn-limit", 0, "Sets a limit to the amount of http connections, 0 means no limit")
	flags.DurationVar(&cfg.ServerGracefulShutdownTimeout, prefix+"server.graceful-shutdown-timeout", defaultServerGracefulShutdown, "Graceful shutdown period")
	flags.DurationVar(&cfg.HTTPServerReadTimeout, prefix+"server.http-server-read-timeout", defaultHTTPReadTimeout, "HTTP request read timeout")
	flags.DurationVar(&cfg.HTTPServerWriteTimeout, prefix+"server.http-server-write-timeout", defaultHTTPWriteTimeout, "HTTP request write timeout")
	flags.DurationVar(&cfg.HTTPServerIdleTimeout, prefix+"server.http-server-idle-timeout", defaultHTTPIdleTimeout, "HTTP request idle timeout")
	flags.Int64Var(&cfg.HTTPMaxRequestSizeLimit, prefix+"server.http-max-req-size-limit", defaultHTTPRequestSizeLimit, "HTTP max request body size limit in bytes")
	flags.StringVar(&cfg.PathPrefix, prefix+"server.path-prefix", "", "Base path to serve all API routes from (e.g. /v1/)")
	flags.IntVar(&cfg.GRPCListenPort, prefix+"server.grpc-listen-port", defaultGrpcPort, "Sets listen address port for the http server")
}

// Server initializes an Router webserver as well as the desired middleware configuration
type Server struct {
	cfg          Config
	httpListener net.Listener
	grpcListener net.Listener
	Router       *mux.Router
	HTTPServer   *http.Server
	GRPCServer   *grpc.Server
	log          log.Logger
}

// NewServer initializes an httpserver with a router and all the configuration parameters given.
// Note that all the provided middlewares are wrapped in order.
func NewServer(log log.Logger, cfg Config, router *mux.Router, middlewares []middleware.Interface) (*Server, error) {
	if router == nil {
		return nil, fmt.Errorf("router must be initialized")
	}

	// Setup listeners first, so we can fail early if the port is in use.
	httpListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.HTTPListenAddress, cfg.HTTPListenPort))
	if err != nil {
		return nil, err
	}
	if cfg.HTTPConnLimit > 0 {
		httpListener = netutil.LimitListener(httpListener, cfg.HTTPConnLimit)
	}

	_ = level.Info(log).Log("msg", "server listening on address", "addr", httpListener.Addr().String())

	if cfg.PathPrefix != "" {
		router = router.PathPrefix(cfg.PathPrefix).Subrouter()
	}

	httpServer := &http.Server{
		ReadTimeout:  cfg.HTTPServerReadTimeout,
		WriteTimeout: cfg.HTTPServerWriteTimeout,
		IdleTimeout:  cfg.HTTPServerIdleTimeout,
		Handler:      middleware.Merge(middlewares...).Wrap(router),
	}

	grpcServer := grpc.NewServer()

	grpcListener, grpcListenerErr := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", cfg.GRPCListenPort))
	if grpcListenerErr != nil {
		return nil, err
	}

	_ = level.Info(log).Log("msg", "GRPC server listening on address", "addr", grpcListener.Addr().String())

	return &Server{
		cfg:          cfg,
		httpListener: httpListener,
		grpcListener: grpcListener,

		Router:     router,
		HTTPServer: httpServer,
		GRPCServer: grpcServer,
		log:        log,
	}, nil
}

// Addr returns the address the server is listening on.
func (s *Server) Addr() net.Addr {
	return s.httpListener.Addr()
}

// Handler returns two functions to run and stop the server.
func (s *Server) Handler() (run func() error, stop func(error)) {
	return s.Run, s.Shutdown
}

func (s *Server) Run() error {
	errChan := make(chan error, 1)

	go func() {
		level.Info(s.log).Log("msg", "Starting http server", "addr", s.httpListener.Addr().String())

		err := s.HTTPServer.Serve(s.httpListener)
		if err == http.ErrServerClosed {
			err = nil
		}

		errChan <- err
	}()

	go func() {
		level.Info(s.log).Log("msg", "Starting grpc server", "addr", s.grpcListener.Addr().String())

		err := s.GRPCServer.Serve(s.grpcListener)
		if errors.Is(err, grpc.ErrServerStopped) {
			err = nil
		}

		errChan <- err
	}()

	return <-errChan
}

func (s *Server) Shutdown(_ error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ServerGracefulShutdownTimeout)
	defer cancel()

	level.Info(s.log).Log("msg", "Shutting down http server")
	if err := s.HTTPServer.Shutdown(ctx); err != nil {
		_ = level.Error(s.log).Log("msg", "Server shutdown error", "err", err)
	}
	level.Info(s.log).Log("msg", "Server shut down correctly")

	level.Info(s.log).Log("msg", "Shutting down grpc server")
	s.GRPCServer.GracefulStop()
}
