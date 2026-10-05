-- 273-279 ship together. Historical rows have NULL tenant attribution and are
-- intentionally excluded; do not scan/backfill the historical usage table.
-- NOT VALID enforces all subsequent writes without scanning existing usage.
DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='budget_reservations'::regclass AND conname='budget_reservation_usage_identity') THEN
  ALTER TABLE budget_reservations ADD CONSTRAINT budget_reservation_usage_identity
   UNIQUE(id,workspace_id,project_id,actor_user_id,api_key_id,billing_principal_user_id);
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='usage_logs'::regclass AND conname='usage_logs_tenant_shape') THEN
  ALTER TABLE usage_logs ADD CONSTRAINT usage_logs_tenant_shape CHECK (
   (workspace_id IS NULL AND project_id IS NULL AND billing_principal_user_id IS NULL AND budget_reservation_id IS NULL)
   OR (workspace_id IS NOT NULL AND project_id IS NOT NULL AND billing_principal_user_id IS NOT NULL AND budget_reservation_id IS NOT NULL)
  ) NOT VALID;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='usage_logs'::regclass AND conname='usage_logs_tenant_project_fk') THEN
  ALTER TABLE usage_logs ADD CONSTRAINT usage_logs_tenant_project_fk
   FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) NOT VALID;
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='usage_logs'::regclass AND conname='usage_logs_tenant_reservation_fk') THEN
  ALTER TABLE usage_logs ADD CONSTRAINT usage_logs_tenant_reservation_fk
   FOREIGN KEY(budget_reservation_id,workspace_id,project_id,user_id,api_key_id,billing_principal_user_id)
   REFERENCES budget_reservations(id,workspace_id,project_id,actor_user_id,api_key_id,billing_principal_user_id) NOT VALID;
 END IF;
END $$;

-- Hour buckets support exact daily/monthly views in any IANA timezone. The
-- dashboard reads source rows only for partial boundary hours. Dimensions use
-- the frozen usage snapshot, never a key's current tenant or billing owner.
CREATE TABLE IF NOT EXISTS usage_tenant_hourly_rollups (
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id),
 project_id BIGINT NOT NULL,
 bucket_start TIMESTAMPTZ NOT NULL,
 api_key_id BIGINT NOT NULL,
 resolved_platform TEXT NOT NULL,
 model TEXT NOT NULL,
 request_count BIGINT NOT NULL CHECK (request_count >= 0),
 actual_cost NUMERIC(30,10) NOT NULL,
 PRIMARY KEY(workspace_id,bucket_start,project_id,api_key_id,resolved_platform,model),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id),
 CHECK (bucket_start = date_trunc('hour',bucket_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')
);
CREATE INDEX IF NOT EXISTS usage_tenant_rollups_project_hour ON usage_tenant_hourly_rollups(project_id,bucket_start);
CREATE OR REPLACE VIEW usage_tenant_daily_rollups AS
 SELECT workspace_id,project_id,(bucket_start AT TIME ZONE 'UTC')::date AS bucket_date,
        api_key_id,resolved_platform,model,SUM(request_count)::bigint AS request_count,SUM(actual_cost) AS actual_cost
 FROM usage_tenant_hourly_rollups GROUP BY 1,2,3,4,5,6;
COMMENT ON TABLE usage_tenant_hourly_rollups IS 'Transactional tenant usage aggregates; supports daily FinOps without scanning whole-period usage_logs.';
COMMENT ON VIEW usage_tenant_daily_rollups IS 'UTC daily tenant usage; other timezones use hour aggregates and indexed partial hours.';

CREATE OR REPLACE FUNCTION accumulate_tenant_usage_after_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost)
 SELECT workspace_id,project_id,date_trunc('hour',created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC',
        api_key_id,COALESCE(resolved_platform,''),model,COUNT(*),SUM(actual_cost)
 FROM inserted_tenant_usage WHERE workspace_id IS NOT NULL
 GROUP BY 1,2,3,4,5,6 ORDER BY 1,3,2,4,5,6
 ON CONFLICT(workspace_id,bucket_start,project_id,api_key_id,resolved_platform,model)
 DO UPDATE SET request_count=usage_tenant_hourly_rollups.request_count+EXCLUDED.request_count,
               actual_cost=usage_tenant_hourly_rollups.actual_cost+EXCLUDED.actual_cost;
 RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS usage_logs_tenant_rollup_insert ON usage_logs;
CREATE TRIGGER usage_logs_tenant_rollup_insert AFTER INSERT ON usage_logs
 REFERENCING NEW TABLE AS inserted_tenant_usage FOR EACH STATEMENT
 EXECUTE FUNCTION accumulate_tenant_usage_after_insert();

CREATE OR REPLACE FUNCTION reconcile_tenant_usage_rollup() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE old_hour TIMESTAMPTZ; new_hour TIMESTAMPTZ;
BEGIN
 old_hour := date_trunc('hour',OLD.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';
 IF TG_OP='UPDATE' THEN
  new_hour := date_trunc('hour',NEW.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';
  IF ROW(OLD.workspace_id,OLD.project_id,old_hour,OLD.api_key_id,OLD.resolved_platform,OLD.model)
     IS NOT DISTINCT FROM ROW(NEW.workspace_id,NEW.project_id,new_hour,NEW.api_key_id,NEW.resolved_platform,NEW.model) THEN
   UPDATE usage_tenant_hourly_rollups SET actual_cost=actual_cost+NEW.actual_cost-OLD.actual_cost
   WHERE workspace_id=OLD.workspace_id AND project_id=OLD.project_id AND bucket_start=old_hour
     AND api_key_id=OLD.api_key_id AND resolved_platform=COALESCE(OLD.resolved_platform,'') AND model=OLD.model;
   RETURN NEW;
  END IF;
 END IF;
 IF OLD.workspace_id IS NOT NULL THEN
  UPDATE usage_tenant_hourly_rollups SET request_count=request_count-1,actual_cost=actual_cost-OLD.actual_cost
   WHERE workspace_id=OLD.workspace_id AND project_id=OLD.project_id AND bucket_start=old_hour
     AND api_key_id=OLD.api_key_id AND resolved_platform=COALESCE(OLD.resolved_platform,'') AND model=OLD.model;
  DELETE FROM usage_tenant_hourly_rollups WHERE workspace_id=OLD.workspace_id AND project_id=OLD.project_id AND bucket_start=old_hour
    AND api_key_id=OLD.api_key_id AND resolved_platform=COALESCE(OLD.resolved_platform,'') AND model=OLD.model AND request_count=0;
 END IF;
 IF TG_OP='UPDATE' AND NEW.workspace_id IS NOT NULL THEN
  INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost)
   VALUES(NEW.workspace_id,NEW.project_id,new_hour,NEW.api_key_id,COALESCE(NEW.resolved_platform,''),NEW.model,1,NEW.actual_cost)
  ON CONFLICT(workspace_id,bucket_start,project_id,api_key_id,resolved_platform,model)
   DO UPDATE SET request_count=usage_tenant_hourly_rollups.request_count+1,
                 actual_cost=usage_tenant_hourly_rollups.actual_cost+EXCLUDED.actual_cost;
 END IF;
 RETURN NULL;
END $$;
DROP TRIGGER IF EXISTS usage_logs_tenant_rollup_update ON usage_logs;
CREATE TRIGGER usage_logs_tenant_rollup_update AFTER UPDATE ON usage_logs FOR EACH ROW
 WHEN ((OLD.workspace_id IS NOT NULL OR NEW.workspace_id IS NOT NULL) AND
  ROW(OLD.workspace_id,OLD.project_id,OLD.created_at,OLD.api_key_id,OLD.resolved_platform,OLD.model,OLD.actual_cost)
  IS DISTINCT FROM ROW(NEW.workspace_id,NEW.project_id,NEW.created_at,NEW.api_key_id,NEW.resolved_platform,NEW.model,NEW.actual_cost))
 EXECUTE FUNCTION reconcile_tenant_usage_rollup();
DROP TRIGGER IF EXISTS usage_logs_tenant_rollup_delete ON usage_logs;
CREATE TRIGGER usage_logs_tenant_rollup_delete AFTER DELETE ON usage_logs FOR EACH ROW
 WHEN (OLD.workspace_id IS NOT NULL) EXECUTE FUNCTION reconcile_tenant_usage_rollup();
