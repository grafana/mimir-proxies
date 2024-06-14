package storage

import (
	"context"

	"github.com/prometheus/prometheus/storage"
)

// Queryable is a storage.Queryable that returns an expanded version of storage.Querier.
type Queryable interface {
	Querier(mint, maxt int64) (Querier, error)
}

// Querier is a storage.Querier that can also query series using label matchers.
type Querier interface {
	storage.Querier
	SeriesQuerier
}

// SeriesQuerier provides querying series using label matchers.
// https://prometheus.io/docs/prometheus/latest/querying/api/#finding-series-by-label-matchers
type SeriesQuerier interface {
	Series(ctx context.Context, labelMatchers []string) ([]map[string]string, error)
}
