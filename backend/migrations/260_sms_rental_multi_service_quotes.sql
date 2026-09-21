-- Bind multi-service rental selections to the short-lived quote so a client
-- cannot swap or append services after price/stock confirmation.
ALTER TABLE sms_quotes
    ADD COLUMN IF NOT EXISTS service_codes_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE INDEX IF NOT EXISTS idx_sms_order_services_order_status
    ON sms_order_services(order_id, status);
