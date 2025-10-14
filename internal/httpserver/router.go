package httpserver

import (
	"github.com/gin-gonic/gin"

	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
)

// 1.- RegisterAuthRoutes wires authentication HTTP handlers into the Gin router tree.
func RegisterAuthRoutes(router gin.IRouter, handler authhttp.Handler) {
	// 2.- Group versioned API routes under the /v1 prefix.
	v1 := router.Group("/v1")

	// 3.- Mount authentication endpoints on /v1/auth.
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", handler.Register)
	authGroup.POST("/login", handler.Login)
	authGroup.POST("/logout", handler.Logout)
	authGroup.POST("/refresh", handler.Refresh)

	// 4.- Expose a user endpoint under /v1/user for principal introspection.
	userGroup := v1.Group("/user")
	userGroup.GET("", handler.CurrentUser)
}
