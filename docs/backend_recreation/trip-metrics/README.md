# Trip Metrics Endpoint

## Summary
`GET /trips/:id/metrics` returns timing statistics collected by `trip.Manager`. The handler is defined in `backend/internal/api/server.go` and relays the `trip.Manager.MetricsFor` result directly to clients.

## Route
- **Method:** `GET`
- **Path:** `/trips/{tripID}/metrics`
- **Authentication:** Required whenever an `auth.Validator` is present.

## Success Response
- **Status:** `200 OK`
- **Body:**
  ```json
  {
    "TotalActive": 0,
    "TotalPaused": 0,
    "StartedAt": "RFC3339 timestamp"
  }
  ```
- Durations are encoded using Go's `time.Duration` JSON format (nanoseconds).

## Failure Responses
- `401 Unauthorized` when authentication fails.
- `404 Not Found` when the trip ID is absent from the manager state.

## Implementation Notes
1. The handler authenticates the request with `Server.requireAuth` before inspecting the trip.
2. `trip.Manager.MetricsFor` returns a `(Metrics, bool)` tuple; the handler uses the boolean to decide between `200` and `404`.
3. The response body is the `trip.Metrics` struct serialized directly by Gin.

## Reproduction Checklist
- Maintain trip lifecycle state using the same `trip.Manager` instance that services `/trips/:id/state` actions.
- Return a JSON body with the exact field names produced by the `trip.Metrics` struct to preserve compatibility.
- Propagate the `bool` from `MetricsFor` into a `404` status when metrics are unavailable.
