# Task Catalogue Endpoint

## Summary
The task catalogue endpoints expose the curated work items returned by `internal/http/tasks.Handler.List`. The handler serializes the slice of `tasks.Task` values produced by the injected service into the canonical success envelope consumed by the Next.js dashboard.

## Routes
- **GET `/api/tasks`** – Public entry point used by the frontend during local development.
- **GET `/v1/tasks`** – Authenticated variant registered under the `/v1` group and protected by `middleware.Authentication`.

## Authentication
`/api/tasks` does not apply authentication. `/v1/tasks` requires an `Authorization: Bearer <token>` header that passes `internal/auth.Service.ValidateAccessToken`. Successful validation stores an `auth.Principal` on the request context for downstream handlers.

## Response Schema
Both routes return the same JSON envelope:
```json
{
  "status": "success",
  "data": {
    "items": [
      {
        "id": "string",
        "title": "string",
        "status": "string",
        "priority": "string",
        "assignee": "string",
        "due_date": "RFC3339 date"
      }
    ]
  },
  "meta": {}
}
```
- The object inside `items` mirrors the `tasks.Task` struct defined in `internal/http/tasks/handlers.go`.
- The handler currently populates an empty `meta` object because pagination is not applied.

## Behaviour
1. `tasks.Handler.List` extracts the request context (falling back to `context.Background()` when necessary) and calls the injected service's `List` method.
2. On success, it wraps the returned slice in a map keyed by `items` before delegating to `respond.Success` with HTTP status `200`.
3. When the service returns an error, the handler responds with `respond.InternalError`, yielding the canonical error envelope with status code `500`.

## Example Payload
```json
{
  "status": "success",
  "data": {
    "items": [
      {
        "id": "task-1",
        "title": "Prepare launch checklist",
        "status": "in_progress",
        "priority": "high",
        "assignee": "Ava",
        "due_date": "2024-05-01T09:00:00Z"
      }
    ]
  },
  "meta": {}
}
```
