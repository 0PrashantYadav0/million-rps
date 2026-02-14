package database

import (
	"context"
	"database/sql"
	"sync"

	_ "github.com/lib/pq"
	"million-rps/internal/config"
	"million-rps/pkg/logger"
)

var (
	pool *sql.DB
	once sync.Once
)

// DB returns the global database connection pool (initialized on first use).
func DB(ctx context.Context) *sql.DB {
	once.Do(func() {
		cfg := config.Get()
		if cfg.DatabaseURL == "" {
			logger.Error(ctx, "DATABASE_URL is not set")
			return
		}
		db, err := sql.Open("postgres", cfg.DatabaseURL)
		if err != nil {
			logger.Error(ctx, "Failed to open database", "error", err)
			return
		}
		db.SetMaxOpenConns(cfg.DBPoolSize)
		db.SetMaxIdleConns(cfg.DBPoolSize / 2)
		pool = db
		logger.Info(ctx, "Database pool initialized", "max_open", cfg.DBPoolSize)
	})
	return pool
}

// InitDB initializes the DB pool and returns it (for backward compatibility / main).
func InitDB(ctx context.Context) *sql.DB {
	return DB(ctx)
}
