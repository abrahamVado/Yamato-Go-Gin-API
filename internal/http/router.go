package http

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/requestid"
	"github.com/gin-contrib/secure"
	"github.com/gin-gonic/gin"

	"github.com/abrahamVado/Yamato-Go-Gin-API/internal/config"
	"github.com/abrahamVado/Yamato-Go-Gin-API/internal/http/handlers"
	"github.com/abrahamVado/Yamato-Go-Gin-API/internal/http/response"
)

func NewRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Core middleware
	r.Use(gin.Recovery())
	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(requestid.New())

	// Security headers
	r.Use(secure.New(secure.Config{
		SSLRedirect:          false,
		STSSeconds:           31536000,
		STSIncludeSubdomains: true,
		FrameDeny:            true,
		ContentTypeNosniff:   true,
		BrowserXssFilter:     true,
		ReferrerPolicy:       "strict-origin-when-cross-origin",
	}))

	// CORS for cross-site cookies
	r.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-CSRF-Token", "Authorization"},
		ExposeHeaders:    []string{"X-Request-Id"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health/readiness
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/readyz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ready": true}) })

	// Contract routes (stubbed 501)
	auth := r.Group("/auth")
	{
		auth.POST("/register", handlers.NotImplemented)
		auth.POST("/login", handlers.NotImplemented)
		auth.POST("/refresh", handlers.NotImplemented)
		auth.POST("/logout", handlers.NotImplemented)
		auth.GET("/session", handlers.NotImplemented)

		auth.GET("/oauth/:provider", handlers.NotImplemented)
		auth.GET("/oauth/:provider/callback", handlers.NotImplemented)

		mfa := auth.Group("/mfa")
		{
			mfa.POST("/webauthn/begin", handlers.NotImplemented)
			mfa.POST("/webauthn/finish", handlers.NotImplemented)
		}

		auth.POST("/magic-link", handlers.NotImplemented)
		auth.POST("/magic-link/consume", handlers.NotImplemented)

		auth.GET("/oidc/.well-known/jwks.json", handlers.NotImplemented)
	}

	// 404 in our envelope
	r.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "Route not found")
	})

	return r
}
