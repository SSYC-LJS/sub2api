CREATE TABLE IF NOT EXISTS image_canvas_histories (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    api_key_name TEXT NOT NULL DEFAULT '',
    operation VARCHAR(32) NOT NULL DEFAULT 'generate',
    model TEXT NOT NULL,
    prompt TEXT NOT NULL,
    size TEXT NOT NULL DEFAULT '',
    output_format VARCHAR(32) NOT NULL DEFAULT 'png',
    image_url TEXT NOT NULL DEFAULT '',
    b64_json TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL DEFAULT 'image/png',
    source_image_url TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_image_canvas_histories_user_created
    ON image_canvas_histories (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_image_canvas_histories_api_key
    ON image_canvas_histories (api_key_id);
