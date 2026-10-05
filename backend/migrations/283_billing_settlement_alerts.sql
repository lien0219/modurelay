-- Durable alert transitions are distinct from billing receipts and Redis
-- recovery snapshots. They never authorize another charge or replace dedup.
CREATE TABLE IF NOT EXISTS billing_settlement_alerts (
 reservation_id UUID PRIMARY KEY REFERENCES budget_reservations(id) ON DELETE CASCADE,
 request_id TEXT NOT NULL CHECK (request_id=trim(request_id) AND length(request_id)>0),
 api_key_id BIGINT NOT NULL REFERENCES api_keys(id),
 state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','recovered')),
 data JSONB NOT NULL,
 pending_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 recovered_at TIMESTAMPTZ,
 UNIQUE(request_id,api_key_id),
 CHECK ((state='pending' AND recovered_at IS NULL) OR (state='recovered' AND recovered_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS billing_settlement_alerts_pending ON billing_settlement_alerts(pending_at) WHERE state='pending';
CREATE OR REPLACE FUNCTION guard_billing_settlement_alert_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.reservation_id,NEW.request_id,NEW.api_key_id,NEW.data,NEW.pending_at)
  IS DISTINCT FROM ROW(OLD.reservation_id,OLD.request_id,OLD.api_key_id,OLD.data,OLD.pending_at) THEN
  RAISE EXCEPTION 'billing settlement alert snapshot is immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.state='recovered' AND ROW(NEW.state,NEW.recovered_at) IS DISTINCT FROM ROW(OLD.state,OLD.recovered_at) THEN
  RAISE EXCEPTION 'billing settlement alert terminal state is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS billing_settlement_alert_snapshot_guard ON billing_settlement_alerts;
CREATE TRIGGER billing_settlement_alert_snapshot_guard BEFORE UPDATE ON billing_settlement_alerts
 FOR EACH ROW EXECUTE FUNCTION guard_billing_settlement_alert_snapshot();
