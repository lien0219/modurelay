-- Durable, fail-closed ledger for provider mutations whose HTTP result can be
-- ambiguous. Restore uses it immediately; the generic operation fields also
-- retain the evidence required to recover a future add-service request.
CREATE TABLE IF NOT EXISTS sms_rental_recovery_operations (
    id BIGSERIAL PRIMARY KEY,
    operation_type TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE RESTRICT,
    result_order_id BIGINT REFERENCES sms_orders(id) ON DELETE RESTRICT,
    restore_quote_id TEXT UNIQUE REFERENCES sms_rental_restore_quotes(id) ON DELETE RESTRICT,
    service_quote_id TEXT UNIQUE REFERENCES sms_rental_service_quotes(id) ON DELETE RESTRICT,
    provider_history_order_id TEXT NOT NULL DEFAULT '',
    provider_parent_order_id TEXT NOT NULL DEFAULT '',
    provider_result_order_id TEXT NOT NULL DEFAULT '',
    -- IDs observed before restore_user/add_service are persisted verbatim so
    -- recovery never guesses from a provider-wide "latest order" query.
    restore_baseline_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    recovery_baseline JSONB NOT NULL DEFAULT '[]'::jsonb,
    service_codes_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    service_code TEXT NOT NULL DEFAULT '',
    country_code TEXT NOT NULL DEFAULT '',
    phone_number TEXT NOT NULL DEFAULT '',
    currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    provider_completed_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'prepared',
    settlement_status TEXT NOT NULL DEFAULT 'held',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT sms_rental_recovery_operation_type_check
        CHECK (operation_type IN ('restore', 'add_service')),
    CONSTRAINT sms_rental_recovery_idempotency_key_nonempty
        CHECK (btrim(idempotency_key) <> ''),
    CONSTRAINT sms_rental_recovery_baseline_array_check
        CHECK (
            jsonb_typeof(restore_baseline_ids) = 'array'
            AND jsonb_typeof(recovery_baseline) = 'array'
        ),
    CONSTRAINT sms_rental_recovery_services_array_check
        CHECK (jsonb_typeof(service_codes_snapshot) = 'array'),
    CONSTRAINT sms_rental_recovery_currency_nonempty
        CHECK (btrim(currency_snapshot) <> ''),
    CONSTRAINT sms_rental_recovery_amounts_nonnegative
        CHECK (reserved_amount >= 0 AND captured_amount >= 0 AND released_amount >= 0),
    CONSTRAINT sms_rental_recovery_amounts_bounded
        CHECK (captured_amount + released_amount <= reserved_amount),
    CONSTRAINT sms_rental_recovery_status_check
        CHECK (status IN (
            'prepared', 'provider_pending', 'provider_unknown', 'reconciling',
            'recovered', 'completed', 'failed', 'cancelled', 'manual_review'
        )),
    CONSTRAINT sms_rental_recovery_settlement_status_check
        CHECK (settlement_status IN ('held', 'captured', 'released', 'manual_review')),
    CONSTRAINT sms_rental_recovery_quote_check CHECK (
        (operation_type = 'restore'
            AND restore_quote_id IS NOT NULL
            AND service_quote_id IS NULL
            AND btrim(provider_history_order_id) <> '')
        OR
        (operation_type = 'add_service'
            AND restore_quote_id IS NULL
            AND service_quote_id IS NOT NULL
            AND btrim(provider_parent_order_id) <> ''
            AND btrim(service_code) <> '')
    ),
    UNIQUE(user_id, operation_type, idempotency_key)
);

ALTER TABLE sms_rental_recovery_operations
    ADD COLUMN IF NOT EXISTS operation_type TEXT NOT NULL DEFAULT 'restore',
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_id BIGINT,
    ADD COLUMN IF NOT EXISTS source_order_id BIGINT,
    ADD COLUMN IF NOT EXISTS result_order_id BIGINT,
    ADD COLUMN IF NOT EXISTS restore_quote_id TEXT,
    ADD COLUMN IF NOT EXISTS service_quote_id TEXT,
    ADD COLUMN IF NOT EXISTS provider_history_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_parent_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS provider_result_order_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS restore_baseline_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS recovery_baseline JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS service_codes_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS service_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS country_code TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS phone_number TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    ADD COLUMN IF NOT EXISTS reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS provider_completed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'prepared',
    ADD COLUMN IF NOT EXISTS settlement_status TEXT NOT NULL DEFAULT 'held',
    ADD COLUMN IF NOT EXISTS last_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_operation_type_check') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_operation_type_check CHECK (operation_type IN ('restore','add_service')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_idempotency_key_nonempty') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_idempotency_key_nonempty CHECK (btrim(idempotency_key) <> '') NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_baseline_array_check') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_baseline_array_check CHECK (jsonb_typeof(restore_baseline_ids) = 'array' AND jsonb_typeof(recovery_baseline) = 'array') NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_services_array_check') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_services_array_check CHECK (jsonb_typeof(service_codes_snapshot) = 'array') NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_currency_nonempty') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_currency_nonempty CHECK (btrim(currency_snapshot) <> '') NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_amounts_nonnegative') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_amounts_nonnegative CHECK (reserved_amount >= 0 AND captured_amount >= 0 AND released_amount >= 0) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_amounts_bounded') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_amounts_bounded CHECK (captured_amount + released_amount <= reserved_amount) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_status_check') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_status_check CHECK (status IN ('prepared','provider_pending','provider_unknown','reconciling','recovered','completed','failed','cancelled','manual_review')) NOT VALID;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sms_rental_recovery_settlement_status_check') THEN
        ALTER TABLE sms_rental_recovery_operations ADD CONSTRAINT sms_rental_recovery_settlement_status_check CHECK (settlement_status IN ('held','captured','released','manual_review')) NOT VALID;
    END IF;
END $$;

-- A provider history order can be restored at most once. Retries must resume
-- the same ledger row so an ambiguous response can never create a second debit.
CREATE UNIQUE INDEX IF NOT EXISTS idx_sms_rental_recovery_restore_once
    ON sms_rental_recovery_operations(user_id, provider_history_order_id)
    WHERE operation_type = 'restore' AND provider_history_order_id <> '';

CREATE INDEX IF NOT EXISTS idx_sms_rental_recovery_pending
    ON sms_rental_recovery_operations(status, settlement_status, updated_at)
    WHERE status IN ('provider_pending', 'provider_unknown', 'reconciling', 'manual_review');

CREATE INDEX IF NOT EXISTS idx_sms_rental_recovery_provider_result
    ON sms_rental_recovery_operations(provider_result_order_id)
    WHERE provider_result_order_id <> '';

-- Recovery evidence for add-service operations whose provider response is
-- ambiguous. The baseline contains active provider order ids observed before
-- mutation so reconciliation can only bind a genuinely new matching order.
ALTER TABLE sms_rental_service_charges
    ADD COLUMN IF NOT EXISTS recovery_baseline JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS reconciliation_status TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_sms_rental_service_charges_reconcile
    ON sms_rental_service_charges(status, reconciliation_status, updated_at);
