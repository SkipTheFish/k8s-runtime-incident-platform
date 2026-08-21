package traces

import (
	"context"

	"observability-platform/internal/query"
)

type TracesService struct {
	pipeline *query.QueryPipeline
}

func NewTracesService(p *query.QueryPipeline) *TracesService {
	return &TracesService{pipeline: p}
}

// 查询 trace by id
func (s *TracesService) GetTrace(ctx context.Context, tenant, traceID string) (*query.QueryResult, error) {

	q := &query.Query{
		Type:   query.QueryTypeTraces,
		Mode:   "trace_id",
		Expr:   traceID,
		Tenant: tenant,
	}

	return s.pipeline.Run(ctx, q)
}

// 搜索 trace
func (s *TracesService) Search(ctx context.Context, tenant, service string) (*query.QueryResult, error) {

	q := &query.Query{
		Type:   query.QueryTypeTraces,
		Mode:   "search",
		Expr:   service,
		Tenant: tenant,
	}

	return s.pipeline.Run(ctx, q)
}
