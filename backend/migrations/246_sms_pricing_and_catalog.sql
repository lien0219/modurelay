-- Administrator-controlled SMS pricing and provider capability metadata.
-- Provider costs remain provider-owned; this policy controls only the
-- platform-facing sale price.
INSERT INTO settings (key, value)
VALUES ('sms_pricing_settings', '{"cost_multiplier":1.30,"fixed_markup":0,"unknown_grade_multiplier":1,"unknown_grade_fixed_markup":0,"temporary_expiry_minutes":10,"self_service_cancel_after_minutes":1}')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE sms_providers
    ADD COLUMN IF NOT EXISTS service_catalog JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS country_catalog JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE sms_orders
    ADD COLUMN IF NOT EXISTS provider_refund_status TEXT NOT NULL DEFAULT 'not_requested',
    ADD COLUMN IF NOT EXISTS provider_refund_reference TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_sms_orders_expires_active
    ON sms_orders(expires_at)
    WHERE status IN ('active','provider_unknown','reconciling');
