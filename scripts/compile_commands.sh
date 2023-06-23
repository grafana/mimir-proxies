#!/bin/bash
# Builds command binaries
set -eufo pipefail
export SHELLOPTS	# propagate set to children by default
IFS=$'\t\n'

command -v go >/dev/null 2>&1 || { echo 'Please install go'; exit 1; }

# export GOPRIVATE="github.com/grafana/*"
export GOPRIVATE=""
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
GIT_COMMIT="${DRONE_COMMIT:-$(git rev-list -1 HEAD)}"
COMMIT_UNIX_TIMESTAMP="$(git show -s --format=%ct "${GIT_COMMIT}")"
# DOCKER_TAG="$(bash scripts/docker-tag.sh)"
DOCKER_TAG="TODO"
VERSION=$(cat CHANGELOG.md | grep "^## \[" |head -n 1 | cut -d\[ -f 2- | cut -d\] -f 1)

for cmd in datadog-proxy-writes graphite-proxy-writes mimir-whisper-converter
do
    go build \
    -tags netgo \
    -o "dist/${cmd}" \
    -ldflags "\
        -w \
        -extldflags '-static' \
        -X 'main.version=$VERSION' \
        -X 'github.com/grafana/mimir-proxies/pkg/appcommon.CommitUnixTimestamp=${COMMIT_UNIX_TIMESTAMP}' \
        -X 'github.com/grafana/mimir-proxies/pkg/appcommon.DockerTag=${DOCKER_TAG}' \
        " \
    "github.com/grafana/mimir-proxies/cmd/${cmd}"

    echo "Succesfully built ${cmd} into dist/${cmd}"
done
