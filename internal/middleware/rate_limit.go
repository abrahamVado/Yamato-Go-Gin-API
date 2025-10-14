package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/example/Yamato-Go-Gin-API/internal/config"
	"github.com/example/Yamato-Go-Gin-API/internal/http/respond"
)

// RateLimitOverride describes dynamic throttling values stored in the settings table.
type RateLimitOverride struct {
	Requests *int
	Duration *time.Duration
	Burst    *int
}

// RateLimitOverrideProvider exposes a settings lookup used by the rate limiter to load overrides.
type RateLimitOverrideProvider interface {
	RateLimitOverride(ctx context.Context, key string) (RateLimitOverride, bool, error)
}

// RateLimiterOption customises the rate limiter middleware during construction.
type RateLimiterOption func(*rateLimiterSettings)

type rateLimiterSettings struct {
	provider         RateLimitOverrideProvider
	allowlistEnabled bool
	allowlist        map[string]struct{}
}

// WithRateLimitOverrideProvider injects a settings-backed override provider.
func WithRateLimitOverrideProvider(provider RateLimitOverrideProvider) RateLimiterOption {
	// 1.- Capture the provider so the middleware can request overrides at build time.
	return func(settings *rateLimiterSettings) {
		settings.provider = provider
	}
}

// WithRateLimitAllowlist configures IP addresses that bypass the rate limiter.
func WithRateLimitAllowlist(enabled bool, ips []string) RateLimiterOption {
	// 1.- Capture the allowlist and normalise its entries for fast lookups.
	return func(settings *rateLimiterSettings) {
		settings.allowlistEnabled = enabled
		settings.allowlist = buildAllowlistSet(ips)
	}
}

type rateLimitPolicy struct {
	requests int
	duration time.Duration
	burst    int
}

type rateLimiterRuntime struct {
	client           redis.Cmdable
	policy           rateLimitPolicy
	enabled          bool
	allowlistEnabled bool
	allowlist        map[string]struct{}
}

// RateLimiter constructs a Redis-backed sliding window limiter for incoming requests.
func RateLimiter(client redis.Cmdable, cfg config.RateLimitConfig, opts ...RateLimiterOption) gin.HandlerFunc {
	// 1.- Seed option defaults before applying customisations.
	settings := rateLimiterSettings{allowlist: map[string]struct{}{}}
	for _, opt := range opts {
		opt(&settings)
	}

	// 2.- Start from the static configuration values and evaluate runtime overrides.
	policy := rateLimitPolicy{
		requests: cfg.Requests,
		duration: cfg.Duration,
		burst:    cfg.Burst,
	}
	if policy.duration <= 0 {
		policy.duration = time.Minute
	}
	if policy.burst < 0 {
		policy.burst = 0
	}
	if settings.provider != nil && strings.TrimSpace(cfg.SettingsKey) != "" {
		if override, ok, err := settings.provider.RateLimitOverride(context.Background(), cfg.SettingsKey); err == nil && ok {
			if override.Requests != nil && *override.Requests > 0 {
				policy.requests = *override.Requests
			}
			if override.Duration != nil && *override.Duration > 0 {
				policy.duration = *override.Duration
			}
			if override.Burst != nil && *override.Burst >= 0 {
				policy.burst = *override.Burst
			}
		}
	}

	// 3.- Materialise the runtime state consumed by the Gin middleware.
	runtime := &rateLimiterRuntime{
		client:           client,
		policy:           policy,
		enabled:          cfg.Enabled,
		allowlistEnabled: settings.allowlistEnabled,
		allowlist:        settings.allowlist,
	}
	if runtime.allowlist == nil {
		runtime.allowlist = map[string]struct{}{}
	}

	// 4.- Return the middleware handler that enforces the sliding window strategy.
	return runtime.handle
}

func (r *rateLimiterRuntime) handle(ctx *gin.Context) {
	// 1.- Short-circuit when the limiter is disabled, misconfigured, or lacks a Redis client.
	if !r.enabled || r.client == nil || r.policy.requests <= 0 {
		ctx.Next()
		return
	}

	// 2.- Determine the logical bucket using the registered route path.
	bucket := ctx.FullPath()
	if bucket == "" {
		bucket = ctx.Request.URL.Path
	}

	// 3.- Publish static headers early so clients can introspect the active limits.
	ctx.Header("X-RateLimit-Bucket", bucket)
	ctx.Header("X-RateLimit-Limit", strconv.Itoa(r.policy.requests))
        ctx.Header("X-RateLimit-Burst", strconv.Itoa(r.policy.burst))

	// 4.- Skip throttling entirely for allowlisted clients when the toggle is active.
	if r.allowlistEnabled {
		if _, ok := r.allowlist[ctx.ClientIP()]; ok {
			totalAllowed := r.policy.requests + r.policy.burst
			ctx.Header("X-RateLimit-Remaining", strconv.Itoa(totalAllowed))
			ctx.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(r.policy.duration).Unix(), 10))
			ctx.Next()
			return
		}
	}

	// 5.- Evaluate the sliding window counters atomically via a Redis pipeline.
	now := time.Now()
	nowMicro := now.UnixMicro()
	windowStart := now.Add(-r.policy.duration).UnixMicro()
	key := fmt.Sprintf("ratelimit:%s:%s", bucket, ctx.ClientIP())
	pipe := r.client.TxPipeline()
	ctxCtx := ctx.Request.Context()
	pipe.ZRemRangeByScore(ctxCtx, key, "0", strconv.FormatInt(windowStart, 10))
	member := fmt.Sprintf("%d-%s", nowMicro, uuid.NewString())
	pipe.ZAdd(ctxCtx, key, redis.Z{Score: float64(nowMicro), Member: member})
	countCmd := pipe.ZCard(ctxCtx, key)
	pipe.Expire(ctxCtx, key, r.policy.duration)
	if _, err := pipe.Exec(ctxCtx); err != nil {
		ctx.Next()
		return
	}

	// 6.- Determine the remaining quota and prepare rate limit headers.
	totalAllowed := int64(r.policy.requests + r.policy.burst)
	count := countCmd.Val()
	remaining := totalAllowed - count
	if remaining < 0 {
		remaining = 0
	}
        ctx.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))
        ctx.Header("X-RateLimit-Reset", strconv.FormatInt(now.Add(r.policy.duration).Unix(), 10))

	// 7.- Reject the request when the quota has been exhausted.
	if count > totalAllowed {
		_, _ = r.client.ZRem(ctxCtx, key, member).Result()
		retry := r.policy.duration
		if retry <= 0 {
			retry = time.Minute
		}
		seconds := int(math.Ceil(retry.Seconds()))
		if seconds < 1 {
			seconds = 1
		}
		ctx.Header("Retry-After", strconv.Itoa(seconds))
		env := respond.NewError(http.StatusTooManyRequests, "rate limit exceeded", map[string]interface{}{"bucket": bucket, "retry_after": seconds})
		respond.WriteError(ctx, env)
		ctx.Abort()
		return
	}

	// 8.- Continue processing when the request is within the quota.
	ctx.Next()
}

// IPAllowlist enforces that only specific client IPs can access the wrapped routes.
func IPAllowlist(enabled bool, ips []string) gin.HandlerFunc {
	// 1.- Build a lookup table for the allowlist to achieve constant-time membership checks.
	allowed := buildAllowlistSet(ips)

	// 2.- Return the Gin middleware that applies the allowlist policy.
	return func(ctx *gin.Context) {
		// 3.- Skip enforcement entirely when the toggle is disabled.
		if !enabled {
			ctx.Next()
			return
		}

		// 4.- Allow requests from recognised IP addresses.
		if _, ok := allowed[ctx.ClientIP()]; ok {
			ctx.Next()
			return
		}

		// 5.- Reject unknown addresses with a structured error payload.
		env := respond.NewError(http.StatusForbidden, "ip not allowlisted", map[string]interface{}{"ip": ctx.ClientIP()})
		respond.WriteError(ctx, env)
		ctx.Abort()
	}
}

func buildAllowlistSet(ips []string) map[string]struct{} {
	// 1.- Normalise input values by trimming whitespace and removing empty entries.
	allowlist := make(map[string]struct{}, len(ips))
	for _, raw := range ips {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		allowlist[ip] = struct{}{}
	}
	// 2.- Return the set representation for constant-time lookups.
	return allowlist
}
