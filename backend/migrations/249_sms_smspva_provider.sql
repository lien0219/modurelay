-- Replace the second public SMS channel with SMSPVA while keeping legacy
-- providers in the catalog as disabled BETA integrations. Do not overwrite
-- stored credentials or administrator enablement choices for existing rows.

INSERT INTO sms_providers (code,name,base_url,enabled,health_status,capabilities,metadata)
VALUES (
  'smspva',
  'SMSPVA',
  'https://api.smspva.com',
  FALSE,
  'unknown',
  '{"supports_temporary":true,"supports_rental":false,"supports_polling":true,"supports_cancel":true,"supports_refund":true,"supports_resend":true,"supports_voice":true,"supports_operator_selection":true,"supports_service_selection":true}'::jsonb,
  '{"beta":false}'::jsonb
)
ON CONFLICT (code) DO UPDATE SET
  name=EXCLUDED.name,
  base_url=EXCLUDED.base_url,
  capabilities=EXCLUDED.capabilities,
  metadata=COALESCE(sms_providers.metadata,'{}'::jsonb) || '{"beta":false}'::jsonb,
  updated_at=NOW();

UPDATE sms_providers
SET metadata=COALESCE(metadata,'{}'::jsonb) || '{"beta":false}'::jsonb,
    updated_at=NOW()
WHERE code='5sim';

UPDATE sms_providers
SET metadata=COALESCE(metadata,'{}'::jsonb) || '{"beta":true}'::jsonb,
    enabled=FALSE,
    updated_at=NOW()
WHERE code NOT IN ('5sim','smspva');

UPDATE sms_channels c
SET provider_id=p.id,
    public_name='渠道2',
    role='primary',
    updated_at=NOW()
FROM sms_providers p
WHERE c.code='channel_2' AND p.code='smspva';

UPDATE sms_channels
SET public_name='渠道1', sort_order=1, updated_at=NOW()
WHERE code='channel_1';

UPDATE sms_channels
SET enabled=FALSE, visible=FALSE, healthy=FALSE, updated_at=NOW()
WHERE code NOT IN ('channel_1','channel_2');
