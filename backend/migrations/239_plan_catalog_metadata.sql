ALTER TABLE plan_catalog_items
    ADD COLUMN IF NOT EXISTS group_name VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS rate_multiplier NUMERIC(18, 4) NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS daily_limit_usd NUMERIC(18, 4),
    ADD COLUMN IF NOT EXISTS weekly_limit_usd NUMERIC(18, 4),
    ADD COLUMN IF NOT EXISTS monthly_limit_usd NUMERIC(18, 4);

ALTER TABLE plan_catalog_items
    DROP CONSTRAINT IF EXISTS plan_catalog_rate_multiplier_nonnegative,
    ADD CONSTRAINT plan_catalog_rate_multiplier_nonnegative CHECK (rate_multiplier >= 0),
    DROP CONSTRAINT IF EXISTS plan_catalog_daily_limit_nonnegative,
    ADD CONSTRAINT plan_catalog_daily_limit_nonnegative CHECK (daily_limit_usd IS NULL OR daily_limit_usd >= 0),
    DROP CONSTRAINT IF EXISTS plan_catalog_weekly_limit_nonnegative,
    ADD CONSTRAINT plan_catalog_weekly_limit_nonnegative CHECK (weekly_limit_usd IS NULL OR weekly_limit_usd >= 0),
    DROP CONSTRAINT IF EXISTS plan_catalog_monthly_limit_nonnegative,
    ADD CONSTRAINT plan_catalog_monthly_limit_nonnegative CHECK (monthly_limit_usd IS NULL OR monthly_limit_usd >= 0);
