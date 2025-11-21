package routes

import (
	"context"
	"database/sql"
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
	storagetasks "github.com/example/Yamato-Go-Gin-API/internal/storage/tasks"
	userstore "github.com/example/Yamato-Go-Gin-API/internal/storage/users"
	dbtooling "github.com/example/Yamato-Go-Gin-API/internal/tooling/db"

	appcontrollers "github.com/example/Yamato-Go-Gin-API/app/http/controllers"

	_ "github.com/lib/pq"
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

	// 1.1.- Open Postgres connection shared across HTTP services.
	dsn, err := dbtooling.BuildPostgresDSNFromEnv()
	if err != nil {
		panic(err)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		panic(err)
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

	// 8.- Bootstrap auth services and repositories.
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

	// Use Postgres-backed user store instead of in-memory.
	var userStore authhttp.UserStore = userstore.NewStore(db)

	verificationSvc := memoryplatform.NewVerificationService(userStore, jwtSecret, time.Minute)

	// 9.- Build HTTP handlers/controllers for auth, phone verification, notifications and tasks.
	authHandler := authhttp.NewHandler(authSvc, userStore, verificationSvc)
	authMiddleware := middleware.Authentication(authSvc, userStore)
	httpserver.RegisterAuthRoutes(router, authHandler, authMiddleware)

	// phone verification controller (from app/http/controllers/phone_verification_controller.go)
	phoneCtrl := appcontrollers.NewPhoneVerificationController(db)

	notificationSvc := memoryplatform.NewNotificationService(memoryplatform.DefaultNotifications())
	notificationHandler := notificationshttp.NewHandler(notificationSvc)

	// Tasks backed by Postgres using the same db handle.
	taskSvc, err := storagetasks.NewRepository(db)
	if err != nil {
		panic(err)
	}
	taskHandler := taskhttp.NewHandler(taskSvc)

	// 10.- Public API endpoints (no authentication required).
	api.GET("/tasks", taskHandler.List)

	// 10.1.- Phone verification public endpoints.
	api.POST("/phone-verifications", phoneCtrl.RequestCode)
	api.POST("/phone-verifications/confirm", phoneCtrl.ConfirmCode)

	// 11.- Authenticated API endpoints under /v1.
	protected := router.Group("/v1")
	protected.Use(authMiddleware)

	// 11.1.- Authenticated tasks.
	protected.GET("/tasks", taskHandler.List)

	// 11.2.- Authenticated notification management for the dashboard.
	notificationsGroup := protected.Group("/notifications")
	notificationsGroup.GET("", notificationHandler.List)
	notificationsGroup.PATCH(":id", notificationHandler.MarkRead)

	// 11.3.- Authenticated listing of unverified phone verifications for operators.
	// Example: GET /v1/phone-verifications/unverified
	protected.GET("/phone-verifications/unverified", phoneCtrl.ListUnverified)
}
