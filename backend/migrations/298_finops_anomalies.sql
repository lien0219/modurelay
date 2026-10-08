-- Phase E: bounded, read-only FinOps anomaly evidence and workflow state.
-- This migration never updates usage_logs or any historical attribution.

CREATE TABLE IF NOT EXISTS finops_anomaly_detection_leases (
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 bucket_start TIMESTAMPTZ NOT NULL,
 detector_version VARCHAR(32) NOT NULL CHECK (length(trim(detector_version)) BETWEEN 1 AND 32),
 lease_token TEXT,
 claimed_until TIMESTAMPTZ,
 completed_at TIMESTAMPTZ,
 attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 last_error_code VARCHAR(64),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(workspace_id,bucket_start,detector_version),
 CHECK (bucket_start=date_trunc('hour',bucket_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
 CHECK (completed_at IS NULL OR claimed_until IS NULL OR completed_at >= bucket_start)
);
ALTER TABLE finops_anomaly_detection_leases ADD COLUMN IF NOT EXISTS lease_token TEXT;
CREATE INDEX IF NOT EXISTS finops_anomaly_leases_due
 ON finops_anomaly_detection_leases(completed_at,claimed_until,workspace_id,bucket_start);

-- Detector batches start from one completed UTC hour. These leading bucket
-- indexes keep workspace discovery bounded without touching usage_logs.
CREATE INDEX IF NOT EXISTS finops_anomaly_tenant_rollups_bucket_workspace
 ON usage_tenant_hourly_rollups(bucket_start,workspace_id);
CREATE INDEX IF NOT EXISTS finops_anomaly_service_rollups_bucket_workspace
 ON usage_service_account_hourly_rollups(bucket_start,workspace_id);

CREATE TABLE IF NOT EXISTS finops_anomaly_snapshots (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 project_id BIGINT,
 scope_type VARCHAR(32) NOT NULL CHECK (scope_type IN ('workspace','project','platform','model','api_key','service_account')),
 scope_id BIGINT,
 dimension_type VARCHAR(32) NOT NULL CHECK (dimension_type IN ('workspace','project','platform','model','api_key','service_account')),
 dimension_value VARCHAR(512) NOT NULL CHECK (length(trim(dimension_value)) BETWEEN 1 AND 512),
 detector_type VARCHAR(32) NOT NULL CHECK (detector_type IN ('spend_spike','request_spike','unit_cost_spike')),
 detector_version VARCHAR(32) NOT NULL CHECK (length(trim(detector_version)) BETWEEN 1 AND 32),
 window_start TIMESTAMPTZ NOT NULL,
 window_end TIMESTAMPTZ NOT NULL,
 baseline_start TIMESTAMPTZ,
 baseline_end TIMESTAMPTZ,
 observed_spend NUMERIC(30,10) NOT NULL,
 expected_spend NUMERIC(30,10) NOT NULL,
 spend_delta NUMERIC(30,10) NOT NULL,
 observed_requests BIGINT NOT NULL CHECK (observed_requests >= 0),
 expected_requests NUMERIC(30,10) NOT NULL,
 observed_unit_cost NUMERIC(30,10) NOT NULL,
 expected_unit_cost NUMERIC(30,10) NOT NULL,
 baseline_sample_count INTEGER NOT NULL CHECK (baseline_sample_count >= 0),
 baseline_mad NUMERIC(30,10) NOT NULL,
 relative_increase NUMERIC(30,10) NOT NULL,
 score NUMERIC(30,10) NOT NULL,
 severity VARCHAR(16) NOT NULL CHECK (severity IN ('low','medium','high','critical')),
 fingerprint CHAR(64) NOT NULL UNIQUE CHECK (fingerprint ~ '^[0-9a-f]{64}$'),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id),
 CHECK (window_end > window_start),
 CHECK (baseline_end IS NULL OR baseline_start IS NULL OR baseline_end >= baseline_start),
 CHECK (scope_id IS NULL OR scope_id > 0),
 CHECK (observed_spend >= 0 AND observed_spend < 'Infinity'::numeric AND expected_spend >= 0 AND expected_spend < 'Infinity'::numeric AND spend_delta >= 0 AND spend_delta < 'Infinity'::numeric AND observed_spend <> 'NaN'::numeric AND expected_spend <> 'NaN'::numeric AND spend_delta <> 'NaN'::numeric),
 CHECK (expected_requests >= 0 AND expected_requests < 'Infinity'::numeric AND observed_unit_cost >= 0 AND observed_unit_cost < 'Infinity'::numeric AND expected_unit_cost >= 0 AND expected_unit_cost < 'Infinity'::numeric AND baseline_mad >= 0 AND baseline_mad < 'Infinity'::numeric AND relative_increase >= 0 AND relative_increase < 'Infinity'::numeric AND score >= 0 AND score < 'Infinity'::numeric),
 CHECK ((dimension_type IN ('workspace','project','api_key','service_account') AND scope_id IS NOT NULL) OR (dimension_type IN ('platform','model') AND scope_id IS NULL))
);
CREATE INDEX IF NOT EXISTS finops_anomaly_snapshots_scope_window
 ON finops_anomaly_snapshots(workspace_id,project_id,window_end DESC,id DESC);
CREATE INDEX IF NOT EXISTS finops_anomaly_snapshots_dimension
 ON finops_anomaly_snapshots(workspace_id,dimension_type,dimension_value,window_start DESC);

CREATE TABLE IF NOT EXISTS finops_anomaly_findings (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 project_id BIGINT,
 snapshot_id BIGINT NOT NULL REFERENCES finops_anomaly_snapshots(id) ON DELETE RESTRICT,
 fingerprint CHAR(64) NOT NULL UNIQUE REFERENCES finops_anomaly_snapshots(fingerprint) ON DELETE RESTRICT,
 status VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved')),
 first_detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 acknowledged_at TIMESTAMPTZ,
 acknowledged_by_user_id BIGINT REFERENCES users(id),
 resolved_at TIMESTAMPTZ,
 resolved_by_user_id BIGINT REFERENCES users(id),
 resolution_reason VARCHAR(1000) NOT NULL DEFAULT '' CHECK (length(resolution_reason) <= 1000),
 version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id)
);
CREATE INDEX IF NOT EXISTS finops_anomaly_findings_scope_status
 ON finops_anomaly_findings(workspace_id,project_id,status,last_detected_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS finops_anomaly_findings_status
 ON finops_anomaly_findings(status,last_detected_at DESC,id DESC);

CREATE TABLE IF NOT EXISTS finops_anomaly_detector_status (
 id SMALLINT PRIMARY KEY CHECK (id=1),
 last_successful_scan TIMESTAMPTZ,
 last_processed_bucket TIMESTAMPTZ,
 last_failure_code VARCHAR(64),
 lag_seconds BIGINT NOT NULL DEFAULT 0 CHECK (lag_seconds >= 0),
 candidate_count BIGINT NOT NULL DEFAULT 0 CHECK (candidate_count >= 0),
 finding_count BIGINT NOT NULL DEFAULT 0 CHECK (finding_count >= 0),
 scan_duration_ms BIGINT NOT NULL DEFAULT 0 CHECK (scan_duration_ms >= 0),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE FUNCTION guard_finops_anomaly_snapshot_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'finops anomaly evidence snapshot is immutable' USING ERRCODE='23514';
END $$;
DROP TRIGGER IF EXISTS finops_anomaly_snapshot_immutable_guard ON finops_anomaly_snapshots;
CREATE TRIGGER finops_anomaly_snapshot_immutable_guard
 BEFORE UPDATE OR DELETE ON finops_anomaly_snapshots
 FOR EACH ROW EXECUTE FUNCTION guard_finops_anomaly_snapshot_immutable();

CREATE OR REPLACE FUNCTION guard_finops_anomaly_finding_transition() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.workspace_id IS DISTINCT FROM NEW.workspace_id OR OLD.project_id IS DISTINCT FROM NEW.project_id OR
    OLD.snapshot_id IS DISTINCT FROM NEW.snapshot_id OR OLD.fingerprint IS DISTINCT FROM NEW.fingerprint OR
    OLD.first_detected_at IS DISTINCT FROM NEW.first_detected_at OR OLD.last_detected_at IS DISTINCT FROM NEW.last_detected_at THEN
  RAISE EXCEPTION 'finops anomaly finding evidence is immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.status='open' AND NEW.status NOT IN ('open','acknowledged','resolved') THEN
  RAISE EXCEPTION 'invalid finops anomaly status transition' USING ERRCODE='23514';
 END IF;
 IF OLD.status='acknowledged' AND NEW.status NOT IN ('acknowledged','resolved') THEN
  RAISE EXCEPTION 'invalid finops anomaly status transition' USING ERRCODE='23514';
 END IF;
 IF OLD.status='resolved' AND NEW.status <> 'resolved' THEN
  RAISE EXCEPTION 'invalid finops anomaly status transition' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS finops_anomaly_finding_transition_guard ON finops_anomaly_findings;
CREATE TRIGGER finops_anomaly_finding_transition_guard
 BEFORE UPDATE ON finops_anomaly_findings
 FOR EACH ROW EXECUTE FUNCTION guard_finops_anomaly_finding_transition();

-- Extend the existing closed scalar payload contract for anomaly events. The
-- detector still emits only bounded evidence fields; no row or secret is put
-- into a domain event.
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
     'anomaly_id','detector_type','detector_version','dimension_type','dimension_value','severity','observed_spend','expected_spend','spend_delta','observed_requests','expected_requests','observed_unit_cost','expected_unit_cost','score','window_start','window_end','resolution_reason')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;
