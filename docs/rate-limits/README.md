# Rate Limiting Architecture

The Yamato API enforces throttling through the `RateLimiter` middleware, which wraps every Gin request with a Redis-backed sliding window counter. The middleware starts from the static configuration loaded via `internal/config.Load`, honoring defaults (100 requests per minute with a burst of 20) while allowing the process environment to override `RATE_LIMIT_REQUESTS`, `RATE_LIMIT_DURATION`, `RATE_LIMIT_BURST`, and `RATE_LIMIT_ENABLED`.【F:internal/middleware/rate_limit.go†L63-L115】【F:internal/config/config.go†L122-L145】

## Runtime Overrides Through Settings

When `RATE_LIMIT_SETTINGS_KEY` is populated, the middleware asks the configured `RateLimitOverrideProvider` for runtime overrides before serving traffic. This provider is expected to read the JSON payload stored in the `settings` table (seeded by the bootstrapper) so operators can raise or lower quotas without restarting the API. All resolved values replace the static configuration on the fly, ensuring the request counters, reset timestamps, and `Retry-After` header reflect the latest limits.【F:internal/middleware/rate_limit.go†L27-L113】【F:seeds/seeder.go†L231-L270】

## Response Semantics

For every request, the middleware emits `X-RateLimit-*` headers that report the active bucket, total allowance, burst size, remaining quota, and reset instant. Once the sliding window exceeds the configured total allowance, the middleware evicts the latest entry, responds with HTTP 429, and surfaces the retry interval through the `Retry-After` header. When allowlisting is enabled, the middleware bypasses enforcement for matching IP addresses while still publishing headers for observability.【F:internal/middleware/rate_limit.go†L116-L188】

## Operational Tips

* Use `make migrate-up` and `make seed` to ensure the `settings` table exists before applying overrides from the admin console or background jobs.【F:Makefile†L9-L28】【F:seeds/seeder.go†L75-L170】
* Store high-priority overrides (for example, incident responses) under a dedicated settings key so the middleware can read them through `RATE_LIMIT_SETTINGS_KEY` without conflicting with other configuration bundles.【F:internal/middleware/rate_limit.go†L80-L107】
