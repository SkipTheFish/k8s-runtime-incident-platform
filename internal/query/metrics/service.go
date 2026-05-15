package metrics

import (
	"context"
	"time"

	"observability-platform/internal/query"
)

type MetricsService struct {
	pipeline *query.QueryPipeline
}

func NewMetricsService(pipeline *query.QueryPipeline) *MetricsService {
	return &MetricsService{pipeline: pipeline}
}

func (s *MetricsService) Query(ctx context.Context, q *query.Query) (*query.QueryResult, error) {
	q.Type = query.QueryTypeMetrics
	q.Mode = query.QueryModeInstant
	return s.pipeline.Run(ctx, q)
}

func (s *MetricsService) Range(ctx context.Context, q *query.Query) (*query.QueryResult, error) {
	q.Type = query.QueryTypeMetrics
	q.Mode = query.QueryModeRange

	if q.Step == 0 {
		q.Step = 30 * time.Second
	}

	return s.pipeline.Run(ctx, q)
}

func (s *MetricsService) Labels(ctx context.Context, tenant string) (*query.QueryResult, error) {
	q := &query.Query{
		Type:   query.QueryTypeMetrics,
		Mode:   "labels",
		Expr:   "__labels__",
		Tenant: tenant,
	}

	return s.pipeline.Run(ctx, q)
}