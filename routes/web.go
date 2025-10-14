package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/http/diagnostics"
	"github.com/example/Yamato-Go-Gin-API/internal/observability"
)

// 1.- options captures optional dependencies used while registering routes.
type options struct {
	metrics *observability.Metrics
}

// 1.- Option customizes route registration with optional dependencies.
type Option func(*options)

// 1.- WithMetrics injects the Prometheus metrics handler.
func WithMetrics(metrics *observability.Metrics) Option {
	return func(opts *options) {
		opts.metrics = metrics
	}
}

// RegisterRoutes maps HTTP endpoints to their handlers.
func RegisterRoutes(router *gin.Engine, opts ...Option) {
	// 1.- Collect optional dependencies passed by the caller.
	configured := options{}
	for _, opt := range opts {
		opt(&configured)
	}

	// 2.- Prepare diagnostics handlers responsible for service monitoring.
	diagHandler := diagnostics.NewHandler("Yamato API")

	// 3.- Define a root route returning a welcome message.
	router.GET("/", func(ctx *gin.Context) {
		// 1.- Respond with a plain text message similar to Laravel's welcome route.
		ctx.String(http.StatusOK, "Welcome to the Yamato Go Gin API")
	})

	// 4.- Group API routes under the /api prefix.
	api := router.Group("/api")

	// 5.- Register a health check endpoint for monitoring purposes.
	api.GET("/health", diagHandler.Health)

	// 6.- Expose a readiness probe used by orchestrators to gate traffic.
	router.GET("/ready", diagHandler.Ready)

	// 7.- Publish the Prometheus metrics endpoint when instrumentation is configured.
	if configured.metrics != nil {
		router.GET("/metrics", gin.WrapH(configured.metrics.Handler()))
	}
}
