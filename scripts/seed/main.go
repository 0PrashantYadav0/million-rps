// Seed adds 10,000 todos to the database. Run from project root: go run ./scripts/seed
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"million-rps/internal/database"

	"github.com/google/uuid"
)

func main() {
	loadEnvFile(".env")

	ctx := context.Background()
	db := database.InitDB(ctx)
	if db == nil {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set or DB connection failed")
		os.Exit(1)
	}

	if err := database.MigrateOrCreateSchema(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "Schema failed:", err)
		os.Exit(1)
	}

	const total = 10_000
	const batchSize = 500
	userID := "seed-user"
	start := time.Now()

	for batch := 0; batch < total/batchSize; batch++ {
		args := make([]interface{}, 0, batchSize*5)
		placeholders := make([]string, 0, batchSize)
		for i := 0; i < batchSize; i++ {
			n := batch*batchSize + i + 1
			placeholders = append(placeholders, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,NOW(),NOW())",
				5*i+1, 5*i+2, 5*i+3, 5*i+4, 5*i+5))
			args = append(args,
				uuid.New().String(),
				fmt.Sprintf("Todo %d", n),
				fmt.Sprintf("Description for todo %d", n),
				false,
				userID,
			)
		}
		q := `INSERT INTO todos (id, title, description, completed, user_id, created_at, updated_at) VALUES ` +
			strings.Join(placeholders, ",")
		_, err := db.ExecContext(ctx, q, args...)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Insert failed:", err)
			os.Exit(1)
		}
		fmt.Printf("\rInserted %d / %d", (batch+1)*batchSize, total)
	}

	fmt.Printf("\nDone: %d todos in %v\n", total, time.Since(start))
}

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
