# Trip State Endpoint

## Summary
`POST /trips/:id/state` mutates the lifecycle of a trip managed by `trip.Manager`. The handler in `backend/internal/api/server.go` translates the `action` field into method calls on the manager.

## Route
- **Method:** `POST`
- **Path:** `/trips/{tripID}/state`
- **Authentication:** Required whenever the server is configured with an `auth.Validator`.

## Request Body
```json
{
  "action": "start | pause | resume | cancel | complete"
}
```
- The `action` value must be one of the literals accepted by `handleTripAction`.

## Success Response
- **Status:** `204 No Content`
- **Body:** Empty.

## Failure Responses
- `400 Bad Request` when the JSON payload is invalid or when the action results in `trip.ErrInvalidTransition`.
- `401 Unauthorized` when authentication fails.

## Implementation Notes
1. `Server.requireAuth` enforces authentication before payload processing.
2. `handleTripAction` switches on the `action` string and calls the corresponding method: `StartTrip`, `PauseTrip`, `ResumeTrip`, `CancelTrip`, or `CompleteTrip`.
3. The trip manager persists events via its configured repository when transitions succeed.

## Reproduction Checklist
- Ensure `trip.Manager` is initialized with a `trip.EventRepository` if persistence is required.
- Validate the `action` field before invoking the manager to surface helpful errors.
- Return `204` on success and include `application/json` error bodies on validation failures, mirroring Gin's default behavior in the current implementation.
