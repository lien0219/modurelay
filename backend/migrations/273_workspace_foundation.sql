-- Tenant history has its own lifetime. No cascade deletes and no usage rewrite.
CREATE TABLE IF NOT EXISTS workspaces (
 id BIGSERIAL PRIMARY KEY,
 name VARCHAR(100) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
 slug VARCHAR(63) NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
 type VARCHAR(20) NOT NULL CHECK (type IN ('personal','organization')),
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','archived')),
 owner_user_id BIGINT NOT NULL REFERENCES users(id),
 billing_owner_user_id BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (type <> 'organization' OR slug NOT LIKE 'personal-%'),
 CHECK (type <> 'personal' OR owner_user_id = billing_owner_user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS workspaces_personal_owner ON workspaces(owner_user_id) WHERE type='personal';
CREATE INDEX IF NOT EXISTS workspaces_billing_owner ON workspaces(billing_owner_user_id);
CREATE TABLE IF NOT EXISTS workspace_members (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 user_id BIGINT NOT NULL REFERENCES users(id),
 role VARCHAR(20) NOT NULL CHECK (role IN ('owner','admin','developer','billing','viewer')),
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended')),
 invited_by_user_id BIGINT REFERENCES users(id),
 joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,user_id)
);
CREATE INDEX IF NOT EXISTS workspace_members_user ON workspace_members(user_id,workspace_id);
CREATE TABLE IF NOT EXISTS projects (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 name VARCHAR(100) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
 slug VARCHAR(63) NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
 description VARCHAR(2000) NOT NULL DEFAULT '',
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
 is_default BOOLEAN NOT NULL DEFAULT false,
 created_by_user_id BIGINT NOT NULL REFERENCES users(id),
 allowed_group_ids BIGINT[],
 allowed_models TEXT[],
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,slug), UNIQUE(workspace_id,id)
);
CREATE UNIQUE INDEX IF NOT EXISTS projects_one_default ON projects(workspace_id) WHERE is_default;
CREATE TABLE IF NOT EXISTS workspace_invitations (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 email VARCHAR(320) NOT NULL CHECK (email=lower(trim(email))),
 role VARCHAR(20) NOT NULL CHECK (role IN ('owner','admin','developer','billing','viewer')),
 token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 invited_by_user_id BIGINT NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL,
 accepted_at TIMESTAMPTZ,
 revoked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (expires_at > created_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_invitations_pending_email ON workspace_invitations(workspace_id,email) WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE TABLE IF NOT EXISTS workspace_audit_logs (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 project_id BIGINT,
 actor_user_id BIGINT NOT NULL REFERENCES users(id),
 action VARCHAR(100) NOT NULL,
 target_type VARCHAR(40) NOT NULL,
 target_id BIGINT NOT NULL,
 metadata JSONB NOT NULL DEFAULT '{}',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id)
);
CREATE INDEX IF NOT EXISTS workspace_audit_scope ON workspace_audit_logs(workspace_id,id DESC);

-- Nullable addition only: old keys are populated in bounded transactions.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS project_id BIGINT REFERENCES projects(id);

CREATE OR REPLACE FUNCTION ensure_personal_workspace(p_user_id BIGINT) RETURNS BIGINT AS $$
DECLARE w_id BIGINT; user_state TEXT; user_deleted TIMESTAMPTZ;
BEGIN
 SELECT status,deleted_at INTO user_state,user_deleted FROM users WHERE id=p_user_id FOR UPDATE;
 IF NOT FOUND OR user_deleted IS NOT NULL THEN
  RAISE EXCEPTION 'user not found' USING ERRCODE='P0002';
 END IF;
 SELECT id INTO w_id FROM workspaces WHERE type='personal' AND owner_user_id=p_user_id;
 IF w_id IS NULL THEN
  INSERT INTO workspaces(name,slug,type,owner_user_id,billing_owner_user_id)
  VALUES ('Personal','personal-'||p_user_id,'personal',p_user_id,p_user_id) RETURNING id INTO w_id;
  INSERT INTO workspace_members(workspace_id,user_id,role) VALUES(w_id,p_user_id,'owner');
  INSERT INTO projects(workspace_id,name,slug,is_default,created_by_user_id) VALUES(w_id,'Default','default',true,p_user_id);
  INSERT INTO workspace_audit_logs(workspace_id,actor_user_id,action,target_type,target_id)
  VALUES(w_id,p_user_id,'workspace_created','workspace',w_id);
 END IF;
 RETURN w_id;
END; $$ LANGUAGE plpgsql;
CREATE OR REPLACE FUNCTION bootstrap_personal_workspace() RETURNS trigger AS $$
BEGIN
 PERFORM ensure_personal_workspace(NEW.id);
 RETURN NEW;
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS users_personal_workspace_bootstrap ON users;
CREATE TRIGGER users_personal_workspace_bootstrap AFTER INSERT ON users
FOR EACH ROW EXECUTE FUNCTION bootstrap_personal_workspace();

-- Enforce lifecycle obligations inside the caller's existing transaction,
-- including setup/admin/Ent/raw-SQL paths. Lock ordering is workspace ID order.
CREATE OR REPLACE FUNCTION guard_workspace_user_lifecycle() RETURNS trigger AS $$
DECLARE w RECORD;
BEGIN
 IF (NEW.deleted_at IS NOT NULL AND OLD.deleted_at IS NULL) OR (NEW.status <> 'active' AND OLD.status='active') THEN
  FOR w IN SELECT ws.* FROM workspaces ws WHERE ws.status <> 'archived' AND
   (ws.owner_user_id=OLD.id OR ws.billing_owner_user_id=OLD.id OR EXISTS
    (SELECT 1 FROM workspace_members m WHERE m.workspace_id=ws.id AND m.user_id=OLD.id))
   ORDER BY ws.id FOR UPDATE
  LOOP
   IF w.type='organization' AND (
    w.billing_owner_user_id=OLD.id OR
    (EXISTS(SELECT 1 FROM workspace_members m WHERE m.workspace_id=w.id AND m.user_id=OLD.id AND m.role='owner' AND m.status='active') AND
     NOT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=w.id AND m.user_id<>OLD.id AND m.role='owner' AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL))) THEN
    RAISE EXCEPTION 'organization billing owner or last owner must be replaced first' USING ERRCODE='23514', CONSTRAINT='workspace_user_obligation';
   END IF;
   IF NEW.deleted_at IS NOT NULL AND w.type='personal' THEN
    UPDATE workspaces SET status='archived',updated_at=now() WHERE id=w.id;
    INSERT INTO workspace_audit_logs(workspace_id,actor_user_id,action,target_type,target_id)
    VALUES(w.id,OLD.id,'workspace_archived','workspace',w.id);
   END IF;
  END LOOP;
  -- Admin deletion currently deletes the creator's keys before the user. Even
  -- those soft-deleted rows must block the unsafe transaction and roll it back.
  IF NEW.deleted_at IS NOT NULL AND EXISTS (
   SELECT 1 FROM api_keys k JOIN projects p ON p.id=k.project_id JOIN workspaces ws ON ws.id=p.workspace_id
   WHERE k.user_id=OLD.id AND ws.type='organization' AND ws.status<>'archived') THEN
   RAISE EXCEPTION 'organization key creator cannot be deleted while workspace is live' USING ERRCODE='23514', CONSTRAINT='workspace_user_obligation';
  END IF;
 END IF;
 RETURN NEW;
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS users_workspace_lifecycle_guard ON users;
CREATE TRIGGER users_workspace_lifecycle_guard BEFORE UPDATE OF status,deleted_at ON users
FOR EACH ROW EXECUTE FUNCTION guard_workspace_user_lifecycle();
