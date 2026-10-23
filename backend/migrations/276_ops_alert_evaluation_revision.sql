-- Rule changes must not inherit a different window's sustained evidence.
-- Delivery-only edits (silence/recipients) retain the evaluation revision.
ALTER TABLE ops_alert_rules
 ADD COLUMN IF NOT EXISTS evaluation_revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE ops_alert_rule_states
 ADD COLUMN IF NOT EXISTS rule_revision BIGINT NOT NULL DEFAULT 1;
ALTER TABLE ops_alert_resource_states
 ADD COLUMN IF NOT EXISTS rule_revision BIGINT NOT NULL DEFAULT 1;
