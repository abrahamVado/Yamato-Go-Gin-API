package bootstrap

import (
        "fmt"
        "log"
        "os"
        "strings"
        "time"

        "github.com/gin-gonic/gin"

        "github.com/example/Yamato-Go-Gin-API/config"
        internalconfig "github.com/example/Yamato-Go-Gin-API/internal/config"
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

        //1.- Create a new Gin router with default middleware settings.
        router := gin.Default()

        //2.- Register custom middleware responsible for consistent request metadata and error responses.
        router.Use(middleware.RequestID())
        router.Use(middleware.ErrorHandler())

        //3.- Attach a permissive CORS policy so the Next.js frontend can consume the API locally.
        router.Use(middleware.CORS(resolveCORSConfig()))

        //4.- Provision the Prometheus metrics registry and expose its HTTP handler.
        metrics, err := observability.NewMetrics(appConfig.Name)
        if err != nil {
                log.Fatalf("failed to configure metrics: %v", err)
        }

        //5.- Register HTTP routes used by the application.
        routes.RegisterRoutes(router, routes.WithMetrics(metrics))

        //6.- Display a startup message with application information.
        fmt.Printf("Starting %s on port %s\n", appConfig.Name, appConfig.Port)

        //7.- Return the prepared router and configuration for further usage.
        return router, appConfig
}

// resolveCORSConfig prepares the runtime CORS configuration.
func resolveCORSConfig() internalconfig.CORSConfig {
        //1.- Extract comma-separated origins from the environment while providing sensible defaults.
        allowedOrigins := []string{"http://localhost:3000"}
        if raw := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGINS")); raw != "" {
                if parsed := splitAndTrim(raw); len(parsed) > 0 {
                        allowedOrigins = parsed
                }
        }

        //2.- Build the shared configuration structure with conservative defaults.
        return internalconfig.CORSConfig{
                AllowOrigins:     allowedOrigins,
                AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
                AllowHeaders:     []string{"Authorization", "Content-Type"},
                ExposeHeaders:    []string{},
                AllowCredentials: true,
                MaxAge:           12 * time.Hour,
        }
}

// splitAndTrim converts a comma-separated string into a slice with whitespace removed.
func splitAndTrim(value string) []string {
        //1.- Return early when the input is empty to keep the caller logic straightforward.
        if strings.TrimSpace(value) == "" {
                return []string{}
        }

        //2.- Iterate through the comma-separated entries while trimming stray whitespace.
        parts := strings.Split(value, ",")
        cleaned := make([]string, 0, len(parts))
        for _, part := range parts {
                trimmed := strings.TrimSpace(part)
                if trimmed != "" {
                        cleaned = append(cleaned, trimmed)
                }
        }

        //3.- Return the sanitised slice ready for configuration usage.
        return cleaned
}
