-- Make SMS quote consumption and balance settlement durable and idempotent.
-- Existing orders stay on the legacy settlement path because an upgrade must
-- not guess whether an older process already adjusted the aggregate balance.
ALTER TABLE sms_quotes
    ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS consumed_order_id BIGINT REFERENCES sms_orders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sms_quotes_user_expires_at
    ON sms_quotes(user_id, expires_at);

ALTER TABLE sms_orders
    ADD COLUMN IF NOT EXISTS reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refunded_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS settlement_status TEXT NOT NULL DEFAULT 'legacy';

CREATE INDEX IF NOT EXISTS idx_sms_orders_settlement_status
    ON sms_orders(settlement_status, updated_at);
