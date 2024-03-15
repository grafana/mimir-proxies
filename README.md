<a href="https://goreportcard.com/report/github.com/grafana/mimir-proxies"><img src="https://goreportcard.com/badge/github.com/grafana/mimir-proxies" alt="Go Report Card" /></a>

# Grafana Mimir proxies

Grafana Mimir proxies are a collection of open source software projects that provide native ingest capability for third-party applications into [Mimir](https://grafana.com/oss/mimir/).

Details of the Datadog write proxy can be found [here](cmd/datadog-proxy-writes/README.md).

Details of the Graphite write proxy can be found [here](cmd/graphite-proxy-writes/README.md).

A proxy for Influx Line protocol can be found in the [grafana/influx2cortex](https://github.com/grafana/influx2cortex/) repository.

## The future

This is an initial “as is” release of the Graphite, Datadog and Influx write proxies, hence the release via two different github repositories. In time the Influx write proxy will move from its original/current home to be consolidated in this repository.

There is plenty of work planned to refactor the existing proxies, and a common framework for creating future write proxies with less duplication/boiler-plate code. All three existing proxies were developed internally by different teams, so we are taking the best approaches from all three and combining them with future write proxies in mind whilst also consolidating the existing proxies. We consider this to be part of our tech debt, and don’t want this to stagnate or rot, so look out for upcoming improvements in many areas (logging, tracing, testing, maintainability, etc)!

Because of this, there may be changes to interfaces, code structure, command-line-arguments, etc but we will try to only make breaking changes where necessary. Despite these warnings, this is the fundamentally the code that is running in production at scale within Grafana Labs.

We welcome issues/PRs if you have any suggestions or contributions for new proxies/formats/protocols to support.

## Releasing

Releasing should happen semi-automatically through goreleaser and github actions.

On every push to main a github action called `Run Release Please` will run. It will draft the next release and create
a pull request like [this one](https://github.com/grafana/mimir-proxies/pull/136) updating the CHANGELOG. On merge it
will publish the release and attach the binaries to it.