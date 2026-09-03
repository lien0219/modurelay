ALTER TABLE activity_benefit_configs
    ADD COLUMN IF NOT EXISTS random_min_amount NUMERIC(20, 8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS random_max_amount NUMERIC(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE activity_benefit_configs
    DROP CONSTRAINT IF EXISTS activity_benefit_configs_reward_mode_check;

ALTER TABLE activity_benefit_configs
    ADD CONSTRAINT activity_benefit_configs_reward_mode_check CHECK (
        (
            reward_amount > 0
            AND random_min_amount = 0
            AND random_max_amount = 0
        )
        OR (
            reward_amount = 0
            AND random_min_amount > 0
            AND random_max_amount >= random_min_amount
            AND random_max_amount <= 1000000
            AND random_min_amount = ROUND(random_min_amount, 2)
            AND random_max_amount = ROUND(random_max_amount, 2)
        )
        OR (
            reward_amount = 0
            AND total_stock = 0
            AND random_min_amount = 0
            AND random_max_amount = 0
        )
    );

COMMENT ON COLUMN activity_benefit_configs.random_min_amount IS
    'Inclusive random reward lower bound. Used only when reward_amount is zero.';
COMMENT ON COLUMN activity_benefit_configs.random_max_amount IS
    'Inclusive random reward upper bound. Used only when reward_amount is zero.';
