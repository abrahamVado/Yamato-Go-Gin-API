# WebSocket Upgrade Endpoint

## Summary
`GET /ws/:role/:room` upgrades HTTP connections to WebSocket sessions coordinated by `ws.Hub`. The handler is registered in `backend/internal/ws/hub.go` and accepts both rider and driver roles.

## Route
- **Method:** `GET`
- **Path:** `/ws/{role}/{room}`
- **Authentication:** Not enforced by the hub; origin checks are disabled via the upgrader's `CheckOrigin`.

## Path Parameters
- `role`: must be either `rider` or `driver`. Any other value is still accepted but is treated as an opaque role when computing the room key.
- `room`: identifier shared between riders and drivers who should exchange updates.

## Success Behavior
- On upgrade success the handler hands control to `Hub.handleUpgrade`, which:
  1. Creates a `ws.Client` instance.
  2. Registers the client with `Hub.register`.
  3. Starts a writer goroutine and blocks in `readPump` to forward inbound messages to the hub broadcast loop.

## Failure Modes
- If the upgrade fails, the handler logs the error via the hub logger and terminates the HTTP request without emitting JSON.

## Implementation Notes
1. `websocket.Upgrader` uses a permissive origin policy, so deployers should enforce access control at a higher layer when necessary.
2. Broadcast messages are wrapped in a `ws.Message` containing the originating room, role, and payload type (`"update"`).
3. Pings are sent every 50 seconds from the writer goroutine to keep the connection alive.

## Reproduction Checklist
- Initialize `ws.Hub` and call `RegisterRoutes` on the shared Gin router.
- Ensure the upgrader accepts cross-origin requests only if that matches deployment requirements.
- Maintain the register/unregister/broadcast channels to coordinate clients across goroutines.
