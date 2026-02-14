package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"million-rps/internal/config"
	"million-rps/internal/models"
	"million-rps/pkg/logger"
)

const todosCacheKey = "todos:all"

var (
	client *redis.Client
	once   sync.Once
)

// Client returns the global Redis client (initialized on first use).
func Client(ctx context.Context) *redis.Client {
	once.Do(func() {
		cfg := config.Get()
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			logger.Error(ctx, "Invalid REDIS_URL", "error", err, "url", cfg.RedisURL)
			return
		}
		opts.PoolSize = cfg.RedisPoolSize
		client = redis.NewClient(opts)
		if err := client.Ping(ctx).Err(); err != nil {
			logger.Error(ctx, "Redis ping failed", "error", err)
			return
		}
		logger.Info(ctx, "Redis client initialized", "pool_size", cfg.RedisPoolSize)
	})
	return client
}

// GetTodos reads the todos list from Redis. Returns (nil, false) on miss or error.
func GetTodos(ctx context.Context) ([]models.Todo, bool) {
	c := Client(ctx)
	if c == nil {
		return nil, false
	}
	b, err := c.Get(ctx, todosCacheKey).Bytes()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		logger.Debug(ctx, "Redis get todos failed", "error", err)
		return nil, false
	}
	var todos []models.Todo
	if err := json.Unmarshal(b, &todos); err != nil {
		logger.Debug(ctx, "Redis unmarshal todos failed", "error", err)
		return nil, false
	}
	return todos, true
}

// SetTodos writes the todos list to Redis with configured TTL.
func SetTodos(ctx context.Context, todos []models.Todo) {
	c := Client(ctx)
	if c == nil {
		return
	}
	b, err := json.Marshal(todos)
	if err != nil {
		logger.Debug(ctx, "Marshal todos for cache failed", "error", err)
		return
	}
	cfg := config.Get()
	ttl := time.Duration(cfg.CacheTTL) * time.Second
	if err := c.Set(ctx, todosCacheKey, b, ttl).Err(); err != nil {
		logger.Debug(ctx, "Redis set todos failed", "error", err)
	}
}

// InvalidateTodos deletes the todos cache key so the next read goes to DB.
func InvalidateTodos(ctx context.Context) {
	c := Client(ctx)
	if c == nil {
		return
	}
	if err := c.Del(ctx, todosCacheKey).Err(); err != nil {
		logger.Debug(ctx, "Redis invalidate todos failed", "error", err)
	}
}

// CacheKey returns a stable key for a single todo (optional; we use list key for GetTodos).
func CacheKey(id string) string {
	return fmt.Sprintf("todo:%s", id)
}
