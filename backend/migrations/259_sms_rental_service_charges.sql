-- Durable settlement ledger for adding services to active SMS rental numbers.
CREATE TABLE IF NOT EXISTS sms_rental_service_charges (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE RESTRICT,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE RESTRICT,
    quote_id TEXT NOT NULL UNIQUE REFERENCES sms_rental_service_quotes(id) ON DELETE RESTRICT,
    idempotency_key TEXT NOT NULL,
    provider_child_order_id TEXT NOT NULL DEFAULT '',
    currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    settlement_status TEXT NOT NULL DEFAULT 'held',
    status TEXT NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sms_rental_service_charges_idempotency_key_nonempty
        CHECK (btrim(idempotency_key) <> ''),
    CONSTRAINT sms_rental_service_charges_currency_nonempty
        CHECK (btrim(currency_snapshot) <> ''),
    CONSTRAINT sms_rental_service_charges_amounts_nonnegative
        CHECK (reserved_amount >= 0 AND captured_amount >= 0 AND released_amount >= 0),
    CONSTRAINT sms_rental_service_charges_amounts_bounded
        CHECK (captured_amount + released_amount <= reserved_amount),
    CONSTRAINT sms_rental_service_charges_settlement_status_check
        CHECK (settlement_status IN ('held', 'captured', 'released', 'manual_review')),
    CONSTRAINT sms_rental_service_charges_status_check
        CHECK (status IN (
            'pending', 'provider_pending', 'provider_unknown', 'reconciling',
            'completed', 'failed', 'cancelled', 'manual_review'
        )),
    UNIQUE(order_id, idempotency_key)
);

ALTER TABLE sms_rental_service_charges
    ADD COLUMN IF NOT EXISTS user_id BIGINT,
    ADD COLUMN IF NOT EXISTS order_id BIGINT,
    ADD COLUMN IF NOT EXISTS service_id BIGINT,
    ADD COLUMN IF NOT EXISTS quote_id TEXT,
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_child_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS settlement_status TEXT NOT NULL DEFAULT 'held',
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_charges_idempotency_key_nonempty') THEN
        ALTER TABLE sms_rental_service_charges ADD CONSTRAINT sms_rental_service_charges_idempotency_key_nonempty CHECK (btrim(idempotency_key) <> '') NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_charges_currency_nonempty') THEN
        ALTER TABLE sms_rental_service_charges ADD CONSTRAINT sms_rental_service_charges_currency_nonempty CHECK (btrim(currency_snapshot) <> '') NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_charges_amounts_nonnegative') THEN
        ALTER TABLE sms_rental_service_charges ADD CONSTRAINT sms_rental_service_charges_amounts_nonnegative CHECK (reserved_amount >= 0 AND captured_amount >= 0 AND released_amount >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_charges_amounts_bounded') THEN
        ALTER TABLE sms_rental_service_charges ADD CONSTRAINT sms_rental_service_charges_amounts_bounded CHECK (captured_amount + released_amount <= reserved_amount) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_charges_settlement_status_check') THEN
        ALTER TABLE sms_rental_service_charges ADD CONSTRAINT sms_rental_service_charges_settlement_status_check CHECK (settlement_status IN ('held','captured','released','manual_review')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_service_charges_status_check') THEN
        ALTER TABLE sms_rental_service_charges ADD CONSTRAINT sms_rental_service_charges_status_check CHECK (status IN ('pending','provider_pending','provider_unknown','reconciling','completed','failed','cancelled','manual_review')) NOT VALID;
    END IF;
END $$;

-- 259 may have been installed by an earlier build with CASCADE FKs. Replace
-- only those legacy actions; audit rows must survive user/order cleanup and
-- must never disappear as a side effect of deleting business entities.
DO $$
DECLARE
    fk RECORD;
BEGIN
    FOR fk IN
        SELECT c.conname
        FROM pg_constraint c
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
        WHERE c.conrelid = 'sms_rental_service_charges'::regclass
          AND c.contype = 'f'
          AND c.confdeltype = 'c'
          AND ((c.confrelid = 'users'::regclass AND a.attname = 'user_id')
            OR (c.confrelid = 'sms_orders'::regclass AND a.attname = 'order_id'))
    LOOP
        EXECUTE format('ALTER TABLE sms_rental_service_charges DROP CONSTRAINT %I', fk.conname);
    END LOOP;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
        WHERE c.conrelid = 'sms_rental_service_charges'::regclass
          AND c.confrelid = 'users'::regclass
          AND c.contype = 'f'
          AND c.confdeltype = 'r'
          AND a.attname = 'user_id'
    ) THEN
        ALTER TABLE sms_rental_service_charges
            ADD CONSTRAINT sms_rental_service_charges_user_fk_restrict
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY(c.conkey)
        WHERE c.conrelid = 'sms_rental_service_charges'::regclass
          AND c.confrelid = 'sms_orders'::regclass
          AND c.contype = 'f'
          AND c.confdeltype = 'r'
          AND a.attname = 'order_id'
    ) THEN
        ALTER TABLE sms_rental_service_charges
            ADD CONSTRAINT sms_rental_service_charges_order_fk_restrict
            FOREIGN KEY (order_id) REFERENCES sms_orders(id) ON DELETE RESTRICT NOT VALID;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_sms_rental_service_charges_status
    ON sms_rental_service_charges(status, settlement_status, updated_at);
