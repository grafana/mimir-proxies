# Utilities for rewriting blocks

## block-date.jq

Simple jq script that prints out the maxTime value for a tsdb meta.json file

## metric_list.py

Python script that processes a list of graphite whisper files as matchers that
can be passed to a thanos block rewrite command.

Can optionally skip the leaf values (actual metric names) to reduce the size of
the matcher list.

## blocks-by-year.sh

Script that takes a directory which should contain tsdb blocks. For each block,
finds the max date of the block using block-date.jq, and then parses out the
year and moves that block into a directory named for that year. Assumes that
block-date.jq is in the same folder as blocks-by-year.sh.

## block-rewrite.sh

Usage: block-rewrite.sh [workdir] [todelete yaml file] [block IDs....]

Deletes series according to the todelete file provided (created by, for
instance, metric_list.py).

## thanos-parallel.sh

Usage: thanos-parallel.sh [workdir] [todelete yaml file]

Operates on a workdir containing tsdb blocks. Will run thanos in parallel,
deleting series according to the todelete file. Assumes that block-rewrite.sh is
in the same directory.
