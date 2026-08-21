package logs

import (
	"context"
	"net/http"
	"net/http/httptest"
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

func TestLogsPoCReusePipeline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	pipeline := query.NewQueryPipeline(
		&query.DefaultValidator{},
		&query.NoopParser{},
		&query.NoopIsolator{},
		&noopCache{},
		NewLokiExecutor(NewLokiClient(server.URL)),
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

	if res.Source != "loki" {
		t.Fatalf("expected loki, got %s", res.Source)
	}
}
