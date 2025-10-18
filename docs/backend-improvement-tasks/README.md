# Backend Improvement Tasks

## Task 1: Implement Structured Validation for Auth Endpoints
- **Problem:** The auth handlers perform ad-hoc trimming and manual empty checks, but they never surface field-level feedback to clients; the placeholder map returned on validation failure is always empty (for example in `internal/http/auth/handlers.go`). This makes it impossible for the frontend to highlight which field failed and diverges from the `respond` envelope conventions used elsewhere.
- **Why it matters:** Without actionable validation errors, form UX suffers and API clients must guess the issue, increasing support load.
- **Key steps:**
  1. Introduce a reusable validator component (wrapping `github.com/go-playground/validator/v10`) in `internal/http` and register it inside the auth handlers.
  2. Replace the manual checks in `Register`, `Login`, `Refresh`, and `Logout` with struct tags and centralized validation helpers that emit ADR-003 compliant error payloads via `respond.Error`.
  3. Add focused tests in `internal/http/auth/handlers_test.go` that assert both the HTTP status code and the returned field-level error payloads for missing/invalid inputs.
- **Definition of done:** Auth endpoints reject invalid payloads with detailed errors, unit tests cover validation branches, and the responses use the shared envelope/middleware pipeline.

## Task 2: Persist Tasks in Postgres Instead of Memory
- **Problem:** `internal/platform/memory/tasks.go` currently serves a fixed slice guarded by a mutex. There is no migration or repository backing, so the `/api/tasks` feed never reflects real data and cannot be shared between environments.
- **Why it matters:** Production usage requires tasks to be durable and queryable; operators need to manage tasks without redeploying the service.
- **Key steps:**
  1. Add a migration under `migrations/` that creates a `tasks` table with fields matching `internal/http/tasks/Task` (ID, title, status, priority, assignee, due date).
  2. Implement a Postgres-backed repository in `internal/storage/tasks` that satisfies the `tasks.Service` interface and supports sorting/filtering hooks for future growth.
  3. Wire the new repository in the bootstrap sequence (e.g., `cmd` or `internal/httpserver`) so that `/api/tasks` uses the database when configured, while retaining the in-memory service as a fallback for tests.
  4. Extend `tests/router_test.go` (or add new integration coverage) to exercise the endpoint against a temporary database, confirming the migration and repository integration.
- **Definition of done:** `/api/tasks` reads from Postgres, migrations apply cleanly, and automated tests cover the repository-backed flow.

## Task 3: Standardize Error Envelopes Across Auth Handlers
- **Problem:** Auth handlers define bespoke `writeSuccess`/`writeError` helpers that bypass the global error middleware and return payloads inconsistent with `internal/http/respond`. This leads to duplicated envelope logic and forces clients to handle two formats.
- **Why it matters:** Maintaining two response formats increases maintenance risk and complicates frontend consumption. Centralizing on `respond` ensures traceability via middleware and uniform JSON structures.
- **Key steps:**
  1. Refactor `internal/http/auth/handlers.go` to delegate success and error serialization to `respond.Success` and `respond.Error` (or dedicated thin wrappers that feed the middleware).
  2. Update the auth tests to expect the `{ "status": "success" | "error" }` envelopes emitted by `respond`.
  3. Remove the bespoke envelope structs once tests pass, ensuring no dead code lingers.
- **Definition of done:** Auth responses flow through the shared middleware, tests assert the standardized envelopes, and there is only one codepath for rendering API responses.
