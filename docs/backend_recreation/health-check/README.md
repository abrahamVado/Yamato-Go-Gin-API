# Health Check Endpoint

## Summary
The health check endpoint exposes a lightweight readiness probe. It is implemented in `backend/internal/api/server.go` and simply returns a JSON payload indicating that the API process is responsive.

## Route
- **Method:** `GET`
- **Path:** `/health`
- **Authentication:** Not required (the handler does not invoke the validator).

## Success Response
- **Status:** `200 OK`
- **Body:**
  ```json
  {
    "status": "ok"
  }
  ```

## Failure Modes
The handler is deterministic and cannot fail under normal circumstances because it does not read request data or invoke dependencies.

## Implementation Notes
1. The route is registered inside `Server.RegisterRoutes` alongside the bid and trip endpoints.
2. The handler uses `gin.H` to emit the response map via `c.JSON`.

## Reproduction Checklist
- Mount the route on a Gin engine inside `Server.RegisterRoutes`.
- Return the static JSON body with a `200 OK` status.
- Avoid wrapping the endpoint in authentication middleware when recreating the server.
