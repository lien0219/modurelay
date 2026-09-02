-- Allow positive balance redeem codes to participate in recharge lotteries.
ALTER TABLE activity_lottery_qualifications
    ALTER COLUMN payment_order_id DROP NOT NULL;

ALTER TABLE activity_lottery_qualifications
    ADD COLUMN IF NOT EXISTS redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE RESTRICT;

DO $$
BEGIN
    ALTER TABLE activity_lottery_qualifications
        ADD CONSTRAINT activity_lottery_qualifications_source_check CHECK (
            num_nonnulls(payment_order_id, redeem_code_id) = 1
        );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

DO $$
BEGIN
    ALTER TABLE activity_lottery_qualifications
        ADD CONSTRAINT activity_lottery_qualifications_redeem_unique
        UNIQUE (activity_id, redeem_code_id);
EXCEPTION
    WHEN duplicate_object THEN NULL;
END
$$;

