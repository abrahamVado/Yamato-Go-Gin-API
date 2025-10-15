# Health and Readiness Endpoints

## Summary
The production health check reports service availability and dependency status by delegating to the diagnostics handler defined in `internal/http/diagnostics/handlers.go`. The route returns a structured success envelope emitted by `respond.Success` when all checks pass, and surfaces dependency errors when they fail.

## Route
- **Method:** `GET`
- **Path:** `/api/health`
- **Authentication:** Not required (the route is part of the unauthenticated `/api` group).
- **Rate limiting:** Requests can be throttled if a limiter is injected via `diagnostics.WithRateLimiter`; the default `NoopLimiter` allows all traffic.

## Success Response
- **Status:** `200 OK`
- **Body:**
  ```json
  {
    "status": "success",
    "data": {
      "status": "ok",
      "service": "Larago API",
      "checks": {
        "database": {
          "status": "skipped"
        },
        "redis": {
          "status": "skipped"
        }
      }
    },
    "meta": {}
  }
  ```

  - `status` is always `success` for healthy responses because the handler uses `respond.Success`.
  - `data.service` reflects the service label passed to `diagnostics.NewHandler` (defaults to `Larago API`).
  - `data.checks` lists dependency outcomes (`ok`, `error`, or `skipped` when not configured).
  - `meta` is an empty object unless callers supply extra metadata.

## Failure Modes
- When either dependency check fails, the handler responds with `503 Service Unavailable` via `respond.Error`. The payload includes an `errors.checks` object summarizing failure messages.
- If a rate limiter is configured and rejects the call, the handler returns `429 Too Many Requests`, sets `Retry-After` when provided by the limiter, and emits an error envelope describing the exceeded diagnostics quota.

## Health vs. Readiness
- `GET /api/health` performs the full dependency evaluation and reports aggregated status using the success envelope described above. Any failing dependency downgrades the HTTP status to `503`.
- `GET /ready` reuses the same handler but returns a compact success payload (`status: success`, `data.ready: true`) on success. Failures also surface through `respond.Error`. Orchestrators should rely on `/ready` for readiness gating, while `/api/health` is suited for external monitors expecting dependency-level detail.

## Reproduction Checklist
- Register `diagnostics.NewHandler(...).Health` on the `/api/health` route within the Gin router.
- Ensure the handler emits the `respond.Success` envelope and dependency checks before reporting success.
- Propagate rate limiting, database, and Redis dependencies through the handler options when recreating the production behavior.
