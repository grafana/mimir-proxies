# datadog-proxy-writes: A Datadog Proxy for Mimir

datadog-proxy-writes is a proxy that accepts Datadog protocol and writes it to Mimir. The destination can be a local Mimir server or a Grafana Cloud instance/stack.

A memcached server is required in order to cache the host-tags that can be sent separately from metrics.

## Datadog metric ingestion/translation

The Datadog write proxy translates incoming Datadog metrics and combines them with any host tags (which Datadog Agent sends separately) to generate Prometheus series that can be forwarded to Mimir. To facilitate the combining of metric/host-tags the Datadog write proxy requires a local memached server to act as a temporary store of host-tags.

A typical setup would be to use the `DD_ADDITIONAL_ENDPOINTS` environment variable to tell the Datadog Agent to send its metrics to the Datadog write proxy in addition to its existing targets. Similar to how this is done when wanting to forward to Grafana Cloud.

The following Datadog endpoints are supported:
* `/api/v1/series`
* `/api/v1/check_run`
* `/intake`

### Metric translation

    Datadog metric: rack_fans_speed.1{rack:0x13,shelf:04,pos:FL,pos:RR}

    In Prometheus: rack_fans_speed_dot_1{rack="'0x13'",shelf="'04'",pos="'FL,RR'"}

There is a slight incompatibility in the characters allowed in tag/label names between Mimir and Datadog and so some translation is required.
Prometheus metric names and labels must match the regex: `[a-zA-Z_:][a-zA-Z0-9_:]*`

However Datadog allows characters such as a period `.` within its tag/label names which is not allowed by Prometheus. The Datadog write proxy uses the following translation rules for metric names and tags (only the first two rules for metric names):
* Any underscore `_` characters are replaced by a double underscore `__`
* Any period `.` characters are replaced by the string `_dot_`
* Any dash `-` characters are replaced by the string `_dsh_`
* Any slash `/` characters are replaced by the string `_sls_`

## Building

($ indicates the command line prompt)

To build the proxy:

```
$ go mod tidy
$ make build
$ make test
```

This should place a build of `datadog-proxy-writes` in the `dist` subdirectory.

## Running

### Gathering required information

Here we show how to configure and run the Graphite write proxy to talk to an existing Mimir installation running on port 9090 on localhost. If no existing Mimir installation is available, or you would like to quickly install a test installation then follow the [getting-started](https://grafana.com/docs/mimir/latest/operators-guide/getting-started/) instructions.

### Gathering required information

In order to configure a write proxy we need to know the following pieces of information at a minimum:
* The TCP port that the write proxy should listen on
* The endpoint for remote writes within Mimir

The default TCP port for the write proxy is 8000 however it is best to choose a unique non-default port, especially if you are going to be running multiple write proxies (Graphite, Datadog, Influx, etc) on the same host.

If Mimir is configured to listen on port 9009 on localhost then the remote write endpoint will be http://localhost:9009/api/v1/push

### An example invocation

(Pre-built binaries/docker images are on our list of things to do.)

To run the proxy:

```
$ dist/datadog-proxy-writes \
  -server.http-listen-port=8009 \
  -auth.enable=false \
  -memcached-server 127.0.0.1:11211 \
  -server.path-prefix=/datadog/ \
  -write-endpoint http://127.0.0.1:9009/api/prom/push \
  -query-endpoint http://127.0.0.1:9009/api/prom
```

Details of configurable options are available in the `-help` output.

### Datadog agent configuration

You can then point the datadog agent at the http listen port (8080 by default, or 8009 in the above example) by specifying an additional endpoint environment variable when running the datadog agent, for example:

```
DD_ADDITIONAL_ENDPOINTS='{"http://127.0.0.1:8009/datadog": ["grafana-labs"]}'
```

Note this uses `grafana-labs` as the API key value. *Do not use your real Datadog API key here.*

## Grafana Cloud as a destination

If the destination Mimir installation is part of a Grafana cloud instance the `-write-endpoint` argument should be of the form:
  -write-endpoint https://_username_:_password_@_grafana_net_instance_/api/v1/push
where the exact server details can be found on Prometheus instance details page for the stack on grafana.com

The _username_ is the numeric `Username / Instance ID`
The _password_ is the Grafana Cloud API Key with `metrics push` privileges/role.
The _grafana_net_instance_ is server part of the URL to push Prometheus metrics.

## Internal metrics

The datadog-proxy-writes binary exposes internal metrics on a `/metrics` endpoint on a separate port which can be scraped by a local prometheus installation. This is configurable with the `internalserver` command line options.
