# Bid Evaluation Endpoint

## Summary
`POST /bids/evaluate` ranks incoming driver bids for a rider and returns the winning offer. The handler is located in `backend/internal/api/server.go` and delegates the decision logic to `bidding.Arbiter.RankAndSelect` while persisting the result through the configured repository.

## Route
- **Method:** `POST`
- **Path:** `/bids/evaluate`
- **Authentication:** Required when an `auth.Validator` is configured; the handler rejects requests lacking an `Authorization` header that passes `ValidateToken`.

## Request Body
```json
{
  "request": {
    "riderID": "string",
    "tripID": "string",
    "latitude": 0,
    "longitude": 0,
    "maxETA": "duration",
    "maxPrice": 0
  },
  "bids": [
    {
      "id": "string",
      "driverID": "string",
      "tripID": "string",
      "price": 0,
      "latitude": 0,
      "longitude": 0,
      "eta": "duration",
      "expiresAt": "RFC3339 timestamp"
    }
  ]
}
```
- Field names follow Go struct tags defined in `contracts.BidRequest` and `contracts.Bid`.
- Durations are encoded using Go's `time.Duration` JSON representation (nanoseconds).

## Success Response
- **Status:** `200 OK`
- **Body:**
  ```json
  {
    "winner": { /* full bid object */ }
  }
  ```

## Failure Responses
- `400 Bad Request` when the JSON payload cannot be bound to the expected schema.
- `401 Unauthorized` when authentication fails.
- `404 Not Found` when no bids satisfy rider constraints (`RankAndSelect` returns `ok == false`).
- `500 Internal Server Error` when the arbiter surfaces an internal error.

## Implementation Notes
1. Authentication is enforced via `Server.requireAuth`, which aborts the request on failure.
2. The handler binds the JSON payload into an inline struct containing `contracts.BidRequest` and a slice of `contracts.Bid`.
3. `bidding.Arbiter.RankAndSelect` performs pricing and constraint checks before returning the winning bid.
4. The winner is persisted automatically because the arbiter invokes its repository; the handler does not need to trigger persistence explicitly.

## Reproduction Checklist
- Instantiate `bidding.Arbiter` with a repository capable of saving `contracts.AcceptedBid` values.
- Ensure `auth.Validator.ValidateToken` processes the bearer token format expected by clients.
- Call `RankAndSelect` with the decoded request and propagate any returned error or "no bids" condition to HTTP status codes as described.
- Return the winning bid inside a JSON envelope under the `winner` key.
