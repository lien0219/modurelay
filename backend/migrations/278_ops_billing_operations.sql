-- Financial observation is independent from deductions. No worker may execute
-- these payloads to charge a customer. Failed operations require human review.
CREATE TABLE IF NOT EXISTS ops_billing_intents (
 id BIGSERIAL PRIMARY KEY,
 request_id TEXT NOT NULL,
 api_key_id BIGINT NOT NULL,
 request_fingerprint TEXT NOT NULL,
 operation TEXT NOT NULL CHECK (operation IN ('usage','image_reserve','image_capture','image_release')),
 user_id BIGINT NOT NULL,
 payload JSONB NOT NULL,
 state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','committed','failed')),
 error_code TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(request_id,api_key_id,operation,request_fingerprint)
);
CREATE INDEX IF NOT EXISTS idx_ops_billing_intents_pending ON ops_billing_intents(updated_at,id) WHERE state <> 'committed';
CREATE TABLE IF NOT EXISTS ops_billing_compensation_outbox (
 id BIGSERIAL PRIMARY KEY,
 intent_id BIGINT NOT NULL UNIQUE REFERENCES ops_billing_intents(id),
 state TEXT NOT NULL DEFAULT 'queued' CHECK(state IN ('queued','manual_required','resolved')),
 attempts INT NOT NULL DEFAULT 0,
 next_check_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '2 minutes',
 checked_at TIMESTAMPTZ,
 resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_ops_billing_compensation_due ON ops_billing_compensation_outbox(next_check_at,id) WHERE state='queued';
CREATE TABLE IF NOT EXISTS ops_billing_operation_audit (
 id BIGSERIAL PRIMARY KEY,
 outbox_id BIGINT NOT NULL REFERENCES ops_billing_compensation_outbox(id),
 actor_id BIGINT,
 action TEXT NOT NULL,
 note TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS ops_billing_operational_snapshot (
 id INT PRIMARY KEY CHECK(id=1), checked_at TIMESTAMPTZ NOT NULL,
 failed_settlements BIGINT NOT NULL, compensation_backlog BIGINT NOT NULL,
 ledger_mismatches BIGINT NOT NULL, coverage_incomplete BOOLEAN NOT NULL
);
CREATE TABLE IF NOT EXISTS ops_billing_recording_gaps (
 instance_id TEXT PRIMARY KEY,
 missed_count BIGINT NOT NULL,
 acknowledged_count BIGINT NOT NULL DEFAULT 0,
 first_missed_at TIMESTAMPTZ NOT NULL,
 last_missed_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS ops_billing_recording_gap_audit (
 id BIGSERIAL PRIMARY KEY,instance_id TEXT NOT NULL REFERENCES ops_billing_recording_gaps(instance_id),
 actor_id BIGINT NOT NULL,acknowledged_count BIGINT NOT NULL,note TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
