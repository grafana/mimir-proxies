package ingester

import (
	"context"

	"github.com/grafana/mimir-proxies/pkg/datadog/ddstructs"
)

//go:generate mockery --output ingestermock --outpkg ingestermock --case underscore --name Ingester
type Ingester interface {
	StoreMetrics(ctx context.Context, series ddstructs.Series) error
	StoreHostTags(ctx context.Context, hostname string, systemTags []string) error
	StoreCheckRun(ctx context.Context, checks ddstructs.ServiceChecks) error
}
