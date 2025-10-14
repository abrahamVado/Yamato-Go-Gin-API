# ADR-003: Response Payload Structure

## Status
Accepted

## Context
The API currently returns heterogeneous response payloads, which makes it difficult for clients to reliably parse business data, pagination details, and validation feedback. A standard payload layout is required to support consistent documentation, simpler SDK integrations, and future localization work.

## Decision
All HTTP responses returned by the Yamato API MUST conform to one of the following canonical payload envelopes.

### Success Payload
```json
{
  "data": {},
  "meta": {}
}
```

* `data` contains the domain resource or collection returned by the endpoint.
* `meta` captures non-resource metadata. The object MUST be present even when no additional information is required.

#### Pagination Metadata
When an endpoint returns a collection, `meta.pagination` MUST be provided with the following shape:

```json
{
  "pagination": {
    "page": 1,
    "per_page": 25,
    "total_pages": 10,
    "total_records": 250,
    "has_next": true,
    "has_prev": false
  }
}
```

* `page`, `per_page`, `total_pages`, and `total_records` are integers describing the current window and total available results.
* `has_next` and `has_prev` are booleans that allow lightweight navigation checks without recomputing totals.
* Additional pagination keys (for example `next_cursor`) MAY be added when cursor-based pagination is implemented, but they MUST live under `meta.pagination`.

### Failure Payload
```json
{
  "message": "",
  "errors": {}
}
```

* `message` is a human-readable summary of the failure, localized according to the request's `Accept-Language` header (see Localization Requirements).
* `errors` is an object mapping error categories to details.

#### Validation Error Mapping
Validation failures MUST be reported under `errors.validation` using the following structure:

```json
{
  "errors": {
    "validation": {
      "field_name": [
        {
          "code": "string",
          "message": "string",
          "meta": {}
        }
      ]
    }
  }
}
```

* Each key under `validation` corresponds to a field path (e.g., `user.email` for nested attributes).
* The value is an array of error objects so that multiple validation issues per field can be expressed.
* `code` is a machine-readable identifier (e.g., `required`, `max_length_exceeded`).
* `message` is a localized explanation tailored to end users.
* `meta` contains optional structured context (e.g., `{ "limit": 255 }`).

Other error domains (authentication, authorization, rate limiting, etc.) MUST also be nested within `errors` using similarly structured objects.

## Localization Requirements
* Responses MUST honor the `Accept-Language` request header. Unsupported locales fall back to the system default language while logging the mismatch for observability.
* All user-facing strings in `message` and `errors.*[].message` MUST originate from the localization catalog; hard-coded strings are prohibited.
* Error `code` values remain language-neutral to ensure stable programmatic handling across locales.

## Consequences
* Clients can reliably parse success and error responses without endpoint-specific branching.
* Pagination behavior becomes discoverable for tooling and SDKs.
* Validation errors are machine-readable, enabling inline form feedback.
* Localization pipelines can manage translations centrally, with consistent fallbacks and logging.
