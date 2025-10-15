# Notification Center Endpoints

## Summary
The notification center endpoints surface user alerts managed by `internal/http/notifications.Handler`. They are mounted under `/v1/notifications` and require a valid bearer token so the handler can resolve the `auth.Principal` from the request context.

## Routes
- **GET `/v1/notifications`** – Returns a paginated list of notifications for the authenticated principal. Optional query parameters `page` and `per_page` default to `1` and `20` respectively.
- **PATCH `/v1/notifications/:id`** – Marks the addressed notification as read for the authenticated principal.

## Authentication
Both routes are part of the `/v1` group registered in `routes/web.go` and therefore use `middleware.Authentication`. Requests must include `Authorization: Bearer <token>`. When validation succeeds, the middleware stores an `auth.Principal` on the Gin context; the handlers derive the `userID` from `principal.Subject`.

## Response Schemas
### GET `/v1/notifications`
```json
{
  "data": {
    "items": [
      {
        "id": "string",
        "user_id": "string",
        "title": "string",
        "body": "string",
        "read_at": "RFC3339 timestamp | null",
        "created_at": "RFC3339 timestamp"
      }
    ]
  },
  "meta": {
    "pagination": {
      "page": 1,
      "per_page": 20,
      "total": 0,
      "total_pages": 0
    }
  }
}
```
- The notification objects match the `notifications.Notification` struct in `internal/http/notifications/handlers.go`.
- The handler writes the envelope via the local `writeSuccess` helper, which omits the top-level `status` property used by `respond.Success` to remain compatible with ADR-003 payloads.

### PATCH `/v1/notifications/:id`
- **Status:** `204 No Content`
- **Body:** Empty

## Behaviour
1. `Handler.List` pulls the authenticated principal from the Gin context. Missing principals result in a `401` error with message `"missing principal"`.
2. The handler parses `page` and `per_page` query parameters via `parsePagination`, returning `400` with details on validation failures.
3. Successful requests call `Service.List` with the derived `userID`, returning a `200` response containing the notification `items` and pagination metadata.
4. `Handler.MarkRead` uses the same principal lookup, extracts the `:id` parameter, and delegates to `Service.MarkRead`. Missing IDs yield `400`; unknown notifications map to `404` with message `"notification not found"`.
5. When the service confirms the update, the handler sends `204 No Content` and flushes the status immediately.
