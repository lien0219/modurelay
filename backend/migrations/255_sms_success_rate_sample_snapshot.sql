-- Snapshot the sample size used when publishing/pricing a 30-day platform
-- delivery rate so historical quotes and orders remain auditable after the
-- rolling window changes.

ALTER TABLE sms_quotes
    ADD COLUMN IF NOT EXISTS success_rate_sample_size_snapshot INTEGER NOT NULL DEFAULT 0;

ALTER TABLE sms_orders
    ADD COLUMN IF NOT EXISTS success_rate_sample_size_snapshot INTEGER NOT NULL DEFAULT 0;
