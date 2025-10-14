package authorization_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/auth"
	"github.com/example/Yamato-Go-Gin-API/internal/authorization"
	"github.com/example/Yamato-Go-Gin-API/internal/middleware"
)

func TestRequireRoleAllowsPrincipal(t *testing.T) {
	//1.- Run Gin in test mode to avoid noisy logging.
	gin.SetMode(gin.TestMode)
	policy := authorization.NewPolicy()
	router := gin.New()

	//2.- Seed the context with a principal carrying the admin role.
	router.Use(func(ctx *gin.Context) {
		auth.SetPrincipal(ctx, auth.Principal{Subject: "user-1", Roles: []string{"admin"}})
		ctx.Next()
	})

	//3.- Attach the role middleware and respond with OK when reached.
	router.Use(middleware.RequireRole(policy, "admin"))
	router.GET("/", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	//4.- Execute the request and confirm the middleware permitted access.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
}

func TestRequireRoleDeniesWhenMissing(t *testing.T) {
	//1.- Configure the router with a user that lacks the admin role.
	gin.SetMode(gin.TestMode)
	policy := authorization.NewPolicy()
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		auth.SetPrincipal(ctx, auth.Principal{Subject: "user-2", Roles: []string{"member"}})
		ctx.Next()
	})

	//2.- Require the admin role and capture the middleware response.
	router.Use(middleware.RequireRole(policy, "admin"))
	router.GET("/", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	//3.- Perform the request expecting a 403 Forbidden response.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", res.Code)
	}

	//4.- Decode the JSON payload to validate the denial reason.
	var payload map[string]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response body: %v", err)
	}
	if payload["reason"] != "missing role" {
		t.Fatalf("expected missing role reason, got %q", payload["reason"])
	}
}

func TestRequirePermissionDeniesAndAllows(t *testing.T) {
	//1.- Build a router where the first request has the required permission and the second does not.
	gin.SetMode(gin.TestMode)
	policy := authorization.NewPolicy()
	router := gin.New()

	//2.- Provide a helper to inject principals with varying permission sets.
	applyPrincipal := func(perms []string) {
		router.Use(func(ctx *gin.Context) {
			auth.SetPrincipal(ctx, auth.Principal{Subject: "user-3", Permissions: perms})
			ctx.Next()
		})
	}

	//3.- Test the positive path with the pipelines.read permission.
	applyPrincipal([]string{"pipelines.read"})
	router.Use(middleware.RequirePermission(policy, "pipelines.read"))
	router.GET("/ok", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 from positive path, got %d", res.Code)
	}

	//4.- Swap in a principal lacking the permission and expect a denial on a new route.
	router = gin.New()
	router.Use(func(ctx *gin.Context) {
		auth.SetPrincipal(ctx, auth.Principal{Subject: "user-4", Permissions: []string{"pipelines.list"}})
		ctx.Next()
	})
	router.Use(middleware.RequirePermission(policy, "pipelines.read"))
	router.GET("/forbidden", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})

	req = httptest.NewRequest(http.MethodGet, "/forbidden", nil)
	res = httptest.NewRecorder()
	router.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403 from negative path, got %d", res.Code)
	}
}

func TestPolicyInvalidateRefreshesCachedPermissions(t *testing.T) {
	//1.- Seed a principal with an initial permission and authorize successfully.
	policy := authorization.NewPolicy()
	principal := auth.Principal{Subject: "user-5", Permissions: []string{"pipelines.read"}}
	gate := authorization.Gate{AllPermissions: []string{"pipelines.read"}}
	if err := policy.Authorize(principal, gate); err != nil {
		t.Fatalf("expected initial authorization to pass: %v", err)
	}

	//2.- Remove the permission from the principal and confirm the cached result still passes.
	principal.Permissions = nil
	if err := policy.Authorize(principal, gate); err != nil {
		t.Fatalf("expected cached authorization to pass after removal: %v", err)
	}

	//3.- Invalidate the cache entry and expect the next authorization to fail.
	policy.Invalidate(principal.Subject)
	if err := policy.Authorize(principal, gate); !errors.Is(err, authorization.ErrForbidden) {
		t.Fatalf("expected forbidden after invalidation, got %v", err)
	}
}
