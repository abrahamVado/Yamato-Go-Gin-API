package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	internalauth "github.com/example/Yamato-Go-Gin-API/internal/auth"
	authhttp "github.com/example/Yamato-Go-Gin-API/internal/http/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
)

// Authentication validates Bearer tokens and exposes the authenticated principal to handlers.
func Authentication(authSvc *internalauth.Service, users authhttp.UserStore) gin.HandlerFunc {
	// 1.- Return a Gin middleware that enforces Authorization headers.
	return func(ctx *gin.Context) {
		header := ctx.GetHeader("Authorization")
		if header == "" {
			respond.Error(ctx, http.StatusUnauthorized, "missing authorization header", map[string]interface{}{"reason": "bearer token required"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			respond.Error(ctx, http.StatusUnauthorized, "invalid authorization header", map[string]interface{}{"reason": "bearer scheme required"})
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			respond.Error(ctx, http.StatusUnauthorized, "invalid authorization header", map[string]interface{}{"reason": "token missing"})
			return
		}

		claims, err := authSvc.ValidateAccessToken(ctx.Request.Context(), token)
		if err != nil {
			respond.Error(ctx, http.StatusUnauthorized, "invalid or expired token", map[string]interface{}{"details": err.Error()})
			return
		}

		principal := internalauth.Principal{Subject: claims.Subject, Roles: []string{"member"}, Permissions: []string{}}
		internalauth.SetPrincipal(ctx, principal)

		if users != nil {
			if user, err := users.FindByID(ctx.Request.Context(), claims.Subject); err == nil {
				ctx.Set("auth.user.email", user.Email)
				ctx.Set("auth.user.name", user.Name)
			}
		}

		ctx.Next()
	}
}
