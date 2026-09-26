-- A provider order is a single external asset and must never be bound to two local orders.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM sms_orders
        WHERE BTRIM(provider_order_id) <> ''
        GROUP BY provider_id, provider_order_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'duplicate SMS provider order bindings must be reconciled before migration 269';
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_sms_orders_provider_order_id
    ON sms_orders(provider_id, provider_order_id)
    WHERE BTRIM(provider_order_id) <> '';
