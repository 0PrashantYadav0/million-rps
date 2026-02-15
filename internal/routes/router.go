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

	// Health for load balancers and K8s probes
	router.GET("/health", controller.Health)
	router.GET("/ready", controller.Ready)

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
