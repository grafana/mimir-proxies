package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/grafana/mimir-proxies/pkg/datadog/apiwrite"
	"github.com/grafana/mimir-proxies/pkg/datadog/htstorage"
	"github.com/grafana/mimir-proxies/pkg/remoteread"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"

	"github.com/grafana/mimir-proxies/pkg/appcommon"
	"github.com/grafana/mimir-proxies/pkg/datadog/ingester"

	"github.com/grafana/mimir-proxies/pkg/memcached"

	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/client_golang/prometheus"
)

// This value will be overridden during the build process using -ldflags.
var version = "development"

func Run() (err error) {
	var (
		apiConfig         apiwrite.Config
		appConfig         appcommon.Config
		remoteReadConfig  remoteread.Config
		remoteWriteConfig remotewrite.Config
		memcacheConfig    memcached.Config
		htCacheConfig     htstorage.CacheConfig
		metricPrefix      = "datadog_proxy"
	)

	flagext.RegisterFlags(
		&apiConfig,
		&appConfig,
		&remoteReadConfig,
		&remoteWriteConfig,
		&memcacheConfig,
		&htCacheConfig,
	)
	versionFlag := flag.Bool("version", false, "Display the version of the binary")
	flag.Parse()

	if *versionFlag {
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", version)
		os.Exit(0)
	}

	if appConfig.ServiceName == "" {
		appConfig.ServiceName = "datadog-proxy-writes"
	}

	reg := prometheus.DefaultRegisterer

	app, err := appcommon.New(appConfig, reg, metricPrefix, nil)
	if err != nil {
		return err
	}
	defer func() {
		innerErr := app.Close()
		if err == nil {
			err = innerErr
		}
	}()

	readAPI, err := remoteread.NewAPI(remoteReadConfig)
	if err != nil {
		return err
	}
	readAPI = remoteread.NewMeasuredAPI(readAPI, remoteread.NewRecorder(metricPrefix, reg), app.LogProvider, app.Tracer, time.Now)

	var memcacheClient memcached.Client
	memcacheClient, err = memcached.NewClient(memcacheConfig)
	if err != nil {
		return err
	}
	memcacheClient = memcached.NewMeasuredClient(memcacheClient, memcached.NewRecorder(reg), time.Now)

	remoteWriteRecorder := remotewrite.NewRecorder(metricPrefix, reg)
	remoteWriter, err := remotewrite.NewClient(remoteWriteConfig, remoteWriteRecorder, nil)
	if err != nil {
		return err
	}
	remoteWriter = remotewrite.NewMeasuredClient(remoteWriter, remoteWriteRecorder, app.Tracer, time.Now)

	htStorage := htstorage.NewCachedStorage(
		app.LogProvider,
		htstorage.NewCortexStorage(remoteWriter, readAPI, time.Now),
		memcacheClient,
		htstorage.MemcacheKeygen{},
		htstorage.NewCacheRecorder(reg),
		app.Tracer,
		htCacheConfig,
		time.AfterFunc,
	)

	in := ingester.New(
		htStorage,
		ingester.NewRecorder(reg),
		remoteWriter,
	)

	api := apiwrite.NewAPI(apiConfig, app.LogProvider, in, app.Tracer)
	api.RegisterAll(app.Server.Router)

	// Handle OS Signals
	err = app.Group.Run()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error running application: %s", err)
		os.Exit(1)
	}
	os.Exit(0)
}
