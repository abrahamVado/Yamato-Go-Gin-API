# ADR 001: Authentication Tokens with JWT Access and Refresh Strategy

## Status
Accepted

## Context
The API currently issues JSON Web Tokens (JWT) for client authentication, but the absence of a documented policy around token lifetimes, refresh cycles, and server-side tracking complicates implementation and onboarding. A shared understanding is required to guide engineering work around Redis-backed session state, rotation of refresh tokens, and security guarantees such as revocation, replay protection, and key management.

## Decision
We will implement an authentication model that issues a short-lived access token and a longer-lived refresh token as a coupled pair. The access token will remain stateless and signed with the configured HMAC secret, while the refresh token identifier will be stored in Redis to support rotation and revocation. Each successful refresh will invalidate the previous refresh token by marking its Redis entry as used and issuing a new access/refresh pair. Redis will set expirations aligned with refresh token TTLs to ensure automatic cleanup. Revocation will be enforced by deleting Redis keys for compromised tokens and by honoring a server-maintained allowlist of active refresh identifiers.

## Details
### Token Lifetimes
- Access tokens expire in minutes (default 15) to limit exposure if intercepted.
- Refresh tokens expire in days (default 7) and are single-use.

### Rotation Rules
1. Clients must present both access and refresh tokens to obtain new credentials.
2. Successful refresh requests rotate the refresh token: the old identifier is deleted from Redis and replaced with a new entry for the newly issued token.
3. Redis records include user ID, issued-at timestamp, client metadata (IP, user-agent), and a one-time nonce to detect replay.
4. Any attempt to reuse an already consumed refresh token triggers revocation of the entire session chain.

### Redis Usage
- Keys follow the pattern `auth:refresh:<token-id>` to simplify lookup and bulk revocation per user (`auth:refresh:user:<user-id>` stores a set of active token IDs).
- Each refresh key stores serialized session context and uses an expiration equal to the refresh TTL.
- Redis transactions (MULTI/EXEC) ensure atomic rotation: we mark the old token as used, create the new token entry, and update user sets in a single transaction.
- A daily job scans for stale user sets and removes orphaned token IDs.

### Security Considerations
- JWT signatures use the configured HMAC secret with regular rotation and overlap periods to allow seamless key changes.
- Access tokens include `jti`, `sub`, `aud`, `iss`, `exp`, `iat`, and a rotation counter claim to bind to the latest refresh cycle.
- Refresh tokens are random 256-bit values encoded in URL-safe base64 and never logged.
- HTTPS is mandatory for all endpoints issuing or accepting tokens.
- Rate limiting protects refresh endpoints from brute-force attempts.
- Compromise handling: administrators can revoke all tokens for a user by deleting the Redis set and associated keys; global revocation can be achieved by bumping a server-wide `refresh:version` stored in Redis and embedding the version in issued tokens.

## Alternatives
- **Stateless-only JWTs:** Reject storing refresh tokens in Redis and rely solely on short-lived access tokens. This was rejected because it weakens revocation capabilities and complicates session continuity.
- **Database-backed refresh storage:** Use Postgres instead of Redis for refresh token tracking. This was rejected due to higher latency for token rotation and added operational overhead compared with Redis, which we already operate for caching.
- **Opaque tokens for both access and refresh:** Replace JWTs with opaque identifiers stored in Redis. This was rejected because existing services already validate JWT claims locally without additional network hops.

## Consequences
- Engineering teams must integrate Redis checks during refresh flows and implement rotation logic exactly once in shared middleware.
- Redis availability becomes critical for login and refresh; we must ensure replication and monitoring are in place.
- Documentation and client SDKs require updates to honor single-use refresh tokens and handle revocation errors gracefully.
- Additional automated testing is needed to cover Redis-based revocation paths and key rotation scenarios.

## References
- OWASP Cheat Sheet Series: JSON Web Token (JWT) Cheat Sheet
- NIST SP 800-63B: Digital Identity Guidelines — Authentication and Lifecycle Management
