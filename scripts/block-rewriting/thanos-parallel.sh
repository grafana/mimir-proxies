#!/bin/bash

# Exit immediately on any command failure
# Treat unset variables as an error
# Take return value from last nonzero pipe command
set -euo pipefail

if [ "$5"x == "x" ] ; then
  echo "need workdir, tmp parent dir, dest dir, to-delete yaml, number of parallel execs"
  exit 1
fi

workdir="$1"
tmpparent="$2"
destdir="$3"
todelete="$4"
parallel="$5"
rundir=$(dirname $0)

if [ ! -d $tmpparent ] ; then
  echo "tmpparent invalid: $tmpparent"
  exit 1
fi

if [ ! -d $destdir ] ; then
  echo "destdir invalid: $destdir"
  exit 1
fi

tmpdir=$(mktemp -d -p $tmpparent)

function cleanup() {
  # Cleanup from previous runs: remove tmpparent
  rm -rf $tmpparent/*

  # Delete processed old blocks
  thanos tools bucket cleanup --delete-delay=0h --objstore.config "
  type: FILESYSTEM
  config:
    directory: $workdir/
  "

  # Remove empty blocks
  for d in $(ls -1 $workdir/ | grep 01) ; do
    if [ ! -d $workdir/$d/chunks ] ; then
      echo "Deleting empty block $d"
      rm -rf $workdir/$d
    fi
  done

  # Move finished new blocks to dest -- this is custom for this backup process,
  # not generalizable.
  for d in "$workdir"/0* ; do
    processed=$(cat $d/meta.json | grep deletions_applied | wc -l)
    if [ $processed -gt 0 ] ; then
      echo Moving finished block $d
      mv $d "$destdir"/
    fi
  done
}

# Run cleanup before work in case this is a resumed run.
cleanup()

# List blocks in the directory, randomly sorted to spread the work around.
blocks=$(ls -1 $workdir | grep 01 | sort -R)

# Split the list into chunks of 1 and run $parallel instances of thanos at a time.
echo $blocks | xargs -P $parallel -n 10 $rundir/block-rewrite.sh $workdir $tmpparent $todelete

cleanup()