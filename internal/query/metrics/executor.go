package metrics

import (
	"context"

	"observability-platform/internal/datasource/prometheus"
	"observability-platform/internal/query"
)

const QueryModeLabels query.QueryMode = "labels"

type PrometheusExecutor struct {
	client *prometheus.Client
}

func NewPrometheusExecutor(client *prometheus.Client) *PrometheusExecutor {
	return &PrometheusExecutor{client: client}
}

func (e *PrometheusExecutor) Execute(ctx context.Context, q *query.Query) (*query.QueryResult, error) {
	switch q.Mode {
	case query.QueryModeInstant:
		res, err := e.client.Query(ctx, q.Expr)
		if err != nil {
			return nil, err
		}

		return &query.QueryResult{
			Data:   res,
			Source: "prometheus",
			Cached: false,
		}, nil

	case query.QueryModeRange:
		res, err := e.client.RangeQuery(
			ctx,
			q.Expr,
			q.StartTime,
			q.EndTime,
			q.Step.String(),
		)
		if err != nil {
			return nil, err
		}

		return &query.QueryResult{
			Data:   res,
			Source: "prometheus",
			Cached: false,
		}, nil

	case QueryModeLabels:
		res, err := e.client.Labels(ctx)
		if err != nil {
			return nil, err
		}

		return &query.QueryResult{
			Data:   res,
			Source: "prometheus",
			Cached: false,
		}, nil

	default:
		res, err := e.client.Query(ctx, q.Expr)
		if err != nil {
			return nil, err
		}

		return &query.QueryResult{
			Data:   res,
			Source: "prometheus",
			Cached: false,
		}, nil
	}
}