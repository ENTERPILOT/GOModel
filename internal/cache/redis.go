package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// DefaultRedisKey is the default key used to store the model cache in Redis.
	DefaultRedisKey = "gomodel:models"

	// DefaultRedisTTL is the default time-to-live for cached data (24 hours).
	// This ensures stale data eventually expires if the application stops updating.
	DefaultRedisTTL = 24 * time.Hour
)

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	// URL is the Redis connection URL (e.g., "redis://localhost:6379" or "redis://:password@host:6379/0")
	URL string

	// Key is the Redis key to store the model cache (defaults to "gomodel:models")
	Key string

	// TTL is the time-to-live for cached data (defaults to 24 hours)
	TTL time.Duration
}

// RedisCache implements Cache using Redis for distributed storage.
// This is suitable for multi-instance deployments behind a load balancer.
type RedisCache struct {
	client *redis.Client
	key    string
	ttl    time.Duration
}

// NewRedisCache creates a new Redis-based cache.
func NewRedisCache(cfg RedisConfig) (*RedisCache, error) {
	opts, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis URL: %w", err)
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	key := cfg.Key
	if key == "" {
		key = DefaultRedisKey
	}

	ttl := cfg.TTL
	if ttl == 0 {
		ttl = DefaultRedisTTL
	}

	slog.Info("redis cache connected", "key", key, "ttl", ttl)

	return &RedisCache{
		client: client,
		key:    key,
		ttl:    ttl,
	}, nil
}

// Get retrieves the model cache from Redis.
func (c *RedisCache) Get(ctx context.Context) (*ModelCache, error) {
	data, err := c.client.Get(ctx, c.key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No cache yet, not an error
		}
		return nil, fmt.Errorf("failed to get cache from redis: %w", err)
	}

	var cache ModelCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache from redis: %w", err)
	}

	return &cache, nil
}

// Set stores the model cache in Redis.
func (c *RedisCache) Set(ctx context.Context, cache *ModelCache) error {
	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := c.client.Set(ctx, c.key, data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache in redis: %w", err)
	}

	return nil
}

// Close closes the Redis connection.
func (c *RedisCache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
