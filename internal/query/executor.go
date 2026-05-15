package query

import "context"

type QueryExecutor interface {
	Execute(ctx context.Context, q *Query) (*QueryResult, error)
}