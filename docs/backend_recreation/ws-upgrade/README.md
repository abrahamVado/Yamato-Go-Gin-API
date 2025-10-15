# WebSocket Upgrade Server

## Overview
`internal/websocket/server.go` contains the current WebSocket upgrade entrypoint. The `Server` coordinates authentication, broker subscriptions, and message fan-out for authenticated clients; it replaces the earlier `ws.Hub` example referenced in this recreation track.

## Handshake Requirements
- **HTTP Method & Path:** Any route may be used as long as it delegates to `Server.Handle`. No Gin route is currently wired in `routes/web.go`, so consumers must register a route manually.
- **Subprotocol Header:** Clients must send `Sec-WebSocket-Protocol: bearer,<token>` so the server can parse the bearer token and echo the `bearer` subprotocol in the upgrade response. Missing or malformed headers cause a `401 Unauthorized` response before authentication runs.
- **Authentication:** The supplied token is validated via the injected `Authenticator`. Authentication failures result in `401 Unauthorized`.

## Channel Selection
After a successful handshake, the server derives broker channels from the authenticated principal:
1. Every principal is subscribed to `notifications:<subject>` so user-specific notifications flow to the connection.
2. Principals that report the `admin` role receive an additional `admin:events` subscription.

These subscriptions are established through the injected broker’s `Subscribe` method. Failures while subscribing close the WebSocket with `1011` (internal error).

## Message Envelope
Events from the broker are serialized to JSON before being sent to the client. Each message is wrapped in an `EventEnvelope` containing the originating channel and the raw payload string, preserving the broker’s original JSON.

```json
{
  "channel": "notifications:user-123",
  "payload": "{\"message\":\"hello\"}"
}
```

The server enforces a 5 second write deadline per message to avoid stalled connections. If writing fails, the connection is closed.

## Integration Checklist
1. Instantiate the WebSocket `Server` with concrete authenticator and broker implementations.
2. Register an HTTP route (for example, `router.GET("/ws", gin.WrapH(http.HandlerFunc(server.Handle)))`) in the hosting application.
3. Ensure clients include the `bearer` subprotocol header and handle the JSON envelope described above.
