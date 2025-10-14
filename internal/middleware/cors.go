package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/config"
)

// CORS wires a configurable cross-origin resource sharing policy into Gin.
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	// 1.- Start from Gin's default CORS configuration before applying application overrides.
	corsCfg := cors.Config{
		AllowOrigins:     cfg.AllowOrigins,
		AllowMethods:     cfg.AllowMethods,
		AllowHeaders:     cfg.AllowHeaders,
		ExposeHeaders:    cfg.ExposeHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	}

	// 2.- Normalise zero MaxAge values to avoid Gin rejecting the configuration.
	if corsCfg.MaxAge <= 0 {
		corsCfg.MaxAge = 12 * time.Hour
	}

	// 3.- Delegate the heavy lifting to gin-contrib/cors which returns the middleware handler.
	return cors.New(corsCfg)
}
