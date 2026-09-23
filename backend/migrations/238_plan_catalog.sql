-- Independent external-link plan catalog. This intentionally does not reference
-- payment_plans or payment_orders: purchasing is completed by an administrator-
-- configured external HTTPS/HTTP destination.
CREATE TABLE IF NOT EXISTS plan_catalog_items (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    subtitle VARCHAR(160) NOT NULL DEFAULT '',
    description VARCHAR(600) NOT NULL DEFAULT '',
    price NUMERIC(18, 4) NOT NULL,
    original_price NUMERIC(18, 4),
    currency VARCHAR(8) NOT NULL DEFAULT 'CNY',
    billing_period VARCHAR(24) NOT NULL DEFAULT 'monthly',
    badge VARCHAR(40) NOT NULL DEFAULT '',
    accent VARCHAR(24) NOT NULL DEFAULT 'indigo',
    benefits JSONB NOT NULL DEFAULT '[]'::jsonb,
    payment_url VARCHAR(2048) NOT NULL,
    is_published BOOLEAN NOT NULL DEFAULT FALSE,
    is_featured BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT plan_catalog_price_nonnegative CHECK (price >= 0),
    CONSTRAINT plan_catalog_original_price_valid CHECK (original_price IS NULL OR original_price >= price),
    CONSTRAINT plan_catalog_currency_valid CHECK (currency IN ('CNY', 'USD', 'EUR', 'HKD')),
    CONSTRAINT plan_catalog_period_valid CHECK (billing_period IN ('monthly', 'quarterly', 'yearly', 'one_time', 'custom')),
    CONSTRAINT plan_catalog_accent_valid CHECK (accent IN ('indigo', 'emerald', 'amber', 'rose', 'slate')),
    CONSTRAINT plan_catalog_benefits_array CHECK (jsonb_typeof(benefits) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_plan_catalog_public
    ON plan_catalog_items (sort_order ASC, id ASC)
    WHERE is_published = TRUE;
