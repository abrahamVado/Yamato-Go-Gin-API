# WebSocket Capabilities and Occupancy

## Summary
The WebSocket implementation in this codebase focuses on authenticated event delivery through `internal/websocket/server.go`. There is no `GET /ws/rooms/:room/occupants` endpoint or equivalent occupancy probe wired into the Gin router.

## Current Behavior
- The WebSocket server authenticates clients via a bearer token advertised in the `Sec-WebSocket-Protocol` header, subscribes them to broker channels, and forwards events as JSON envelopes.
- Routes defined in `routes/web.go` expose REST and notification APIs only; none register the WebSocket server or an occupancy endpoint.

## Implications for Recreation Work
- Any recreation that requires occupancy counts must introduce new HTTP handlers and shared state—those facilities are absent today.
- To experiment with the WebSocket server, register a route manually (for example `router.GET("/ws", gin.WrapH(http.HandlerFunc(server.Handle)))`) and attach a broker implementation capable of counting occupants if needed.
