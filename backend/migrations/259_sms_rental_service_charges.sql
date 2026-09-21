-- Durable settlement ledger for adding services to active SMS rental numbers.
CREATE TABLE IF NOT EXISTS sms_rental_service_charges (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE RESTRICT,
    quote_id TEXT NOT NULL UNIQUE REFERENCES sms_rental_service_quotes(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL,
    provider_child_order_id TEXT NOT NULL DEFAULT '',
    reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    settlement_status TEXT NOT NULL DEFAULT 'held',
    status TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(order_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_sms_rental_service_charges_status
    ON sms_rental_service_charges(status, settlement_status, updated_at);
