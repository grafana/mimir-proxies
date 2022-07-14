# graphite-proxy-writes: A Graphite proxy for Mimir

graphite-proxy-writes is a proxy that accepts metrics via the Graphite protocol and writes them to Mimir. The destination can be a local Mimir server or a Grafana Cloud instance/stack.

## Graphite metric ingestion and translation

The Graphite write proxy accepts the ingest requests (usually via [Carbon Relay NG](https://github.com/grafana/carbon-relay-ng)) and then translates the incoming Graphite metrics into Prometheus metrics. The Graphite to Prometheus metric translation differentiates between untagged Graphite metrics and tagged Graphite metrics, with our proxy supporting both. The name mapping scheme for the two looks as follows:

### Untagged Graphite metrics

    Graphite metric: some.test.metric

    In Prometheus: graphite_untagged{__n000__="some", __n001__="test", __n002__="metric"}

### Tagged Graphite metrics

    Graphite metric: some.test.metric;my_tag=my_value;another_tag=another_value

    In Prometheus: graphite_tagged{name="some.test.metric", my_tag="my_value", another_tag="another_value"}

## Building

To build the proxy:

($ indicates the command line prompt)

```
$ go mod tidy
$ make build
$ make test
```

This should place a build of `graphite-proxy-writes` in the `dist` subdirectory.

## Running

Here we show how to configure and run the Graphite write proxy to talk to an existing Mimir installation running on port 9090 on localhost. If no existing Mimir installation is available, or you would like to quickly install a test installation then follow the [getting-started](https://grafana.com/docs/mimir/latest/operators-guide/getting-started/) instructions.

### Gathering required information

In order to configure a write proxy we need to know the following pieces of information at a minimum:
* The TCP port that the write proxy should listen on
* The endpoint for remote writes within Mimir

The default TCP port for the write proxy is 8000 however it is best to choose a unique non-default port, especially if you are going to be running multiple write proxies (Graphite, Datadog, Influx, etc) on the same host.

If Mimir is configured to listen on port 9009 on localhost then the remote write endpoint will be http://localhost:9009/api/v1/push 

### Label-name validation

If you have tagged metrics with label names containing characters not supported by Mimir then you will need to add the `-skip-label-validation` command line argument when running `graphite-proxy-writes`. In order to maintain compatibility with Prometheus, Mimir only accepts label names that match the following regex `[a-zA-Z_:][a-zA-Z0-9_:]*` by default. Setting the `-skip-label-validation` command line argument of the Graphite write proxy makes it send the `X-Mimir-SkipLabelNameValidation` header with each metric push, which tells Mimir to skip its strict label name validation. This also needs to be [enabled within Mimir itself](https://grafana.com/docs/mimir/latest/operators-guide/configuring/reference-configuration-parameters/) with the `skip_label_name_validation_header_enabled` configuration parameter.

### An example invocation

(Pre-built binaries/docker images are on our list of things to do.)

To run the proxy:

```
$ dist/graphite-proxy-writes \
  -auth.enable=false \
  -server.http-listen-address 127.0.0.1 \
  -server.http-listen-port 8008 \
  -write-endpoint http://localhost:9009/api/v1/push
```

Details of configurable options are available in the `-help` output.

### Example output at startup

```
level=info ts=2022-07-07T08:01:30.118133934Z component=jaeger msg="Setting up tracing" service_name=graphite-proxy-writes
level=info ts=2022-07-07T08:01:30.119256598Z msg="server listening on address" addr=127.0.0.1:8008
level=info ts=2022-07-07T08:01:30.119307665Z msg="GRPC server listening on address" addr=[::]:9095
level=info ts=2022-07-07T08:01:30.119417505Z msg="Starting app" docker_tag=TODO
level=info ts=2022-07-07T08:01:30.119443692Z msg="graphite is using remote write API" address=http://localhost:9009/api/v1/push
level=info ts=2022-07-07T08:01:30.119478052Z msg="Waiting for stop signal..."
level=info ts=2022-07-07T08:01:30.119494601Z msg="Starting grpc server" addr=[::]:9095
level=info ts=2022-07-07T08:01:30.119513819Z msg="Starting internal server" addr=:8081
level=info ts=2022-07-07T08:01:30.119518999Z msg="Starting http server" addr=127.0.0.1:8008
```

### Example metric send

We can send a simple metric to the write proxy using the Graphite write proxy running above using the following command:

```
$ NOW=`date +%s` ; curl -H "Content-Type: application/json" "http://localhost:8008/graphite/metrics" -d '[{"Name": "AMetricName","Metric": "AMetricName.Foo.Bar","Interval": 10,"Value": 1000.123,"Unit": "unknown","Time": $NOW,"Mtype": "gauge","interval": 10}]'
```

This should produce the output similar to this from the Graphite write proxy:

```
level=info ts=2022-07-07T08:01:47.524782825Z org_id=fake method=graphiteWriter.ServeHTTP level=debug msg="successful series write" len=1 duration=5.073465ms
level=info ts=2022-07-07T08:01:47.524815239Z elapsed=5.140252ms traceID=26443354707a1bdf sampled=false orgID=fake method=POST uri=/graphite/metrics status=200
```

### Checking metrics received

We can check that the metrics were received by Mimir by checking the labels that are present:

```
$ curl -G http://localhost:9009/prometheus/api/v1/labels
{"status":"success","data":["__n000__","__name__"]}
```

The above shows we have two labels `__n000__` and `__name__`. Looking at the values for these labels we see:

```
$ curl -G http://localhost:9009/prometheus/api/v1/label/__name__/values
{"status":"success","data":["graphite_untagged"]}
```

```
$ curl -G http://localhost:9009/prometheus/api/v1/label/__n000__/values
{"status":"success","data":["AMetricName"]}
```

Alternatively the metrics can be queried using Grafana configured to point to Mimir as a Prometheus type data-source (see `getting started` guide linked above.)

## Carbon-Relay-NG configuration

Many people choose to forward their Graphite metrics via [Carbon Relay NG](https://github.com/grafana/carbon-relay-ng) as this provides the ability to handle many more Graphite protocols for metric ingestion, and can also provide some temporary buffering/forwarding capabilities.

With CRNG configured to send to the Graphite write proxy using the following example config snippets:

```
...
## Admin ##
admin_addr = "0.0.0.0:2005"
http_addr = "0.0.0.0:8091"

## Inputs ##
### plaintext Carbon ###
listen_addr = "0.0.0.0:2014"
# close inbound plaintext connections if they've been idle for this long ("0s" to disable)
plain_read_timeout = "5s"
### Pickle Carbon ###
pickle_addr = "0.0.0.0:2015"
# close inbound pickle connections if they've been idle for this long ("0s" to disable)
pickle_read_timeout = "5s"
...
[[route]]
key = 'grafanaNet'
type = 'grafanaNet'
addr = 'http://localhost:8008/graphite/metrics'
apikey = 'a'
...
```

We can now send to CRNG via HTTP (on port tcp/8091), plaintext via port tcp/2014, or pickle via port tcp/2015. For example:

```
$ TSTAMP=`date +%s` ; echo "power.usage 123.456 ${TSTAMP}" | nc localhost 2014
```

And we can see the `power` label is present in the list of values for the `__n000__` label:

```
$ curl -G http://localhost:9009/prometheus/api/v1/label/__n000__/values
{"status":"success","data":["AMetricName","carbon-relay-ng","power","service_is_carbon-relay-ng"]}
```

## Grafana Cloud as a destination

If the destination Mimir installation is part of a Grafana cloud instance the `-write-endpoint` argument should be of the form:
  -write-endpoint https://_username_:_password_@_grafana_net_instance_/api/v1/push
where the exact server details can be found on Prometheus instance details page for the stack on grafana.com

The _username_ is the numeric `Username / Instance ID`
The _password_ is the Grafana Cloud API Key with `metrics push` privileges/role.
The _grafana_net_instance_ is server part of the URL to push Prometheus metrics.

## Internal metrics

The graphite-proxy-writes binary exposes internal metrics on a `/metrics` endpoint on a separate port which can be scraped by a local prometheus installation. This is configurable with the `internalserver` command line options.
