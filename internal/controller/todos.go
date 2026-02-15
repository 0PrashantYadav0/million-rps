package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"million-rps/internal/cache"
	"million-rps/internal/database"
	"million-rps/internal/models"
	"million-rps/internal/queue"
	"million-rps/internal/repository"
	"million-rps/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

var getTodosGroup singleflight.Group

// GetTodos is the public handler: returns todos as JSON (cache-first as raw bytes for max throughput). Supports ?limit=N for pagination (smaller payload = higher RPS).
func GetTodos(c *gin.Context) {
	ctx := c.Request.Context()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "0"))

	if limit > 0 {
		if b, ok := cache.GetRawTodosLimit(ctx, limit); ok {
			c.Data(http.StatusOK, "application/json", b)
			return
		}
		key := "todos:limit:" + strconv.Itoa(limit)
		v, err, _ := getTodosGroup.Do(key, func() (interface{}, error) {
			todos, err := repository.GetRange(context.Background(), limit, 0)
			if err != nil {
				return nil, err
			}
			return json.Marshal(todos)
		})
		if err != nil {
			if ctx.Err() != nil || isContextErr(err) {
				return
			}
			logger.Error(ctx, "GetTodos repository failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get todos"})
			return
		}
		b := v.([]byte)
		c.Data(http.StatusOK, "application/json", b)
		go cache.SetRawTodosLimitAsync(limit, b)
		return
	}

	if b, ok := cache.GetRawTodos(ctx); ok {
		c.Data(http.StatusOK, "application/json", b)
		return
	}
	v, err, _ := getTodosGroup.Do("todos", func() (interface{}, error) {
		todos, err := repository.GetAll(context.Background())
		if err != nil {
			return nil, err
		}
		return json.Marshal(todos)
	})
	if err != nil {
		if ctx.Err() != nil || isContextErr(err) {
			return
		}
		logger.Error(ctx, "GetTodos repository failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get todos"})
		return
	}
	b := v.([]byte)
	c.Data(http.StatusOK, "application/json", b)
	go cache.SetRawTodosAsync(b)
}

func isContextErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

// Health returns 200 if the process is alive. Used by load balancers.
func Health(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

// Ready returns 200 if DB and Redis are reachable. Used by K8s readiness probes.
func Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if cache.Client(ctx) == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis unavailable"})
		return
	}
	db := database.DB(ctx)
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "database unavailable"})
		return
	}
	if err := db.PingContext(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "database ping failed"})
		return
	}
	c.String(http.StatusOK, "OK")
}

// CreateTodo (auth): validates body, publishes to Kafka, returns 202 Accepted.
func CreateTodo(c *gin.Context) {
	ctx := c.Request.Context()
	userID, _ := c.Get("user")
	uid, _ := userID.(string)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	var body struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	id := uuid.New().String()
	cmd := &models.TodoCommand{
		Action:      "create",
		ID:          id,
		Title:       body.Title,
		Description: body.Description,
		UserID:      uid,
		RequestedAt: time.Now(),
	}
	if err := queue.PublishTodoCommand(ctx, cmd); err != nil {
		logger.Error(ctx, "CreateTodo publish failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Request queued failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"id": id, "message": "Todo creation queued"})
}

// UpdateTodo (auth): publishes update command to Kafka, returns 202.
func UpdateTodo(c *gin.Context) {
	ctx := c.Request.Context()
	userID, _ := c.Get("user")
	uid, _ := userID.(string)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing todo id"})
		return
	}
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Completed   *bool  `json:"completed"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	cmd := &models.TodoCommand{
		Action:      "update",
		ID:          id,
		Title:       body.Title,
		Description: body.Description,
		Completed:   body.Completed,
		UserID:      uid,
		RequestedAt: time.Now(),
	}
	if err := queue.PublishTodoCommand(ctx, cmd); err != nil {
		logger.Error(ctx, "UpdateTodo publish failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Request queued failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"id": id, "message": "Todo update queued"})
}

// DeleteTodo (auth): publishes delete command to Kafka, returns 202.
func DeleteTodo(c *gin.Context) {
	ctx := c.Request.Context()
	userID, _ := c.Get("user")
	uid, _ := userID.(string)
	if uid == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing todo id"})
		return
	}
	cmd := &models.TodoCommand{
		Action:      "delete",
		ID:          id,
		UserID:      uid,
		RequestedAt: time.Now(),
	}
	if err := queue.PublishTodoCommand(ctx, cmd); err != nil {
		logger.Error(ctx, "DeleteTodo publish failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Request queued failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"id": id, "message": "Todo deletion queued"})
}
