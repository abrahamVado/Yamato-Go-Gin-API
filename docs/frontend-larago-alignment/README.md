# Larago Frontend Integration Audit

## Context
- The Go service now brands itself as **Larago**, updating the welcome banner and diagnostics metadata accordingly.【F:routes/web.go†L35-L42】【F:config/app.go†L11-L24】
- The Next.js frontend stitches API URLs by trimming `NEXT_PUBLIC_API_BASE_URL` and appending relative paths, so any mismatch between the expected and actual prefixes surfaces immediately in production.【F:frontend/web/src/lib/api-client.ts†L3-L75】

## Authentication flows
- Login and registration screens call `auth/login` and `auth/register` through the shared API client, while the sign-in API route proxies directly to `/auth/login` on the configured backend.【F:frontend/web/src/app/(public)/login/page.tsx†L54-L86】【F:frontend/web/src/app/(public)/register/page.tsx†L110-L137】【F:frontend/web/src/app/private/api/auth/signin/route.ts†L3-L38】
- Larago, however, exposes these handlers under the versioned `/v1/auth` prefix (e.g., `/v1/auth/login`, `/v1/auth/register`, `/v1/auth/logout`).【F:internal/httpserver/router.go†L10-L23】【F:docs/api/api.md†L20-L35】
- **Action:** either publish Larago behind a base URL that already ends with `/v1` (for example, `https://api.example.com/v1`) or teach the frontend to include the `/v1` prefix when resolving authentication paths.

## Current principal lookup
- The user navigation fetches `user` without a versioned prefix to populate the profile menu.【F:frontend/web/src/components/private/user-nav.tsx†L100-L134】
- Larago’s equivalent lives at `/v1/user`, registered alongside the auth routes.【F:internal/httpserver/router.go†L10-L23】【F:docs/api/api.md†L31-L35】
- **Action:** align the frontend path with `/v1/user` or expose an unversioned alias on the backend to preserve compatibility.

## Notifications API
- The notifications hook performs `GET notifications` and `PATCH notifications` requests, passing the target ID inside the JSON body.【F:frontend/web/src/components/notifications/use-notifications.ts†L11-L38】
- Larago serves notifications at `/v1/notifications` and requires the notification identifier in the URL path for updates (PATCH `/v1/notifications/{id}`).【F:internal/http/notifications/handlers.go†L89-L155】【F:internal/http/notifications/handlers_test.go†L100-L152】【F:docs/api/api.md†L37-L44】
- **Action:** update the frontend to request `/v1/notifications` and issue `PATCH /v1/notifications/{id}` calls, removing the redundant `id` payload field.

## Email verification and resend
- The verification screen expects Laravel-style endpoints: `GET email/verify/{id}/{hash}` plus `POST email/verification-notification` for resends.【F:frontend/web/src/app/(public)/verify-email/page.tsx†L101-L195】
- Larago’s published API catalog does not include matching email verification endpoints, so these flows cannot complete against the Go backend today.【F:docs/api/api.md†L11-L91】
- **Action:** implement Larago equivalents for the verification and resend routes or provide a dedicated compatibility shim for the frontend until the Go service adds parity.

## Summary of required adjustments
1. //1.- Ensure the deployed Larago base URL includes `/v1` or modify the frontend API client to inject the versioned prefix for authentication and user lookups.
2. //2.- Bring the notifications hook in line with Larago’s `/v1/notifications` routing and path-parameter semantics.
3. //3.- Add Larago endpoints for email verification and resend flows, or proxy the existing Laravel implementation until those handlers are ported.
4. //4.- Confirm any future admin integrations honor Larago’s route structure before wiring additional frontend modules.
