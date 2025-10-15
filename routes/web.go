package routes

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/config"
	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/http/diagnostics"
	notificationshttp "github.com/example/Yamato-Go-Gin-API/internal/http/notifications"
	taskhttp "github.com/example/Yamato-Go-Gin-API/internal/http/tasks"
	"github.com/example/Yamato-Go-Gin-API/internal/httpserver"
	"github.com/example/Yamato-Go-Gin-API/internal/middleware"
	"github.com/example/Yamato-Go-Gin-API/internal/observability"
	memoryplatform "github.com/example/Yamato-Go-Gin-API/internal/platform/memory"
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
	diagHandler := diagnostics.NewHandler("Larago API")

	// 3.- Define a root route returning a welcome message.
	router.GET("/", func(ctx *gin.Context) {
		// 1.- Respond with a plain text message similar to Laravel's welcome route.
		ctx.String(http.StatusOK, "Welcome to the Larago API")
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

	// 8.- Bootstrap in-memory platform services for local development flows.
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "development-jwt-secret"
	}
	jwtIssuer := os.Getenv("JWT_ISSUER")
	if jwtIssuer == "" {
		jwtIssuer = "yamato"
	}

	redis := memoryplatform.NewRedis()
	authSvc, err := internalauth.NewService(config.JWTConfig{
		Secret:            jwtSecret,
		Issuer:            jwtIssuer,
		AccessExpiration:  15 * time.Minute,
		RefreshExpiration: 24 * time.Hour,
	}, redis)
	if err != nil {
		panic(err)
	}

	userStore := memoryplatform.NewUserStore()
	verificationSvc := memoryplatform.NewVerificationService(userStore, jwtSecret, time.Minute)
	authHandler := authhttp.NewHandler(authSvc, userStore, verificationSvc)
	authMiddleware := middleware.Authentication(authSvc, userStore)
	httpserver.RegisterAuthRoutes(router, authHandler, authMiddleware)

	notificationSvc := memoryplatform.NewNotificationService(memoryplatform.DefaultNotifications())
	notificationHandler := notificationshttp.NewHandler(notificationSvc)

	taskSvc := memoryplatform.NewTaskService(memoryplatform.DefaultTasks())
	taskHandler := taskhttp.NewHandler(taskSvc)

	// 9.- Provide an unauthenticated tasks endpoint consumed by the Next.js frontend.
	api.GET("/tasks", taskHandler.List)

	// 10.- Continue exposing authenticated task and notification endpoints under /v1.
	protected := router.Group("/v1")
	protected.Use(authMiddleware)
	protected.GET("/tasks", taskHandler.List)

	// 11.- Surface notification management endpoints alongside tasks for the dashboard.
	notificationsGroup := protected.Group("/notifications")
	notificationsGroup.GET("", notificationHandler.List)
	notificationsGroup.PATCH(":id", notificationHandler.MarkRead)
}
