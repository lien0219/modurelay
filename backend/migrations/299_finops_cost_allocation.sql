-- Phase F: tenant-scoped FinOps allocation configuration and immutable snapshots.
-- This migration is append-only. It intentionally does not update usage_logs.

CREATE TABLE IF NOT EXISTS workspace_cost_centers (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 code VARCHAR(63) NOT NULL CHECK (code ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
 name VARCHAR(120) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
 description VARCHAR(2000) NOT NULL DEFAULT '' CHECK (length(description) <= 2000),
 status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
 created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 archived_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,code),
 UNIQUE(workspace_id,id),
 CHECK ((status='active' AND archived_at IS NULL) OR (status='archived' AND archived_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS workspace_cost_centers_workspace_status
 ON workspace_cost_centers(workspace_id,status,code);

CREATE TABLE IF NOT EXISTS workspace_allocation_tags (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 tag_key VARCHAR(63) NOT NULL CHECK (tag_key ~ '^[a-z0-9][a-z0-9_.-]{0,62}$'),
 tag_value VARCHAR(255) NOT NULL CHECK (length(trim(tag_value)) BETWEEN 1 AND 255),
 description VARCHAR(500) NOT NULL DEFAULT '' CHECK (length(description) <= 500),
 status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
 created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 archived_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,tag_key,tag_value),
 UNIQUE(workspace_id,id),
 CHECK (tag_key !~* '(secret|token|password|credential|api[_-]?key)'),
 CHECK ((status='active' AND archived_at IS NULL) OR (status='archived' AND archived_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS workspace_allocation_tags_workspace_status
 ON workspace_allocation_tags(workspace_id,status,tag_key,tag_value);

CREATE TABLE IF NOT EXISTS project_cost_allocations (
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 cost_center_id BIGINT,
 environment VARCHAR(80) NOT NULL DEFAULT 'development',
 policy_revision BIGINT NOT NULL DEFAULT 1 CHECK (policy_revision > 0),
 updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(workspace_id,project_id),
 UNIQUE(project_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,cost_center_id) REFERENCES workspace_cost_centers(workspace_id,id),
 CHECK (environment IN ('production','staging','development','testing'))
);
CREATE INDEX IF NOT EXISTS project_cost_allocations_workspace_center
 ON project_cost_allocations(workspace_id,cost_center_id,environment);

CREATE TABLE IF NOT EXISTS project_cost_allocation_tags (
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 tag_id BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(workspace_id,project_id,tag_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES project_cost_allocations(workspace_id,project_id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,tag_id) REFERENCES workspace_allocation_tags(workspace_id,id) ON DELETE RESTRICT
);

-- API keys predate the tenant composite model and expose a global primary key.
-- This supporting uniqueness lets the override table enforce its project binding
-- at the database boundary as well as in the repository admission check.
CREATE UNIQUE INDEX IF NOT EXISTS api_keys_id_project_unique ON api_keys(id,project_id);

CREATE TABLE IF NOT EXISTS api_key_allocation_overrides (
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 api_key_id BIGINT PRIMARY KEY,
 cost_center_id BIGINT,
 environment VARCHAR(80) NOT NULL DEFAULT 'development',
 policy_revision BIGINT NOT NULL DEFAULT 1 CHECK (policy_revision > 0),
 updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,api_key_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,cost_center_id) REFERENCES workspace_cost_centers(workspace_id,id),
 FOREIGN KEY(api_key_id,project_id) REFERENCES api_keys(id,project_id) ON DELETE CASCADE,
 CHECK (environment IN ('production','staging','development','testing'))
);
CREATE INDEX IF NOT EXISTS api_key_allocation_overrides_scope
 ON api_key_allocation_overrides(workspace_id,project_id,api_key_id);

CREATE TABLE IF NOT EXISTS api_key_allocation_override_tags (
 workspace_id BIGINT NOT NULL,
 api_key_id BIGINT NOT NULL,
 tag_id BIGINT NOT NULL,
 PRIMARY KEY(workspace_id,api_key_id,tag_id),
 FOREIGN KEY(workspace_id,api_key_id) REFERENCES api_key_allocation_overrides(workspace_id,api_key_id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,tag_id) REFERENCES workspace_allocation_tags(workspace_id,id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS service_account_allocation_overrides (
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 service_account_id BIGINT PRIMARY KEY,
 cost_center_id BIGINT,
 environment VARCHAR(80) NOT NULL DEFAULT 'development',
 policy_revision BIGINT NOT NULL DEFAULT 1 CHECK (policy_revision > 0),
 updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(workspace_id,service_account_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,cost_center_id) REFERENCES workspace_cost_centers(workspace_id,id),
 FOREIGN KEY(service_account_id,project_id) REFERENCES service_accounts(id,project_id) ON DELETE CASCADE,
 CHECK (environment IN ('production','staging','development','testing'))
);
CREATE INDEX IF NOT EXISTS service_account_allocation_overrides_scope
 ON service_account_allocation_overrides(workspace_id,project_id,service_account_id);

CREATE TABLE IF NOT EXISTS service_account_allocation_override_tags (
 workspace_id BIGINT NOT NULL,
 service_account_id BIGINT NOT NULL,
 tag_id BIGINT NOT NULL,
 PRIMARY KEY(workspace_id,service_account_id,tag_id),
 FOREIGN KEY(workspace_id,service_account_id) REFERENCES service_account_allocation_overrides(workspace_id,service_account_id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,tag_id) REFERENCES workspace_allocation_tags(workspace_id,id) ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION finops_allocation_tags_are_safe(value JSONB) RETURNS BOOLEAN
LANGUAGE SQL IMMUTABLE STRICT AS $$
 SELECT jsonb_typeof(value)='object'
   AND (SELECT count(*) FROM jsonb_object_keys(value)) <= 32
   AND NOT EXISTS (
     SELECT 1 FROM jsonb_each(value) AS e(key,field)
     WHERE key !~ '^[a-z0-9][a-z0-9_.-]{0,62}$'
        OR key ~* '(secret|token|password|credential|api[_-]?key)'
        OR jsonb_typeof(field) <> 'string'
        OR length(field#>>'{}') < 1 OR length(field#>>'{}') > 255
   )
$$;

CREATE TABLE IF NOT EXISTS budget_reservation_allocation_snapshots (
 reservation_id UUID PRIMARY KEY REFERENCES budget_reservations(id) ON DELETE CASCADE,
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 billing_principal_user_id BIGINT NOT NULL REFERENCES users(id),
 cost_center_id BIGINT,
 environment VARCHAR(80) NOT NULL,
 allocation_tags JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (finops_allocation_tags_are_safe(allocation_tags)),
 policy_revision BIGINT NOT NULL DEFAULT 0 CHECK (policy_revision >= 0),
 allocation_source VARCHAR(32) NOT NULL DEFAULT 'unallocated' CHECK (allocation_source IN ('api_key_override','service_account_override','project_default','unallocated')),
 cost_center_code VARCHAR(63) NOT NULL DEFAULT '',
 cost_center_name VARCHAR(120) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(reservation_id,workspace_id,project_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id),
 FOREIGN KEY(workspace_id,cost_center_id) REFERENCES workspace_cost_centers(workspace_id,id),
 CHECK (environment IN ('unallocated','production','staging','development','testing'))
);
CREATE INDEX IF NOT EXISTS budget_reservation_allocation_scope
 ON budget_reservation_allocation_snapshots(workspace_id,project_id,created_at DESC);

CREATE TABLE IF NOT EXISTS usage_allocation_snapshots (
 usage_log_id BIGINT PRIMARY KEY REFERENCES usage_logs(id) ON DELETE CASCADE,
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 billing_principal_user_id BIGINT NOT NULL REFERENCES users(id),
 api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
 service_account_id BIGINT,
 cost_center_id BIGINT,
 environment VARCHAR(80) NOT NULL,
 allocation_tags JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (finops_allocation_tags_are_safe(allocation_tags)),
 policy_revision BIGINT NOT NULL DEFAULT 0 CHECK (policy_revision >= 0),
 allocation_source VARCHAR(32) NOT NULL DEFAULT 'unallocated' CHECK (allocation_source IN ('api_key_override','service_account_override','project_default','unallocated')),
 cost_center_code VARCHAR(63) NOT NULL DEFAULT '',
 cost_center_name VARCHAR(120) NOT NULL DEFAULT '',
 actual_cost NUMERIC(30,10) NOT NULL CHECK (actual_cost >= 0 AND actual_cost < 'Infinity'::numeric AND actual_cost <> 'NaN'::numeric),
 created_at TIMESTAMPTZ NOT NULL,
 captured_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(usage_log_id,workspace_id,project_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id),
 FOREIGN KEY(workspace_id,cost_center_id) REFERENCES workspace_cost_centers(workspace_id,id),
 CHECK (environment IN ('unallocated','production','staging','development','testing'))
);
CREATE INDEX IF NOT EXISTS usage_allocation_snapshots_scope_time
 ON usage_allocation_snapshots(workspace_id,project_id,created_at DESC,usage_log_id DESC);
CREATE INDEX IF NOT EXISTS usage_allocation_snapshots_center_time
 ON usage_allocation_snapshots(workspace_id,cost_center_id,created_at DESC);
CREATE INDEX IF NOT EXISTS usage_allocation_snapshots_environment_time
 ON usage_allocation_snapshots(workspace_id,environment,created_at DESC);

-- Legacy tenant-attributed rows may predate allocation snapshots. Keep the
-- bounded report fallback on the same scope/time index; NULL-scope historical
-- rows remain intentionally outside a workspace report.
CREATE INDEX IF NOT EXISTS usage_logs_allocation_scope_time
 ON usage_logs(workspace_id,project_id,created_at,id)
 WHERE workspace_id IS NOT NULL AND project_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS usage_allocation_hourly_rollups (
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 bucket_start TIMESTAMPTZ NOT NULL,
 cost_center_id BIGINT,
 cost_center_key BIGINT GENERATED ALWAYS AS (COALESCE(cost_center_id,0)) STORED,
 environment VARCHAR(80) NOT NULL,
 request_count BIGINT NOT NULL CHECK (request_count >= 0),
 actual_cost NUMERIC(30,10) NOT NULL CHECK (actual_cost >= 0 AND actual_cost < 'Infinity'::numeric),
 PRIMARY KEY(workspace_id,project_id,bucket_start,cost_center_key,environment),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id),
 FOREIGN KEY(workspace_id,cost_center_id) REFERENCES workspace_cost_centers(workspace_id,id),
 CHECK (bucket_start=date_trunc('hour',bucket_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')
);
CREATE INDEX IF NOT EXISTS usage_allocation_hourly_rollups_scope_time
 ON usage_allocation_hourly_rollups(workspace_id,project_id,bucket_start);

CREATE OR REPLACE FUNCTION guard_finops_allocation_snapshot_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  RAISE EXCEPTION 'finops allocation snapshot is immutable' USING ERRCODE='23514';
 END IF;
 RETURN OLD;
END $$;
DROP TRIGGER IF EXISTS budget_reservation_allocation_snapshot_guard ON budget_reservation_allocation_snapshots;
CREATE TRIGGER budget_reservation_allocation_snapshot_guard
 BEFORE UPDATE OR DELETE ON budget_reservation_allocation_snapshots
 FOR EACH ROW EXECUTE FUNCTION guard_finops_allocation_snapshot_immutable();
DROP TRIGGER IF EXISTS usage_allocation_snapshot_guard ON usage_allocation_snapshots;
CREATE TRIGGER usage_allocation_snapshot_guard
 BEFORE UPDATE OR DELETE ON usage_allocation_snapshots
 FOR EACH ROW EXECUTE FUNCTION guard_finops_allocation_snapshot_immutable();

CREATE OR REPLACE FUNCTION capture_usage_allocation_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE snap RECORD;
BEGIN
 IF NEW.workspace_id IS NULL OR NEW.project_id IS NULL OR NEW.budget_reservation_id IS NULL THEN
  RETURN NEW;
 END IF;
 -- Async video may first persist a zero-cost placeholder and fill the final
 -- amount during validated settlement. Capture the allocation at settlement,
 -- while normal zero-cost usage rows remain reportable.
 IF TG_OP='INSERT' AND NEW.video_count > 0 AND NEW.actual_cost=0
    AND NEW.total_cost > 0 AND NEW.rate_multiplier > 0 THEN
  RETURN NEW;
 END IF;
 SELECT r.workspace_id,r.project_id,r.billing_principal_user_id,
        COALESCE(a.cost_center_id,NULL)::bigint AS cost_center_id,
        COALESCE(a.environment,'unallocated') AS environment,
        COALESCE(a.allocation_tags,'{}'::jsonb) AS allocation_tags,
        COALESCE(a.allocation_source,'unallocated') AS allocation_source,
        COALESCE(a.cost_center_code,'') AS cost_center_code,COALESCE(a.cost_center_name,'') AS cost_center_name,
        COALESCE(a.policy_revision,0)::bigint AS policy_revision
   INTO snap
   FROM budget_reservations r
   LEFT JOIN budget_reservation_allocation_snapshots a ON a.reservation_id=r.id
  WHERE r.id=NEW.budget_reservation_id
    AND r.workspace_id=NEW.workspace_id AND r.project_id=NEW.project_id
    AND r.billing_principal_user_id=NEW.billing_principal_user_id;
 IF NOT FOUND THEN
  RETURN NEW;
 END IF;
 INSERT INTO usage_allocation_snapshots(
   usage_log_id,workspace_id,project_id,billing_principal_user_id,api_key_id,service_account_id,
   cost_center_id,environment,allocation_tags,policy_revision,actual_cost,created_at,allocation_source,cost_center_code,cost_center_name)
 VALUES(NEW.id,snap.workspace_id,snap.project_id,snap.billing_principal_user_id,NEW.api_key_id,NEW.service_account_id,
   snap.cost_center_id,snap.environment,snap.allocation_tags,snap.policy_revision,NEW.actual_cost,NEW.created_at,snap.allocation_source,snap.cost_center_code,snap.cost_center_name)
 ON CONFLICT(usage_log_id) DO NOTHING;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS usage_logs_allocation_snapshot_capture ON usage_logs;
CREATE TRIGGER usage_logs_allocation_snapshot_capture
 AFTER INSERT OR UPDATE OF workspace_id,project_id,billing_principal_user_id,budget_reservation_id,actual_cost ON usage_logs
 FOR EACH ROW EXECUTE FUNCTION capture_usage_allocation_snapshot();

CREATE OR REPLACE FUNCTION update_usage_allocation_hourly_rollup() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE bucket TIMESTAMPTZ;
BEGIN
 IF TG_OP='DELETE' THEN
  bucket := date_trunc('hour',OLD.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';
  UPDATE usage_allocation_hourly_rollups
     SET request_count=request_count-1,actual_cost=actual_cost-OLD.actual_cost
   WHERE workspace_id=OLD.workspace_id AND project_id=OLD.project_id AND bucket_start=bucket
     AND cost_center_key=COALESCE(OLD.cost_center_id,0) AND environment=OLD.environment;
  DELETE FROM usage_allocation_hourly_rollups
   WHERE workspace_id=OLD.workspace_id AND project_id=OLD.project_id AND bucket_start=bucket
     AND cost_center_key=COALESCE(OLD.cost_center_id,0) AND environment=OLD.environment
     AND request_count=0;
  RETURN OLD;
 END IF;
 bucket := date_trunc('hour',NEW.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';
 INSERT INTO usage_allocation_hourly_rollups(workspace_id,project_id,bucket_start,cost_center_id,environment,request_count,actual_cost)
 VALUES(NEW.workspace_id,NEW.project_id,bucket,NEW.cost_center_id,NEW.environment,1,NEW.actual_cost)
 ON CONFLICT(workspace_id,project_id,bucket_start,cost_center_key,environment)
 DO UPDATE SET request_count=usage_allocation_hourly_rollups.request_count+1,
               actual_cost=usage_allocation_hourly_rollups.actual_cost+EXCLUDED.actual_cost;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS usage_allocation_snapshot_rollup_capture ON usage_allocation_snapshots;
CREATE TRIGGER usage_allocation_snapshot_rollup_capture
 AFTER INSERT ON usage_allocation_snapshots FOR EACH ROW EXECUTE FUNCTION update_usage_allocation_hourly_rollup();
DROP TRIGGER IF EXISTS usage_allocation_snapshot_rollup_delete ON usage_allocation_snapshots;
CREATE TRIGGER usage_allocation_snapshot_rollup_delete
 AFTER DELETE ON usage_allocation_snapshots FOR EACH ROW EXECUTE FUNCTION update_usage_allocation_hourly_rollup();

CREATE OR REPLACE FUNCTION guard_finops_allocation_reference_archive() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status='archived' AND OLD.status<>'archived' THEN
  IF TG_TABLE_NAME='workspace_cost_centers' AND (
      EXISTS(SELECT 1 FROM project_cost_allocations WHERE workspace_id=OLD.workspace_id AND cost_center_id=OLD.id)
   OR EXISTS(SELECT 1 FROM api_key_allocation_overrides WHERE workspace_id=OLD.workspace_id AND cost_center_id=OLD.id)
   OR EXISTS(SELECT 1 FROM service_account_allocation_overrides WHERE workspace_id=OLD.workspace_id AND cost_center_id=OLD.id)) THEN
   RAISE EXCEPTION 'cost center is still used by active allocation configuration' USING ERRCODE='23514';
  ELSIF TG_TABLE_NAME='workspace_allocation_tags' AND (
      EXISTS(SELECT 1 FROM project_cost_allocation_tags WHERE workspace_id=OLD.workspace_id AND tag_id=OLD.id)
   OR EXISTS(SELECT 1 FROM api_key_allocation_override_tags WHERE workspace_id=OLD.workspace_id AND tag_id=OLD.id)
   OR EXISTS(SELECT 1 FROM service_account_allocation_override_tags WHERE workspace_id=OLD.workspace_id AND tag_id=OLD.id)) THEN
   RAISE EXCEPTION 'allocation tag is still used by active allocation configuration' USING ERRCODE='23514';
  END IF;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS workspace_cost_centers_archive_guard ON workspace_cost_centers;
CREATE TRIGGER workspace_cost_centers_archive_guard BEFORE UPDATE OF status ON workspace_cost_centers
 FOR EACH ROW EXECUTE FUNCTION guard_finops_allocation_reference_archive();
DROP TRIGGER IF EXISTS workspace_allocation_tags_archive_guard ON workspace_allocation_tags;
CREATE TRIGGER workspace_allocation_tags_archive_guard BEFORE UPDATE OF status ON workspace_allocation_tags
 FOR EACH ROW EXECUTE FUNCTION guard_finops_allocation_reference_archive();

COMMENT ON TABLE usage_allocation_snapshots IS 'Immutable Phase F allocation evidence; legacy usage rows without a reservation remain unallocated.';
COMMENT ON TABLE usage_allocation_hourly_rollups IS 'Transactional base-dimension rollup for bounded allocation reporting; never rewrites usage history.';
