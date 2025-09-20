# Yamato Auth (Go + Gin) — Scaffold

**Decisions (locked):**
- IDs: **BIGINT**
- JWT: **HS256** (`JWT_SECRET`)
- Cookies: **cross-site** (`SameSite=None; Secure`), CORS required

This is a runnable skeleton that mounts all contract routes and returns **501 Not Implemented**,
plus `/healthz` and `/readyz`. It wires CORS, security headers, request ID, and cookie helpers
aligned with your contract. Fill handlers incrementally.

## Quick start
```bash
go mod tidy
cp .env.example .env
go run ./cmd/server
# open http://localhost:8083/healthz
```

## Env
See `.env.example` for keys and docs.

## Routes mounted
- `POST  /auth/register`
- `POST  /auth/login`
- `POST  /auth/refresh`
- `POST  /auth/logout`
- `GET   /auth/session`
- `GET   /auth/oauth/:provider`
- `GET   /auth/oauth/:provider/callback`
- `POST  /auth/mfa/webauthn/begin`
- `POST  /auth/mfa/webauthn/finish`
- `POST  /auth/magic-link`
- `POST  /auth/magic-link/consume`
- `GET   /auth/oidc/.well-known/jwks.json`
- `GET   /healthz`
- `GET   /readyz`

## Next steps
- Implement DB migrations in `migrations/` (BIGINT schema provided).
- Implement handlers in `internal/http/handlers/auth.go` (replace 501s).
- Use `internal/auth/jwt.go` + `internal/auth/cookies.go` helpers to set cookies and sign HS256 tokens.
- Add persistence in `internal/store/` using Postgres (pgx or sqlx).

## Contract
Keep the error envelope shape:
```json
{ "error": { "code": "SOME_CODE", "message": "Human message" } }
```
