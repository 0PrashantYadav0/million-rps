package routes

import (
	"million-rps/internal/controller"
	"million-rps/internal/middleware"

	"github.com/gin-gonic/gin"
)

func Router() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Public: no auth
	router.GET("/todos", controller.GetTodos)

	// Protected: JWT required
	api := router.Group("")
	api.Use(middleware.AuthMiddleware())
	{
		api.POST("/todos", controller.CreateTodo)
		api.PUT("/todos/:id", controller.UpdateTodo)
		api.DELETE("/todos/:id", controller.DeleteTodo)
	}

	return router
}
