package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"million-rps/internal/config"
	"million-rps/internal/models"
	"million-rps/pkg/logger"

	"github.com/redis/go-redis/v9"
)

const (
	todosCacheKey = "todos:all"
	todosLimitPrefix = "todos:limit:"
)

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
		opts.MinIdleConns = opts.PoolSize / 4
		opts.ReadTimeout = 15 * time.Second
		opts.WriteTimeout = 10 * time.Second
		opts.PoolTimeout = 10 * time.Second
		client = redis.NewClient(opts)
		if err := client.Ping(ctx).Err(); err != nil {
			logger.Error(ctx, "Redis ping failed", "error", err)
			return
		}
		logger.Info(ctx, "Redis client initialized", "pool_size", cfg.RedisPoolSize)
	})
	return client
}

// getRaw returns cached bytes for key. Used for zero-copy response path.
func getRaw(ctx context.Context, key string) ([]byte, bool) {
	c := Client(ctx)
	if c == nil {
		return nil, false
	}
	b, err := c.Get(ctx, key).Bytes()
	if err == redis.Nil || err != nil {
		return nil, false
	}
	return b, true
}

func setRawAsync(key string, b []byte) {
	if len(b) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := Client(ctx)
	if c == nil {
		return
	}
	cfg := config.Get()
	ttl := time.Duration(cfg.CacheTTL) * time.Second
	_ = c.Set(ctx, key, b, ttl).Err()
}

// GetRawTodos returns the cached full list as raw JSON bytes (no unmarshal). Use on the hot path for max throughput.
func GetRawTodos(ctx context.Context) ([]byte, bool) {
	return getRaw(ctx, todosCacheKey)
}

// SetRawTodosAsync writes raw JSON bytes for full list to Redis in the background.
func SetRawTodosAsync(b []byte) {
	setRawAsync(todosCacheKey, b)
}

// GetRawTodosLimit returns cached raw JSON for first `limit` todos. Key is "todos:limit:N".
func GetRawTodosLimit(ctx context.Context, limit int) ([]byte, bool) {
	return getRaw(ctx, todosLimitPrefix+strconv.Itoa(limit))
}

// SetRawTodosLimitAsync caches raw JSON for first `limit` todos in the background.
func SetRawTodosLimitAsync(limit int, b []byte) {
	setRawAsync(todosLimitPrefix+strconv.Itoa(limit), b)
}

// GetTodos reads the todos list from Redis. Returns (nil, false) on miss or error.
func GetTodos(ctx context.Context) ([]models.Todo, bool) {
	b, ok := GetRawTodos(ctx)
	if !ok {
		return nil, false
	}
	var todos []models.Todo
	if err := json.Unmarshal(b, &todos); err != nil {
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
		return
	}
	cfg := config.Get()
	ttl := time.Duration(cfg.CacheTTL) * time.Second
	_ = c.Set(ctx, todosCacheKey, b, ttl).Err()
}

// SetTodosAsync writes the todos list to Redis in the background. Used by code that has []Todo.
func SetTodosAsync(todos []models.Todo) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	SetTodos(ctx, todos)
}

// InvalidateTodos deletes the todos cache key so the next read goes to DB.
func InvalidateTodos(ctx context.Context) {
	c := Client(ctx)
	if c == nil {
		return
	}
	_ = c.Del(ctx, todosCacheKey).Err()
}

// CacheKey returns a stable key for a single todo (optional; we use list key for GetTodos).
func CacheKey(id string) string {
	return fmt.Sprintf("todo:%s", id)
}
