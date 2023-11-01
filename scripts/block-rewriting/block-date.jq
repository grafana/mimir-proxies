#!/usr/bin/env -S jq -f

{
  "maxTime": (.maxTime / 1000 | todateiso8601),
}
