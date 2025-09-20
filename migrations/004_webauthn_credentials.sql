-- WebAuthn credentials
CREATE TABLE IF NOT EXISTS webauthn_credentials (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_id BYTEA NOT NULL UNIQUE,
  public_key BYTEA NOT NULL,
  sign_count INT NOT NULL DEFAULT 0,
  transports JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
