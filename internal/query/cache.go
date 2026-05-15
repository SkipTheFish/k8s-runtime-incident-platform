package query

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	appcache "observability-platform/internal/cache"
)

type QueryCache interface {
	Get(ctx context.Context, q *Query) (*QueryResult, bool)
	Set(ctx context.Context, q *Query, result *QueryResult, ttl time.Duration)
}

type RedisQueryCache struct {
	cache *appcache.RedisCache
}

func NewRedisQueryCache(cache *appcache.RedisCache) *RedisQueryCache {
	return &RedisQueryCache{cache: cache}
}

func (c *RedisQueryCache) Get(ctx context.Context, q *Query) (*QueryResult, bool) {
	var result QueryResult
	ok := c.cache.Get(ctx, c.key(q), &result)
	if !ok {
		return nil, false
	}

	result.Cached = true
	return &result, true
}

func (c *RedisQueryCache) Set(ctx context.Context, q *Query, result *QueryResult, ttl time.Duration) {
	c.cache.Set(ctx, c.key(q), result, ttl)
}

func (c *RedisQueryCache) key(q *Query) string {
	raw := fmt.Sprintf(
		"%s:%s:%s:%s:%d:%d:%s",
		q.Tenant,
		q.Type,
		q.Mode,
		q.Expr,
		q.StartTime.Unix(),
		q.EndTime.Unix(),
		q.Step.String(),
	)

	sum := sha256.Sum256([]byte(raw))
	return "query:" + hex.EncodeToString(sum[:])
}