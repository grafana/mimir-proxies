package remotewrite

import (
	"context"
	"time"

	"github.com/grafana/mimir/pkg/mimirpb"
	"github.com/opentracing/opentracing-go"
)

type MeasuredClient struct {
	client   Client
	recorder Recorder
	tracer   opentracing.Tracer
	timeNow  func() time.Time
}

func NewMeasuredClient(client Client, recorder Recorder, tracer opentracing.Tracer, timeNow func() time.Time) Client {
	return &MeasuredClient{
		client:   client,
		recorder: recorder,
		tracer:   tracer,
		timeNow:  timeNow,
	}
}

func (mc *MeasuredClient) Write(ctx context.Context, req *mimirpb.WriteRequest) (err error) {
	sp, ctx := opentracing.StartSpanFromContextWithTracer(ctx, mc.tracer, "remotewrite.Write")
	defer sp.Finish()
	sp.LogKV("series_count", len(req.Timeseries))
	if len(req.Metadata) > 0 {
		sp.LogKV("example_metric", req.Metadata[0].MetricFamilyName)
	}
	defer func(t0 time.Time) {
		mc.recorder.measure("Write", mc.timeNow().Sub(t0), err)
	}(mc.timeNow())
	return mc.client.Write(ctx, req)
}
