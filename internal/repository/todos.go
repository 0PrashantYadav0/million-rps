package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"million-rps/internal/database"
	"million-rps/internal/models"
	"million-rps/pkg/logger"
)

// GetAll returns all todos from the database.
func GetAll(ctx context.Context) ([]models.Todo, error) {
	db := database.DB(ctx)
	if db == nil {
		return nil, sql.ErrNoRows
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, title, description, completed, user_id, created_at, updated_at FROM todos ORDER BY created_at DESC`)
	if err != nil {
		logger.Error(ctx, "Repository GetTodos failed", "error", err)
		return nil, err
	}
	defer rows.Close()
	var todos []models.Todo
	for rows.Next() {
		var t models.Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Completed, &t.UserID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			logger.Error(ctx, "Repository scan todo failed", "error", err)
			return nil, err
		}
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// Create inserts a new todo.
func Create(ctx context.Context, todo *models.Todo) error {
	db := database.DB(ctx)
	if db == nil {
		return sql.ErrNoRows
	}
	if todo.ID == "" {
		todo.ID = uuid.New().String()
	}
	now := time.Now()
	todo.CreatedAt = now
	todo.UpdatedAt = now
	_, err := db.ExecContext(ctx,
		`INSERT INTO todos (id, title, description, completed, user_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		todo.ID, todo.Title, todo.Description, todo.Completed, todo.UserID, todo.CreatedAt, todo.UpdatedAt)
	if err != nil {
		logger.Error(ctx, "Repository Create failed", "error", err)
		return err
	}
	return nil
}

// Update updates an existing todo by ID (and user_id for safety).
func Update(ctx context.Context, id, userID, title, description string, completed *bool) error {
	db := database.DB(ctx)
	if db == nil {
		return sql.ErrNoRows
	}
	now := time.Now()
	if title != "" || description != "" || completed != nil {
		_, err := db.ExecContext(ctx,
			`UPDATE todos SET title = COALESCE(NULLIF($1,''), title), description = COALESCE(NULLIF($2,''), description),
			 completed = COALESCE($3, completed), updated_at = $4 WHERE id = $5 AND user_id = $6`,
			title, description, completed, now, id, userID)
		if err != nil {
			logger.Error(ctx, "Repository Update failed", "error", err, "id", id)
			return err
		}
		return nil
	}
	return nil
}

// Delete removes a todo by ID and user_id.
func Delete(ctx context.Context, id, userID string) error {
	db := database.DB(ctx)
	if db == nil {
		return sql.ErrNoRows
	}
	_, err := db.ExecContext(ctx, `DELETE FROM todos WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		logger.Error(ctx, "Repository Delete failed", "error", err, "id", id)
		return err
	}
	return nil
}
