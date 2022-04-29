# datadog-proxy-writes: A Datadog Proxy for Mimir

datadog-proxy-writes is a proxy that accepts Datadog protocol and writes it to Mimir.

A memcached server is required in order to cache the host-tags that can be sent separately from metrics.

## Building

To build the proxy:

```
go mod tidy
make build
```

This should place a build of `datadog-proxy-writes` in the `dist` subdirectory.

## Running

To run the proxy:

```
dist/datadog-proxy-writes -server.http-listen-port=8080 -auth.enable=false -memcached-server 127.0.0.1:11211 -server.path-prefix=/datadog/ -write-endpoint http://127.0.0.1:9090/api/prom/push -query-endpoint http://127.0.0.1:9090/api/prom
```

Details of configurable options are available in the `-help` output.

## Datadog agent configuration

You can then point the datadog agent at port 8080 by specifying an additional endpoint environment variable when running the datadog agent, for example:

```
DD_ADDITIONAL_ENDPOINTS='{"http://127.0.0.1:8080/datadog": ["grafana-labs"]}'
```

Note this uses `grafana-labs` as the API key value. Do not use your real Datadog API key here.

## Internal metrics

The datadog-proxy-writes binary exposes internal metrics on a `/metrics` endpoint on a separate port which can be scraped by a local prometheus installation. This is configurable with the `internalserver` command line options.
