package traces

import (
	"context"
	"testing"
	"time"

	"observability-platform/internal/query"
)

type noopCache struct{}

func (c *noopCache) Get(ctx context.Context, q *query.Query) (*query.QueryResult, bool) {
	return nil, false
}

func (c *noopCache) Set(ctx context.Context, q *query.Query, result *query.QueryResult, ttl time.Duration) {}

func TestTracesPoCReusePipeline(t *testing.T) {
	pipeline := query.NewQueryPipeline(
		&query.DefaultValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		&noopCache{},
		NewTempoExecutor(),
		&query.DefaultFormatter{},
	)

	service := NewTracesService(pipeline)

	res, err := service.Query(context.Background(), &query.Query{
		Expr:   `trace_id=abc`,
		Tenant: "t1",
	})

	if err != nil {
		t.Fatal(err)
	}

	if res.Source != "tempo-poc" {
		t.Fatalf("expected tempo-poc, got %s", res.Source)
	}
}