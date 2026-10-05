CREATE TABLE IF NOT EXISTS workspace_budget_policies (
 workspace_id BIGINT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
 amount NUMERIC(20,8) NOT NULL CHECK (amount >= 0 AND amount < 'Infinity'::numeric),
 hard_limit BOOLEAN NOT NULL DEFAULT FALSE,
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 timezone TEXT NOT NULL DEFAULT 'UTC' CHECK (timezone = trim(timezone) AND length(timezone) > 0),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS project_budget_policies (
 project_id BIGINT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
 amount NUMERIC(20,8) NOT NULL CHECK (amount >= 0 AND amount < 'Infinity'::numeric),
 hard_limit BOOLEAN NOT NULL DEFAULT FALSE,
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 timezone TEXT NOT NULL DEFAULT 'UTC' CHECK (timezone = trim(timezone) AND length(timezone) > 0),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS budget_counters (
 scope_type TEXT NOT NULL CHECK (scope_type IN ('workspace','project')),
 scope_id BIGINT NOT NULL CHECK (scope_id > 0),
 workspace_scope_id BIGINT GENERATED ALWAYS AS (CASE WHEN scope_type='workspace' THEN scope_id END) STORED REFERENCES workspaces(id),
 project_scope_id BIGINT GENERATED ALWAYS AS (CASE WHEN scope_type='project' THEN scope_id END) STORED REFERENCES projects(id),
 period_start DATE NOT NULL CHECK (extract(day FROM period_start)=1),
 spent NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (spent >= 0 AND spent < 'Infinity'::numeric),
 reserved NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (reserved >= 0 AND reserved < 'Infinity'::numeric),
 PRIMARY KEY(scope_type, scope_id, period_start)
);
CREATE TABLE IF NOT EXISTS budget_reservations (
 id UUID PRIMARY KEY,
 request_id TEXT NOT NULL CHECK (request_id=trim(request_id) AND length(request_id)>0),
 actor_user_id BIGINT NOT NULL REFERENCES users(id),
 api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 project_id BIGINT NOT NULL REFERENCES projects(id),
 billing_principal_user_id BIGINT NOT NULL REFERENCES users(id),
 -- Each scope uses its policy's local month label. Distinct timezones can
 -- legitimately identify different months at the same admission instant.
 period_start DATE NOT NULL CHECK (extract(day FROM period_start)=1),
 period_end DATE NOT NULL,
 project_period_start DATE NOT NULL CHECK (extract(day FROM project_period_start)=1),
 project_period_end DATE NOT NULL,
 estimate NUMERIC(20,8) NOT NULL CHECK (estimate >= 0 AND estimate < 'Infinity'::numeric),
 actual NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (actual >= 0 AND actual < 'Infinity'::numeric),
 status TEXT NOT NULL CHECK (status IN ('pending','finalized','released')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 finalized_at TIMESTAMPTZ,
 UNIQUE(request_id, api_key_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id),
 CHECK (period_end=(period_start+interval '1 month')::date),
 CHECK (project_period_end=(project_period_start+interval '1 month')::date),
 CHECK ((status='pending' AND actual=0 AND finalized_at IS NULL) OR
        (status='released' AND actual=0 AND finalized_at IS NOT NULL) OR
        (status='finalized' AND finalized_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS budget_reservations_scope_status ON budget_reservations(workspace_id, project_id, status);
CREATE INDEX IF NOT EXISTS budget_reservations_pending_principal ON budget_reservations(billing_principal_user_id) WHERE status='pending';
CREATE INDEX IF NOT EXISTS budget_reservations_pending_actor ON budget_reservations(actor_user_id) WHERE status='pending';

-- A transferred principal or archived scope may still have accepted work. Keep
-- its user row available to wallet/subscription recovery until holds settle.
-- Admission locks these same users before inserting a reservation, serializing
-- deletion with any in-flight admission without changing historical attribution.
CREATE OR REPLACE FUNCTION guard_pending_tenant_user_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL AND EXISTS (
  SELECT 1 FROM budget_reservations WHERE status='pending'
   AND (actor_user_id=OLD.id OR billing_principal_user_id=OLD.id)
 ) THEN
  RAISE EXCEPTION 'user has pending tenant work; settle or release it before deletion'
   USING ERRCODE='23514', CONSTRAINT='workspace_user_obligation';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS users_pending_tenant_deletion_guard ON users;
CREATE TRIGGER users_pending_tenant_deletion_guard BEFORE UPDATE OF deleted_at ON users
 FOR EACH ROW EXECUTE FUNCTION guard_pending_tenant_user_deletion();

-- Admission attribution and month labels are a durable billing snapshot.
-- Settlement may update only a pending row, never reopen a terminal hold.
CREATE OR REPLACE FUNCTION guard_budget_reservation_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.id,NEW.request_id,NEW.actor_user_id,NEW.api_key_id,NEW.workspace_id,NEW.project_id,
        NEW.billing_principal_user_id,NEW.period_start,NEW.period_end,NEW.project_period_start,NEW.project_period_end,NEW.estimate)
    IS DISTINCT FROM
    ROW(OLD.id,OLD.request_id,OLD.actor_user_id,OLD.api_key_id,OLD.workspace_id,OLD.project_id,
        OLD.billing_principal_user_id,OLD.period_start,OLD.period_end,OLD.project_period_start,OLD.project_period_end,OLD.estimate) THEN
  RAISE EXCEPTION 'budget reservation admission snapshot is immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.status<>'pending' AND ROW(NEW.status,NEW.actual,NEW.finalized_at) IS DISTINCT FROM ROW(OLD.status,OLD.actual,OLD.finalized_at) THEN
  RAISE EXCEPTION 'budget reservation terminal state is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS budget_reservation_snapshot_guard ON budget_reservations;
CREATE TRIGGER budget_reservation_snapshot_guard BEFORE UPDATE ON budget_reservations
 FOR EACH ROW EXECUTE FUNCTION guard_budget_reservation_snapshot();

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS workspace_id BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS project_id BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS billing_principal_user_id BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS resolved_platform TEXT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS budget_reservation_id UUID;
-- Hot-table indexes are created online by 279_usage_tenant_indexes_notx.sql.

-- Usage attribution is an immutable historical snapshot. A legacy row whose
-- tenant columns are all NULL may be completed once by the video settlement
-- compatibility path; an attributed row can never be rewritten or cleared.
CREATE OR REPLACE FUNCTION guard_usage_log_tenant_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.workspace_id IS NULL AND OLD.project_id IS NULL
    AND OLD.billing_principal_user_id IS NULL AND OLD.resolved_platform IS NULL
    AND OLD.budget_reservation_id IS NULL THEN
   RETURN NEW;
 END IF;
 IF ROW(NEW.workspace_id,NEW.project_id,NEW.billing_principal_user_id,NEW.resolved_platform,NEW.budget_reservation_id)
    IS DISTINCT FROM
    ROW(OLD.workspace_id,OLD.project_id,OLD.billing_principal_user_id,OLD.resolved_platform,OLD.budget_reservation_id) THEN
   RAISE EXCEPTION 'usage log tenant attribution is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS usage_log_tenant_snapshot_guard ON usage_logs;
CREATE TRIGGER usage_log_tenant_snapshot_guard BEFORE UPDATE ON usage_logs
 FOR EACH ROW EXECUTE FUNCTION guard_usage_log_tenant_snapshot();
