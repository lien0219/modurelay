-- Persistent, user-owned canvas projects. The document contains only public
-- canvas state and asset IDs; credentials, object keys, and signed URLs are
-- deliberately excluded from this schema.
CREATE TABLE IF NOT EXISTS canvas_projects (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    title VARCHAR(160) NOT NULL DEFAULT 'Untitled canvas',
    document JSONB NOT NULL DEFAULT '{"schema_version":1,"nodes":[],"edges":[],"viewport":{"x":0,"y":0,"zoom":1}}'::jsonb,
    schema_version INTEGER NOT NULL DEFAULT 1,
    revision INTEGER NOT NULL DEFAULT 1,
    document_bytes INTEGER NOT NULL DEFAULT 0,
    last_opened_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_canvas_projects_user_updated
    ON canvas_projects (user_id, updated_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS canvas_revisions (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES canvas_projects(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    revision INTEGER NOT NULL,
    source VARCHAR(24) NOT NULL,
    schema_version INTEGER NOT NULL,
    document JSONB NOT NULL,
    document_bytes INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_canvas_revisions_project_created
    ON canvas_revisions (project_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS canvas_assets (
    id BIGSERIAL PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES canvas_projects(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    object_key VARCHAR(768) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'uploading',
    source VARCHAR(24) NOT NULL,
    source_task_id VARCHAR(96) NOT NULL DEFAULT '',
    source_image_index INTEGER,
    idempotency_key VARCHAR(128) NOT NULL DEFAULT '',
    file_name VARCHAR(255) NOT NULL DEFAULT '',
    content_type VARCHAR(128) NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    sha256 VARCHAR(64) NOT NULL DEFAULT '',
    width INTEGER,
    height INTEGER,
    delete_attempts INTEGER NOT NULL DEFAULT 0,
    delete_next_attempt_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_canvas_assets_user_idempotency
    ON canvas_assets (user_id, idempotency_key)
    WHERE idempotency_key <> '';
CREATE INDEX IF NOT EXISTS idx_canvas_assets_project_ready
    ON canvas_assets (project_id, created_at DESC, id DESC)
    WHERE status = 'ready';
CREATE INDEX IF NOT EXISTS idx_canvas_assets_delete_pending
    ON canvas_assets (delete_next_attempt_at, id)
    WHERE status = 'delete_pending';
