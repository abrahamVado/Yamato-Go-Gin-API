# Room Occupancy Endpoint

## Summary
`GET /ws/rooms/:room/occupants` exposes the number of active WebSocket clients across all roles that are joined to a specific room. The handler is implemented in `backend/internal/ws/hub.go` and is part of `ws.Hub.RegisterRoutes`.

## Route
- **Method:** `GET`
- **Path:** `/ws/rooms/{room}/occupants`
- **Authentication:** Not enforced.

## Success Response
- **Status:** `200 OK`
- **Body:**
  ```json
  {
    "room": "string",
    "occupants": 0
  }
  ```

## Failure Modes
The handler cannot fail under normal circumstances because it only reads in-memory state. If the room is unknown, it simply reports zero occupants.

## Implementation Notes
1. The handler calls `Hub.count(room)` which iterates over `hub.rooms` and sums clients whose key suffix matches the requested room ID.
2. Room keys are namespaced by role using the format `<role>:<room>`; `count` ignores the role while tallying.

## Reproduction Checklist
- Maintain `hub.rooms` as a map of room keys to sets of `*Client` instances.
- Implement a `count` helper that iterates over the map with a read lock to keep the operation race-free.
- Return the JSON envelope shown above without requiring authentication.
