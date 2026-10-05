-- Nullable hot-table additions only; historical human identities stay NULL SA.
CREATE TABLE IF NOT EXISTS service_accounts (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 project_id BIGINT NOT NULL,
 name VARCHAR(100) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
 slug VARCHAR(63) NOT NULL CHECK (slug ~ '^[a-z0-9][a-z0-9-]{0,62}$'),
 description VARCHAR(2000) NOT NULL DEFAULT '',
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
 created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
 disabled_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(project_id,slug), UNIQUE(id,project_id), UNIQUE(id,workspace_id,project_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id)
);
CREATE INDEX IF NOT EXISTS service_accounts_workspace_project ON service_accounts(workspace_id,project_id,id);

ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS key_suffix VARCHAR(16);
ALTER TABLE api_keys ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_execution_principal CHECK (
 (user_id IS NOT NULL AND service_account_id IS NULL)
 OR (user_id IS NULL AND service_account_id IS NOT NULL AND project_id IS NOT NULL)
) NOT VALID;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_machine_digest CHECK (
 service_account_id IS NULL OR key ~ '^sha256:[0-9a-f]{64}$'
) NOT VALID;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_service_account_project_fk
 FOREIGN KEY(service_account_id,project_id) REFERENCES service_accounts(id,project_id) NOT VALID;

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE usage_logs ALTER COLUMN user_id DROP NOT NULL;
ALTER TABLE usage_logs ADD CONSTRAINT usage_logs_execution_principal CHECK (
 (user_id IS NOT NULL AND service_account_id IS NULL)
 OR (user_id IS NULL AND service_account_id IS NOT NULL AND workspace_id IS NOT NULL
     AND project_id IS NOT NULL AND budget_reservation_id IS NOT NULL)
) NOT VALID;
ALTER TABLE usage_logs ADD CONSTRAINT usage_logs_service_account_scope_fk
 FOREIGN KEY(service_account_id,workspace_id,project_id) REFERENCES service_accounts(id,workspace_id,project_id) NOT VALID;

ALTER TABLE budget_reservations ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE budget_reservations ALTER COLUMN actor_user_id DROP NOT NULL;
ALTER TABLE budget_reservations ADD CONSTRAINT budget_reservations_execution_principal CHECK (
 (actor_user_id IS NOT NULL AND service_account_id IS NULL)
 OR (actor_user_id IS NULL AND service_account_id IS NOT NULL)
) NOT VALID;
ALTER TABLE budget_reservations ADD CONSTRAINT budget_reservations_service_account_scope_fk
 FOREIGN KEY(service_account_id,workspace_id,project_id) REFERENCES service_accounts(id,workspace_id,project_id) NOT VALID;

ALTER TABLE batch_image_jobs ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE batch_image_jobs ADD CONSTRAINT batch_image_jobs_service_account_shape CHECK (
 service_account_id IS NULL OR (workspace_id IS NOT NULL AND project_id IS NOT NULL
  AND billing_principal_user_id IS NOT NULL AND user_id=billing_principal_user_id)
) NOT VALID;
ALTER TABLE batch_image_jobs ADD CONSTRAINT batch_image_jobs_service_account_scope_fk
 FOREIGN KEY(service_account_id,workspace_id,project_id) REFERENCES service_accounts(id,workspace_id,project_id) NOT VALID;

CREATE OR REPLACE FUNCTION guard_service_account_scope() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.id,NEW.workspace_id,NEW.project_id) IS DISTINCT FROM ROW(OLD.id,OLD.workspace_id,OLD.project_id) THEN
  RAISE EXCEPTION 'service account project binding is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER service_accounts_scope_guard BEFORE UPDATE ON service_accounts
 FOR EACH ROW EXECUTE FUNCTION guard_service_account_scope();

-- Preserve the existing legacy personal-project assignment path from 274.
CREATE OR REPLACE FUNCTION guard_api_key_principal() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.service_account_id IS DISTINCT FROM OLD.service_account_id
    OR (OLD.service_account_id IS NOT NULL AND NEW.user_id IS DISTINCT FROM OLD.user_id) THEN
  RAISE EXCEPTION 'API key execution principal is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER api_keys_principal_guard BEFORE UPDATE ON api_keys
 FOR EACH ROW EXECUTE FUNCTION guard_api_key_principal();

CREATE OR REPLACE FUNCTION guard_budget_reservation_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.id,NEW.request_id,NEW.actor_user_id,NEW.service_account_id,NEW.api_key_id,NEW.workspace_id,NEW.project_id,
        NEW.billing_principal_user_id,NEW.period_start,NEW.period_end,NEW.project_period_start,NEW.project_period_end,NEW.estimate)
    IS DISTINCT FROM
    ROW(OLD.id,OLD.request_id,OLD.actor_user_id,OLD.service_account_id,OLD.api_key_id,OLD.workspace_id,OLD.project_id,
        OLD.billing_principal_user_id,OLD.period_start,OLD.period_end,OLD.project_period_start,OLD.project_period_end,OLD.estimate) THEN
  RAISE EXCEPTION 'budget reservation admission snapshot is immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.status<>'pending' AND ROW(NEW.status,NEW.actual,NEW.finalized_at) IS DISTINCT FROM ROW(OLD.status,OLD.actual,OLD.finalized_at) THEN
  RAISE EXCEPTION 'budget reservation terminal state is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION validate_machine_budget_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.service_account_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM api_keys k JOIN projects p ON p.id=k.project_id
  WHERE k.id=NEW.api_key_id AND k.user_id IS NULL AND k.service_account_id=NEW.service_account_id
   AND k.project_id=NEW.project_id AND p.workspace_id=NEW.workspace_id
 ) THEN
  RAISE EXCEPTION 'machine budget credential identity mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER budget_reservations_machine_identity_guard BEFORE INSERT ON budget_reservations
 FOR EACH ROW EXECUTE FUNCTION validate_machine_budget_identity();

-- MATCH SIMPLE composite FKs skip the whole check when actor_user_id is NULL.
-- Compare every frozen dimension explicitly for both human and machine usage.
CREATE OR REPLACE FUNCTION validate_usage_reservation_identity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.budget_reservation_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM budget_reservations r WHERE r.id=NEW.budget_reservation_id
   AND r.actor_user_id IS NOT DISTINCT FROM NEW.user_id
   AND r.service_account_id IS NOT DISTINCT FROM NEW.service_account_id
   AND r.api_key_id=NEW.api_key_id AND r.workspace_id=NEW.workspace_id
   AND r.project_id=NEW.project_id AND r.billing_principal_user_id=NEW.billing_principal_user_id
 ) THEN
  RAISE EXCEPTION 'usage reservation identity mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER usage_logs_reservation_identity_guard BEFORE INSERT OR UPDATE ON usage_logs
 FOR EACH ROW EXECUTE FUNCTION validate_usage_reservation_identity();

CREATE OR REPLACE FUNCTION guard_usage_log_tenant_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 -- Machine attribution is always immutable, even on all-NULL historical rows.
 IF NEW.service_account_id IS DISTINCT FROM OLD.service_account_id THEN
  RAISE EXCEPTION 'usage service account snapshot is immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.workspace_id IS NULL AND OLD.project_id IS NULL
    AND OLD.billing_principal_user_id IS NULL AND OLD.resolved_platform IS NULL
    AND OLD.budget_reservation_id IS NULL THEN
   IF NEW.workspace_id IS NULL AND NEW.project_id IS NULL
      AND NEW.billing_principal_user_id IS NULL AND NEW.resolved_platform IS NULL
      AND NEW.budget_reservation_id IS NULL THEN
     RETURN NEW;
   END IF;
   IF OLD.video_count <= 0 OR OLD.actual_cost <> 0
      OR OLD.total_cost <= 0 OR OLD.rate_multiplier <= 0 OR NEW.video_count <= 0
      OR NEW.workspace_id IS NULL OR NEW.project_id IS NULL
      OR NEW.billing_principal_user_id IS NULL OR NEW.budget_reservation_id IS NULL
      OR ROW(NEW.user_id,NEW.api_key_id) IS DISTINCT FROM ROW(OLD.user_id,OLD.api_key_id)
      OR NOT EXISTS (
        SELECT 1 FROM budget_reservations r WHERE r.id=NEW.budget_reservation_id AND r.status='finalized'
          AND r.actor_user_id=NEW.user_id AND r.service_account_id IS NULL
          AND r.api_key_id=NEW.api_key_id AND r.workspace_id=NEW.workspace_id
          AND r.project_id=NEW.project_id AND r.billing_principal_user_id=NEW.billing_principal_user_id
          AND r.actual=NEW.actual_cost
      ) THEN
     RAISE EXCEPTION 'legacy usage log tenant attribution requires validated video settlement' USING ERRCODE='23514';
   END IF;
   RETURN NEW;
 END IF;
 IF ROW(NEW.user_id,NEW.api_key_id,NEW.workspace_id,NEW.project_id,NEW.billing_principal_user_id,NEW.resolved_platform,NEW.budget_reservation_id)
    IS DISTINCT FROM
    ROW(OLD.user_id,OLD.api_key_id,OLD.workspace_id,OLD.project_id,OLD.billing_principal_user_id,OLD.resolved_platform,OLD.budget_reservation_id) THEN
   RAISE EXCEPTION 'usage log tenant attribution is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION validate_batch_image_tenant_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF ROW(NEW.batch_id,NEW.user_id,NEW.api_key_id,NEW.service_account_id,NEW.workspace_id,NEW.project_id,NEW.billing_principal_user_id)
   IS DISTINCT FROM ROW(OLD.batch_id,OLD.user_id,OLD.api_key_id,OLD.service_account_id,OLD.workspace_id,OLD.project_id,OLD.billing_principal_user_id) THEN
   RAISE EXCEPTION 'batch image attribution is immutable' USING ERRCODE='23514';
  END IF;
  IF OLD.budget_reservation_id IS NOT NULL AND NEW.budget_reservation_id IS DISTINCT FROM OLD.budget_reservation_id THEN
   RAISE EXCEPTION 'batch image budget reservation is immutable' USING ERRCODE='23514';
  END IF;
  IF OLD.provider_create_started_at IS NOT NULL AND NEW.provider_create_started_at IS DISTINCT FROM OLD.provider_create_started_at THEN
   RAISE EXCEPTION 'batch image provider create marker is immutable' USING ERRCODE='23514';
  END IF;
 END IF;
 IF NEW.service_account_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM api_keys k WHERE k.id=NEW.api_key_id AND k.user_id IS NULL
   AND k.service_account_id=NEW.service_account_id AND k.project_id=NEW.project_id
 ) THEN
  RAISE EXCEPTION 'batch image machine credential mismatch' USING ERRCODE='23514';
 END IF;
 IF NEW.budget_reservation_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM budget_reservations r WHERE r.id=NEW.budget_reservation_id
   AND r.request_id='batch_image_hold:'||NEW.batch_id
   AND r.service_account_id IS NOT DISTINCT FROM NEW.service_account_id
   AND ((NEW.service_account_id IS NULL AND r.actor_user_id=NEW.user_id)
        OR (NEW.service_account_id IS NOT NULL AND r.actor_user_id IS NULL AND NEW.user_id=NEW.billing_principal_user_id))
   AND r.api_key_id=NEW.api_key_id AND r.workspace_id=NEW.workspace_id AND r.project_id=NEW.project_id
   AND r.billing_principal_user_id=NEW.billing_principal_user_id AND r.estimate=round(NEW.estimated_cost,8)
 ) THEN
  RAISE EXCEPTION 'batch image budget attribution mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS batch_image_jobs_tenant_snapshot_guard ON batch_image_jobs;
CREATE TRIGGER batch_image_jobs_tenant_snapshot_guard
 BEFORE INSERT OR UPDATE OF batch_id,user_id,api_key_id,service_account_id,workspace_id,project_id,
 billing_principal_user_id,budget_reservation_id,provider_create_started_at ON batch_image_jobs
 FOR EACH ROW EXECUTE FUNCTION validate_batch_image_tenant_snapshot();
