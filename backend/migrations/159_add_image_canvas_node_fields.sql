ALTER TABLE image_canvas_histories
    ADD COLUMN IF NOT EXISTS root_node_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS parent_node_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS node_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'completed',
    ADD COLUMN IF NOT EXISTS error_message TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_image_canvas_histories_user_root_created
    ON image_canvas_histories (user_id, root_node_id, created_at ASC, id ASC)
    WHERE deleted_at IS NULL;
