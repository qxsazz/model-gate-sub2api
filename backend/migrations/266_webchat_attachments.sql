-- Keep binary uploads and extracted document text outside history JSON and its existing text cap.
ALTER TABLE webchat_sessions
    ADD COLUMN attachment_bytes integer NOT NULL DEFAULT 0 CHECK (attachment_bytes BETWEEN 0 AND 12582912),
    ADD COLUMN extracted_bytes integer NOT NULL DEFAULT 0 CHECK (extracted_bytes BETWEEN 0 AND 524288);

CREATE TABLE webchat_attachments (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES webchat_sessions(id) ON DELETE CASCADE,
    name text NOT NULL,
    mime_type text NOT NULL,
    data bytea NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 4194304),
    extracted_text text NOT NULL DEFAULT '' CHECK (octet_length(extracted_text) <= 131072)
);
CREATE INDEX webchat_attachments_session ON webchat_attachments(session_id);
