package main

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"million-rps/internal/cache"
	"million-rps/internal/config"
	"million-rps/internal/database"
	"million-rps/internal/queue"
	"million-rps/internal/routes"
	"million-rps/internal/worker"
	"million-rps/pkg/logger"
)

func main() {
	loadEnvFile(".env")

	ctx := context.Background()
	config.Get()

	// Initialize DB pool (required for workers and cache miss path)
	db := database.InitDB(ctx)
	if db == nil {
		logger.Error(ctx, "Database not available; exiting")
		os.Exit(1)
	}
	if err := database.MigrateOrCreateSchema(ctx); err != nil {
		logger.Error(ctx, "Schema migration failed", "error", err)
		os.Exit(1)
	}

	// Pre-warm Redis (optional; cache works lazily)
	cache.Client(ctx)

	// Pre-warm Kafka producer and ensure topic exists
	queue.Producer(ctx)
	queue.EnsureTopic(ctx)

	// Start worker pool in background (consumes Kafka, writes to DB, invalidates cache)
	go worker.Run(ctx)

	server := &http.Server{
		Addr:         ":" + config.Get().HTTPPort,
		Handler:      routes.Router(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	go func() {
		logger.Info(ctx, "HTTP server listening", "port", config.Get().HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(ctx, "Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info(ctx, "Shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "Server shutdown error", "error", err)
	}
	logger.Info(ctx, "Server stopped")
}

// loadEnvFile reads a .env file and sets env vars (only if not already set).
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
			val = strings.Trim(val, `"`)
		} else if strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'") {
			val = strings.Trim(val, "'")
		}
		if key != "" && os.Getenv(key) == "" {
			_ = os.Setenv(key, val)
		}
	}
}
