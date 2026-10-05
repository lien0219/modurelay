-- A separate machine breakdown keeps the established tenant aggregates intact.
-- Dimensions come only from immutable request-time usage, with no backfill.
CREATE TABLE IF NOT EXISTS usage_service_account_hourly_rollups (
 service_account_id BIGINT NOT NULL,
 workspace_id BIGINT NOT NULL,
 project_id BIGINT NOT NULL,
 bucket_start TIMESTAMPTZ NOT NULL,
 api_key_id BIGINT NOT NULL,
 resolved_platform TEXT NOT NULL,
 model TEXT NOT NULL,
 request_count BIGINT NOT NULL CHECK (request_count >= 0),
 actual_cost NUMERIC(30,10) NOT NULL,
 input_tokens BIGINT NOT NULL,
 output_tokens BIGINT NOT NULL,
 PRIMARY KEY(workspace_id,bucket_start,project_id,service_account_id,api_key_id,resolved_platform,model),
 FOREIGN KEY(service_account_id,workspace_id,project_id) REFERENCES service_accounts(id,workspace_id,project_id),
 CHECK (bucket_start=date_trunc('hour',bucket_start AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')
);
CREATE INDEX IF NOT EXISTS usage_service_account_rollups_account_hour
 ON usage_service_account_hourly_rollups(service_account_id,bucket_start);
CREATE INDEX IF NOT EXISTS usage_service_account_rollups_project_hour
 ON usage_service_account_hourly_rollups(project_id,bucket_start);

CREATE OR REPLACE FUNCTION accumulate_service_account_usage_after_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO usage_service_account_hourly_rollups(service_account_id,workspace_id,project_id,bucket_start,
  api_key_id,resolved_platform,model,request_count,actual_cost,input_tokens,output_tokens)
 SELECT service_account_id,workspace_id,project_id,date_trunc('hour',created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC',
  api_key_id,COALESCE(resolved_platform,''),model,COUNT(*),SUM(actual_cost),SUM(input_tokens),SUM(output_tokens)
 FROM inserted_service_account_usage WHERE service_account_id IS NOT NULL
 GROUP BY 1,2,3,4,5,6,7 ORDER BY 2,4,3,1,5,6,7
 ON CONFLICT(workspace_id,bucket_start,project_id,service_account_id,api_key_id,resolved_platform,model)
 DO UPDATE SET request_count=usage_service_account_hourly_rollups.request_count+EXCLUDED.request_count,
  actual_cost=usage_service_account_hourly_rollups.actual_cost+EXCLUDED.actual_cost,
  input_tokens=usage_service_account_hourly_rollups.input_tokens+EXCLUDED.input_tokens,
  output_tokens=usage_service_account_hourly_rollups.output_tokens+EXCLUDED.output_tokens;
 RETURN NULL;
END $$;
CREATE TRIGGER usage_logs_service_account_rollup_insert AFTER INSERT ON usage_logs
 REFERENCING NEW TABLE AS inserted_service_account_usage FOR EACH STATEMENT
 EXECUTE FUNCTION accumulate_service_account_usage_after_insert();

CREATE OR REPLACE FUNCTION reconcile_service_account_usage_rollup() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE old_hour TIMESTAMPTZ; new_hour TIMESTAMPTZ;
BEGIN
 old_hour := date_trunc('hour',OLD.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';
 IF TG_OP='UPDATE' THEN
  new_hour := date_trunc('hour',NEW.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC';
  IF ROW(OLD.service_account_id,OLD.workspace_id,OLD.project_id,old_hour,OLD.api_key_id,OLD.resolved_platform,OLD.model)
   IS NOT DISTINCT FROM ROW(NEW.service_account_id,NEW.workspace_id,NEW.project_id,new_hour,NEW.api_key_id,NEW.resolved_platform,NEW.model) THEN
   UPDATE usage_service_account_hourly_rollups SET actual_cost=actual_cost+NEW.actual_cost-OLD.actual_cost,
    input_tokens=input_tokens+NEW.input_tokens-OLD.input_tokens,output_tokens=output_tokens+NEW.output_tokens-OLD.output_tokens
   WHERE service_account_id=OLD.service_account_id AND workspace_id=OLD.workspace_id AND project_id=OLD.project_id
    AND bucket_start=old_hour AND api_key_id=OLD.api_key_id AND resolved_platform=COALESCE(OLD.resolved_platform,'') AND model=OLD.model;
   RETURN NEW;
  END IF;
 END IF;
 IF OLD.service_account_id IS NOT NULL THEN
  UPDATE usage_service_account_hourly_rollups SET request_count=request_count-1,actual_cost=actual_cost-OLD.actual_cost,
   input_tokens=input_tokens-OLD.input_tokens,output_tokens=output_tokens-OLD.output_tokens
  WHERE service_account_id=OLD.service_account_id AND workspace_id=OLD.workspace_id AND project_id=OLD.project_id
   AND bucket_start=old_hour AND api_key_id=OLD.api_key_id AND resolved_platform=COALESCE(OLD.resolved_platform,'') AND model=OLD.model;
  DELETE FROM usage_service_account_hourly_rollups
  WHERE service_account_id=OLD.service_account_id AND workspace_id=OLD.workspace_id AND project_id=OLD.project_id
   AND bucket_start=old_hour AND api_key_id=OLD.api_key_id AND resolved_platform=COALESCE(OLD.resolved_platform,'') AND model=OLD.model AND request_count=0;
 END IF;
 IF TG_OP='UPDATE' AND NEW.service_account_id IS NOT NULL THEN
  INSERT INTO usage_service_account_hourly_rollups(service_account_id,workspace_id,project_id,bucket_start,
   api_key_id,resolved_platform,model,request_count,actual_cost,input_tokens,output_tokens)
  VALUES(NEW.service_account_id,NEW.workspace_id,NEW.project_id,new_hour,NEW.api_key_id,COALESCE(NEW.resolved_platform,''),
   NEW.model,1,NEW.actual_cost,NEW.input_tokens,NEW.output_tokens)
  ON CONFLICT(workspace_id,bucket_start,project_id,service_account_id,api_key_id,resolved_platform,model)
  DO UPDATE SET request_count=usage_service_account_hourly_rollups.request_count+1,
   actual_cost=usage_service_account_hourly_rollups.actual_cost+EXCLUDED.actual_cost,
   input_tokens=usage_service_account_hourly_rollups.input_tokens+EXCLUDED.input_tokens,
   output_tokens=usage_service_account_hourly_rollups.output_tokens+EXCLUDED.output_tokens;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER usage_logs_service_account_rollup_update AFTER UPDATE ON usage_logs FOR EACH ROW
 WHEN ((OLD.service_account_id IS NOT NULL OR NEW.service_account_id IS NOT NULL) AND
  ROW(OLD.service_account_id,OLD.workspace_id,OLD.project_id,OLD.created_at,OLD.api_key_id,OLD.resolved_platform,OLD.model,OLD.actual_cost,OLD.input_tokens,OLD.output_tokens)
  IS DISTINCT FROM ROW(NEW.service_account_id,NEW.workspace_id,NEW.project_id,NEW.created_at,NEW.api_key_id,NEW.resolved_platform,NEW.model,NEW.actual_cost,NEW.input_tokens,NEW.output_tokens))
 EXECUTE FUNCTION reconcile_service_account_usage_rollup();
CREATE TRIGGER usage_logs_service_account_rollup_delete AFTER DELETE ON usage_logs FOR EACH ROW
 WHEN (OLD.service_account_id IS NOT NULL) EXECUTE FUNCTION reconcile_service_account_usage_rollup();
