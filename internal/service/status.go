package service

import (
	"context"
	"fmt"

	"github.com/Taier05/InfraView/internal/datasource"
)

func (s *Service) DataSourceStatus(ctx context.Context) (DataSourceStatus, Meta, error) {
	result, err := s.store.GetOrLoad(ctx, healthCacheKey, s.options.HealthTTL, s.options.MaxStale, func(loadCtx context.Context) (any, error) {
		return s.provider.Health(loadCtx)
	})
	if err != nil {
		return DataSourceStatus{}, Meta{}, mapProviderError(err)
	}
	health, ok := result.Value.(datasource.Health)
	if !ok {
		return DataSourceStatus{}, Meta{}, fmt.Errorf("service: health cache contained %T", result.Value)
	}
	return DataSourceStatus{Healthy: health.Healthy, CheckedAt: health.CheckedAt}, resultMetaAt(result, health.CheckedAt), nil
}
