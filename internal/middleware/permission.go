package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/authorization"
)

// 1.- RequirePermission verifies that the caller holds every provided permission slug.
func RequirePermission(policy *authorization.Policy, permissions ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		//2.- Fetch the authenticated principal from the context bag.
		principal, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing principal"})
			return
		}

		//2.- Evaluate the permission gate through the shared policy instance.
		gate := authorization.Gate{AllPermissions: permissions}
		if err := policy.Authorize(principal, gate); err != nil {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "reason": "missing permission"})
			return
		}

		ctx.Next()
	}
}
