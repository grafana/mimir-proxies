package remoteread

import (
	"context"
	"time"

	"github.com/prometheus/prometheus/model/labels"

	"github.com/prometheus/prometheus/storage"
)

func NewMeasuredStorageQueryable(q storage.Queryable, rec Recorder, timeNow func() time.Time) storage.Queryable {
	return &measuredStorageQueryable{q, rec, timeNow}
}

type measuredStorageQueryable struct {
	queryable storage.Queryable
	rec       Recorder
	timeNow   func() time.Time
}

func (m *measuredStorageQueryable) Querier(ctx context.Context, mint, maxt int64) (storage.Querier, error) {
	q, err := m.queryable.Querier(ctx, mint, maxt)
	if err != nil {
		return nil, err
	}
	return &measuredStorageQuerier{ctx, q, m.rec, m.timeNow}, nil
}

type measuredStorageQuerier struct {
	ctx     context.Context
	querier storage.Querier
	rec     Recorder
	timeNow func() time.Time
}

func (m *measuredStorageQuerier) Select(sortSeries bool, hints *storage.SelectHints, matchers ...*labels.Matcher) (set storage.SeriesSet) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.Select", m.timeNow().Sub(t0), set.Err())
	}(m.timeNow())
	return m.querier.Select(sortSeries, hints, matchers...)
}

func (m *measuredStorageQuerier) LabelValues(name string, matchers ...*labels.Matcher) (_ []string, _ storage.Warnings, err error) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.LabelValues", m.timeNow().Sub(t0), err)
	}(m.timeNow())
	return m.querier.LabelValues(name, matchers...)
}

func (m *measuredStorageQuerier) LabelNames(matchers ...*labels.Matcher) (_ []string, _ storage.Warnings, err error) {
	defer func(t0 time.Time) {
		m.rec.measure("StorageQuerier.LabelNames", m.timeNow().Sub(t0), err)
	}(m.timeNow())
	return m.querier.LabelNames(matchers...)
}

func (m *measuredStorageQuerier) Close() error {
	return m.querier.Close()
}
