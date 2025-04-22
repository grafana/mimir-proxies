package remoteread

import (
	"context"
	"time"

	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/util/annotations"

	remotereadstorage "github.com/grafana/mimir-proxies/pkg/remoteread/storage"
)

func NewMeasuredStorageQueryable(q remotereadstorage.Queryable, rec Recorder, timeNow func() time.Time) remotereadstorage.Queryable {
	return &measuredStorageQueryable{q, rec, timeNow}
}

type measuredStorageQueryable struct {
	queryable remotereadstorage.Queryable
	rec       Recorder
	timeNow   func() time.Time
}

func (m *measuredStorageQueryable) Querier(mint, maxt int64) (remotereadstorage.Querier, error) {
	q, err := m.queryable.Querier(mint, maxt)
	if err != nil {
		return nil, err
	}
	return &measuredStorageQuerier{q, m.rec, m.timeNow}, nil
}

type measuredStorageQuerier struct {
	querier remotereadstorage.Querier
	rec     Recorder
	timeNow func() time.Time
}

func (m *measuredStorageQuerier) Select(ctx context.Context, sortSeries bool, hints *storage.SelectHints, matchers ...*labels.Matcher) (set storage.SeriesSet) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.Select", m.timeNow().Sub(t0), set.Err())
	}(m.timeNow())
	return m.querier.Select(ctx, sortSeries, hints, matchers...)
}

func (m *measuredStorageQuerier) LabelValues(ctx context.Context, name string, hints *storage.LabelHints, matchers ...*labels.Matcher) (_ []string, _ annotations.Annotations, err error) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.LabelValues", m.timeNow().Sub(t0), err)
	}(m.timeNow())
	return m.querier.LabelValues(ctx, name, hints, matchers...)
}

func (m *measuredStorageQuerier) LabelNames(ctx context.Context, hints *storage.LabelHints, matchers ...*labels.Matcher) (_ []string, _ annotations.Annotations, err error) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.LabelNames", m.timeNow().Sub(t0), err)
	}(m.timeNow())
	return m.querier.LabelNames(ctx, hints, matchers...)
}

func (m *measuredStorageQuerier) Series(ctx context.Context, matchers []string) (_ []map[string]string, err error) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.Series", m.timeNow().Sub(t0), err)
	}(m.timeNow())
	return m.querier.Series(ctx, matchers)
}

func (m *measuredStorageQuerier) Close() error {
	return m.querier.Close()
}
