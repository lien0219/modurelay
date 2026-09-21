-- Advanced SMS rental capabilities for provider-backed long-term numbers.
-- Additive only: existing SMS orders remain valid and are backfilled with their
-- primary service relation. Restore/add-service quotes are short-lived and
-- consumed atomically before provider mutations.
CREATE TABLE IF NOT EXISTS sms_order_services (
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE RESTRICT,
    provider_service_code TEXT NOT NULL DEFAULT '',
    provider_order_id TEXT NOT NULL DEFAULT '',
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0
        CONSTRAINT sms_order_services_provider_cost_nonnegative CHECK (provider_cost_snapshot >= 0),
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0
        CONSTRAINT sms_order_services_sale_price_nonnegative CHECK (sale_price_snapshot >= 0),
    status TEXT NOT NULL DEFAULT 'active'
        CONSTRAINT sms_order_services_status_check CHECK (
            status IN (
                'pending', 'active', 'provider_unknown', 'reconciling',
                'completed', 'cancelled', 'expired', 'failed', 'refunded', 'removed'
            )
        ),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(order_id, service_id)
);

-- An interrupted deployment may have created one of these tables before the
-- current definition was published.  CREATE TABLE IF NOT EXISTS does not
-- reconcile such a table, so add every non-key projection column explicitly.
-- Defaults keep the upgrade non-destructive for existing rows.
ALTER TABLE sms_order_services
    ADD COLUMN IF NOT EXISTS provider_service_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Resolve the latest consumed quote once for the whole backfill. Build the
-- covering partial index first: a correlated LATERAL lookup would rescan
-- sms_quotes for every historical rental order on large installations.
CREATE INDEX IF NOT EXISTS idx_sms_quotes_consumed_order_latest
    ON sms_quotes(consumed_order_id, consumed_at DESC, created_at DESC)
    WHERE consumed_order_id IS NOT NULL;

WITH latest_consumed_quotes AS (
    SELECT DISTINCT ON (consumed_order_id)
        consumed_order_id,
        provider_service_code
    FROM sms_quotes
    WHERE consumed_order_id IS NOT NULL
    ORDER BY consumed_order_id, consumed_at DESC NULLS LAST, created_at DESC
)
INSERT INTO sms_order_services (
    order_id, service_id, provider_service_code, provider_order_id,
    provider_cost_snapshot, sale_price_snapshot, status, metadata
)
SELECT
    o.id, o.service_id, COALESCE(NULLIF(q.provider_service_code,''), s.code),
    o.provider_order_id, o.provider_cost_snapshot, o.sale_price_snapshot,
    o.status,
    '{"primary":true}'::jsonb
FROM sms_orders o
JOIN sms_services s ON s.id=o.service_id
LEFT JOIN latest_consumed_quotes q ON q.consumed_order_id=o.id
WHERE o.product_type='rental'
ON CONFLICT (order_id, service_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_sms_order_services_provider_order
    ON sms_order_services(provider_order_id)
    WHERE provider_order_id <> '';

-- The table may have been created by an interrupted/early rollout before the
-- inline checks above existed. Add them idempotently for that case as NOT
-- VALID: existing rows remain available while all future writes are guarded;
-- validation can be scheduled separately after inspecting legacy data.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_order_services_provider_cost_nonnegative') THEN
        ALTER TABLE sms_order_services ADD CONSTRAINT sms_order_services_provider_cost_nonnegative CHECK (provider_cost_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_order_services_sale_price_nonnegative') THEN
        ALTER TABLE sms_order_services ADD CONSTRAINT sms_order_services_sale_price_nonnegative CHECK (sale_price_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_order_services_status_check') THEN
        ALTER TABLE sms_order_services ADD CONSTRAINT sms_order_services_status_check CHECK (status IN ('pending','active','provider_unknown','reconciling','completed','cancelled','expired','failed','refunded','removed')) NOT VALID;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sms_rental_service_quotes (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE RESTRICT,
    provider_service_code TEXT NOT NULL,
    rent_days INTEGER NOT NULL CHECK (rent_days > 0 AND rent_days <= 366),
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0
        CONSTRAINT sms_rental_service_quotes_provider_cost_nonnegative CHECK (provider_cost_snapshot >= 0),
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0
        CONSTRAINT sms_rental_service_quotes_sale_price_nonnegative CHECK (sale_price_snapshot >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE sms_rental_service_quotes
    ADD COLUMN IF NOT EXISTS user_id BIGINT,
    ADD COLUMN IF NOT EXISTS order_id BIGINT,
    ADD COLUMN IF NOT EXISTS service_id BIGINT,
    ADD COLUMN IF NOT EXISTS provider_service_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rent_days INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_sms_rental_service_quotes_user_expiry
    ON sms_rental_service_quotes(user_id, expires_at);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_quotes_provider_cost_nonnegative') THEN
        ALTER TABLE sms_rental_service_quotes ADD CONSTRAINT sms_rental_service_quotes_provider_cost_nonnegative CHECK (provider_cost_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_quotes_sale_price_nonnegative') THEN
        ALTER TABLE sms_rental_service_quotes ADD CONSTRAINT sms_rental_service_quotes_sale_price_nonnegative CHECK (sale_price_snapshot >= 0) NOT VALID;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS sms_rental_restore_quotes (
    id TEXT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    provider_history_order_id TEXT NOT NULL,
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0
        CONSTRAINT sms_rental_restore_quotes_provider_cost_nonnegative CHECK (provider_cost_snapshot >= 0),
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0
        CONSTRAINT sms_rental_restore_quotes_sale_price_nonnegative CHECK (sale_price_snapshot >= 0),
    service_code TEXT NOT NULL,
    country_code TEXT NOT NULL,
    phone_number TEXT NOT NULL DEFAULT '',
    duration_days INTEGER NOT NULL DEFAULT 0
        CONSTRAINT sms_rental_restore_quotes_duration_nonnegative CHECK (duration_days >= 0),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE sms_rental_restore_quotes
    ADD COLUMN IF NOT EXISTS user_id BIGINT,
    ADD COLUMN IF NOT EXISTS source_order_id BIGINT,
    ADD COLUMN IF NOT EXISTS provider_history_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS service_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS country_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phone_number TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS duration_days INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS consumed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_sms_rental_restore_quotes_user_expiry
    ON sms_rental_restore_quotes(user_id, expires_at);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_restore_quotes_provider_cost_nonnegative') THEN
        ALTER TABLE sms_rental_restore_quotes ADD CONSTRAINT sms_rental_restore_quotes_provider_cost_nonnegative CHECK (provider_cost_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_restore_quotes_sale_price_nonnegative') THEN
        ALTER TABLE sms_rental_restore_quotes ADD CONSTRAINT sms_rental_restore_quotes_sale_price_nonnegative CHECK (sale_price_snapshot >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_restore_quotes_duration_nonnegative') THEN
        ALTER TABLE sms_rental_restore_quotes ADD CONSTRAINT sms_rental_restore_quotes_duration_nonnegative CHECK (duration_days >= 0) NOT VALID;
    END IF;
END $$;
