package bootstrap

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/config"
	"github.com/example/Yamato-Go-Gin-API/routes"
)

// SetupRouter prepares the Gin engine with application routes and middleware.
func SetupRouter() (*gin.Engine, config.AppConfig) {
	// 1.- Load configuration required for the application instance.
	appConfig := config.LoadAppConfig()

	// 2.- Create a new Gin router with default middleware settings.
	router := gin.Default()

	// 3.- Register HTTP routes used by the application.
	routes.RegisterRoutes(router)

	// 4.- Display a startup message with application information.
	fmt.Printf("Starting %s on port %s\n", appConfig.Name, appConfig.Port)

	// 5.- Return the prepared router and configuration for further usage.
	return router, appConfig
}
