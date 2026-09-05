package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/arjun118/autocomplete/internal/suggest"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string) (*RedisCache, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr, // e.g. "localhost:6379"
		PoolSize:     50,   // connection pool size
		MinIdleConns: 10,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &RedisCache{client: rdb}, nil
}

func (c *RedisCache) Get(prefix string) ([]suggest.Reco, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	val, err := c.client.Get(ctx, prefix).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false // cache miss
		}
		return nil, false
	}
	var recos []suggest.Reco
	if err := json.Unmarshal(val, &recos); err != nil {
		return nil, false
	}
	return recos, true
}

func (c *RedisCache) Set(prefix string, suggestions []suggest.Reco) error {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	data, err := json.Marshal(suggestions)
	if err != nil {
		return err
	}
	// 0 expiration -> rely on redis allkeys-lfu eviction
	return c.client.Set(ctx, prefix, data, 0).Err()
}

func (c *RedisCache) SetPipelined(ctx context.Context, pipe redis.Pipeliner, prefix string, suggestions []suggest.Reco) error {
	data, err := json.Marshal(suggestions)
	if err != nil {
		return err
	}
	pipe.Set(ctx, prefix, data, 0)
	return nil
}
