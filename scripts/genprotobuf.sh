#!/usr/bin/env bash
# Generate all protobuf bindings.
set -euo pipefail
export SHELLOPTS        # propagate set to children by default
IFS=$'\t\n'
umask 0077

command -v protoc >/dev/null 2>&1 || { echo "protoc not installed,  Aborting." >&2; exit 1; }

if ! [[ "$0" =~ scripts/genprotobuf.sh ]]; then
	echo "must be run from repository root"
	exit 255
fi

# It's necessary to run go mod vendor because protoc needs the source files to resolve the imports
echo "INFO: Running go mod vendor"

DIRS=( "protos/errorx/v1")

command -v protoc-gen-go >/dev/null 2>&1 || { echo "protoc-gen-go is not installed"; exit 1; }

echo "INFO: Generating code"
for dir in "${DIRS[@]}"; do
	protoc \
	--go_out=.  \
	"${dir}"/*.proto
done

echo "INFO: Proto files are up to date"