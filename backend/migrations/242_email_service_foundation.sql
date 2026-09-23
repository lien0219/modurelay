-- Email verification foundation. The provider is deliberately kept behind the
-- channel boundary; user responses never expose provider identifiers or costs.
CREATE TABLE IF NOT EXISTS email_providers (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    credential_ref TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    health_status TEXT NOT NULL DEFAULT 'unknown',
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    billing JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_channels (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    public_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'primary' CHECK (role IN ('primary','backup')),
    provider_id BIGINT NOT NULL REFERENCES email_providers(id),
    email_type TEXT NOT NULL DEFAULT 'temporary_gmail',
    privacy_level TEXT NOT NULL DEFAULT 'public_temporary',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    visible BOOLEAN NOT NULL DEFAULT TRUE,
    healthy BOOLEAN NOT NULL DEFAULT FALSE,
    sale_price NUMERIC(20,8) NOT NULL DEFAULT 0,
    order_ttl_seconds INTEGER NOT NULL DEFAULT 900,
    capture_policy TEXT NOT NULL DEFAULT 'on_target_email_received',
    refund_policy TEXT NOT NULL DEFAULT 'refund_if_no_message',
    polling_backoff JSONB NOT NULL DEFAULT '[2,4,7,10,15,20,30,45,60]'::jsonb,
    max_provider_requests_per_order INTEGER NOT NULL DEFAULT 60,
    sort_order INTEGER NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_services (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_service_match_rules (
    id BIGSERIAL PRIMARY KEY,
    service_id BIGINT NOT NULL REFERENCES email_services(id) ON DELETE CASCADE,
    sender_exact TEXT NOT NULL DEFAULT '',
    sender_domain TEXT NOT NULL DEFAULT '',
    subject_contains TEXT NOT NULL DEFAULT '',
    subject_regex TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_orders (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    order_no TEXT NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    channel_id BIGINT NOT NULL REFERENCES email_channels(id),
    provider_id BIGINT NOT NULL REFERENCES email_providers(id),
    service_id BIGINT NOT NULL REFERENCES email_services(id),
    provider_inbox_id TEXT NOT NULL DEFAULT '',
    email_address TEXT NOT NULL DEFAULT '',
    address_type TEXT NOT NULL DEFAULT 'gmail',
    status TEXT NOT NULL DEFAULT 'creating',
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    provider_cost_estimate_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    pricing_rule_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    success_rate_snapshot NUMERIC(8,5),
    success_rate_grade_snapshot TEXT NOT NULL DEFAULT '',
    refund_policy_snapshot TEXT NOT NULL DEFAULT 'refund_if_no_message',
    capture_policy_snapshot TEXT NOT NULL DEFAULT 'on_target_email_received',
    reserved_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    captured_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    released_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    refunded_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    inbox_created_at TIMESTAMPTZ,
    waiting_started_at TIMESTAMPTZ,
    first_message_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    poll_count INTEGER NOT NULL DEFAULT 0,
    next_poll_at TIMESTAMPTZ,
    last_polled_at TIMESTAMPTZ,
    provider_request_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_public_message TEXT NOT NULL DEFAULT '',
    error_admin_message TEXT NOT NULL DEFAULT '',
    refund_status TEXT NOT NULL DEFAULT 'not_requested',
    refund_reason TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS email_messages (
    id BIGSERIAL PRIMARY KEY,
    email_order_id BIGINT NOT NULL REFERENCES email_orders(id) ON DELETE CASCADE,
    provider_message_id TEXT NOT NULL DEFAULT '',
    from_address TEXT NOT NULL DEFAULT '',
    from_name TEXT NOT NULL DEFAULT '',
    to_address TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    text_body TEXT NOT NULL DEFAULT '',
    html_body TEXT NOT NULL DEFAULT '',
    verification_code TEXT NOT NULL DEFAULT '',
    verification_url TEXT NOT NULL DEFAULT '',
    verification_confidence NUMERIC(5,4) NOT NULL DEFAULT 0,
    verification_method TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dedupe_hash TEXT NOT NULL,
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(email_order_id, dedupe_hash)
);

CREATE TABLE IF NOT EXISTS email_order_events (
    id BIGSERIAL PRIMARY KEY,
    email_order_id BIGINT NOT NULL REFERENCES email_orders(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT 'system',
    idempotency_key TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS email_provider_usage (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES email_providers(id) ON DELETE CASCADE,
    email_order_id BIGINT REFERENCES email_orders(id) ON DELETE SET NULL,
    operation TEXT NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    success BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    rate_limited BOOLEAN NOT NULL DEFAULT FALSE,
    estimated_request_cost NUMERIC(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_orders_user_created ON email_orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_orders_poll ON email_orders(status, next_poll_at);
CREATE INDEX IF NOT EXISTS idx_email_messages_order_received ON email_messages(email_order_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_email_usage_provider_created ON email_provider_usage(provider_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_email_order_events_idempotency
    ON email_order_events(email_order_id, event_type, idempotency_key)
    WHERE idempotency_key <> '';

INSERT INTO email_providers (code, name, base_url, capabilities)
VALUES ('emailnator', 'Emailnator / Gmailnator', 'https://gmailnator.p.rapidapi.com',
        '{"supports_temporary_inbox":true,"supports_generate_single":true,"supports_generate_bulk":true,"supports_inbox_list":true,"supports_message_read":true,"supports_message_delete":true,"supports_polling":true,"supports_verification_code":true,"supports_verification_url":true,"supports_html_message":true,"supports_webhook":false,"supports_sse":false,"supports_imap":false,"supports_smtp":false,"supports_custom_domain":false,"supports_private_inbox":false,"supports_persistent_inbox":false,"supports_provider_refund":false}')
ON CONFLICT (code) DO NOTHING;

UPDATE email_providers
SET metadata = COALESCE(metadata, '{}'::jsonb) || '{"portal_url":"https://rapidapi.com/collection/gmailnator-api"}'::jsonb
WHERE code = 'emailnator';

INSERT INTO email_channels (code, public_name, role, provider_id, email_type, privacy_level, sale_price, sort_order)
SELECT 'email_channel_1', '邮箱通道1', 'primary', id, 'temporary_gmail', 'public_temporary', 0.30, 1
FROM email_providers WHERE code = 'emailnator'
ON CONFLICT (code) DO NOTHING;

INSERT INTO email_services (code, name, sort_order)
VALUES ('google','Google',1), ('openai','OpenAI',2), ('github','GitHub',3), ('microsoft','Microsoft',4), ('discord','Discord',5), ('amazon','Amazon',6), ('other','其他',99)
ON CONFLICT (code) DO NOTHING;

-- Keep seeded public labels user-facing and provider-neutral even when this
-- migration is applied on top of an earlier development snapshot.
UPDATE email_channels SET public_name = '邮箱通道1' WHERE code = 'email_channel_1';
UPDATE email_services SET name = '其他' WHERE code = 'other';

-- Conservative defaults. A verified sender domain (including subdomains)
-- identifies the target service; generic "other" orders accept any sender.
INSERT INTO email_service_match_rules (service_id, sender_domain, subject_contains, priority)
SELECT id, 'google.com', '', 10 FROM email_services s
WHERE s.code = 'google' AND NOT EXISTS (
    SELECT 1 FROM email_service_match_rules r
    WHERE r.service_id = s.id AND r.sender_domain = 'google.com' AND r.priority = 10
);
INSERT INTO email_service_match_rules (service_id, sender_domain, subject_contains, priority)
SELECT id, 'openai.com', '', 10 FROM email_services s
WHERE s.code = 'openai' AND NOT EXISTS (
    SELECT 1 FROM email_service_match_rules r
    WHERE r.service_id = s.id AND r.sender_domain = 'openai.com' AND r.priority = 10
);
INSERT INTO email_service_match_rules (service_id, sender_domain, subject_contains, priority)
SELECT id, 'github.com', '', 10 FROM email_services s
WHERE s.code = 'github' AND NOT EXISTS (
    SELECT 1 FROM email_service_match_rules r
    WHERE r.service_id = s.id AND r.sender_domain = 'github.com' AND r.priority = 10
);
INSERT INTO email_service_match_rules (service_id, sender_domain, subject_contains, priority)
SELECT id, 'microsoft.com', '', 10 FROM email_services s
WHERE s.code = 'microsoft' AND NOT EXISTS (
    SELECT 1 FROM email_service_match_rules r
    WHERE r.service_id = s.id AND r.sender_domain = 'microsoft.com' AND r.priority = 10
);
INSERT INTO email_service_match_rules (service_id, sender_domain, subject_contains, priority)
SELECT id, 'discord.com', '', 10 FROM email_services s
WHERE s.code = 'discord' AND NOT EXISTS (
    SELECT 1 FROM email_service_match_rules r
    WHERE r.service_id = s.id AND r.sender_domain = 'discord.com' AND r.priority = 10
);
INSERT INTO email_service_match_rules (service_id, sender_domain, subject_contains, priority)
SELECT id, 'amazon.com', '', 10 FROM email_services s
WHERE s.code = 'amazon' AND NOT EXISTS (
    SELECT 1 FROM email_service_match_rules r
    WHERE r.service_id = s.id AND r.sender_domain = 'amazon.com' AND r.priority = 10
);

UPDATE email_service_match_rules
SET subject_contains = ''
WHERE priority = 10 AND sender_domain IN ('google.com','openai.com','github.com','microsoft.com','discord.com','amazon.com')
  AND subject_contains = 'verification';

INSERT INTO settings (key, value) VALUES
    ('email_service_enabled', 'false'),
    ('email_message_retention_days', '7'),
    ('email_verification_secret_retention_hours', '24'),
    ('email_success_rate_minimum_sample_size', '20'),
    ('email_success_grade_s_threshold', '0.95'),
    ('email_success_grade_a_threshold', '0.90'),
    ('email_success_grade_b_threshold', '0.80'),
    ('email_success_grade_c_threshold', '0.60')
ON CONFLICT (key) DO NOTHING;
