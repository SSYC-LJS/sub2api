-- 154: Capture user request and gateway response bodies for audit/debugging.
-- Disabled by default at application config level; this table stores only bounded snapshots.
CREATE TABLE IF NOT EXISTS request_response_logs (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    endpoint TEXT,
    model TEXT,
    stream BOOLEAN NOT NULL DEFAULT false,
    status_code INT NOT NULL DEFAULT 0,
    request_body TEXT,
    response_body TEXT,
    request_truncated BOOLEAN NOT NULL DEFAULT false,
    response_truncated BOOLEAN NOT NULL DEFAULT false,
    request_body_bytes INT NOT NULL DEFAULT 0,
    response_body_bytes INT NOT NULL DEFAULT 0,
    duration_ms INT NOT NULL DEFAULT 0,
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_response_logs_created_at
    ON request_response_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_request_response_logs_user_created
    ON request_response_logs(user_id, created_at DESC)
    WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_request_response_logs_api_key_created
    ON request_response_logs(api_key_id, created_at DESC)
    WHERE api_key_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_request_response_logs_group_created
    ON request_response_logs(group_id, created_at DESC)
    WHERE group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_request_response_logs_request_id
    ON request_response_logs(request_id)
    WHERE request_id IS NOT NULL;
