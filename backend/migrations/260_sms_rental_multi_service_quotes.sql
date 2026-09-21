-- Provider-priced multi-service rental quotes. The capability remains disabled
-- until the provider's aggregate charging semantics have been validated with a
-- real account; this table only preserves a future immutable quote snapshot.
CREATE TABLE IF NOT EXISTS sms_rental_multi_service_quotes (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id BIGINT NOT NULL REFERENCES sms_channels(id) ON DELETE RESTRICT,
    provider_id BIGINT NOT NULL REFERENCES sms_providers(id) ON DELETE RESTRICT,
    country_id BIGINT NOT NULL REFERENCES sms_countries(id) ON DELETE RESTRICT,
    provider_country_code TEXT NOT NULL,
    service_codes_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    operator_code TEXT NOT NULL DEFAULT 'any',
    duration_value INTEGER NOT NULL,
    duration_unit TEXT NOT NULL,
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    stock_snapshot INTEGER NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    consumed_order_id BIGINT UNIQUE REFERENCES sms_orders(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sms_rental_multi_quotes_services_array_check CHECK (
        jsonb_typeof(service_codes_snapshot) = 'array'
        AND jsonb_array_length(service_codes_snapshot) BETWEEN 2 AND 32
    ),
    CONSTRAINT sms_rental_multi_quotes_duration_check CHECK (
        duration_value > 0 AND duration_unit IN ('week', 'month')
    ),
    CONSTRAINT sms_rental_multi_quotes_amounts_nonnegative CHECK (
        provider_cost_snapshot >= 0 AND sale_price_snapshot >= 0
    ),
    CONSTRAINT sms_rental_multi_quotes_stock_nonnegative CHECK (stock_snapshot >= 0),
    CONSTRAINT sms_rental_multi_quotes_currency_nonempty CHECK (btrim(currency_snapshot) <> '')
);

ALTER TABLE sms_rental_multi_service_quotes
    ADD COLUMN IF NOT EXISTS user_id BIGINT,
    ADD COLUMN IF NOT EXISTS channel_id BIGINT,
    ADD COLUMN IF NOT EXISTS provider_id BIGINT,
    ADD COLUMN IF NOT EXISTS country_id BIGINT,
    ADD COLUMN IF NOT EXISTS provider_country_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS service_codes_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS operator_code TEXT NOT NULL DEFAULT 'any',
    ADD COLUMN IF NOT EXISTS duration_value INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS duration_unit TEXT NOT NULL DEFAULT 'week',
    ADD COLUMN IF NOT EXISTS provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS stock_snapshot INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS consumed_order_id BIGINT UNIQUE,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_multi_quotes_services_array_check') THEN
        ALTER TABLE sms_rental_multi_service_quotes ADD CONSTRAINT sms_rental_multi_quotes_services_array_check CHECK (jsonb_typeof(service_codes_snapshot) = 'array' AND jsonb_array_length(service_codes_snapshot) BETWEEN 2 AND 32) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_multi_quotes_duration_check') THEN
        ALTER TABLE sms_rental_multi_service_quotes ADD CONSTRAINT sms_rental_multi_quotes_duration_check CHECK (duration_value > 0 AND duration_unit IN ('week','month')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_multi_quotes_amounts_nonnegative') THEN
        ALTER TABLE sms_rental_multi_service_quotes ADD CONSTRAINT sms_rental_multi_quotes_amounts_nonnegative CHECK (provider_cost_snapshot >= 0 AND sale_price_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_multi_quotes_stock_nonnegative') THEN
        ALTER TABLE sms_rental_multi_service_quotes ADD CONSTRAINT sms_rental_multi_quotes_stock_nonnegative CHECK (stock_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_multi_quotes_currency_nonempty') THEN
        ALTER TABLE sms_rental_multi_service_quotes ADD CONSTRAINT sms_rental_multi_quotes_currency_nonempty CHECK (btrim(currency_snapshot) <> '') NOT VALID;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_sms_rental_multi_quotes_user_expiry
    ON sms_rental_multi_service_quotes(user_id, expires_at);

CREATE INDEX IF NOT EXISTS idx_sms_rental_multi_quotes_provider_expiry
    ON sms_rental_multi_service_quotes(provider_id, expires_at);
