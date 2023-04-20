#!/bin/bash
set -eufo pipefail

# Usage:
#   convert-whisper.sh $WORKER_ID $NUM_WORKERS

echo 'Scanning for date range...'
dateRangeOpts=$(/app/mimir-whisper-converter --whisper-directory /input daterange)
echo $dateRangeOpts
echo ''

echo 'Running pass 1...'
/app/mimir-whisper-converter \
  --whisper-directory /input \
  $dateRangeOpts \
  --intermediate-directory /output/intermediate \
  --blocks-directory /output/blocks/data \
  --workerID $1 \
  --workers $2 \
  pass1
echo 'Done.'
echo ''

echo 'Running pass 2...'
/app/mimir-whisper-converter \
  --whisper-directory /input \
  $dateRangeOpts \
  --intermediate-directory /output/intermediate \
  --blocks-directory /output/blocks/data \
  --workerID $1 \
  --workers $2 \
  pass2
echo 'Done.'
echo ''
