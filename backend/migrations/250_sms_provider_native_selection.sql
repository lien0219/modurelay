-- Provider-native SMS selection snapshots. 5SIM/SMSPVA use live provider catalogs;
-- legacy mapping tables remain only for backwards-compatible BETA adapters.
ALTER TABLE sms_quotes ADD COLUMN IF NOT EXISTS operator_code TEXT NOT NULL DEFAULT 'any';
ALTER TABLE sms_quotes ADD COLUMN IF NOT EXISTS voice_mode INTEGER NOT NULL DEFAULT 0 CHECK (voice_mode BETWEEN 0 AND 2);
CREATE INDEX IF NOT EXISTS idx_sms_quotes_provider_selection
  ON sms_quotes(provider_id, provider_service_code, provider_country_code, operator_code, voice_mode);
