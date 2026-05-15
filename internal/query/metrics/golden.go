package metrics

import (
	"context"
	"fmt"
	"time"

	"observability-platform/internal/query"
)

type GoldenSignals struct {
	Latency    *query.QueryResult `json:"latency"`
	ErrorRate  *query.QueryResult `json:"error_rate"`
	Throughput *query.QueryResult `json:"throughput"`
	Availability *query.QueryResult `json:"availability"`
}

func (s *MetricsService) GoldenSignals(ctx context.Context, tenant string, serviceName string) (*GoldenSignals, error) {
	now := time.Now()
	start := now.Add(-30 * time.Minute)

	build := func(expr string) *query.Query {
		return &query.Query{
			Type:      query.QueryTypeMetrics,
			Mode:      query.QueryModeRange,
			Expr:      expr,
			Tenant:    tenant,
			StartTime: start,
			EndTime:   now,
			Step:      30 * time.Second,
		}
	}

	latencyExpr := fmt.Sprintf(
		`histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket{service="%s"}[5m])) by (le))`,
		serviceName,
	)

	errorExpr := fmt.Sprintf(
		`sum(rate(http_requests_total{service="%s",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="%s"}[5m]))`,
		serviceName,
		serviceName,
	)

	throughputExpr := fmt.Sprintf(
		`sum(rate(http_requests_total{service="%s"}[5m]))`,
		serviceName,
	)

	availabilityExpr := fmt.Sprintf(
		`1 - (sum(rate(http_requests_total{service="%s",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="%s"}[5m])))`,
		serviceName,
		serviceName,
	)

	latency, err := s.Range(ctx, build(latencyExpr))
	if err != nil {
		return nil, err
	}

	errorRate, err := s.Range(ctx, build(errorExpr))
	if err != nil {
		return nil, err
	}

	throughput, err := s.Range(ctx, build(throughputExpr))
	if err != nil {
		return nil, err
	}

	availability, err := s.Range(ctx, build(availabilityExpr))
	if err != nil {
		return nil, err
	}

	return &GoldenSignals{
		Latency:      latency,
		ErrorRate:    errorRate,
		Throughput:   throughput,
		Availability: availability,
	}, nil
}