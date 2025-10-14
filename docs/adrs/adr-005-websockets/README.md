# ADR-005: WebSocket Integration Guidelines

## Status
Accepted

## Context
The WebSocket integration enables clients to subscribe to real-time events from the Yamato service. Consistent protocol negotiation, channel naming, permission enforcement, and downstream fan-out ensure predictable behavior across user agents and services. This ADR captures the agreed decisions for handling these concerns so that future features build on a stable baseline.

## Decision Drivers
- Guarantee authenticated usage of WebSocket channels that expose private data.
- Provide deterministic channel naming that scales with multi-tenant and resource-specific subscriptions.
- Ensure the authorization layer mirrors the HTTP API without duplicating complex logic.
- Allow multiple subscribers to receive event notifications without overloading the primary publisher.

## Sec-WebSocket-Protocol Bearer Handling
Clients authenticate by supplying a bearer token through the `Sec-WebSocket-Protocol` header during the WebSocket handshake. The server inspects the comma-separated protocol list and extracts the first entry matching the `bearer,token` pattern.

//1.- Reject connections when no valid token is found; this mirrors HTTP 401 semantics.
//2.- Validate the extracted token via the existing JWT verification pipeline before accepting the handshake.
//3.- Normalize the header casing and whitespace to avoid subtle mismatches between browsers and CLI clients.

If validation fails at any step, the handshake is aborted with an authentication error, preventing unauthenticated clients from subscribing to events.

## Channel Naming Strategy
Channels follow the pattern `tenant:{tenantID}:resource:{resourceType}:{resourceID}`. This layout keeps channels unique per tenant while allowing coarse-grained resource subscriptions.

//1.- Use wildcard suffixes (e.g., `tenant:123:resource:orders:*`) for clients that need aggregate streams.
//2.- Reject malformed channel requests and fall back to server-selected defaults to protect infrastructure.
//3.- Document the canonical channel list in API references so frontend teams can subscribe without guesswork.

## Permission Checks
WebSocket permission checks reuse the HTTP authorization policies.

//1.- Evaluate the bearer token against route-specific policies when the client attempts to join a channel.
//2.- Cache positive authorization decisions for the duration of the connection to minimize redundant policy evaluations.
//3.- Immediately revoke access and close the connection if downstream services signal a permission change.

## Event Fan-Out
The service delegates message fan-out to a dedicated broadcaster component.

//1.- Publish events from the core services into a message queue that feeds the broadcaster.
//2.- The broadcaster maps queue topics to WebSocket channels and forwards payloads to all subscribed connections.
//3.- Backpressure handling ensures a slow subscriber does not block the rest; the broadcaster buffers per connection and disconnects clients that exceed limits.

## Consequences
- Using the `Sec-WebSocket-Protocol` header keeps the handshake HTTP compliant without introducing custom headers that some proxies strip.
- Canonical channel naming simplifies client SDKs and makes permission audits easier.
- Centralized fan-out minimizes load on the core services and allows future scalability improvements (e.g., sharding by tenant).

## References
- [RFC 6455 – The WebSocket Protocol](https://www.rfc-editor.org/rfc/rfc6455)
- [JWT Profile for OAuth 2.0 Access Tokens](https://datatracker.ietf.org/doc/html/rfc9068)
