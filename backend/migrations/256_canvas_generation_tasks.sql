CREATE TABLE canvas_generation_tasks (
 id UUID PRIMARY KEY, user_id BIGINT NOT NULL, api_key_id BIGINT NOT NULL,
 path TEXT NOT NULL, content_type TEXT NOT NULL, request_body BYTEA,
 request_hash TEXT NOT NULL, metadata JSONB NOT NULL DEFAULT '{}',
 remote_addr TEXT NOT NULL DEFAULT '', user_agent TEXT NOT NULL DEFAULT '', request_host TEXT NOT NULL DEFAULT '',
 status TEXT NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','completed','failed','interrupted')),
 cancel_requested BOOLEAN NOT NULL DEFAULT false,
 response_status INTEGER NOT NULL DEFAULT 0, response_body JSONB, error TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at TIMESTAMPTZ NOT NULL DEFAULT now()+interval '7 days'
);
CREATE INDEX canvas_generation_tasks_user ON canvas_generation_tasks(user_id,created_at DESC);
CREATE INDEX canvas_generation_tasks_queue ON canvas_generation_tasks(created_at) WHERE status='queued';
ALTER TABLE canvas_assets ADD COLUMN file_deleted_at TIMESTAMPTZ;
-- Expired assets previously marked deleted have already had their files removed.
UPDATE canvas_assets SET file_deleted_at=deleted_at WHERE deleted_at IS NOT NULL;
