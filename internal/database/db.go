package database

import (
	"context"
	"database/sql"
	"sync"

	"million-rps/internal/config"
	"million-rps/pkg/logger"

	_ "github.com/lib/pq"
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

// MigrateOrCreateSchema creates the todos table and indexes if they do not exist.
func MigrateOrCreateSchema(ctx context.Context) error {
	db := DB(ctx)
	if db == nil {
		return nil
	}
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS todos (
			id          TEXT PRIMARY KEY,
			title       TEXT NOT NULL,
			description TEXT,
			completed   BOOLEAN NOT NULL DEFAULT FALSE,
			user_id     TEXT NOT NULL,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_todos_user_id ON todos(user_id);
		CREATE INDEX IF NOT EXISTS idx_todos_created_at ON todos(created_at DESC);
	`)
	if err != nil {
		return err
	}
	logger.Info(ctx, "Schema ensured (todos table)")
	return nil
}
