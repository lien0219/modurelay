-- Resource identity and notification intent survive evaluator/Pod restarts.
ALTER TABLE ops_alert_events ADD COLUMN IF NOT EXISTS resource_key TEXT NOT NULL DEFAULT '';
UPDATE ops_alert_events SET resource_key = COALESCE(dimensions->>'resource_key', '') WHERE resource_key = '';
-- Keep evidence, but close pre-existing duplicate firing rows before enforcing uniqueness.
WITH duplicates AS (
 SELECT id, row_number() OVER (PARTITION BY rule_id, resource_key ORDER BY fired_at DESC, id DESC) AS ordinal
 FROM ops_alert_events WHERE status = 'firing' AND rule_id IS NOT NULL
)
UPDATE ops_alert_events SET status = 'manual_resolved', resolved_at = NOW()
WHERE id IN (SELECT id FROM duplicates WHERE ordinal > 1);
CREATE UNIQUE INDEX IF NOT EXISTS ops_alert_active_resource_unique
 ON ops_alert_events(rule_id, resource_key) WHERE status = 'firing';
CREATE TABLE IF NOT EXISTS ops_alert_resource_states (
 rule_id BIGINT NOT NULL,
 resource_key TEXT NOT NULL,
 consecutive_breaches INT NOT NULL DEFAULT 0,
 last_evaluated_at TIMESTAMPTZ,
 data_status TEXT NOT NULL DEFAULT '',
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY (rule_id, resource_key)
);
CREATE TABLE IF NOT EXISTS ops_alert_deliveries (
 id BIGSERIAL PRIMARY KEY,
 event_id BIGINT NOT NULL REFERENCES ops_alert_events(id) ON DELETE CASCADE,
 channel TEXT NOT NULL CHECK (channel IN ('email','webhook')),
 transition TEXT NOT NULL CHECK (transition IN ('firing','upgrade','resolved')),
 target TEXT NOT NULL,
 payload JSONB NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','sending','sent','failed')),
 attempts INT NOT NULL DEFAULT 0,
 last_error TEXT NOT NULL DEFAULT '',
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 lease_token TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 sent_at TIMESTAMPTZ,
 UNIQUE (event_id, channel, transition, target)
);
CREATE INDEX IF NOT EXISTS ops_alert_deliveries_due ON ops_alert_deliveries(next_attempt_at, id)
 WHERE status IN ('pending','sending');

-- Optional Prometheus source: an unconfigured source remains Unknown.
INSERT INTO ops_alert_rules(name,description,enabled,metric_type,operator,threshold,window_minutes,sustained_minutes,severity,notify_email,cooldown_minutes,created_at,updated_at)
VALUES('监控数据源不可用','Prometheus 兼容查询源连续两分钟无法响应，未配置时保持未知。',true,'monitoring_source_unavailable','>',0,1,2,'P1',true,10,NOW(),NOW())
ON CONFLICT(name) DO NOTHING;

-- Earlier installs seeded these latency rules but had no evaluator implementation.
UPDATE ops_alert_rules SET metric_type='http_p95_ms', min_requests=GREATEST(min_requests,20),updated_at=NOW() WHERE metric_type='p95_latency_ms';
UPDATE ops_alert_rules SET metric_type='http_p99_ms', min_requests=GREATEST(min_requests,20),updated_at=NOW() WHERE metric_type='p99_latency_ms';

-- Historical 037 included a Goose Down section, while the production runner
-- executes whole SQL files. Restore this table forward without changing its checksum.
CREATE TABLE IF NOT EXISTS ops_alert_silences (
 id BIGSERIAL PRIMARY KEY,
 rule_id BIGINT NOT NULL,
 platform VARCHAR(64) NOT NULL,
 group_id BIGINT,
 region VARCHAR(64),
 until TIMESTAMPTZ NOT NULL,
 reason TEXT,
 created_by BIGINT,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ops_alert_silences_lookup ON ops_alert_silences(rule_id,platform,group_id,region,until);
ALTER TABLE ops_alert_silences ADD COLUMN IF NOT EXISTS resource_key TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS ops_alert_silence_resource_lookup ON ops_alert_silences(rule_id, resource_key, until) WHERE resource_key <> '';

CREATE TABLE IF NOT EXISTS ops_alert_delivery_attempts (
 id BIGSERIAL PRIMARY KEY,
 delivery_id BIGINT NOT NULL REFERENCES ops_alert_deliveries(id) ON DELETE CASCADE,
 lease_token TEXT NOT NULL UNIQUE,
 attempt INT NOT NULL,
 status TEXT NOT NULL,
 error TEXT NOT NULL DEFAULT '',
 completed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS ops_alert_delivery_attempts_delivery ON ops_alert_delivery_attempts(delivery_id, id DESC);

-- An incident links overlapping rule events on one resource, preserving each
-- original event and starting a new incident after all of its rules recover.
CREATE TABLE IF NOT EXISTS ops_alert_incidents (
 id BIGSERIAL PRIMARY KEY,
 resource_key TEXT NOT NULL,
 severity TEXT NOT NULL,
 status TEXT NOT NULL DEFAULT 'firing',
 opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 resolved_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS ops_alert_incident_active ON ops_alert_incidents(resource_key) WHERE status='firing';
ALTER TABLE ops_alert_events ADD COLUMN IF NOT EXISTS incident_id BIGINT REFERENCES ops_alert_incidents(id);
CREATE INDEX IF NOT EXISTS ops_alert_event_incident ON ops_alert_events(incident_id, status) WHERE incident_id IS NOT NULL;
