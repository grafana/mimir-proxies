package ingester

import (
	"context"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"

	"github.com/grafana/mimir-proxies/pkg/errorx"
)

type disabledIngester struct{}

func NewDisabled() Ingester {
	return disabledIngester{}
}

func (disabledIngester) StoreMetrics(ctx context.Context, series ddstructs.Series) error {
	return errorx.Disabled{}
}

func (disabledIngester) StoreHostTags(ctx context.Context, hostname string, systemTags []string) error {
	return errorx.Disabled{}
}

func (disabledIngester) StoreCheckRun(ctx context.Context, checks ddstructs.ServiceChecks) error {
	return errorx.Disabled{}
}
