package nightingale

import (
	"context"

	"github.com/Taier05/InfraView/internal/datasource"
)

type Provider struct{}

func New() datasource.Provider {
	return &Provider{}
}

func (*Provider) Health(context.Context) (datasource.Health, error) {
	return datasource.Health{}, datasource.ErrNotConfigured
}

func (*Provider) ListHosts(context.Context) ([]datasource.Host, error) {
	return nil, datasource.ErrNotConfigured
}

func (*Provider) GetHost(context.Context, string) (datasource.Host, error) {
	return datasource.Host{}, datasource.ErrNotConfigured
}

func (*Provider) GetCurrentMetrics(context.Context, []string) (map[string]datasource.CurrentMetrics, error) {
	return nil, datasource.ErrNotConfigured
}

func (*Provider) QueryRange(context.Context, datasource.RangeRequest) ([]datasource.Series, error) {
	return nil, datasource.ErrNotConfigured
}

func (*Provider) QueryAggregateRange(context.Context, datasource.AggregateRangeRequest) ([]datasource.Series, error) {
	return nil, datasource.ErrNotConfigured
}

var _ datasource.Provider = (*Provider)(nil)
