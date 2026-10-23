-- Minimum request sample for rate rules, and persisted alert evaluation progress.
-- rule_id has no foreign key, matching ops_alert_events.rule_id. A deleted rule
-- leaves the list, and serial ids are not reused, so a leftover state row is not
-- loaded for a live rule. DeleteAlertRule removes the state row in the same transaction.

ALTER TABLE ops_alert_rules
    ADD COLUMN IF NOT EXISTS min_requests INT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS ops_alert_rule_states (
    rule_id BIGINT PRIMARY KEY,
    consecutive_breaches INT NOT NULL DEFAULT 0,
    last_evaluated_at TIMESTAMPTZ,
    data_status VARCHAR(32) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


