-- Align the persisted SMSPVA capability contract with the runtime adapter.
-- This migration is additive and deliberately does not enable the provider or
-- claim capabilities that are not confirmed by the official API contract.
INSERT INTO sms_providers (code, name, base_url, enabled, health_status, capabilities, metadata)
VALUES (
    'smspva',
    'SMSPVA',
    'https://api.smspva.com',
    FALSE,
    'unknown',
    '{"supports_temporary":true,"supports_rental":true,"supports_rental_cancel":true,"supports_webhook":false,"supports_polling":true,"supports_cancel":true,"supports_refund":false,"supports_refund_status":false,"supports_finish":true,"supports_ban":false,"supports_extend":true,"supports_resend":true,"supports_voice":true,"supports_voice_sms":true,"supports_voice_caller_id":true,"supports_voice_call":true,"supports_operator_selection":true,"supports_service_selection":true,"supports_conversion_stats":true}'::jsonb,
    '{"beta":false,"refund_confirmation":"unconfirmed"}'::jsonb
)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    base_url = EXCLUDED.base_url,
    capabilities = EXCLUDED.capabilities,
    metadata = COALESCE(sms_providers.metadata, '{}'::jsonb) || EXCLUDED.metadata,
    updated_at = NOW();

-- Keep the public second channel disabled until credentials, health and manual
-- acceptance are explicitly completed by an administrator.
UPDATE sms_channels
SET provider_id = (SELECT id FROM sms_providers WHERE code = 'smspva'),
    public_name = '渠道2',
    updated_at = NOW()
WHERE code = 'channel_2';

-- Additive message provenance fields. Existing rows retain their original
-- message text/code/timestamps and are intentionally not backfilled.
ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS sender TEXT NOT NULL DEFAULT '';
ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS provider_received_at TIMESTAMPTZ;
ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS message_type TEXT NOT NULL DEFAULT '';
ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS service_code TEXT NOT NULL DEFAULT '';
ALTER TABLE sms_messages ADD COLUMN IF NOT EXISTS other_sms BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_sms_messages_order_text
    ON sms_messages(order_id, message_text);
