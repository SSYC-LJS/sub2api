ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_parent_account BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS parent_child_accounts (
    id BIGSERIAL PRIMARY KEY,
    parent_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    child_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    allocated_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    used_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_parent_child_accounts_different_users CHECK (parent_user_id <> child_user_id),
    CONSTRAINT chk_parent_child_accounts_quota_non_negative CHECK (allocated_quota >= 0 AND used_quota >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_parent_child_accounts_child_active
    ON parent_child_accounts (child_user_id)
    WHERE deleted_at IS NULL AND status = 'active';

CREATE INDEX IF NOT EXISTS idx_parent_child_accounts_parent_active
    ON parent_child_accounts (parent_user_id, status)
    WHERE deleted_at IS NULL;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS payer_user_id BIGINT,
    ADD COLUMN IF NOT EXISTS parent_account_id BIGINT,
    ADD COLUMN IF NOT EXISTS parent_quota_used DECIMAL(20,8) NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_logs_parent_account_created
    ON usage_logs (parent_account_id, created_at DESC)
    WHERE parent_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_logs_payer_user_created
    ON usage_logs (payer_user_id, created_at DESC)
    WHERE payer_user_id IS NOT NULL;
