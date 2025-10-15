# Health and Readiness Endpoints

## Summary
The diagnostics handler defined in `internal/http/diagnostics/handlers.go` exposes two probe endpoints: `/api/health` for deep health checks and `/ready` for lightweight readiness reporting. Both routes are registered in `routes/web.go` and are publicly accessible in the development bootstrap.

## Routes
- **GET `/api/health`** – Executes optional database and Redis checks, returning detailed component status inside the canonical success envelope.
- **GET `/ready`** – Evaluates the same checks to decide whether the service is ready to accept traffic. Failures return an error envelope with component diagnostics.

## Response Schemas
### GET `/api/health`
```json
{
  "status": "success",
  "data": {
    "status": "ok",
    "service": "Larago API",
    "checks": {
      "database": { "status": "skipped" },
      "redis": { "status": "skipped" }
    }
  },
  "meta": {}
}
```
- The `status` field inside `data` resolves to `"ok"` when all configured dependencies respond without error; otherwise it becomes `"error"` and the HTTP status changes to `503`.
- Dependency entries include an `error` property when failures occur.

### GET `/ready`
Successful responses mirror the health payload but replace `data` with:
```json
{
  "status": "success",
  "data": {
    "ready": true,
    "checks": {
      "database": { "status": "skipped" },
      "redis": { "status": "skipped" }
    }
  },
  "meta": {}
}
```
- When any dependency check reports `error`, the handler responds with HTTP `503` and an error envelope produced by `respond.Error` containing the failing components under `errors.checks`.

## Behaviour
1. `diagnostics.NewHandler` seeds the handler with `NoopLimiter`, ensuring probes remain responsive when rate limiting is not configured.
2. Each request first passes through `applyRateLimit`; denied requests yield HTTP `429` with a retry hint and the message `"too many requests"`.
3. Database and Redis checks execute via `checkDatabase` and `checkRedis`. When the dependencies are absent, the handler marks their status as `"skipped"`.
4. `/api/health` aggregates the check outcomes to derive the HTTP status code and returns the `healthData` struct through `respond.Success`.
5. `/ready` emits `readinessData` using the same helpers and switches to an error response when any dependency reports `"error"`.
