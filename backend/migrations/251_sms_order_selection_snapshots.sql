-- Persist the provider-native selection used for each purchased SMS order.
ALTER TABLE sms_orders ADD COLUMN IF NOT EXISTS operator_code TEXT NOT NULL DEFAULT 'any';
ALTER TABLE sms_orders ADD COLUMN IF NOT EXISTS voice_mode INTEGER NOT NULL DEFAULT 0 CHECK (voice_mode BETWEEN 0 AND 2);
CREATE INDEX IF NOT EXISTS idx_sms_orders_selection ON sms_orders(provider_id,service_id,country_id,operator_code,voice_mode);
