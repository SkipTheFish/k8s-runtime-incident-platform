package query

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockCache struct {
	data map[string]*QueryResult
}

func newMockCache() *mockCache {
	return &mockCache{
		data: map[string]*QueryResult{},
	}
}

func (m *mockCache) Get(ctx context.Context, q *Query) (*QueryResult, bool) {
	v, ok := m.data[q.Expr]
	if ok {
		v.Cached = true
	}
	return v, ok
}

func (m *mockCache) Set(ctx context.Context, q *Query, result *QueryResult, ttl time.Duration) {
	m.data[q.Expr] = result
}

type mockExecutor struct {
	called bool
	err    error
}

func (m *mockExecutor) Execute(ctx context.Context, q *Query) (*QueryResult, error) {
	m.called = true

	if m.err != nil {
		return nil, m.err
	}

	return &QueryResult{
		Data:   "ok",
		Source: "mock",
		Cached: false,
	}, nil
}

func TestPipelineSuccess(t *testing.T) {
	cache := newMockCache()
	executor := &mockExecutor{}

	pipeline := NewQueryPipeline(
		&DefaultValidator{},
		&NoopParser{},
		&NoopIsolator{},
		cache,
		executor,
		&DefaultFormatter{},
	)

	res, err := pipeline.Run(context.Background(), &Query{
		Type:   QueryTypeMetrics,
		Mode:   QueryModeInstant,
		Expr:   "up",
		Tenant: "t1",
	})

	if err != nil {
		t.Fatal(err)
	}

	if res.Data != "ok" {
		t.Fatalf("expected ok, got %v", res.Data)
	}

	if !executor.called {
		t.Fatal("expected executor called")
	}
}

func TestPipelineCacheHit(t *testing.T) {
	cache := newMockCache()
	cache.data["up"] = &QueryResult{
		Data:   "cached",
		Source: "mock",
	}

	executor := &mockExecutor{}

	pipeline := NewQueryPipeline(
		&DefaultValidator{},
		&NoopParser{},
		&NoopIsolator{},
		cache,
		executor,
		&DefaultFormatter{},
	)

	res, err := pipeline.Run(context.Background(), &Query{
		Type:   QueryTypeMetrics,
		Mode:   QueryModeInstant,
		Expr:   "up",
		Tenant: "t1",
	})

	if err != nil {
		t.Fatal(err)
	}

	if res.Data != "cached" {
		t.Fatalf("expected cached, got %v", res.Data)
	}

	if !res.Cached {
		t.Fatal("expected cached=true")
	}

	if executor.called {
		t.Fatal("executor should not be called on cache hit")
	}
}

func TestPipelineValidateFailed(t *testing.T) {
	cache := newMockCache()
	executor := &mockExecutor{}

	pipeline := NewQueryPipeline(
		&DefaultValidator{},
		&NoopParser{},
		&NoopIsolator{},
		cache,
		executor,
		&DefaultFormatter{},
	)

	_, err := pipeline.Run(context.Background(), &Query{
		Type:   QueryTypeMetrics,
		Mode:   QueryModeInstant,
		Expr:   "",
		Tenant: "t1",
	})

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPipelineExecutorFailed(t *testing.T) {
	cache := newMockCache()
	executor := &mockExecutor{
		err: errors.New("executor failed"),
	}

	pipeline := NewQueryPipeline(
		&DefaultValidator{},
		&NoopParser{},
		&NoopIsolator{},
		cache,
		executor,
		&DefaultFormatter{},
	)

	_, err := pipeline.Run(context.Background(), &Query{
		Type:   QueryTypeMetrics,
		Mode:   QueryModeInstant,
		Expr:   "up",
		Tenant: "t1",
	})

	if err == nil {
		t.Fatal("expected executor error")
	}
}