-- Persist SMS quotes so purchases can validate the exact channel, price, and
-- provider mapping that the user confirmed instead of trusting a client price.
CREATE TABLE IF NOT EXISTS sms_quotes (
    id TEXT PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES sms_channels(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL REFERENCES sms_providers(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE CASCADE,
    country_id BIGINT NOT NULL REFERENCES sms_countries(id) ON DELETE CASCADE,
    product_type TEXT NOT NULL CHECK (product_type IN ('temporary','rental')),
    provider_service_code TEXT NOT NULL,
    provider_country_code TEXT NOT NULL,
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    success_rate_snapshot NUMERIC(8,5),
    success_rate_source_snapshot TEXT NOT NULL DEFAULT 'unavailable',
    success_rate_grade_snapshot TEXT NOT NULL DEFAULT '',
    success_rate_multiplier_snapshot NUMERIC(20,8) NOT NULL DEFAULT 1,
    fixed_markup_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    stock INTEGER NOT NULL DEFAULT 0,
    estimated_delivery_seconds INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_quotes_expires_at ON sms_quotes(expires_at);
