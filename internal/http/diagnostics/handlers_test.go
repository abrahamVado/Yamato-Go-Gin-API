package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/middleware"
)

// 1.- TestHealthSuccess verifies that the health endpoint returns a healthy payload when dependencies pass.
func TestHealthSuccess(t *testing.T) {
	// 1.- Force Gin into test mode to avoid noisy logging during assertions.
	gin.SetMode(gin.TestMode)

	// 2.- Prepare stub dependencies that always succeed for deterministic testing.
	db := &stubDB{}
	cache := &stubRedis{}
	handler := NewHandler("Test Service", WithDatabase(db), WithRedis(cache))

	// 3.- Create a synthetic HTTP context invoking the health handler.
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.GET("/api/health", handler.Health)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	// 4.- Execute the handler and capture the emitted response.
	router.ServeHTTP(recorder, req)

	// 5.- Assert that the response status code indicates success.
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d", recorder.Code)
	}

	// 6.- Decode the JSON payload for field-level assertions.
	var envelope map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 7.- Validate that the data field reports the expected component health.
	if envelope["status"] != "success" {
		t.Fatalf("expected envelope status success, got %v", envelope["status"])
	}
	data, ok := envelope["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", envelope["data"])
	}
	checks, ok := data["checks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected checks to be a map, got %T", data["checks"])
	}
	database, ok := checks["database"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected database check to be a map, got %T", checks["database"])
	}
	if status := database["status"]; status != "ok" {
		t.Fatalf("expected database check status ok, got %v", status)
	}
}

// 1.- TestHealthFailurePropagatesError ensures dependency failures surface to clients.
func TestHealthFailurePropagatesError(t *testing.T) {
	// 1.- Configure a stub database that simulates an outage via PingContext.
	expectedErr := errors.New("database unreachable")
	db := &stubDB{err: expectedErr}
	handler := NewHandler("Test Service", WithDatabase(db))

	// 2.- Arrange the Gin test context targeting the health route.
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.GET("/api/health", handler.Health)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	// 3.- Execute the handler and capture the degraded response.
	router.ServeHTTP(recorder, req)

	// 4.- Expect a 503 status indicating dependency failure.
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recorder.Code)
	}

	// 5.- Decode the error envelope to confirm the failure is reported.
	var envelope map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	// 6.- Validate that the error payload references the database outage.
	if envelope["status"] != "error" {
		t.Fatalf("expected envelope status error, got %v", envelope["status"])
	}
	errorsField, ok := envelope["errors"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected errors to be a map, got %T", envelope["errors"])
	}
	checks, ok := errorsField["checks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected checks error map, got %T", errorsField["checks"])
	}
	if msg, ok := checks["database"].(string); !ok || msg != expectedErr.Error() {
		t.Fatalf("expected database error %q, got %v (%t)", expectedErr.Error(), msg, ok)
	}
}

// 1.- TestReadyReflectsDependencyStatus verifies readiness mirrors dependency health.
func TestReadyReflectsDependencyStatus(t *testing.T) {
	// 1.- Leverage a failing Redis stub to emulate readiness degradation.
	cache := &stubRedis{err: errors.New("redis timeout")}
	handler := NewHandler("Test Service", WithRedis(cache))

	// 2.- Invoke the readiness endpoint using a Gin test context.
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.GET("/ready", handler.Ready)
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)

	// 3.- Execute the handler and confirm the failure response.
	router.ServeHTTP(recorder, req)

	// 4.- Expect the readiness probe to return a service unavailable status.
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", recorder.Code)
	}

	// 5.- Parse the error envelope to confirm the Redis outage was surfaced.
	var envelope map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if envelope["status"] != "error" {
		t.Fatalf("expected envelope status error, got %v", envelope["status"])
	}
	errorsField, ok := envelope["errors"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected errors to be a map, got %T", envelope["errors"])
	}
	checks, ok := errorsField["checks"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected checks error map, got %T", errorsField["checks"])
	}
	if msg, ok := checks["redis"].(string); !ok || msg != cache.err.Error() {
		t.Fatalf("expected redis error %q, got %v (%t)", cache.err.Error(), msg, ok)
	}
}

// 1.- TestRateLimitFailure short-circuits when the limiter rejects the request.
func TestRateLimitFailure(t *testing.T) {
	// 1.- Configure a limiter stub that denies the incoming request.
	limiter := &stubLimiter{result: RateLimiterResult{Allowed: false, RetryAfter: time.Second}}
	handler := NewHandler("Test Service", WithRateLimiter(limiter))

	// 2.- Invoke the health endpoint under rate-limited conditions.
	recorder := httptest.NewRecorder()
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.GET("/api/health", handler.Health)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	// 3.- Execute the handler, expecting a throttled response.
	router.ServeHTTP(recorder, req)

	// 4.- Ensure a 429 status is returned when the limiter rejects the call.
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", recorder.Code)
	}

	// 5.- The limiter should have been invoked with a non-empty key.
	if limiter.lastKey == "" {
		t.Fatalf("expected rate limiter to receive a key")
	}
}

// 1.- stubDB implements DBPinger for unit testing scenarios.
type stubDB struct {
	err   error
	calls int
}

// 2.- PingContext records the invocation count and returns the configured error.
func (s *stubDB) PingContext(ctx context.Context) error {
	s.calls++
	return s.err
}

// 1.- stubRedis implements RedisPinger for unit testing scenarios.
type stubRedis struct {
	err   error
	calls int
}

// 2.- Ping records the call and returns the configured outcome.
func (s *stubRedis) Ping(ctx context.Context) error {
	s.calls++
	return s.err
}

// 1.- stubLimiter implements RateLimiter and captures the evaluated key.
type stubLimiter struct {
	result  RateLimiterResult
	err     error
	lastKey string
}

// 2.- Allow returns the configured result while storing the key for assertions.
func (s *stubLimiter) Allow(ctx context.Context, key string) (RateLimiterResult, error) {
	s.lastKey = key
	if s.err != nil {
		return RateLimiterResult{}, s.err
	}
	return s.result, nil
}
