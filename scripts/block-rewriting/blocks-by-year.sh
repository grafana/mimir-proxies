#/bin/bash

for d in 0* ; do
  year=$(/app/bin/block-date.jq $d/meta.json | grep maxTime | cut -d\" -f4 | cut -d- -f 1)
  mkdir -p $year
  mv $d $year/
done

