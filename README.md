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

Releasing is done manually, and is based on the scripts that Mimir uses.
Releases will appear in the github project for mimir-proxies.

Currently the release configuration only builds the mimir-whisper-converter, not
all of the commands.

1. Increment the version number in VERSION.
2. Add a heading to CHANGELOG.md to describe the major changes.
3. Create a release branch: `git checkout -b release-$(cat VERSION)`
4. Commit changes.
5. run `./scripts/release/tag-release.sh` to sign and tag the branch.
6. run `./scripts/release/create-draft-release.sh`.
7. Go to the link printed at the end of the build and upload, and check that the
   release makes sense.
8. Either fix it, or click Edit and Publish the release.
9. Merge the release PR into main

If you run into problems with the tagged release, you can delete it. You'll need
to do that both locally and remotely:

```sh
git tag -d mimir-proxies-$(cat VERSION)
git push --delete origin mimir-proxies-$(cat VERSION)
```