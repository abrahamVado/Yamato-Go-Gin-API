package bootstrap

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/config"
	"github.com/example/Yamato-Go-Gin-API/internal/middleware"
	"github.com/example/Yamato-Go-Gin-API/internal/observability"
	"github.com/example/Yamato-Go-Gin-API/routes"
	"go.uber.org/zap"
)

// SetupRouter prepares the Gin engine with application routes and middleware.
func SetupRouter() (*gin.Engine, config.AppConfig) {
	// 1.- Load configuration required for the application instance.
	appConfig := config.LoadAppConfig()

	// 2.- Build the structured logger and replace the global Zap instance for reuse.
	logger, err := observability.NewLogger(observability.LoggerConfig{
		Environment: os.Getenv("APP_ENV"),
		Service:     appConfig.Name,
	})
	if err != nil {
		log.Fatalf("failed to configure logger: %v", err)
	}
	zap.ReplaceGlobals(logger)

	// 3.- Create a new Gin router with default middleware settings.
	router := gin.Default()

	// 4.- Register custom middleware responsible for consistent request metadata and error responses.
	router.Use(middleware.RequestID())
	router.Use(middleware.ErrorHandler())

	// 5.- Provision the Prometheus metrics registry and expose its HTTP handler.
	metrics, err := observability.NewMetrics(appConfig.Name)
	if err != nil {
		log.Fatalf("failed to configure metrics: %v", err)
	}

	// 6.- Register HTTP routes used by the application.
	routes.RegisterRoutes(router, routes.WithMetrics(metrics))

	// 7.- Display a startup message with application information.
	fmt.Printf("Starting %s on port %s\n", appConfig.Name, appConfig.Port)

	// 8.- Return the prepared router and configuration for further usage.
	return router, appConfig
}
