-- Keep SMS provider reconciliation explicit and rate-limited.  A pending
-- purchase, cancellation, refund, and automatic expiry require different
-- provider calls and must not share one ambiguous refund flag.
ALTER TABLE sms_orders
    ADD COLUMN IF NOT EXISTS reconciliation_action TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reconciliation_attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reconcile_after TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_sms_orders_reconcile_after
    ON sms_orders(reconcile_after, updated_at)
    WHERE status IN ('active', 'provider_unknown', 'reconciling');

