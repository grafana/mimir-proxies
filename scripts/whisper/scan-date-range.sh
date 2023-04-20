#!/bin/bash
set -eufo pipefail

/app/mimir-whisper-converter --whisper-directory /input daterange > daterange.out
cat daterange.out
