-- Phase A: tenant-owned teams and project access grants.
-- Existing workspaces intentionally remain all-projects compatible. The access
-- mode is an explicit control-plane switch; no historical usage is rewritten.
ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS project_access_mode VARCHAR(20) NOT NULL DEFAULT 'all_projects'
    CHECK (project_access_mode IN ('all_projects', 'assigned_projects'));

-- Composite ownership keys let the database validate that a child record never
-- crosses workspace boundaries while keeping the public IDs opaque to callers.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'workspace_members_workspace_id_id_key'
    ) THEN
        ALTER TABLE workspace_members
            ADD CONSTRAINT workspace_members_workspace_id_id_key UNIQUE (workspace_id, id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS workspace_teams (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
    slug VARCHAR(63) NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
    description VARCHAR(2000) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, id),
    UNIQUE (workspace_id, slug)
);
CREATE INDEX IF NOT EXISTS workspace_teams_workspace_status ON workspace_teams(workspace_id, status, id DESC);

CREATE TABLE IF NOT EXISTS workspace_team_members (
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    team_id BIGINT NOT NULL,
    workspace_member_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, team_id, workspace_member_id),
    FOREIGN KEY (workspace_id, team_id) REFERENCES workspace_teams(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, workspace_member_id) REFERENCES workspace_members(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_team_members_member ON workspace_team_members(workspace_id, workspace_member_id, team_id);

CREATE TABLE IF NOT EXISTS project_access_grants (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    project_id BIGINT NOT NULL,
    subject_type VARCHAR(20) NOT NULL CHECK (subject_type IN ('member', 'team')),
    subject_id BIGINT NOT NULL CHECK (subject_id > 0),
    role VARCHAR(20) NOT NULL CHECK (role IN ('viewer', 'developer', 'admin')),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, project_id, subject_type, subject_id),
    FOREIGN KEY (workspace_id, project_id) REFERENCES projects(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS project_access_grants_subject ON project_access_grants(workspace_id, subject_type, subject_id, project_id);
CREATE INDEX IF NOT EXISTS project_access_grants_project ON project_access_grants(workspace_id, project_id, role, id DESC);

-- Subject IDs are intentionally polymorphic, so ordinary foreign keys cannot
-- prove that a member or team belongs to the same workspace. Keep that
-- invariant at the database boundary as well as in the repository mutation
-- transaction; this protects controlled SQL maintenance paths from IDOR.
CREATE OR REPLACE FUNCTION validate_project_access_grant_subject() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.subject_type = 'member' THEN
        IF NOT EXISTS (
            SELECT 1 FROM workspace_members
            WHERE workspace_id = NEW.workspace_id AND id = NEW.subject_id
        ) THEN
            RAISE EXCEPTION 'project access member subject belongs to another workspace'
                USING ERRCODE = '23514', CONSTRAINT = 'project_access_grant_subject_scope';
        END IF;
    ELSIF NEW.subject_type = 'team' THEN
        IF NOT EXISTS (
            SELECT 1 FROM workspace_teams
            WHERE workspace_id = NEW.workspace_id AND id = NEW.subject_id
        ) THEN
            RAISE EXCEPTION 'project access team subject belongs to another workspace'
                USING ERRCODE = '23514', CONSTRAINT = 'project_access_grant_subject_scope';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS project_access_grants_subject_scope ON project_access_grants;
CREATE TRIGGER project_access_grants_subject_scope
BEFORE INSERT OR UPDATE OF workspace_id, subject_type, subject_id ON project_access_grants
FOR EACH ROW EXECUTE FUNCTION validate_project_access_grant_subject();

-- Extend the immutable domain-event allowlist for Phase A identifiers without
-- allowing arbitrary objects or credentials into durable event payloads.
CREATE OR REPLACE FUNCTION domain_event_payload_is_safe(value JSONB) RETURNS BOOLEAN
LANGUAGE SQL IMMUTABLE STRICT AS $$
 SELECT CASE
  WHEN jsonb_typeof(value) IS DISTINCT FROM 'object'
    OR jsonb_typeof(value->'data') IS DISTINCT FROM 'object'
    OR jsonb_typeof(value->'subject') IS DISTINCT FROM 'object' THEN FALSE
  ELSE
   NOT EXISTS (SELECT 1 FROM jsonb_object_keys(value) AS k(key)
    WHERE key NOT IN ('id','type','version','created_at','workspace_id','project_id','actor_user_id','subject','data'))
   AND NOT EXISTS (SELECT 1 FROM jsonb_object_keys(value->'subject') AS k(key) WHERE key NOT IN ('type','id'))
   AND NOT EXISTS (SELECT 1 FROM jsonb_each(value->'data') AS d(key,field)
    WHERE key NOT IN ('name','slug','status','role','user_id','member_id','invitation_id','key_id','key_name',
     'service_account_id','credential_id','credential_name','old_credential_id','new_credential_id','expires_at',
     'project_id','workspace_id','scope_type','scope_id','period_start','period_end','policy_revision','threshold',
     'quota_type','used','limit','amount','spent','reserved','estimated_amount','actual_amount','reason_code',
     'request_id','task_id','model','platform','previous_status','delivery_id','category','team_id','grant_id',
     'subject_type','project_access_mode')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;
