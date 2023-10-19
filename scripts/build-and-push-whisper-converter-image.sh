#!/bin/bash
set -eufo pipefail

command -v go >/dev/null 2>&1 || { echo 'Please install go'; exit 1; }
command -v docker >/dev/null 2>&1 || { echo 'Please install docker'; exit 1; }

if [ $# -eq 0 ]  ; then
  echo "Need a docker registry location, like myreg/images"
  echo "'mimir-whisper-converter' will be appended to this path as the image name"
  exit 1
fi
DOCKER_REGISTRY="$1"

# Build the executable
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
go build \
  -tags netgo \
  -o dist/mimir-whisper-converter \
  -ldflags "-w -extldflags '-static'" \
  "github.com/grafana/mimir-proxies/cmd/mimir-whisper-converter"

# Build the docker image
docker build \
  --platform linux/amd64 \
  -f cmd/mimir-whisper-converter/Dockerfile \
  -t $DOCKER_REGISTRY/mimir-whisper-converter:latest \
  .

# Push the docker image
docker image push $DOCKER_REGISTRY/mimir-whisper-converter:latest

echo 'Done'
