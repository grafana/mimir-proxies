#/bin/bash


dryrun=0
if [ "$1" == "--dry-run" ] ; then
  dryrun=1
  shift
fi

if [ "$1"x == "x" ] ; then
  echo "need working directory."
  echo "optional: --dry run"
  exit 1
fi

rundir=$(dirname $0)
workdir="$1"

for d in "$workdir"/0* ; do
  year=$($rundir/block-date.jq $d/meta.json | grep maxTime | cut -d\" -f4 | cut -d- -f 1)
  if [ $dryrun -eq 1 ] ; then
    echo mkdir -p "$workdir"/$year
    echo mv $d "$workdir"/$year/
  else
    mkdir -p "$workdir"/$year
    mv $d "$workdir"/$year/
  fi
done

