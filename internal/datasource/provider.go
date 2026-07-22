package datasource

import "context"

type Provider interface {
	Health(context.Context) (Health, error)
	ListHosts(context.Context) ([]Host, error)
	GetHost(context.Context, string) (Host, error)
	GetCurrentMetrics(context.Context, []string) (map[string]CurrentMetrics, error)
	QueryRange(context.Context, RangeRequest) ([]Series, error)
	QueryAggregateRange(context.Context, AggregateRangeRequest) ([]Series, error)
}
