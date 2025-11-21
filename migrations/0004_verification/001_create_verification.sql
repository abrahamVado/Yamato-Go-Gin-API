-- 1.- Create the phone_verifications table to store OTP codes per phone.
CREATE TABLE IF NOT EXISTS phone_verifications (
    id          BIGSERIAL PRIMARY KEY,          -- auto-incrementing ID
    phone       TEXT NOT NULL,                  -- E.164 like +52...
    code        TEXT NOT NULL,                  -- verification code (e.g. 6 digits)
    status      TEXT NOT NULL,                  -- 'pending' | 'verified' | 'expired'
    name        TEXT NOT NULL,                  -- identified name
    expires_at  TIMESTAMPTZ NOT NULL,           -- when this code stops being valid
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2.- Speed up lookups by phone and recency (latest code per phone).
CREATE INDEX IF NOT EXISTS phone_verifications_phone_created_idx
    ON phone_verifications (phone, created_at DESC);

-- 3.- Help queries that filter by active status (pending/verified) and expiry.
CREATE INDEX IF NOT EXISTS phone_verifications_status_expiry_idx
    ON phone_verifications (phone, status, expires_at DESC);
