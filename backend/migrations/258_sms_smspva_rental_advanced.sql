-- Advanced SMS rental capabilities for provider-backed long-term numbers.
-- Additive only: existing SMS orders remain valid and are backfilled with their
-- primary service relation. Restore/add-service quotes are short-lived and
-- consumed atomically before provider mutations.
CREATE TABLE IF NOT EXISTS sms_order_services (
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE RESTRICT,
    provider_service_code TEXT NOT NULL DEFAULT '',
    provider_order_id TEXT NOT NULL DEFAULT '',
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(order_id, service_id)
);

INSERT INTO sms_order_services (
    order_id, service_id, provider_service_code, provider_order_id,
    provider_cost_snapshot, sale_price_snapshot, status, metadata
)
SELECT
    o.id, o.service_id, COALESCE(NULLIF(q.provider_service_code,''), s.code),
    o.provider_order_id, o.provider_cost_snapshot, o.sale_price_snapshot,
    CASE WHEN o.status IN ('active','completed') THEN 'active' ELSE o.status END,
    '{"primary":true}'::jsonb
FROM sms_orders o
JOIN sms_services s ON s.id=o.service_id
LEFT JOIN LATERAL (
    SELECT provider_service_code
    FROM sms_quotes
    WHERE consumed_order_id=o.id
    ORDER BY consumed_at DESC NULLS LAST, created_at DESC
    LIMIT 1
) q ON TRUE
WHERE o.product_type='rental'
ON CONFLICT (order_id, service_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS sms_rental_service_quotes (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE RESTRICT,
    provider_service_code TEXT NOT NULL,
    rent_days INTEGER NOT NULL CHECK (rent_days > 0 AND rent_days <= 366),
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_rental_service_quotes_user_expiry
    ON sms_rental_service_quotes(user_id, expires_at);

CREATE TABLE IF NOT EXISTS sms_rental_restore_quotes (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    provider_history_order_id TEXT NOT NULL,
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    service_code TEXT NOT NULL,
    country_code TEXT NOT NULL,
    phone_number TEXT NOT NULL DEFAULT '',
    duration_days INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_rental_restore_quotes_user_expiry
    ON sms_rental_restore_quotes(user_id, expires_at);
