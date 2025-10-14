# ADR 002: Role-Based Access Control (RBAC) Strategy

## Status
Accepted

## Context
The Yamato API must regulate access to resources belonging to teams, pipelines, and deployment assets. A lightweight RBAC layer is required to map:

- **Principals** (end users, service accounts, and automated jobs) that authenticate through the existing JWT flow.
- **Roles** that define collections of permissions for team-scoped and global operations.
- **Permissions** expressed as named capabilities (for example, `pipelines.read`, `pipelines.trigger`, `teams.manage_members`).

The API already exposes middleware hooks where authorization guards can be attached, and the infrastructure team maintains a Redis cluster for distributed state.

## Decision
We will implement RBAC with the following structure and integration points.

### Role and Permission Structure
- **Role definitions** are static YAML files committed with the service. Each role declares:
  - `id`: a unique slug (for example, `team_viewer`, `team_admin`).
  - `scope`: either `team` (roles bound to a team) or `global` (organization-wide privileges).
  - `permissions`: a flat list of permission strings.
- **Permission catalog** is centralized in `internal/rbac/permissions.go` (planned) to provide compile-time references and prevent typos.
- **Role assignment** happens through a join table `team_role_memberships` that links a principal, team, and role. Global roles omit the team identifier.
- **Inheritance** is intentionally avoided to reduce evaluation complexity. Instead, composite roles (for example, `team_admin`) list the union of permissions they require.
- **Derived permissions** follow a namespace convention (`resource.verb`). Middleware performs exact string checks, while policy helpers can supply syntactic sugar (e.g., `CanTriggerPipeline(teamID)` wrapping the permission string).

### Middleware Strategy
- **Authentication middleware** enriches the Gin context with a `Principal` struct containing subject, account type, and resolved team memberships.
- **Authorization middleware** consumes declarative `Gate` descriptors that list required permissions and optional scope constraints.
- **Route integration** occurs during router setup; each protected route attaches `RequirePermissions(...)` middleware before the handler.
- **Fallback handling** returns `403 Forbidden` with a structured JSON payload when the gate fails. Denials are logged with correlation IDs for auditing.
- **Bypass paths** (such as health checks) avoid the middleware entirely to minimize latency.

### Team Join Flow
- When a user joins a team, an event is pushed into the background queue.
- A worker resolves default roles for the new member (e.g., `team_viewer`) and writes memberships into PostgreSQL.
- The membership service emits an invalidation message over Redis Pub/Sub so all API instances refresh cached permission bundles for the affected principal.

### Gate Evaluation Flow
1. **Context Extraction**: Middleware retrieves the `Principal` from the request context and determines target resources (team ID, pipeline ID, etc.).
2. **Cache Lookup**: A composite cache key (`principalID:teamID`) is used to fetch the compiled permission set from Redis. Entries include an ETag version.
3. **Staleness Check**: If the cached bundle is missing or expired (based on TTL or ETag mismatch), the middleware calls the membership service to rebuild permissions from the database.
4. **Permission Check**: The gate verifies that every required permission string exists in the compiled set. Team-scoped gates filter permissions to the matching team ID.
5. **Decision Logging**: Pass/fail outcomes with gate metadata are emitted to structured logs and optionally to an audit stream.

### Caching Strategy
- **Warm cache** during login: the authentication service preloads the user's global and default team permissions into Redis with a 15-minute TTL.
- **On-demand rebuilds** refresh the cache when middleware detects a miss or staleness.
- **Write-through updates**: after role changes, the membership service updates Redis directly to reduce latency on the next request.
- **Background sweeper** periodically deletes aged keys to prevent unbounded growth.

### Redis Interactions
- **Data structures**:
  - Hashes store permission bundles: `rbac:bundle:<principalID>` with fields per team (`team:<teamID>` => JSON encoded list).
  - Sets track global roles: `rbac:global:<principalID>` with role IDs for quick inspection.
  - Channels (`rbac:invalidate`) broadcast invalidation messages carrying principal and optional team IDs.
- **Connection handling** uses a shared pool with health checks; requests fall back to direct database reads if Redis is unavailable.
- **Observability** includes metrics for hit rate, rebuild latency, and invalidation fan-out.

## Consequences
- Centralized role definitions simplify reviews but require deployment to roll out changes.
- Redis provides low-latency permission checks, yet the system must tolerate cache outages via graceful degradation.
- Explicit team joins keep membership logic auditable and enable post-join automation hooks.

## Alternatives Considered
- **Database-only reads** were rejected because they introduce high latency under load.
- **Permission inheritance trees** were postponed due to complexity and limited immediate need.
- **JWT-embedded permissions** were skipped to avoid frequent token refreshes after role changes.

## References
- [ADR 001: Router Strategy](../adr-001-router/README.md) *(hypothetical reference for continuity)*

