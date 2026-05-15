package query

import "context"

type QueryIsolator interface {
	Isolate(ctx context.Context, q *Query) error
}

type NoopIsolator struct{}

func (i *NoopIsolator) Isolate(ctx context.Context, q *Query) error {
	return nil
}