-- Add a durable delivery marker for 30-day platform SMS success statistics.
-- Success means the provider actually delivered at least one SMS. User-driven
-- cancellation/refund and still-pending orders are intentionally excluded from
-- the failure denominator by the service query.

ALTER TABLE sms_orders
    ADD COLUMN IF NOT EXISTS first_sms_received_at TIMESTAMPTZ;

UPDATE sms_orders o
SET first_sms_received_at = m.first_received_at
FROM (
    SELECT order_id, MIN(received_at) AS first_received_at
    FROM sms_messages
    GROUP BY order_id
) m
WHERE o.id = m.order_id
  AND o.first_sms_received_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_sms_orders_delivery_stats_30d
    ON sms_orders(provider_id, service_id, country_id, operator_code, created_at DESC)
    WHERE product_type='temporary'
      AND voice_mode=0
      AND provider_order_id<>'';

CREATE INDEX IF NOT EXISTS idx_sms_messages_order_received
    ON sms_messages(order_id, received_at);
