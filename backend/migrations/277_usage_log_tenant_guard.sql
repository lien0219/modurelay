-- Tighten the legacy usage compatibility path without rewriting migration 275.
-- Historical NULL attribution is accepted only when the row is completed by a
-- finalized, identity-matching budget reservation. Do not trust a session GUC
-- as an authorization marker: any SQL caller could set a custom GUC.
CREATE OR REPLACE FUNCTION guard_usage_log_tenant_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.workspace_id IS NULL AND OLD.project_id IS NULL
    AND OLD.billing_principal_user_id IS NULL AND OLD.resolved_platform IS NULL
    AND OLD.budget_reservation_id IS NULL THEN
   IF NEW.workspace_id IS NULL AND NEW.project_id IS NULL
      AND NEW.billing_principal_user_id IS NULL AND NEW.resolved_platform IS NULL
      AND NEW.budget_reservation_id IS NULL THEN
     RETURN NEW;
   END IF;
   IF OLD.video_count <= 0 OR OLD.actual_cost <> 0
      OR OLD.total_cost <= 0 OR OLD.rate_multiplier <= 0
      OR NEW.video_count <= 0
      OR NEW.workspace_id IS NULL OR NEW.project_id IS NULL
      OR NEW.billing_principal_user_id IS NULL OR NEW.budget_reservation_id IS NULL
      OR NOT EXISTS (
        SELECT 1
        FROM budget_reservations r
        WHERE r.id = NEW.budget_reservation_id
          AND r.status = 'finalized'
          AND r.actor_user_id = NEW.user_id
          AND r.api_key_id = NEW.api_key_id
          AND r.workspace_id = NEW.workspace_id
          AND r.project_id = NEW.project_id
          AND r.billing_principal_user_id = NEW.billing_principal_user_id
          AND r.actual = NEW.actual_cost
      ) THEN
     RAISE EXCEPTION 'legacy usage log tenant attribution requires validated video settlement' USING ERRCODE='23514';
   END IF;
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
