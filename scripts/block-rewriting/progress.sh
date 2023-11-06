#!/bin/sh

for d in "$1"/0* ; do
        processed=$(cat $d/meta.json | grep deletions_applied | wc -l)
        if [ $processed -gt 0 ] ; then
                echo $d done
        fi
done | wc -l
