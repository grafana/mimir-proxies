package ingester

import (
	"context"

	"github.com/grafana/mimir/pkg/mimirpb"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddprom"
	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"
	"github.com/grafana/mimir-proxies/pkg/datadog/htstorage"
	"github.com/grafana/mimir-proxies/pkg/remotewrite"

	"github.com/grafana/mimir-proxies/pkg/errorx"
)

type ingester struct {
	client remotewrite.Client

	recorder       Recorder
	hostTagStorage htstorage.Storage
}

func New(
	hostTagStorage htstorage.Storage,
	metricsRecorder Recorder,
	client remotewrite.Client,
) Ingester {
	return &ingester{
		client:         client,
		recorder:       metricsRecorder,
		hostTagStorage: hostTagStorage,
	}
}

func (in *ingester) StoreMetrics(ctx context.Context, ddSeries ddstructs.Series) error {
	translated, err := ddSeriesToPromWriteRequest(ctx, ddSeries, in.hostTagStorage)
	if err != nil {
		return errorx.BadRequest{Msg: "can't translate series", Err: err}
	}
	defer mimirpb.ReuseSlice(translated.Timeseries)

	in.recorder.measureMetricsParsed(len(translated.Timeseries))

	return in.client.Write(ctx, translated)
}

func (in *ingester) StoreHostTags(ctx context.Context, hostname string, systemTags []string) error {
	lbls := append(
		ddprom.TagsToLabels(systemTags).PrompbLabels(),
		ddprom.AllHostTagsPrompbLabel(systemTags),
	)
	// FIXME: record some metrics here: how many tags were stored? how many were deduplicated?
	err := in.hostTagStorage.Set(ctx, hostname, lbls)
	if err != nil {
		return errorx.Internal{Msg: "can't store host tags", Err: err}
	}

	return nil
}

func (in *ingester) StoreCheckRun(ctx context.Context, checks ddstructs.ServiceChecks) error {
	translated, err := ddCheckRunToPromWriteRequest(ctx, checks, in.hostTagStorage)
	if err != nil {
		return errorx.BadRequest{Msg: "can't translate series", Err: err}
	}
	defer mimirpb.ReuseSlice(translated.Timeseries)

	return in.client.Write(ctx, translated)
}
