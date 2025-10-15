package diagnostics

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
)

// 1.- DBPinger captures the contract required for database connectivity checks.
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// 1.- RedisPinger abstracts Redis connectivity probes for health reporting.
type RedisPinger interface {
	Ping(ctx context.Context) error
}

// 1.- RateLimiterResult describes the outcome of a throttling evaluation.
type RateLimiterResult struct {
	Allowed    bool
	RetryAfter time.Duration
}

// 1.- RateLimiter expresses the subset of limiter behaviour required by handlers.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (RateLimiterResult, error)
}

// 1.- Option configures optional dependencies for the diagnostics handler.
type Option func(*Handler)

// 1.- Handler exposes service diagnostic HTTP endpoints.
type Handler struct {
	db      DBPinger
	redis   RedisPinger
	limiter RateLimiter
	service string
}

// 1.- NewHandler builds a diagnostics handler with optional dependencies.
func NewHandler(service string, opts ...Option) Handler {
	// 1.- Seed the handler with safe defaults to keep endpoints responsive.
	h := Handler{
		limiter: NoopLimiter{},
		service: service,
	}

	// 2.- Apply supplied options so callers can provide real dependencies.
	for _, opt := range opts {
		opt(&h)
	}

	// 3.- Guarantee the service label is never empty for response payloads.
	if strings.TrimSpace(h.service) == "" {
		h.service = "Larago API"
	}

	return h
}

// 1.- WithDatabase injects a database dependency into the diagnostics handler.
func WithDatabase(db DBPinger) Option {
	return func(h *Handler) {
		h.db = db
	}
}

// 1.- WithRedis injects a Redis dependency into the diagnostics handler.
func WithRedis(redis RedisPinger) Option {
	return func(h *Handler) {
		h.redis = redis
	}
}

// 1.- WithRateLimiter injects a limiter dependency into the diagnostics handler.
func WithRateLimiter(limiter RateLimiter) Option {
	return func(h *Handler) {
		h.limiter = limiter
	}
}

// 1.- NoopLimiter accepts every request and represents the default limiter.
type NoopLimiter struct{}

// 2.- Allow always grants access, ensuring diagnostics remain available.
func (NoopLimiter) Allow(ctx context.Context, key string) (RateLimiterResult, error) {
	return RateLimiterResult{Allowed: true}, nil
}

// 1.- componentStatus captures the outcome of individual dependency checks.
type componentStatus struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// 1.- healthData models the successful payload returned by /api/health.
type healthData struct {
	Status  string                     `json:"status"`
	Service string                     `json:"service"`
	Checks  map[string]componentStatus `json:"checks"`
}

// 1.- readinessData models the payload returned by the /ready endpoint.
type readinessData struct {
	Ready  bool                       `json:"ready"`
	Checks map[string]componentStatus `json:"checks"`
}

// 1.- Health reports service availability and dependency health.
func (h Handler) Health(ctx *gin.Context) {
	// 1.- Apply rate limiting prior to executing expensive dependency checks.
	if !h.applyRateLimit(ctx, "diagnostics:health") {
		return
	}

	// 2.- Run the database and Redis connectivity checks sequentially.
	dbStatus := h.checkDatabase(ctx.Request.Context())
	redisStatus := h.checkRedis(ctx.Request.Context())

	// 3.- Aggregate check results to determine overall availability.
	checks := map[string]componentStatus{
		"database": dbStatus,
		"redis":    redisStatus,
	}

	// 4.- Derive the correct HTTP status code based on dependency health.
	statusCode := http.StatusOK
	overall := "ok"
	if dbStatus.Status == "error" || redisStatus.Status == "error" {
		statusCode = http.StatusServiceUnavailable
		overall = "error"
	}

	// 5.- Respond using the canonical success envelope when healthy.
	if statusCode == http.StatusOK {
		respond.Success(ctx, statusCode, healthData{Status: overall, Service: h.service, Checks: checks}, map[string]interface{}{})
		return
	}

	// 6.- Return a structured error response when dependencies fail.
	respond.Error(ctx, statusCode, "health check failed", map[string]interface{}{"checks": h.collectErrors(checks)})
}

// 1.- Ready exposes a lightweight readiness signal for load balancers.
func (h Handler) Ready(ctx *gin.Context) {
	// 1.- Apply the limiter to ensure readiness probes respect quotas.
	if !h.applyRateLimit(ctx, "diagnostics:ready") {
		return
	}

	// 2.- Reuse the dependency checks to evaluate readiness status.
	dbStatus := h.checkDatabase(ctx.Request.Context())
	redisStatus := h.checkRedis(ctx.Request.Context())

	// 3.- Calculate readiness from individual dependency health states.
	ready := dbStatus.Status != "error" && redisStatus.Status != "error"
	checks := map[string]componentStatus{
		"database": dbStatus,
		"redis":    redisStatus,
	}

	// 4.- Serve the readiness payload and propagate failure when necessary.
	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
		respond.Error(ctx, statusCode, "service not ready", map[string]interface{}{"checks": h.collectErrors(checks)})
		return
	}

	// 5.- Deliver a compact success payload when the service is ready.
	respond.Success(ctx, statusCode, readinessData{Ready: true, Checks: checks}, map[string]interface{}{})
}

// 1.- applyRateLimit checks the limiter and renders failures consistently.
func (h Handler) applyRateLimit(ctx *gin.Context, scope string) bool {
	// 1.- Short-circuit when no limiter has been configured.
	if h.limiter == nil {
		return true
	}

	// 2.- Evaluate whether the current request may proceed.
	result, err := h.limiter.Allow(ctx.Request.Context(), h.rateLimitKey(ctx, scope))
	if err != nil {
		respond.InternalError(ctx, err)
		return false
	}

	// 3.- Block the request when the limiter denies the attempt.
	if !result.Allowed {
		if result.RetryAfter > 0 {
			seconds := int(result.RetryAfter.Seconds())
			if seconds < 1 {
				seconds = 1
			}
			ctx.Header("Retry-After", strconv.Itoa(seconds))
		}
		respond.Error(ctx, http.StatusTooManyRequests, "too many requests", map[string]interface{}{"rate_limit": "diagnostics quota exceeded"})
		return false
	}

	return true
}

// 1.- rateLimitKey derives a deterministic limiter key for the current request.
func (h Handler) rateLimitKey(ctx *gin.Context, scope string) string {
	// 1.- Prefer Gin's full path template when available for stable keys.
	path := ctx.FullPath()
	if path == "" {
		path = ctx.Request.URL.Path
	}

	// 2.- Combine the scope, path, and caller IP to align with ADR-006.
	return scope + ":" + path + ":" + ctx.ClientIP()
}

// 1.- checkDatabase executes the optional database connectivity probe.
func (h Handler) checkDatabase(ctx context.Context) componentStatus {
	// 1.- Mark the check as skipped when the dependency is not configured.
	if h.db == nil {
		return componentStatus{Status: "skipped"}
	}

	// 2.- Ping the database and capture the outcome for reporting.
	if err := h.db.PingContext(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}

	return componentStatus{Status: "ok"}
}

// 1.- checkRedis executes the optional Redis connectivity probe.
func (h Handler) checkRedis(ctx context.Context) componentStatus {
	// 1.- Skip the check when Redis has not been supplied.
	if h.redis == nil {
		return componentStatus{Status: "skipped"}
	}

	// 2.- Ping Redis and record either success or the failure message.
	if err := h.redis.Ping(ctx); err != nil {
		return componentStatus{Status: "error", Error: err.Error()}
	}

	return componentStatus{Status: "ok"}
}

// 1.- collectErrors extracts dependency errors for inclusion in failure payloads.
func (h Handler) collectErrors(checks map[string]componentStatus) map[string]string {
	// 1.- Initialise the error map to avoid nil handling downstream.
	errs := map[string]string{}

	// 2.- Iterate through the checks and capture failing components.
	for name, check := range checks {
		if check.Status == "error" {
			errs[name] = check.Error
		}
	}

	return errs
}
