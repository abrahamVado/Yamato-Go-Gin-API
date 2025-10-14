DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'join_request_status') THEN
        CREATE TYPE join_request_status AS ENUM ('pending', 'approved', 'declined');
    END IF;
END
$$;

CREATE TABLE IF NOT EXISTS join_requests (
    id BIGSERIAL PRIMARY KEY,
    team_id BIGINT NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    requester_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status join_request_status NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT TIMEZONE('UTC', NOW()),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT TIMEZONE('UTC', NOW()),
    CONSTRAINT join_requests_unique_requester UNIQUE (team_id, requester_id)
);

CREATE INDEX IF NOT EXISTS idx_join_requests_team_status ON join_requests (team_id, status);
