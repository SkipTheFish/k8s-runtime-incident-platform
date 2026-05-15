package query

import (
	"context"
	"time"
)

type QueryType string

const (
	QueryTypeMetrics QueryType = "metrics"
	QueryTypeLogs    QueryType = "logs"
	QueryTypeTraces  QueryType = "traces"
)

type QueryMode string

const (
	QueryModeInstant QueryMode = "instant"
	QueryModeRange   QueryMode = "range"
)

type Query struct {
	Type      QueryType
	Mode      QueryMode
	Expr      string
	AST       any
	Labels    map[string]string
	Tenant    string
	StartTime time.Time
	EndTime   time.Time
	Step      time.Duration
}

type QueryResult struct {
	Data    any      `json:"data"`
	Source  string   `json:"source"`
	Cached  bool     `json:"cached"`
	Warning []string `json:"warning,omitempty"`
}

type QueryService interface {
	Query(ctx context.Context, q *Query) (*QueryResult, error)
	Range(ctx context.Context, q *Query) (*QueryResult, error)
}