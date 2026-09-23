CREATE TABLE IF NOT EXISTS sms_provider_catalog_syncs (
 provider_id BIGINT PRIMARY KEY REFERENCES sms_providers(id) ON DELETE CASCADE,
 status TEXT NOT NULL DEFAULT 'never', started_at TIMESTAMPTZ, completed_at TIMESTAMPTZ,
 last_success_at TIMESTAMPTZ, next_sync_at TIMESTAMPTZ, duration_ms BIGINT NOT NULL DEFAULT 0,
 service_count INTEGER NOT NULL DEFAULT 0, country_count INTEGER NOT NULL DEFAULT 0,
 failure_reason TEXT NOT NULL DEFAULT '', stale_after INTERVAL NOT NULL DEFAULT INTERVAL '12 hours',
 source TEXT NOT NULL DEFAULT 'provider'
);
CREATE TABLE IF NOT EXISTS sms_provider_catalog_services (
 provider_id BIGINT NOT NULL REFERENCES sms_providers(id) ON DELETE CASCADE,
 provider_service_code TEXT NOT NULL, provider_service_name TEXT NOT NULL DEFAULT '', category TEXT NOT NULL DEFAULT '',
 enabled BOOLEAN NOT NULL DEFAULT TRUE, observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), raw_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
 PRIMARY KEY(provider_id,provider_service_code)
);
CREATE TABLE IF NOT EXISTS sms_provider_catalog_countries (
 provider_id BIGINT NOT NULL REFERENCES sms_providers(id) ON DELETE CASCADE,
 provider_country_id TEXT NOT NULL, provider_country_code TEXT NOT NULL DEFAULT '', iso2 TEXT NOT NULL DEFAULT '',
 name_zh TEXT NOT NULL DEFAULT '', name_en TEXT NOT NULL DEFAULT '', enabled BOOLEAN NOT NULL DEFAULT TRUE,
 observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), raw_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
 PRIMARY KEY(provider_id,provider_country_id)
);
CREATE INDEX IF NOT EXISTS idx_sms_catalog_services_provider ON sms_provider_catalog_services(provider_id,enabled,provider_service_code);
CREATE INDEX IF NOT EXISTS idx_sms_catalog_countries_provider ON sms_provider_catalog_countries(provider_id,enabled,name_en);
