#!/bin/bash

workdir="$1"
todelete="$2"

# List blocks in the directory, randomly sorted to spread the work around.
blocks=$(ls -1 $workdir | grep 01 | sort -R)

# Split the list into chunks of 10 and run 8 instances of thanos at a time.
echo $blocks | xargs -P 8 -n 10 /app/bin/block-rewrite.sh $workdir $todelete


