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

func (c *noopCache) Set(ctx context.Context, q *query.Query, result *query.QueryResult, ttl time.Duration) {
}

func TestTracesPoCReusePipeline(t *testing.T) {
	pipeline := query.NewQueryPipeline(
		&query.DefaultValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		&noopCache{},
		NewTempoExecutor(NewTempoClient("http://unused.example")),
		&query.DefaultFormatter{},
	)

	service := NewTracesService(pipeline)

	res, err := service.Search(context.Background(), "t1", "api")

	if err != nil {
		t.Fatal(err)
	}

	if res.Source != "tempo" {
		t.Fatalf("expected tempo, got %s", res.Source)
	}
}
