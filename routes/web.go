package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/app/http/controllers"
)

// RegisterRoutes maps HTTP endpoints to their handlers.
func RegisterRoutes(router *gin.Engine) {
	// 1.- Create controller instances that will serve route handlers.
	healthController := controllers.NewHealthController()

	// 2.- Define a root route returning a welcome message.
	router.GET("/", func(ctx *gin.Context) {
		// 1.- Respond with a plain text message similar to Laravel's welcome route.
		ctx.String(http.StatusOK, "Welcome to the Yamato Go Gin API")
	})

	// 3.- Group API routes under the /api prefix.
	api := router.Group("/api")

	// 4.- Register a health check endpoint for monitoring purposes.
	api.GET("/health", healthController.Status)
}
