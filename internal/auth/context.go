package auth

import "github.com/gin-gonic/gin"

const principalContextKey = "auth.principal"

// 1.- Principal captures the authenticated subject, attached roles, and permissions.
type Principal struct {
	Subject     string
	Roles       []string
	Permissions []string
}

// 1.- HasRole verifies whether the principal owns the provided role slug.
func (p Principal) HasRole(role string) bool {
	//2.- Iterate through the known roles and compare against the target slug.
	for _, candidate := range p.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

// 1.- SetPrincipal stores the principal on the Gin context for downstream middleware.
func SetPrincipal(ctx *gin.Context, principal Principal) {
	//2.- Use Gin's context storage so handlers can retrieve the current principal.
	ctx.Set(principalContextKey, principal)
}

// 1.- PrincipalFromContext retrieves the authenticated principal from Gin's context bag.
func PrincipalFromContext(ctx *gin.Context) (Principal, bool) {
	//2.- Look up the stored value and ensure it matches the Principal type.
	value, exists := ctx.Get(principalContextKey)
	if !exists {
		return Principal{}, false
	}
	principal, ok := value.(Principal)
	if !ok {
		return Principal{}, false
	}
	return principal, true
}
