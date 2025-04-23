package main

import (
	"flag"
	"fmt"
	"os"

	opentracing "github.com/opentracing/opentracing-go"

	"github.com/grafana/mimir-proxies/pkg/appcommon"
	"github.com/grafana/mimir-proxies/pkg/graphite/writeproxy"
	"github.com/grafana/mimir-proxies/pkg/route"

	"github.com/grafana/mimir/pkg/util/log"

	"github.com/go-kit/log/level"

	"github.com/grafana/mimir-proxies/pkg/remotewrite"

	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/client_golang/prometheus"
)

// This value will be overridden during the build process using -ldflags.
var version = "development"

func Run() (err error) {
	var (
		appConfig    appcommon.Config
		writerConfig writeproxy.Config
		metricPrefix = "graphite_proxy"
	)

	flagext.RegisterFlags(
		&appConfig,
		&writerConfig,
	)
	versionFlag := flag.Bool("version", false, "Display the version of the binary")
	flag.Parse()

	if *versionFlag {
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", version)
		os.Exit(0)
	}

	if appConfig.ServiceName == "" {
		appConfig.ServiceName = "graphite-proxy-writes"
	}

	reg := prometheus.DefaultRegisterer

	var app appcommon.App
	app, err = appcommon.New(appConfig, reg, metricPrefix, nil)
	if err != nil {
		return err
	}
	defer func() {
		innerErr := app.Close()
		if err == nil {
			err = innerErr
		}
	}()

	log.Logger = app.Logger
	opentracing.SetGlobalTracer(app.Tracer)

	remoteWriteRecorder := remotewrite.NewRecorder("graphite_proxy", prometheus.DefaultRegisterer)
	client, err := remotewrite.NewClient(writerConfig.RemoteWriteConfig, remoteWriteRecorder, nil)
	if err != nil {
		return fmt.Errorf("can't instantiate remoteWrite.API for Graphite: %w", err)
	}
	level.Info(log.Logger).Log("msg", "graphite is using remote write API", "address", writerConfig.RemoteWriteConfig.Endpoint)

	remoteWriteProxyRecorder := writeproxy.NewRecorder(prometheus.DefaultRegisterer)
	remoteWriteProxy := writeproxy.NewRemoteWriteProxy(client, remoteWriteProxyRecorder)

	registerer := route.NewMuxRegisterer(app.Server.Router)
	registerer.RegisterRoute("/graphite/metrics", remoteWriteProxy, "POST")

	// Handle OS Signals
	return app.Group.Run()
}

func main() {
	err := Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running application: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
