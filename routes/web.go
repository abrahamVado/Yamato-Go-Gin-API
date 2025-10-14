package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/http/diagnostics"
)

// RegisterRoutes maps HTTP endpoints to their handlers.
func RegisterRoutes(router *gin.Engine) {
	// 1.- Prepare diagnostics handlers responsible for service monitoring.
	diagHandler := diagnostics.NewHandler("Yamato API")

	// 2.- Define a root route returning a welcome message.
	router.GET("/", func(ctx *gin.Context) {
		// 1.- Respond with a plain text message similar to Laravel's welcome route.
		ctx.String(http.StatusOK, "Welcome to the Yamato Go Gin API")
	})

	// 3.- Group API routes under the /api prefix.
	api := router.Group("/api")

	// 4.- Register a health check endpoint for monitoring purposes.
	api.GET("/health", diagHandler.Health)

	// 5.- Expose a readiness probe used by orchestrators to gate traffic.
	router.GET("/ready", diagHandler.Ready)
}
