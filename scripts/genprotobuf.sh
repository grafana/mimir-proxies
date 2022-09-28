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
go mod vendor

DIRS=( "protos/errorx/v1")

command -v protoc-gen-gogofast >/dev/null 2>&1 || { echo "protoc-gen-gogofast is not installed"; exit 1; }
command -v protoc-gen-gogoslick >/dev/null 2>&1 || { echo "protoc-gen-gogoslick is not installed"; exit 1; }

# Set the import path for Proto files
GOGOPROTO_ROOT="$(go list -mod=mod -f '{{ .Dir }}' -m github.com/gogo/protobuf)"
GOGOPROTO_PATH="${GOGOPROTO_ROOT}:${GOGOPROTO_ROOT}/protobuf"
PROTO_PATH="protos:${GOGOPROTO_PATH}:vendor"

echo "INFO: Generating code"
for dir in "${DIRS[@]}"; do
	protoc \
	--gogoslick_out=Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,plugins=grpc:./ \
	-I="${PROTO_PATH}" \
	"${dir}"/*.proto
done

echo "INFO: Proto files are up to date"