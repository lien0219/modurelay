-- New settlements carry immutable evidence in the existing billing transaction.
-- Historical/hold rows stay NULL: missing evidence must never be invented.
ALTER TABLE usage_billing_dedup
    ADD COLUMN IF NOT EXISTS expected_usage_cost NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS balance_cost NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS subscription_cost NUMERIC(20,8);
ALTER TABLE usage_billing_dedup_archive
    ADD COLUMN IF NOT EXISTS expected_usage_cost NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS balance_cost NUMERIC(20,8),
    ADD COLUMN IF NOT EXISTS subscription_cost NUMERIC(20,8);

CREATE TABLE IF NOT EXISTS ops_billing_reconciliation_findings (
    id BIGSERIAL PRIMARY KEY,
    settlement_id BIGINT NOT NULL UNIQUE,
    request_id VARCHAR(255) NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('missing_usage_log', 'amount_mismatch')),
    expected_cost NUMERIC(20,8) NOT NULL,
    actual_cost NUMERIC(20,8),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_ops_billing_findings_open
    ON ops_billing_reconciliation_findings (last_seen_at DESC) WHERE resolved_at IS NULL;

CREATE TABLE IF NOT EXISTS ops_billing_reconciliation_snapshot (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    checked_at TIMESTAMPTZ NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    missing_usage_logs BIGINT NOT NULL,
    amount_mismatches BIGINT NOT NULL,
    scanned BIGINT NOT NULL,
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    cursor_created_at TIMESTAMPTZ,
    cursor_id BIGINT NOT NULL DEFAULT 0
);
