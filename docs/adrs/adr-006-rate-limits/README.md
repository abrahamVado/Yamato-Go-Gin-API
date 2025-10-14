# ADR-006: Rate Limiting Strategy

## Status
Accepted

## Context
The public Yamato API needs predictable throttling so that bursty clients cannot exhaust resources or starve essential traffic. The current gateway already depends on Redis and exposes configuration knobs for enabling a rate limiter, but it lacks documented guidance about how limits are calculated, how overrides should behave, and which operational levers exist for administrators. Without a consistent decision record, future work could diverge and create hard-to-debug throttling behaviour.

## Decision
We adopt a Redis-backed sliding-window rate limiter with the following characteristics:

1. **Sliding-window mechanics** – Each request increments a counter for the caller in a rolling time window. Two counters are tracked per key: one for the current window and another for the preceding window. The effective request count is `current_count + previous_count * overlap_ratio`, where the overlap ratio represents the fraction of the previous window that overlaps the current timestamp. This approach smooths abrupt transitions at window boundaries while preserving a hard cap on sustained throughput.
2. **Redis data model** – Window counters use atomic Lua scripts to ensure increments and reads occur without race conditions. Keys follow a compound pattern: `rl:{scope}:{identifier}:{window_start}`. `scope` encodes the hierarchy level (`global`, `user`, or `route`), `identifier` is either a constant (`all`) or a hashed subject (user ID, API token, or HTTP method + path tuple), and `window_start` is the Unix epoch of the active window rounded down to the configured interval. Each key is assigned a TTL of `duration * 2` so the previous window remains available for overlap calculations.
3. **Override hierarchy** – Limits are evaluated from most specific to least specific: route-specific overrides take precedence, followed by user or API-token overrides, and finally the global baseline defined in configuration. The first rule that provides enough remaining quota is used; otherwise the request is rejected with a `429 Too Many Requests` response. Administrative overrides can temporarily lift limits by writing higher thresholds to Redis keys with shorter TTLs, allowing emergency relief without redeploying the service.
4. **Administrative controls** – A dedicated `rate-limit:control` hash in Redis stores feature flags and emergency multipliers. Operators can toggle the limiter on or off, adjust a global multiplier that scales all quotas (e.g., set to `0` to pause throttling or `1.5` to grant additional headroom), and inspect aggregate metrics surfaced via Prometheus counters (`rate_limit_allowed_total`, `rate_limit_blocked_total`, and `rate_limit_latency_seconds`). A lightweight CLI command will wrap these controls to avoid manual `redis-cli` intervention.

## Consequences
* The sliding-window algorithm smooths traffic while remaining simple to reason about, but it requires Lua scripting support in Redis deployments.
* The key structure makes it straightforward to expire unused buckets while still allowing human inspection during incidents.
* The override hierarchy keeps emergency adjustments isolated: setting a user-level override does not risk raising limits for all routes.
* Administrators gain immediate levers to pause throttling or bump capacity, but they must manage the control hash carefully to prevent stale overrides.

## Implementation Notes
* Limit thresholds come from the existing application configuration (`RATE_LIMIT_REQUESTS`, `RATE_LIMIT_DURATION`, and `RATE_LIMIT_BURST`). Future work will extend configuration files so that per-route and per-user overrides can be declared declaratively.
* Lua scripts should reside under `internal/rate/` and expose functions for incrementing counters and evaluating remaining quota in a single round trip.
* Observability additions will require wiring the limiter to the metrics subsystem so blocked requests are visible during capacity planning.
