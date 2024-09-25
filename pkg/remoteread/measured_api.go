package remoteread

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/grafana/mimir-proxies/pkg/ctxlog"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type MeasuredAPI struct {
	api      API
	recorder Recorder
	log      ctxlog.Provider
	tracer   opentracing.Tracer
	timeNow  func() time.Time
}

func NewMeasuredAPI(api API, rec Recorder, log ctxlog.Provider, tracer opentracing.Tracer, timeNow func() time.Time) API {
	return &MeasuredAPI{
		api:      api,
		recorder: rec,
		log:      log,
		tracer:   tracer,
		timeNow:  timeNow,
	}
}

func (ma *MeasuredAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (v model.Value, w v1.Warnings, err error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, ma.tracer, "remoteread.Query")
	defer sp.Finish()
	sp.LogKV("query", query, "ts", ts)

	defer func(t0 time.Time) {
		ma.recorder.measure("Query", ma.timeNow().Sub(t0), err)
	}(ma.timeNow())
	return ma.api.Query(ctx, query, ts, opts...)
}

func (ma *MeasuredAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (v model.Value, w v1.Warnings, err error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, ma.tracer, "remoteread.QueryRange")
	defer sp.Finish()
	sp.LogKV("query", query, "range", fmt.Sprintf("%+v", r))

	defer func(t0 time.Time) {
		duration := ma.timeNow().Sub(t0)
		ma.recorder.measure("QueryRange", duration, err)
	}(ma.timeNow())
	return ma.api.QueryRange(ctx, query, r, opts...)
}

func (ma *MeasuredAPI) Series(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (ls []model.LabelSet, w v1.Warnings, err error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, ma.tracer, "remoteread.Series")
	defer sp.Finish()
	sp.LogKV("matches", strings.Join(matches, ","), "startTime", startTime, "endTime", endTime)

	defer func(t0 time.Time) {
		ma.recorder.measure("Series", ma.timeNow().Sub(t0), err)
	}(ma.timeNow())
	return ma.api.Series(ctx, matches, startTime, endTime, opts...)
}

func (ma *MeasuredAPI) LabelNames(ctx context.Context, matches []string, startTime, endTime time.Time, opts ...v1.Option) (s []string, w v1.Warnings, err error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, ma.tracer, "remoteread.LabelNames")
	defer sp.Finish()
	sp.LogKV("matches", strings.Join(matches, ","), "startTime", startTime, "endTime", endTime)

	defer func(t0 time.Time) {
		ma.recorder.measure("LabelNames", ma.timeNow().Sub(t0), err)
	}(ma.timeNow())
	return ma.api.LabelNames(ctx, matches, startTime, endTime, opts...)
}

func (ma *MeasuredAPI) LabelValues(ctx context.Context, label string, matches []string, startTime, endTime time.Time, opts ...v1.Option) (v model.LabelValues, w v1.Warnings, err error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, ma.tracer, "remoteread.LabelValues")
	defer sp.Finish()
	sp.LogKV("label", label, "matches", strings.Join(matches, ","), "startTime", startTime, "endTime", endTime)

	defer func(t0 time.Time) {
		ma.recorder.measure("LabelValues", ma.timeNow().Sub(t0), err)
	}(ma.timeNow())
	return ma.api.LabelValues(ctx, label, matches, startTime, endTime, opts...)
}
