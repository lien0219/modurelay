-- Tenant lifecycle jobs retain their parent graph. Historical evidence is never
-- a reason to keep business closure permanently unfinished.
ALTER TABLE workspaces DROP CONSTRAINT IF EXISTS workspaces_status_check;
ALTER TABLE workspaces ADD CONSTRAINT workspaces_status_check
 CHECK(status IN ('active','suspended','archived','pending_deletion','purging','deleted'));

CREATE TABLE IF NOT EXISTS workspace_export_jobs (
 id UUID PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
 requested_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','running','completed','failed','cancelled','expired')),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 progress BIGINT NOT NULL DEFAULT 0 CHECK(progress>=0),
 lease_owner TEXT,
 lease_token UUID,
 lease_expires_at TIMESTAMPTZ,
 available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 object_key TEXT,
 data_cutoff TIMESTAMPTZ,
 artifact_sha256 TEXT NOT NULL DEFAULT '' CHECK(artifact_sha256='' OR artifact_sha256 ~ '^[0-9a-f]{64}$'),
 size_bytes BIGINT NOT NULL DEFAULT 0 CHECK(size_bytes BETWEEN 0 AND 68157440),
 failure_code VARCHAR(64) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 completed_at TIMESTAMPTZ,
 expires_at TIMESTAMPTZ,
 UNIQUE(workspace_id,id),
 CHECK((lease_token IS NULL)=(lease_expires_at IS NULL)),
 CHECK(object_key IS NULL OR object_key ~ ('^workspace-exports/'||workspace_id||'/'||id||'/[0-9a-f-]{36}\.enc$')),
 CHECK(state<>'completed' OR (completed_at IS NOT NULL AND data_cutoff IS NOT NULL AND object_key IS NOT NULL AND artifact_sha256<>''))
);
CREATE INDEX IF NOT EXISTS workspace_export_jobs_due ON workspace_export_jobs(available_at,lease_expires_at,created_at,id)
 WHERE state IN ('pending','running');
CREATE INDEX IF NOT EXISTS workspace_export_jobs_scope ON workspace_export_jobs(workspace_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS workspace_export_jobs_cleanup ON workspace_export_jobs(expires_at,id) WHERE object_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS workspace_export_jobs_one_active ON workspace_export_jobs(workspace_id) WHERE state IN ('pending','running');

-- Attempt ledger keeps orphan object keys across crashes, cancellations and
-- replacement leases. Object deletion never trusts a client-supplied path.
CREATE TABLE IF NOT EXISTS workspace_export_objects (
 object_key TEXT PRIMARY KEY,
 workspace_id BIGINT NOT NULL,
 export_id UUID NOT NULL,
 lease_token UUID NOT NULL,
 cleanup_after TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,export_id) REFERENCES workspace_export_jobs(workspace_id,id),
 CHECK(object_key ~ ('^workspace-exports/'||workspace_id||'/'||export_id||'/[0-9a-f-]{36}\.enc$'))
);
CREATE INDEX IF NOT EXISTS workspace_export_objects_cleanup ON workspace_export_objects(cleanup_after,object_key);

CREATE TABLE IF NOT EXISTS workspace_export_download_grants (
 token_hash BYTEA PRIMARY KEY CHECK(octet_length(token_hash)=32),
 workspace_id BIGINT NOT NULL,
 export_id UUID NOT NULL,
 actor_user_id BIGINT NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,export_id) REFERENCES workspace_export_jobs(workspace_id,id) ON DELETE RESTRICT,
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '60 seconds')
);
CREATE INDEX IF NOT EXISTS workspace_export_grants_expiry ON workspace_export_download_grants(expires_at,token_hash);
CREATE INDEX IF NOT EXISTS workspace_export_grants_scope ON workspace_export_download_grants(workspace_id,actor_user_id,created_at);

CREATE TABLE IF NOT EXISTS workspace_deletion_challenges (
 token_hash BYTEA PRIMARY KEY CHECK(octet_length(token_hash)=32),
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 actor_user_id BIGINT NOT NULL REFERENCES users(id),
 expires_at TIMESTAMPTZ NOT NULL,
 consumed_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '10 minutes')
);
CREATE INDEX IF NOT EXISTS workspace_deletion_challenges_scope ON workspace_deletion_challenges(workspace_id,actor_user_id,created_at DESC);
CREATE INDEX IF NOT EXISTS workspace_deletion_challenges_expiry ON workspace_deletion_challenges(expires_at,token_hash);

CREATE TABLE IF NOT EXISTS workspace_deletion_jobs (
 id UUID PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
 requested_by_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 previous_status TEXT NOT NULL CHECK(previous_status IN ('active','archived')),
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','running','blocked','failed','cancelled','completed')),
 phase VARCHAR(64) NOT NULL DEFAULT 'credentials',
 cursor BIGINT NOT NULL DEFAULT 0 CHECK(cursor>=0),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts BETWEEN 0 AND 5),
 progress BIGINT NOT NULL DEFAULT 0 CHECK(progress>=0),
 lease_owner TEXT,
 lease_token UUID,
 lease_expires_at TIMESTAMPTZ,
 available_at TIMESTAMPTZ NOT NULL,
 earliest_purge_at TIMESTAMPTZ NOT NULL,
 failure_code VARCHAR(64) NOT NULL DEFAULT '',
 blocking_reasons JSONB NOT NULL DEFAULT '[]' CHECK(jsonb_typeof(blocking_reasons)='array' AND jsonb_array_length(blocking_reasons)<=32),
 protected_evidence_retained BOOLEAN NOT NULL DEFAULT true,
 business_closed BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 completed_at TIMESTAMPTZ,
 UNIQUE(workspace_id,id),
 CHECK(earliest_purge_at>=created_at+interval '7 days'),
 CHECK((lease_token IS NULL)=(lease_expires_at IS NULL)),
 CHECK(state<>'completed' OR (business_closed AND completed_at IS NOT NULL)),
 CHECK(NOT business_closed OR state='completed')
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_deletion_jobs_one_active ON workspace_deletion_jobs(workspace_id)
 WHERE state IN ('pending','running','blocked','failed');
CREATE INDEX IF NOT EXISTS workspace_deletion_jobs_due ON workspace_deletion_jobs(available_at,lease_expires_at,id)
 WHERE state IN ('pending','running','blocked');
CREATE INDEX IF NOT EXISTS workspace_deletion_jobs_scope ON workspace_deletion_jobs(workspace_id,created_at DESC,id DESC);

-- Minimal operator-controlled safety blocks; complex legal-hold workflow is
-- outside Phase G. No tenant API can alter an operator's active hold.
CREATE TABLE IF NOT EXISTS workspace_lifecycle_holds (
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 code VARCHAR(64) NOT NULL CHECK(code IN ('LEGAL_HOLD','BUSINESS_HOLD')),
 reason VARCHAR(1000) NOT NULL CHECK(length(trim(reason))>0),
 active BOOLEAN NOT NULL DEFAULT true,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(workspace_id,code)
);
CREATE TABLE IF NOT EXISTS lifecycle_platform_settings (
 id SMALLINT PRIMARY KEY CHECK(id=1),
 deletion_grace_days INTEGER NOT NULL DEFAULT 7 CHECK(deletion_grace_days BETWEEN 7 AND 3650)
);
INSERT INTO lifecycle_platform_settings(id) VALUES(1) ON CONFLICT DO NOTHING;

-- Serialize an operator's hold with the same lock as irreversible batches.
CREATE OR REPLACE FUNCTION lock_workspace_lifecycle_hold() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM workspaces WHERE id=NEW.workspace_id FOR UPDATE;
 RETURN NEW;
END $$;
CREATE TRIGGER workspace_lifecycle_hold_lock BEFORE INSERT OR UPDATE ON workspace_lifecycle_holds
 FOR EACH ROW EXECUTE FUNCTION lock_workspace_lifecycle_hold();
CREATE INDEX IF NOT EXISTS lifecycle_policy_quota_pending ON policy_quota_reservations(workspace_id,status,id);
CREATE INDEX IF NOT EXISTS lifecycle_image_pending ON batch_image_jobs(workspace_id,status,id);
CREATE INDEX IF NOT EXISTS lifecycle_webhook_pending ON workspace_webhook_deliveries(workspace_id,status,id);
CREATE INDEX IF NOT EXISTS lifecycle_budget_workspace_id ON budget_reservations(workspace_id,id);
CREATE INDEX IF NOT EXISTS lifecycle_usage_workspace_id ON usage_logs(workspace_id,id) WHERE workspace_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS lifecycle_allocation_workspace_id ON usage_allocation_snapshots(workspace_id,usage_log_id);

CREATE OR REPLACE FUNCTION guard_workspace_lifecycle_state() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.status IN ('pending_deletion','purging','deleted') AND NEW.status IS DISTINCT FROM OLD.status THEN
  IF OLD.status='pending_deletion' AND NEW.status IN ('active','archived') AND EXISTS
   (SELECT 1 FROM workspace_deletion_jobs WHERE workspace_id=OLD.id AND state='cancelled' AND previous_status=NEW.status) AND NOT EXISTS
   (SELECT 1 FROM workspace_deletion_jobs WHERE workspace_id=OLD.id AND state IN ('pending','running','blocked','failed')) THEN RETURN NEW; END IF;
  IF OLD.status='pending_deletion' AND NEW.status='purging' AND EXISTS
   (SELECT 1 FROM workspace_deletion_jobs WHERE workspace_id=OLD.id AND state='running' AND lease_expires_at>now()) THEN RETURN NEW; END IF;
  IF OLD.status='purging' AND NEW.status='deleted' AND EXISTS
   (SELECT 1 FROM workspace_deletion_jobs WHERE workspace_id=OLD.id AND state='completed' AND business_closed) THEN RETURN NEW; END IF;
  RAISE EXCEPTION 'workspace lifecycle transition conflicts with durable deletion state' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS workspace_lifecycle_state_guard ON workspaces;
CREATE TRIGGER workspace_lifecycle_state_guard BEFORE UPDATE OF status ON workspaces
 FOR EACH ROW EXECUTE FUNCTION guard_workspace_lifecycle_state();

-- Add the retention scalar without broadening the existing event envelope or
-- nested-value rules. Policy writes, audit and outbox remain one transaction.
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
     'subject_type','project_access_mode','domain_id','domain','normalized_domain','provider_id','provider_revision',
     'require_sso','role_source','source_provider_id','role_count','team_count','mapping_revision','connector_id','token_id','resource_id','operation','added_count','removed_count','failure_count',
     'previous_revision','require_mfa','session_max_age_seconds','invitation_policy','allow_external_members','workspace_jit_enabled','approved_identity_provider_mode','changed_fields',
     'anomaly_id','detector_type','detector_version','dimension_type','dimension_value','severity','observed_spend','expected_spend','spend_delta','observed_requests','expected_requests','observed_unit_cost','expected_unit_cost','score','window_start','window_end','resolution_reason',
     'retention_days')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;
