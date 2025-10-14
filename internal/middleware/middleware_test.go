package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/example/Yamato-Go-Gin-API/internal/config"
)

// 1.- TestRateLimiterBuckets validates bucket headers and quota enforcement.
func TestRateLimiterBuckets(t *testing.T) {
	// 2.- Enable Gin test mode to silence log output during assertions.
	gin.SetMode(gin.TestMode)

	// 3.- Start an in-memory Redis instance to back the sliding window limiter.
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	// 4.- Construct the middleware stack with a tight quota to trigger rejection quickly.
	cfg := config.RateLimitConfig{Enabled: true, Requests: 2, Duration: time.Minute, Burst: 0}
	router := gin.New()
	router.Use(RateLimiter(client, cfg))
	router.GET("/limited", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	// 5.- Issue two successful requests to exhaust the available quota.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.1:1000"
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected success status, got %d", res.Code)
		}
		if bucket := res.Header().Get("X-RateLimit-Bucket"); bucket != "/limited" {
			t.Fatalf("expected bucket /limited, got %q", bucket)
		}
	}

	// 6.- Dispatch a third request which should exceed the quota and trigger a 429.
	blockedReq := httptest.NewRequest(http.MethodGet, "/limited", nil)
	blockedReq.RemoteAddr = "192.0.2.1:1000"
	blockedRes := httptest.NewRecorder()
	router.ServeHTTP(blockedRes, blockedReq)

	if blockedRes.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", blockedRes.Code)
	}
	if blockedRes.Header().Get("Retry-After") == "" {
		t.Fatalf("expected retry-after header to be populated")
	}
	if bucket := blockedRes.Header().Get("X-RateLimit-Bucket"); bucket != "/limited" {
		t.Fatalf("expected bucket header /limited on rejection, got %q", bucket)
	}
}

// 1.- TestRateLimiterAllowlistBypass ensures allowlisted IPs skip throttling.
func TestRateLimiterAllowlistBypass(t *testing.T) {
	// 2.- Enable Gin test mode for deterministic output.
	gin.SetMode(gin.TestMode)

	// 3.- Create a backing Redis instance for the limiter state.
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})

	// 4.- Configure a single-request quota to ensure non-allowlisted clients would be blocked.
	cfg := config.RateLimitConfig{Enabled: true, Requests: 1, Duration: time.Minute, Burst: 0}
	router := gin.New()
	router.Use(RateLimiter(client, cfg, WithRateLimitAllowlist(true, []string{"198.51.100.5"})))
	router.GET("/limited", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	// 5.- Send multiple requests from the allowlisted address verifying the quota never trips.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "198.51.100.5:2000"
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("allowlisted request should succeed, got status %d", res.Code)
		}
	}
}

// 1.- TestIPAllowlistToggle verifies strict and permissive modes of the allowlist middleware.
func TestIPAllowlistToggle(t *testing.T) {
	// 2.- Use Gin test mode to keep output clean during the test run.
	gin.SetMode(gin.TestMode)

	// 3.- Create a router protected by an allowlist containing a single trusted IP.
	lockedRouter := gin.New()
	lockedRouter.Use(IPAllowlist(true, []string{"203.0.113.10"}))
	lockedRouter.GET("/secure", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })

	// 4.- Expect requests from unknown addresses to be rejected with 403.
	deniedReq := httptest.NewRequest(http.MethodGet, "/secure", nil)
	deniedReq.RemoteAddr = "203.0.113.20:4000"
	deniedRes := httptest.NewRecorder()
	lockedRouter.ServeHTTP(deniedRes, deniedReq)
	if deniedRes.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden status, got %d", deniedRes.Code)
	}

	// 5.- Verify allowlisted addresses are permitted through the middleware.
	allowedReq := httptest.NewRequest(http.MethodGet, "/secure", nil)
	allowedReq.RemoteAddr = "203.0.113.10:4001"
	allowedRes := httptest.NewRecorder()
	lockedRouter.ServeHTTP(allowedRes, allowedReq)
	if allowedRes.Code != http.StatusOK {
		t.Fatalf("expected success for allowlisted IP, got %d", allowedRes.Code)
	}

	// 6.- Ensure disabling the middleware allows any address to proceed.
	openRouter := gin.New()
	openRouter.Use(IPAllowlist(false, nil))
	openRouter.GET("/open", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
	openReq := httptest.NewRequest(http.MethodGet, "/open", nil)
	openReq.RemoteAddr = "198.51.100.99:5000"
	openRes := httptest.NewRecorder()
	openRouter.ServeHTTP(openRes, openReq)
	if openRes.Code != http.StatusOK {
		t.Fatalf("expected open router to allow request, got %d", openRes.Code)
	}
}
