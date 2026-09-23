-- SMS Verification foundation. Provider credentials are encrypted/managed outside
-- the public DTO boundary; users only ever see channel_* identifiers.
CREATE TABLE IF NOT EXISTS sms_providers (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    base_url TEXT NOT NULL,
    credential_ref TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    health_status TEXT NOT NULL DEFAULT 'unknown',
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sms_channels (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    public_name TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'primary' CHECK (role IN ('primary','backup')),
    provider_id BIGINT NOT NULL REFERENCES sms_providers(id),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    visible BOOLEAN NOT NULL DEFAULT TRUE,
    healthy BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sms_services (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    icon TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT 'other',
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sms_countries (
    id BIGSERIAL PRIMARY KEY,
    iso2 TEXT NOT NULL UNIQUE,
    iso3 TEXT NOT NULL DEFAULT '',
    calling_code TEXT NOT NULL DEFAULT '',
    name_zh TEXT NOT NULL DEFAULT '',
    name_en TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sms_provider_service_mappings (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES sms_providers(id) ON DELETE CASCADE,
    service_id BIGINT NOT NULL REFERENCES sms_services(id) ON DELETE CASCADE,
    provider_service_code TEXT NOT NULL,
    provider_service_name TEXT NOT NULL DEFAULT '',
    temporary_supported BOOLEAN NOT NULL DEFAULT TRUE,
    rental_supported BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE(provider_id, service_id)
);

CREATE TABLE IF NOT EXISTS sms_provider_country_mappings (
    id BIGSERIAL PRIMARY KEY,
    provider_id BIGINT NOT NULL REFERENCES sms_providers(id) ON DELETE CASCADE,
    country_id BIGINT NOT NULL REFERENCES sms_countries(id) ON DELETE CASCADE,
    provider_country_id TEXT NOT NULL,
    provider_country_code TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE(provider_id, country_id)
);

CREATE TABLE IF NOT EXISTS sms_success_rate_rules (
    id BIGSERIAL PRIMARY KEY,
    grade TEXT NOT NULL UNIQUE CHECK (grade IN ('S','A','B','C','D')),
    minimum_rate NUMERIC(8,5) NOT NULL,
    multiplier NUMERIC(20,8) NOT NULL DEFAULT 1,
    fixed_markup NUMERIC(20,8) NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sms_orders (
    id BIGSERIAL PRIMARY KEY,
    public_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL REFERENCES users(id),
    channel_id BIGINT NOT NULL REFERENCES sms_channels(id),
    provider_id BIGINT NOT NULL REFERENCES sms_providers(id),
    service_id BIGINT NOT NULL REFERENCES sms_services(id),
    country_id BIGINT NOT NULL REFERENCES sms_countries(id),
    product_type TEXT NOT NULL CHECK (product_type IN ('temporary','rental')),
    status TEXT NOT NULL DEFAULT 'pending',
    provider_order_id TEXT NOT NULL DEFAULT '',
    phone_number TEXT NOT NULL DEFAULT '',
    provider_cost_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    sale_price_snapshot NUMERIC(20,8) NOT NULL DEFAULT 0,
    success_rate_snapshot NUMERIC(8,5),
    success_rate_source_snapshot TEXT NOT NULL DEFAULT 'unavailable',
    success_rate_grade_snapshot TEXT NOT NULL DEFAULT '',
    success_rate_multiplier_snapshot NUMERIC(20,8) NOT NULL DEFAULT 1,
    currency_snapshot TEXT NOT NULL DEFAULT 'USD',
    expires_at TIMESTAMPTZ,
    idempotency_key TEXT NOT NULL,
    refund_status TEXT NOT NULL DEFAULT 'not_requested',
    refund_reason TEXT NOT NULL DEFAULT '',
    last_provider_error TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, idempotency_key)
);

CREATE TABLE IF NOT EXISTS sms_messages (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    message_text TEXT NOT NULL DEFAULT '',
    verification_code TEXT NOT NULL DEFAULT '',
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS sms_order_events (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES sms_orders(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,
    actor TEXT NOT NULL DEFAULT 'system',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sms_provider_health (
    provider_id BIGINT PRIMARY KEY REFERENCES sms_providers(id) ON DELETE CASCADE,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    stock JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sms_refunds (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES sms_orders(id) ON DELETE CASCADE,
    requested_amount NUMERIC(20,8) NOT NULL,
    provider_amount NUMERIC(20,8) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    standardized_reason TEXT NOT NULL DEFAULT '',
    provider_reference TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sms_orders_user_status ON sms_orders(user_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sms_orders_provider_status ON sms_orders(provider_id, status);
CREATE INDEX IF NOT EXISTS idx_sms_events_order_created ON sms_order_events(order_id, created_at DESC);

INSERT INTO sms_providers (code, name, base_url, enabled, capabilities)
VALUES
    ('5sim', '5SIM', 'https://5sim.net/v1', FALSE, '{"supports_temporary":true,"supports_rental":false,"supports_polling":true,"supports_cancel":true,"supports_refund":true}'),
    ('smspool', 'SMSPool', 'https://api.smspool.net', FALSE, '{"supports_temporary":true,"supports_rental":false,"supports_polling":true,"supports_cancel":true,"supports_refund":true}'),
    ('sms_activate', 'SMS-Activate', 'https://api.sms-activate.org/stubs/handler_api.php', FALSE, '{"supports_temporary":true,"supports_rental":false,"supports_polling":true,"supports_cancel":true,"supports_refund":true}'),
    ('onlinesim', 'OnlineSIM', 'https://onlinesim.io/api', FALSE, '{"supports_temporary":true,"supports_rental":true,"supports_polling":true,"supports_cancel":true,"supports_refund":false}'),
    ('pingme', 'PingMe', 'https://api.pingme.tel', FALSE, '{"supports_temporary":true,"supports_rental":true,"supports_polling":true,"supports_cancel":true,"supports_refund":false}')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sms_channels (code, public_name, role, provider_id, sort_order)
SELECT v.code, v.public_name, v.role, p.id, v.sort_order
FROM (VALUES
    ('channel_1','通道1','primary','5sim',1),
    ('channel_2','通道2','primary','smspool',2),
    ('channel_3','通道3','primary','sms_activate',3),
    ('channel_4','通道4','backup','onlinesim',4),
    ('channel_5','通道5','backup','pingme',5)
) AS v(code, public_name, role, provider_code, sort_order)
JOIN sms_providers p ON p.code = v.provider_code
ON CONFLICT (code) DO NOTHING;

INSERT INTO sms_success_rate_rules (grade, minimum_rate, multiplier)
VALUES ('S', .95, 1.25), ('A', .90, 1.15), ('B', .80, 1.05), ('C', .60, 1), ('D', 0, .95)
ON CONFLICT (grade) DO NOTHING;

INSERT INTO sms_services (code, name, category, sort_order)
VALUES ('google','Google','social',1), ('openai','OpenAI','ai',2), ('telegram','Telegram','social',3),
       ('discord','Discord','social',4), ('whatsapp','WhatsApp','social',5)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sms_countries (iso2, iso3, calling_code, name_zh, name_en, sort_order)
VALUES ('US','USA','+1','美国','United States',1), ('CN','CHN','+86','中国','China',2),
       ('GB','GBR','+44','英国','United Kingdom',3), ('DE','DEU','+49','德国','Germany',4)
ON CONFLICT (iso2) DO NOTHING;

INSERT INTO settings (key, value) VALUES ('sms_service_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
