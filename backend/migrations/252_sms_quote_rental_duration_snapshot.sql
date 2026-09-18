-- Bind rental duration to the quote so clients cannot reuse a quote for another period.
ALTER TABLE sms_quotes ADD COLUMN IF NOT EXISTS duration_value INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sms_quotes ADD COLUMN IF NOT EXISTS duration_unit TEXT NOT NULL DEFAULT '';
