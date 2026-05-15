package logs

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

func TestLogsPoCReusePipeline(t *testing.T) {
	pipeline := query.NewQueryPipeline(
		&query.DefaultValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		&noopCache{},
		NewLokiExecutor(),
		&query.DefaultFormatter{},
	)

	service := NewLogsService(pipeline)

	res, err := service.Query(context.Background(), &query.Query{
		Expr:   `{job="api"}`,
		Tenant: "t1",
	})

	if err != nil {
		t.Fatal(err)
	}

	if res.Source != "loki-poc" {
		t.Fatalf("expected loki-poc, got %s", res.Source)
	}
}