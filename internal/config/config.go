package config

import (
	"context"
	"os"
	"strconv"
	"sync"
)

// Config holds application configuration from environment.
type Config struct {
	HTTPPort        string
	DatabaseURL     string
	DBPoolSize      int
	RedisURL        string
	RedisPoolSize   int
	CacheTTL        int // seconds
	KafkaBrokers    string
	KafkaTopic      string
	KafkaPartitions int
	WorkerPoolSize  int
	JWTSecret       string
}

var (
	cfg     *Config
	cfgOnce sync.Once
)

// Get returns the application config (loads once from env).
func Get() *Config {
	cfgOnce.Do(func() {
		cfg = &Config{
			HTTPPort:        getEnv("HTTP_PORT", "8080"),
			DatabaseURL:     getEnv("DATABASE_URL", ""),
			DBPoolSize:      getIntEnv("DB_POOL_SIZE", 5000),
			RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
			RedisPoolSize:   getIntEnv("REDIS_POOL_SIZE", 5000),
			CacheTTL:        getIntEnv("CACHE_TTL_SEC", 300),
			KafkaBrokers:    getEnv("KAFKA_BROKERS", "localhost:9092"),
			KafkaTopic:      getEnv("KAFKA_TODO_TOPIC", "todo-commands"),
			KafkaPartitions: getIntEnv("KAFKA_PARTITIONS", 32),
			WorkerPoolSize:  getIntEnv("WORKER_POOL_SIZE", 128),
			JWTSecret:       getEnv("JWT_SECRET", ""),
		}
	})
	return cfg
}

// GetJWTSecret returns JWT secret from config (for middleware that only has context).
func GetJWTSecret(ctx context.Context) string {
	return Get().JWTSecret
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getIntEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}
