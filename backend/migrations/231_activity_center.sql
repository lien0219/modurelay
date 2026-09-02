CREATE TABLE IF NOT EXISTS activities (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(80) NOT NULL UNIQUE,
    activity_type VARCHAR(40) NOT NULL,
    title VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    current_config_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activities_type_check CHECK (activity_type IN ('recharge_lottery', 'limited_time_benefit')),
    CONSTRAINT activities_status_check CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT activities_config_version_check CHECK (current_config_version > 0)
);

CREATE INDEX IF NOT EXISTS activities_public_list_idx
    ON activities(status, sort_order, id);

CREATE TABLE IF NOT EXISTS activity_lottery_configs (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    recharge_threshold NUMERIC(20, 8) NOT NULL,
    draws_per_threshold INTEGER NOT NULL DEFAULT 1,
    max_chances_per_order INTEGER NOT NULL DEFAULT 0,
    per_user_draw_limit INTEGER NOT NULL DEFAULT 0,
    daily_draw_limit INTEGER NOT NULL DEFAULT 0,
    daily_limit_timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    starts_at TIMESTAMPTZ NULL,
    ends_at TIMESTAMPTZ NULL,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activity_lottery_configs_version_unique UNIQUE (activity_id, version),
    CONSTRAINT activity_lottery_configs_currency_check CHECK (currency = UPPER(currency) AND char_length(currency) = 3),
    CONSTRAINT activity_lottery_configs_threshold_check CHECK (recharge_threshold > 0 AND recharge_threshold <= 1000000),
    CONSTRAINT activity_lottery_configs_draws_check CHECK (draws_per_threshold > 0 AND draws_per_threshold <= 1000000),
    CONSTRAINT activity_lottery_configs_limits_check CHECK (
        max_chances_per_order BETWEEN 0 AND 1000000
        AND per_user_draw_limit BETWEEN 0 AND 1000000
        AND daily_draw_limit BETWEEN 0 AND 1000000
    ),
    CONSTRAINT activity_lottery_configs_window_check CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at < ends_at)
);

CREATE TABLE IF NOT EXISTS activity_lottery_prizes (
    id BIGSERIAL PRIMARY KEY,
    config_id BIGINT NOT NULL REFERENCES activity_lottery_configs(id) ON DELETE RESTRICT,
    name VARCHAR(120) NOT NULL,
    amount NUMERIC(20, 8) NOT NULL DEFAULT 0,
    probability_ppm INTEGER NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activity_lottery_prizes_probability_check CHECK (probability_ppm > 0 AND probability_ppm <= 1000000),
    CONSTRAINT activity_lottery_prizes_amount_check CHECK (amount BETWEEN 0 AND 1000000)
);

CREATE INDEX IF NOT EXISTS activity_lottery_prizes_config_idx
    ON activity_lottery_prizes(config_id, sort_order, id);

CREATE TABLE IF NOT EXISTS activity_benefit_configs (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    reward_amount NUMERIC(20, 8) NOT NULL,
    total_stock BIGINT NOT NULL,
    per_user_limit INTEGER NOT NULL DEFAULT 1,
    starts_at TIMESTAMPTZ NULL,
    ends_at TIMESTAMPTZ NULL,
    created_by BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activity_benefit_configs_version_unique UNIQUE (activity_id, version),
    CONSTRAINT activity_benefit_configs_currency_check CHECK (currency = UPPER(currency) AND char_length(currency) = 3),
    CONSTRAINT activity_benefit_configs_amount_check CHECK (reward_amount BETWEEN 0 AND 1000000),
    CONSTRAINT activity_benefit_configs_stock_check CHECK (total_stock BETWEEN 0 AND 1000000),
    CONSTRAINT activity_benefit_configs_user_limit_check CHECK (per_user_limit BETWEEN 1 AND 1000000),
    CONSTRAINT activity_benefit_configs_window_check CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at < ends_at)
);

CREATE TABLE IF NOT EXISTS activity_lottery_qualifications (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    config_id BIGINT NOT NULL REFERENCES activity_lottery_configs(id) ON DELETE RESTRICT,
    payment_order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    recharge_amount NUMERIC(20, 8) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    granted_draws INTEGER NOT NULL,
    revoked_draws INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ NULL,
    CONSTRAINT activity_lottery_qualifications_order_unique UNIQUE (activity_id, payment_order_id),
    CONSTRAINT activity_lottery_qualifications_amount_check CHECK (recharge_amount > 0),
    CONSTRAINT activity_lottery_qualifications_draws_check CHECK (
        granted_draws > 0 AND revoked_draws >= 0 AND revoked_draws <= granted_draws
    )
);

CREATE INDEX IF NOT EXISTS activity_lottery_qualifications_user_idx
    ON activity_lottery_qualifications(user_id, activity_id, created_at DESC);

CREATE TABLE IF NOT EXISTS activity_lottery_draw_accounts (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    granted_draws INTEGER NOT NULL DEFAULT 0,
    used_draws INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, activity_id),
    CONSTRAINT activity_lottery_draw_accounts_check CHECK (
        granted_draws >= 0 AND used_draws >= 0 AND used_draws <= granted_draws
    )
);

CREATE TABLE IF NOT EXISTS activity_lottery_draws (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    config_id BIGINT NOT NULL REFERENCES activity_lottery_configs(id) ON DELETE RESTRICT,
    prize_id BIGINT NOT NULL REFERENCES activity_lottery_prizes(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    request_id VARCHAR(64) NOT NULL,
    random_value INTEGER NOT NULL,
    prize_name VARCHAR(120) NOT NULL,
    reward_amount NUMERIC(20, 8) NOT NULL,
    balance_after NUMERIC(20, 8) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activity_lottery_draws_request_unique UNIQUE (user_id, activity_id, request_id),
    CONSTRAINT activity_lottery_draws_random_check CHECK (random_value >= 0 AND random_value < 1000000),
    CONSTRAINT activity_lottery_draws_reward_check CHECK (reward_amount >= 0)
);

CREATE INDEX IF NOT EXISTS activity_lottery_draws_user_idx
    ON activity_lottery_draws(user_id, activity_id, created_at DESC);
CREATE INDEX IF NOT EXISTS activity_lottery_draws_daily_idx
    ON activity_lottery_draws(activity_id, user_id, created_at);

CREATE TABLE IF NOT EXISTS activity_benefit_claims (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    config_id BIGINT NOT NULL REFERENCES activity_benefit_configs(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    request_id VARCHAR(64) NOT NULL,
    reward_amount NUMERIC(20, 8) NOT NULL,
    balance_after NUMERIC(20, 8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activity_benefit_claims_request_unique UNIQUE (user_id, activity_id, request_id),
    CONSTRAINT activity_benefit_claims_reward_check CHECK (reward_amount > 0)
);

CREATE INDEX IF NOT EXISTS activity_benefit_claims_config_idx
    ON activity_benefit_claims(config_id, created_at);
CREATE INDEX IF NOT EXISTS activity_benefit_claims_user_idx
    ON activity_benefit_claims(user_id, activity_id, created_at DESC);

CREATE TABLE IF NOT EXISTS activity_reward_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    activity_id BIGINT NOT NULL REFERENCES activities(id) ON DELETE RESTRICT,
    activity_type VARCHAR(40) NOT NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT NOT NULL,
    amount NUMERIC(20, 8) NOT NULL,
    balance_after NUMERIC(20, 8) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'CNY',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT activity_reward_ledger_source_unique UNIQUE (source_type, source_id),
    CONSTRAINT activity_reward_ledger_amount_check CHECK (amount > 0),
    CONSTRAINT activity_reward_ledger_source_check CHECK (source_type IN ('lottery_draw', 'benefit_claim'))
);

CREATE INDEX IF NOT EXISTS activity_reward_ledger_user_idx
    ON activity_reward_ledger(user_id, created_at DESC);

CREATE OR REPLACE FUNCTION reject_activity_config_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'published activity configuration rows are immutable'
        USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS activity_lottery_configs_immutable ON activity_lottery_configs;
CREATE TRIGGER activity_lottery_configs_immutable
    BEFORE UPDATE OR DELETE ON activity_lottery_configs
    FOR EACH ROW EXECUTE FUNCTION reject_activity_config_mutation();

DROP TRIGGER IF EXISTS activity_lottery_prizes_immutable ON activity_lottery_prizes;
CREATE TRIGGER activity_lottery_prizes_immutable
    BEFORE UPDATE OR DELETE ON activity_lottery_prizes
    FOR EACH ROW EXECUTE FUNCTION reject_activity_config_mutation();

DROP TRIGGER IF EXISTS activity_benefit_configs_immutable ON activity_benefit_configs;
CREATE TRIGGER activity_benefit_configs_immutable
    BEFORE UPDATE OR DELETE ON activity_benefit_configs
    FOR EACH ROW EXECUTE FUNCTION reject_activity_config_mutation();

INSERT INTO settings (key, value)
VALUES ('activity_center_enabled', 'false')
ON CONFLICT (key) DO NOTHING;

INSERT INTO activities (slug, activity_type, title, description, status, enabled, sort_order, current_config_version)
VALUES
    ('recharge-lottery', 'recharge_lottery', '充值转盘抽奖', '充值达标后获得抽奖机会', 'published', FALSE, 10, 1),
    ('limited-time-benefit', 'limited_time_benefit', '限时福利', '在活动时间内领取限量余额福利', 'published', FALSE, 20, 1)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO activity_lottery_configs (
    activity_id, version, currency, recharge_threshold, draws_per_threshold,
    max_chances_per_order, per_user_draw_limit, daily_draw_limit, daily_limit_timezone
)
SELECT id, 1, 'CNY', 100, 1, 10, 0, 5, 'Asia/Shanghai'
FROM activities
WHERE slug = 'recharge-lottery'
ON CONFLICT (activity_id, version) DO NOTHING;

INSERT INTO activity_lottery_prizes (config_id, name, amount, probability_ppm, sort_order)
SELECT c.id, '谢谢参与', 0, 1000000, 10
FROM activity_lottery_configs c
JOIN activities a ON a.id = c.activity_id
WHERE a.slug = 'recharge-lottery' AND c.version = 1
  AND NOT EXISTS (SELECT 1 FROM activity_lottery_prizes p WHERE p.config_id = c.id);

INSERT INTO activity_benefit_configs (
    activity_id, version, currency, reward_amount, total_stock, per_user_limit
)
SELECT id, 1, 'CNY', 0, 0, 1
FROM activities
WHERE slug = 'limited-time-benefit'
ON CONFLICT (activity_id, version) DO NOTHING;
