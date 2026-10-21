-- Hosted canvas state is scoped to the authenticated account, independently of API keys.
CREATE TABLE canvas_states (
    user_id BIGINT NOT NULL,
    name TEXT NOT NULL,
    payload JSONB NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, name)
);
CREATE TABLE canvas_assets (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL,
    storage_key TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    mime_type TEXT NOT NULL,
    byte_size BIGINT NOT NULL,
    api_key_id BIGINT,
    model TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'upload',
    review_status TEXT NOT NULL DEFAULT 'unreviewed' CHECK (review_status IN ('unreviewed', 'approved', 'blocked')),
    review_note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '7 days',
    deleted_at TIMESTAMPTZ,
    UNIQUE (user_id, storage_key)
);
CREATE TABLE canvas_asset_bindings (
    user_id BIGINT NOT NULL,
    storage_key TEXT NOT NULL,
    asset_id UUID NOT NULL REFERENCES canvas_assets(id),
    PRIMARY KEY (user_id, storage_key)
);
CREATE INDEX canvas_assets_hash ON canvas_assets(user_id,content_hash);
CREATE INDEX canvas_assets_expiry ON canvas_assets (expires_at) WHERE deleted_at IS NULL;
CREATE INDEX canvas_assets_review ON canvas_assets (created_at DESC, user_id);
CREATE TABLE canvas_reviews (
    id BIGSERIAL PRIMARY KEY,
    asset_id UUID NOT NULL REFERENCES canvas_assets(id),
    actor_user_id BIGINT NOT NULL,
    action TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE canvas_video_tasks (
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    task_id TEXT NOT NULL,
    remote_addr TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    request_host TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    model TEXT NOT NULL,
    prompt TEXT NOT NULL,
    asset_id UUID REFERENCES canvas_assets(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, api_key_id, task_id)
);
