package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/authorization"
)

// 1.- RequireRole enforces that the principal matches at least one required role slug.
func RequireRole(policy *authorization.Policy, roles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		//2.- Pull the authenticated principal from the request context.
		principal, ok := auth.PrincipalFromContext(ctx)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing principal"})
			return
		}

		//2.- Delegate the role evaluation to the policy, reusing its caching internals.
		gate := authorization.Gate{AnyRoles: roles}
		if err := policy.Authorize(principal, gate); err != nil {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden", "reason": "missing role"})
			return
		}

		ctx.Next()
	}
}
