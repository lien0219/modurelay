-- Phase G: conservative retention and protection of the original financial graph.
-- No historical attribution or financial evidence is rewritten by this migration.
CREATE TABLE IF NOT EXISTS platform_retention_policies (
 category TEXT PRIMARY KEY CHECK (category IN ('operational','financial','audit','security','temporary')),
 minimum_days INTEGER NOT NULL CHECK (minimum_days BETWEEN 0 AND 36500),
 default_days INTEGER NOT NULL CHECK (default_days BETWEEN 0 AND 36500),
 protected BOOLEAN NOT NULL DEFAULT FALSE,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK (default_days=0 OR (minimum_days>0 AND default_days>=minimum_days)),
 CHECK (category<>'financial' OR (protected AND minimum_days=0 AND default_days=0))
);
INSERT INTO platform_retention_policies(category,minimum_days,default_days,protected) VALUES
 ('operational',180,180,FALSE),('financial',0,0,TRUE),('audit',0,0,FALSE),
 ('security',365,365,FALSE),('temporary',7,7,FALSE)
ON CONFLICT(category) DO NOTHING;

CREATE TABLE IF NOT EXISTS workspace_retention_policies (
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
 category TEXT NOT NULL REFERENCES platform_retention_policies(category) ON DELETE RESTRICT,
 retention_days INTEGER NOT NULL CHECK (retention_days BETWEEN 0 AND 36500),
 updated_by_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(workspace_id,category)
);

CREATE OR REPLACE FUNCTION guard_workspace_retention_floor() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE policy RECORD;
BEGIN
 SELECT minimum_days,protected INTO policy FROM platform_retention_policies WHERE category=NEW.category FOR SHARE;
 IF NOT FOUND OR (NEW.retention_days<>0 AND (policy.protected OR policy.minimum_days=0 OR NEW.retention_days<policy.minimum_days)) THEN
  RAISE EXCEPTION 'workspace retention cannot reduce platform protection' USING ERRCODE='23514',CONSTRAINT='workspace_retention_floor';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS workspace_retention_floor_guard ON workspace_retention_policies;
CREATE TRIGGER workspace_retention_floor_guard BEFORE INSERT OR UPDATE ON workspace_retention_policies
 FOR EACH ROW EXECUTE FUNCTION guard_workspace_retention_floor();

-- Zero means indefinite, including missing/unknown policy. A raised platform
-- floor immediately constrains previously saved Workspace settings as well.
CREATE OR REPLACE FUNCTION effective_lifecycle_retention_days(scope BIGINT, classification TEXT)
RETURNS INTEGER LANGUAGE SQL STABLE AS $$
 SELECT COALESCE((SELECT CASE
  WHEN p.protected OR p.minimum_days=0 OR COALESCE(w.retention_days,p.default_days)=0 THEN 0
  ELSE GREATEST(p.minimum_days,COALESCE(w.retention_days,p.default_days)) END
 FROM platform_retention_policies p LEFT JOIN workspace_retention_policies w
  ON w.category=p.category AND w.workspace_id=scope
 WHERE p.category=classification),0)
$$;

CREATE OR REPLACE FUNCTION guard_finops_allocation_snapshot_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'finops allocation snapshot is immutable' USING ERRCODE='23514';
END $$;

-- Evidence keeps its original parents. Do not attempt a cascade and rely on
-- later child triggers to stop it: both parent edges now explicitly RESTRICT.
ALTER TABLE budget_reservation_allocation_snapshots
 DROP CONSTRAINT IF EXISTS budget_reservation_allocation_snapshots_reservation_id_fkey;
ALTER TABLE budget_reservation_allocation_snapshots
 ADD CONSTRAINT budget_reservation_allocation_snapshots_reservation_id_fkey
 FOREIGN KEY(reservation_id) REFERENCES budget_reservations(id) ON DELETE RESTRICT;
ALTER TABLE usage_allocation_snapshots DROP CONSTRAINT IF EXISTS usage_allocation_snapshots_usage_log_id_fkey;
ALTER TABLE usage_allocation_snapshots ADD CONSTRAINT usage_allocation_snapshots_usage_log_id_fkey
 FOREIGN KEY(usage_log_id) REFERENCES usage_logs(id) ON DELETE RESTRICT;

CREATE OR REPLACE FUNCTION guard_tenant_usage_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.workspace_id IS NOT NULL OR OLD.project_id IS NOT NULL OR OLD.billing_principal_user_id IS NOT NULL
  OR OLD.budget_reservation_id IS NOT NULL OR OLD.service_account_id IS NOT NULL
  OR EXISTS(SELECT 1 FROM usage_allocation_snapshots WHERE usage_log_id=OLD.id) THEN
  RAISE EXCEPTION 'tenant usage financial evidence cannot be deleted' USING ERRCODE='23514',CONSTRAINT='tenant_usage_retention';
 END IF;
 RETURN OLD;
END $$;
DROP TRIGGER IF EXISTS usage_logs_financial_delete_guard ON usage_logs;
CREATE TRIGGER usage_logs_financial_delete_guard BEFORE DELETE ON usage_logs
 FOR EACH ROW EXECUTE FUNCTION guard_tenant_usage_deletion();

CREATE OR REPLACE FUNCTION guard_financial_record_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'protected financial evidence cannot be deleted or truncated' USING ERRCODE='23514',CONSTRAINT='financial_retention';
END $$;
DROP TRIGGER IF EXISTS budget_reservations_financial_delete_guard ON budget_reservations;
CREATE TRIGGER budget_reservations_financial_delete_guard BEFORE DELETE ON budget_reservations
 FOR EACH ROW EXECUTE FUNCTION guard_financial_record_deletion();

CREATE OR REPLACE FUNCTION tenant_settlement_marker_protected(request TEXT, credential BIGINT)
RETURNS BOOLEAN LANGUAGE SQL STABLE AS $$
 SELECT EXISTS(SELECT 1 FROM budget_reservations r WHERE r.request_id=request AND r.api_key_id=credential)
 OR EXISTS(SELECT 1 FROM usage_logs u WHERE u.request_id=request AND u.api_key_id=credential AND
  (u.workspace_id IS NOT NULL OR u.project_id IS NOT NULL OR u.billing_principal_user_id IS NOT NULL
   OR u.budget_reservation_id IS NOT NULL OR u.service_account_id IS NOT NULL
   OR EXISTS(SELECT 1 FROM usage_allocation_snapshots s WHERE s.usage_log_id=u.id)))
$$;
CREATE OR REPLACE FUNCTION guard_tenant_settlement_marker_deletion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF tenant_settlement_marker_protected(OLD.request_id,OLD.api_key_id) THEN
  RAISE EXCEPTION 'tenant settlement dedup must retain its financial graph' USING ERRCODE='23514',CONSTRAINT='tenant_settlement_retention';
 END IF;
 RETURN OLD;
END $$;
DROP TRIGGER IF EXISTS usage_billing_dedup_tenant_delete_guard ON usage_billing_dedup;
CREATE TRIGGER usage_billing_dedup_tenant_delete_guard BEFORE DELETE ON usage_billing_dedup
 FOR EACH ROW EXECUTE FUNCTION guard_tenant_settlement_marker_deletion();
DROP TRIGGER IF EXISTS usage_billing_dedup_archive_retention_guard ON usage_billing_dedup_archive;
CREATE TRIGGER usage_billing_dedup_archive_retention_guard BEFORE UPDATE OR DELETE ON usage_billing_dedup_archive
 FOR EACH ROW EXECUTE FUNCTION guard_financial_record_deletion();

-- TRUNCATE bypasses row DELETE guards and may be reached through CASCADE on an
-- unrelated parent. Test teardown drops its disposable database instead.
DO $$ DECLARE protected_table TEXT; BEGIN
 FOREACH protected_table IN ARRAY ARRAY['usage_logs','budget_reservations','budget_counters',
  'budget_reservation_allocation_snapshots','usage_allocation_snapshots','usage_allocation_hourly_rollups',
  'usage_tenant_hourly_rollups','usage_service_account_hourly_rollups','finops_anomaly_snapshots',
  'finops_anomaly_findings','billing_settlement_alerts','usage_billing_dedup','usage_billing_dedup_archive'] LOOP
  EXECUTE format('DROP TRIGGER IF EXISTS financial_truncate_guard ON %I',protected_table);
  EXECUTE format('CREATE TRIGGER financial_truncate_guard BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION guard_financial_record_deletion()',protected_table);
 END LOOP;
END $$;

-- Eligible legacy deletion and lifecycle preflight use bounded indexed paths.
CREATE INDEX IF NOT EXISTS usage_logs_legacy_retention
 ON usage_logs(created_at,id) WHERE workspace_id IS NULL AND project_id IS NULL
 AND billing_principal_user_id IS NULL AND budget_reservation_id IS NULL AND service_account_id IS NULL;

-- Bounded keyset scans must advance through indefinitely retained records;
-- otherwise the oldest retained tenant could starve every finite policy.
CREATE TABLE IF NOT EXISTS domain_event_retention_cursors (
 category TEXT PRIMARY KEY CHECK(category IN ('notifications','webhooks','outbox','events','dedup')),
 after_time TIMESTAMPTZ,
 after_id TEXT,
 after_key BIGINT,
 CHECK ((after_time IS NULL)=(after_id IS NULL)),
 CHECK ((category='dedup' AND after_time IS NOT NULL)=(after_key IS NOT NULL))
);
INSERT INTO domain_event_retention_cursors(category) VALUES
 ('notifications'),('webhooks'),('outbox'),('events'),('dedup') ON CONFLICT(category) DO NOTHING;
CREATE INDEX IF NOT EXISTS usage_billing_dedup_retention
 ON usage_billing_dedup(created_at,request_id,api_key_id);
CREATE INDEX IF NOT EXISTS workspace_webhook_deliveries_retention
 ON workspace_webhook_deliveries(COALESCE(finished_at,delivered_at,created_at),id)
 WHERE status IN ('succeeded','dead') AND lock_owner IS NULL AND locked_at IS NULL;

COMMENT ON TABLE platform_retention_policies IS 'Platform business floors; zero means indefinite. Values do not assert legal compliance.';
COMMENT ON TABLE workspace_retention_policies IS 'Tenant policy cannot lower platform floors or financial evidence protection.';
