package remoteread

import (
	"context"

	"github.com/prometheus/prometheus/prompb"
	"github.com/prometheus/prometheus/storage"
)

type Client interface {
	Type() string
	Read(context.Context, *prompb.Query) (storage.SeriesSet, error)
}
