-- Persist the delivery outcome independently from billing/refund state so
-- 30-day success statistics stay correct after automatic refunds or user
-- cancellations. A delivery is successful once at least one provider SMS is
-- observed. Provider terminal failure/expiry without an SMS is a failure.
-- User-driven cancellation/refund without an SMS is excluded.

ALTER TABLE sms_orders
    ADD COLUMN IF NOT EXISTS delivery_outcome TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS delivery_finalized_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS delivery_failure_reason TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'sms_orders_delivery_outcome_check'
    ) THEN
        ALTER TABLE sms_orders
            ADD CONSTRAINT sms_orders_delivery_outcome_check
            CHECK (delivery_outcome IN ('pending','success','failed','excluded'));
    END IF;
END $$;

UPDATE sms_orders
SET delivery_outcome='success',
    delivery_finalized_at=COALESCE(first_sms_received_at, updated_at)
WHERE first_sms_received_at IS NOT NULL
  AND delivery_outcome='pending';

UPDATE sms_orders
SET delivery_outcome='failed',
    delivery_finalized_at=updated_at,
    delivery_failure_reason=COALESCE(NULLIF(last_provider_error,''), status)
WHERE first_sms_received_at IS NULL
  AND status IN ('failed','expired')
  AND delivery_outcome='pending';

UPDATE sms_orders
SET delivery_outcome='excluded',
    delivery_finalized_at=updated_at
WHERE first_sms_received_at IS NULL
  AND status IN ('cancelled','refunded')
  AND delivery_outcome='pending';

CREATE INDEX IF NOT EXISTS idx_sms_orders_delivery_outcome_30d
    ON sms_orders(provider_id, service_id, country_id, operator_code, delivery_outcome, created_at DESC)
    WHERE product_type='temporary'
      AND voice_mode=0
      AND provider_order_id<>''
      AND delivery_outcome IN ('success','failed');
