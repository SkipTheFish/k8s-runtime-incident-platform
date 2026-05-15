package logs

import (
	"context"
	"time"

	"observability-platform/internal/query"
)

type LogsService struct {
	pipeline *query.QueryPipeline
}

func NewLogsService(pipeline *query.QueryPipeline) *LogsService {
	return &LogsService{pipeline: pipeline}
}

func (s *LogsService) Query(ctx context.Context, q *query.Query) (*query.QueryResult, error) {
	q.Type = query.QueryTypeLogs
	q.Mode = query.QueryModeRange

	if q.Step == 0 {
		q.Step = 10 * time.Second
	}

	return s.pipeline.Run(ctx, q)
}

func (s *LogsService) Context(ctx context.Context, q *query.Query) (*query.QueryResult, error) {
	// 简化：直接复用 Query
	return s.Query(ctx, q)
}

func (s *LogsService) Stats(ctx context.Context, q *query.Query) (*query.QueryResult, error) {
	q.Type = query.QueryTypeLogs
	q.Mode = query.QueryModeRange

	// 正确：必须包一层 ()
	q.Expr = "count_over_time((" + q.Expr + ")[5m])"

	return s.pipeline.Run(ctx, q)
}