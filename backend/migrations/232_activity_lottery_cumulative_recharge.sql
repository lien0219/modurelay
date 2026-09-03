-- Track every qualifying-period recharge so sub-threshold payments can accumulate.
ALTER TABLE activity_lottery_qualifications
    ADD COLUMN IF NOT EXISTS refunded_amount NUMERIC(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE activity_lottery_qualifications
    DROP CONSTRAINT IF EXISTS activity_lottery_qualifications_draws_check;

ALTER TABLE activity_lottery_qualifications
    ADD CONSTRAINT activity_lottery_qualifications_draws_check CHECK (
        granted_draws >= 0 AND revoked_draws >= 0 AND revoked_draws <= granted_draws
    );

ALTER TABLE activity_lottery_qualifications
    DROP CONSTRAINT IF EXISTS activity_lottery_qualifications_refund_check;

ALTER TABLE activity_lottery_qualifications
    ADD CONSTRAINT activity_lottery_qualifications_refund_check CHECK (
        refunded_amount >= 0 AND refunded_amount <= recharge_amount
    );

CREATE INDEX IF NOT EXISTS activity_lottery_qualifications_progress_idx
    ON activity_lottery_qualifications(user_id, activity_id, config_id, created_at, id);
