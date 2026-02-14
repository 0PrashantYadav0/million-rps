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
	KafkaBrokers    []string
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
			DatabaseURL:     os.Getenv("DATABASE_URL"),
			DBPoolSize:      getIntEnv("DB_POOL_SIZE", 100),
			RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
			RedisPoolSize:   getIntEnv("REDIS_POOL_SIZE", 500),
			CacheTTL:        getIntEnv("CACHE_TTL_SEC", 300),
			KafkaBrokers:    getSliceEnv("KAFKA_BROKERS", "localhost:9092"),
			KafkaTopic:      getEnv("KAFKA_TODO_TOPIC", "todo-commands"),
			KafkaPartitions: getIntEnv("KAFKA_PARTITIONS", 16),
			WorkerPoolSize:  getIntEnv("WORKER_POOL_SIZE", 32),
			JWTSecret:       os.Getenv("JWT_SECRET"),
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

func getSliceEnv(key, defaultVal string) []string {
	if v := os.Getenv(key); v != "" {
		var out []string
		for _, s := range splitTrim(v, ",") {
			if s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{defaultVal}
}

func splitTrim(s, sep string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			out = append(out, trim(s[start:i]))
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	out = append(out, trim(s[start:]))
	return out
}

func trim(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}
