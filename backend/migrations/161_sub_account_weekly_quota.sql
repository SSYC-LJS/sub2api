ALTER TABLE parent_child_accounts
    ADD COLUMN IF NOT EXISTS weekly_allocated_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS weekly_used_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS weekly_window_start TIMESTAMPTZ;

ALTER TABLE parent_child_accounts
    DROP CONSTRAINT IF EXISTS chk_parent_child_accounts_weekly_quota_non_negative;

ALTER TABLE parent_child_accounts
    ADD CONSTRAINT chk_parent_child_accounts_weekly_quota_non_negative
    CHECK (weekly_allocated_quota >= 0 AND weekly_used_quota >= 0);
