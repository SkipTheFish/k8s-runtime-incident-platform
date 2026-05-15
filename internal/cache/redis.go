package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedis(addr string) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{
			Addr: addr,
		}),
	}
}

func (r *RedisCache) Get(ctx context.Context, key string, dest any) bool {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	_ = json.Unmarshal([]byte(val), dest)
	return true
}

func (r *RedisCache) Set(ctx context.Context, key string, value any, ttl time.Duration) {
	b, _ := json.Marshal(value)
	r.client.Set(ctx, key, b, ttl)
}