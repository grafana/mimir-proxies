#!/bin/bash

if [ "$3"x == "x" ] ; then
  echo "need workdir, tmp parent dir, to-delete yaml"
  exit 1
fi

workdir="$1"
tmpparent="$2"
todelete="$3"
rundir=$(dirname $0)

# List blocks in the directory, randomly sorted to spread the work around.
blocks=$(ls -1 $workdir | grep 01 | sort -R)

# Split the list into chunks of 10 and run 8 instances of thanos at a time.
echo $blocks | xargs -P 8 -n 10 $rundir/block-rewrite.sh $workdir $tmpparent $todelete
