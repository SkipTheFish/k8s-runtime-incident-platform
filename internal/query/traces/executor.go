package traces

import (
	"context"

	"observability-platform/internal/query"
)

type TempoExecutor struct {
	client *TempoClient
}

func NewTempoExecutor(client *TempoClient) *TempoExecutor {
	return &TempoExecutor{client: client}
}

func (e *TempoExecutor) Execute(ctx context.Context, q *query.Query) (*query.QueryResult, error) {

	switch q.Mode {

	case "trace_id":
		res, err := e.client.GetTrace(ctx, q.Expr)
		if err != nil {
			return nil, err
		}

		return &query.QueryResult{
			Data:   res,
			Source: "tempo",
		}, nil

	case "search":
		res, err := e.client.Search(ctx, q.Expr)
		if err != nil {
			return nil, err
		}

		return &query.QueryResult{
			Data:   res,
			Source: "tempo",
		}, nil

	default:
		return &query.QueryResult{
			Data: map[string]any{
				"message": "unknown trace mode",
			},
			Source: "tempo",
		}, nil
	}
}