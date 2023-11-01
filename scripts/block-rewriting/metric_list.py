#!/usr/bin/python3
import os, sys
import argparse

def walk_metrics(path):
  """Walks filesystem at path and returns the whisper files found.

  Metric names do not include the passed-in path prefix.
  """

  metrics = []
  for root, _, files in os.walk(path):
    for f in files:
      if not f.endswith(".wsp"):
        continue
      # remove path prefix from root
      trimmedRoot = root[len(path):]
      metrics.append(os.path.join(trimmedRoot, f))
  return metrics

def accumulate_filters(metrics, skip_leaves):
  filters = []
  for m in metrics:
    filter = path_to_filter(m, skip_leaves)
    if filter not in filters:
      filters.append(filter)
  return filters

def path_to_filter(path, skip_leaves):
  """Convert a filesystem path to a metric filter pattern"""

  toks = path.split('/')
  if skip_leaves:
    toks = toks[:-1]
  else:
    toks[-1] = toks[-1].split('.')[0]

  filter_items = []
  for i, t in enumerate(toks):
    item = '__n%03d__="%s"' % (i, t)
    filter_items.append(item)
  return ' - matchers: \'{ ' + ', '.join(filter_items) + ' }\''


if __name__ == '__main__':
  parser = argparse.ArgumentParser()
  parser.add_argument('--path', dest='path', help='Top of metrics tree')
  parser.add_argument('--skip-leaves', action=argparse.BooleanOptionalAction, help='If true, don\'t include the leaf metric name in the filter.')
  args = parser.parse_args()

  if not args.path:
    parser.print_help()
    sys.exit(1)

  metrics = walk_metrics(args.path)
  filters = accumulate_filters(metrics, args.skip_leaves)
  for f in filters:
    print (f)



