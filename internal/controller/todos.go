package controller

import (
	"net/http"
	"time"

	"million-rps/internal/cache"
	"million-rps/internal/models"
	"million-rps/internal/queue"
	"million-rps/internal/repository"
	"million-rps/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetTodos is the public handler: returns all todos (cache-first, then DB).
func GetTodos(c *gin.Context) {
	ctx := c.Request.Context()
	if todos, ok := cache.GetTodos(ctx); ok {
		c.JSON(http.StatusOK, todos)
		return
	}
	todos, err := repository.GetAll(ctx)
	if err != nil {
		logger.Error(ctx, "GetTodos repository failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get todos"})
		return
	}
	cache.SetTodos(ctx, todos)
	c.JSON(http.StatusOK, todos)
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
