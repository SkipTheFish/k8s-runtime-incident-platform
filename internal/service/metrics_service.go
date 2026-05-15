package service

import (
	"context"
	"fmt"
	"time"

	"observability-platform/internal/cache"
	"observability-platform/internal/datasource/prometheus"
	"observability-platform/internal/resilience"
)

type MetricsService struct {
	client  *prometheus.Client
	cache   *cache.RedisCache
	breaker *resilience.CircuitBreaker
}

func NewMetricsService(c *prometheus.Client, cache *cache.RedisCache) *MetricsService {
	return &MetricsService{
		client:  c,
		cache:   cache,
		breaker: &resilience.CircuitBreaker{},
	}
}

func (s *MetricsService) Query(ctx context.Context, tenant string, query string) (any, error) {

	cacheKey := fmt.Sprintf("metrics:%s:%s", tenant, query)

	// 1. 读缓存
	var cached any
	if s.cache.Get(ctx, cacheKey, &cached) {
		return cached, nil
	}

	var result *prometheus.QueryResponse

	// 2. 熔断 + 重试
	err := s.breaker.Execute(func() error {
		return resilience.Retry(3, time.Millisecond*200, func() error {
			var err error
			result, err = s.client.Query(ctx, query)
			return err
		})
	})

	if err != nil {
		// fallback：返回缓存
		if cached != nil {
			return cached, nil
		}
		return nil, err
	}

	// 3. 写缓存
	s.cache.Set(ctx, cacheKey, result, 30*time.Second)

	return result, nil
}