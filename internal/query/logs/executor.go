package logs

import (
	"context"
	"observability-platform/internal/query"
	"time"
)

type LokiExecutor struct {
	client *LokiClient
}

func NewLokiExecutor(client *LokiClient) *LokiExecutor {
	return &LokiExecutor{client: client}
}

func (e *LokiExecutor) Execute(ctx context.Context, q *query.Query) (*query.QueryResult, error) {

	start := q.StartTime
	end := q.EndTime

	if start.IsZero() {
		start = end.Add(-30 * time.Minute)
	}

	res, err := e.client.Query(ctx, q.Expr, start, end)
	if err != nil {
		return nil, err
	}

	return &query.QueryResult{
		Data:   res,
		Source: "loki",
		Cached: false,
	}, nil
}
