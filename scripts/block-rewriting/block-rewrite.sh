#!/bin/bash

if [ "$3"x == "x" ] ; then
        echo "need workdir, tmpdir parent, todelete file, and ids"
        exit 1
fi

workdir="$1"
shift
tmpparent="$1"
shift
todelete="$1"
shift

tmpdir=$(mktemp -d -p $tmpparent)

thanos tools bucket rewrite --no-dry-run --prom-blocks --delete-blocks --log.level info \
  --tmp.dir=$tmpdir \
  $(echo $@ | tr " " "\n" | awk '{print "--id " $0}' | tr "\n" " ")  \
  --objstore.config "
type: FILESYSTEM
config:
  directory: $workdir/
" \
  --rewrite.to-delete-config-file=$todelete
\