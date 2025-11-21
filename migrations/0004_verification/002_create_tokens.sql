-- 1.- Device tokens for passwordless access.
CREATE TABLE IF NOT EXISTS device_tokens (
    id           BIGSERIAL PRIMARY KEY,
    phone        TEXT NOT NULL,                 -- same phone as phone_verifications
    token        TEXT NOT NULL UNIQUE,          -- random, long secret
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS device_tokens_phone_idx ON device_tokens (phone);