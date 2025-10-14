# ADR 007: Localization and Message Catalog Strategy

- **Status:** Accepted
- **Date:** 2024-05-19

## Context

The application currently returns English-only messages, such as the health endpoint payload emitted by `HealthController.Status`, and the root route welcome string in `routes/web.go`. Environment configuration already exposes locale settings through `LocaleConfig`, allowing the runtime to express a default locale, supported locale list, and canonical time zone using the `LOCALE_DEFAULT`, `LOCALE_SUPPORTED`, and `LOCALE_TIMEZONE` variables. The configuration loader prioritises process environment variables, then values from a `.env` file, and finally hard-coded defaults.

To support localized responses and developer-friendly coordination, we need a repeatable catalogue format, predictable file placement, and explicit rules for message key naming.

## Decision

### Localization files

- Translation catalogues reside in `resources/locales/{locale}/messages.json` folders within the repository. Each locale folder contains one `messages.json` file, enabling Git-friendly diffs and simplifying runtime loading logic.
- Catalogues are UTF-8 encoded JSON objects where the top-level keys represent message domains (for example, `health` or `welcome`). Domain objects hold the final string resources keyed by their message identifiers.
- Localized strings should remain human-readable and avoid runtime interpolation directives that are not already supported by the Gin stack.

### Environment handling

- `internal/config.Load` remains the single source of truth for localisation settings. It reads configuration in the following order: process environment → `.env` file → struct defaults.
- `LOCALE_DEFAULT` indicates the locale to use when no locale is negotiated or explicitly provided by the caller. Missing values fall back to `en`.
- `LOCALE_SUPPORTED` accepts a comma- or semicolon-separated list. The helper `getStringSlice` normalises whitespace and removes empty entries, producing a deterministic slice that is validated by `internal/config/config_test.go`.
- `LOCALE_TIMEZONE` stores the canonical IANA time zone identifier that should accompany localized timestamps. Defaults to `UTC` when undefined.

### Message key conventions

- Message identifiers follow a dot-delimited format of `{domain}.{subject}[.{qualifier}]` to mirror the directory hierarchy and keep related strings together.
- Domains align with HTTP handler groupings (for example, `health` for `HealthController`, `root` for `/` routes) to aid in code search and reviews.
- Keys are lowercase, use hyphens only when required by an external specification, and should avoid camelCase to maintain consistency across locales.
- JSON catalogues represent these keys as nested objects. For example, the English health status string would live under `{"health": {"status.ok": "Service is healthy", "status.service": "Yamato API"}}`. Handlers map their runtime responses to these keys rather than literal strings.

## Consequences

- Introducing new locales now involves adding a directory and JSON catalogue under `resources/locales` plus wiring the locale into `LOCALE_SUPPORTED`.
- Runtime code must replace hard-coded strings (such as the health payload) with lookups against the catalogue, improving flexibility for future languages without altering handlers.
- The configuration loader already enforces deterministic behaviour through tests, so expanding locale awareness does not require new environment precedence logic.
