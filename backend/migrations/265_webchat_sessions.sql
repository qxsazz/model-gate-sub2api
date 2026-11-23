-- Last successful completed turn defines retention; reading or reserving a turn does not extend it.
CREATE TABLE webchat_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id bigint NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    title text NOT NULL,
    model text NOT NULL,
    version integer NOT NULL CHECK (version BETWEEN 1 AND 32),
    messages jsonb NOT NULL CHECK (jsonb_array_length(messages) = 2 * version),
    content_bytes integer NOT NULL CHECK (content_bytes BETWEEN 1 AND 524288),
    created_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    updated_at timestamptz NOT NULL DEFAULT statement_timestamp(),
    lease_token uuid,
    lease_until timestamptz,
    CHECK ((lease_token IS NULL) = (lease_until IS NULL))
);
CREATE INDEX webchat_sessions_owner_activity ON webchat_sessions(user_id, api_key_id, updated_at DESC, id);
CREATE INDEX webchat_sessions_expiration ON webchat_sessions(updated_at, id);

-- Capacity tokens for in-flight first turns carry no prompt, response, model, or title.
CREATE TABLE webchat_start_leases (
    token uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id bigint NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL DEFAULT (clock_timestamp() + interval '15 minutes')
);
CREATE INDEX webchat_start_leases_owner ON webchat_start_leases(user_id, api_key_id, expires_at);
CREATE INDEX webchat_start_leases_expiration ON webchat_start_leases(expires_at, token);
