-- Make rental renewals safely retryable without charging or extending twice.
ALTER TABLE sms_order_events
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS uq_sms_order_events_idempotency
    ON sms_order_events(order_id, event_type, idempotency_key)
    WHERE idempotency_key <> '';
