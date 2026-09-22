-- Email verification provider expansion:
-- channel 1 = free public temp.tf inboxes
-- channel 2 = paid private Sonjj/SmailPro Gmail/Outlook inboxes
-- Emailnator is retained as an invisible beta fallback.
--
-- Forward/additive migration. Existing email orders retain provider_id/channel_id
-- snapshots and are not rewritten.

INSERT INTO email_providers (
    code, name, base_url, credential_ref, enabled, health_status,
    capabilities, billing, metadata
)
VALUES (
    'temp_tf',
    'temp.tf',
    'https://temp.tf/api',
    '',
    TRUE,
    'healthy',
    '{
      "supports_temporary_inbox":true,
      "supports_generate_single":true,
      "supports_generate_bulk":false,
      "supports_inbox_list":true,
      "supports_message_read":true,
      "supports_message_delete":false,
      "supports_polling":true,
      "supports_verification_code":true,
      "supports_verification_url":true,
      "supports_html_message":true,
      "supports_webhook":false,
      "supports_sse":false,
      "supports_imap":false,
      "supports_smtp":false,
      "supports_custom_domain":false,
      "supports_private_inbox":false,
      "supports_persistent_inbox":false,
      "supports_provider_refund":false
    }'::jsonb,
    '{
      "cost_mode":"fixed_per_order",
      "fixed_cost_per_order":0,
      "rate_limit_per_minute":60,
      "provider_concurrency":2
    }'::jsonb,
    '{
      "portal_url":"https://temp.tf/api",
      "supported_address_types":["gmail","outlook","hotmail"],
      "best_effort":true
    }'::jsonb
)
ON CONFLICT (code) DO UPDATE SET
    base_url=EXCLUDED.base_url,
    capabilities=EXCLUDED.capabilities,
    billing=EXCLUDED.billing,
    metadata=COALESCE(email_providers.metadata,'{}'::jsonb) || EXCLUDED.metadata,
    updated_at=NOW();

INSERT INTO email_providers (
    code, name, base_url, credential_ref, enabled, health_status,
    capabilities, billing, metadata
)
VALUES (
    'sonjj',
    'SmailPro / Sonjj',
    'https://app.sonjj.com',
    'env:EMAIL_SONJJ_API_KEY',
    FALSE,
    'unknown',
    '{
      "supports_temporary_inbox":true,
      "supports_generate_single":true,
      "supports_generate_bulk":false,
      "supports_inbox_list":true,
      "supports_message_read":true,
      "supports_message_delete":false,
      "supports_polling":true,
      "supports_verification_code":true,
      "supports_verification_url":true,
      "supports_html_message":true,
      "supports_webhook":false,
      "supports_sse":false,
      "supports_imap":false,
      "supports_smtp":false,
      "supports_custom_domain":false,
      "supports_private_inbox":true,
      "supports_persistent_inbox":false,
      "supports_provider_refund":false
    }'::jsonb,
    '{
      "cost_mode":"fixed_per_order",
      "fixed_cost_per_order":0.003,
      "estimated_requests_per_order":12,
      "provider_concurrency":4
    }'::jsonb,
    '{
      "portal_url":"https://my.sonjj.com",
      "supported_address_types":["gmail_real","gmail_alias","outlook_real","outlook_alias"]
    }'::jsonb
)
ON CONFLICT (code) DO UPDATE SET
    base_url=EXCLUDED.base_url,
    capabilities=EXCLUDED.capabilities,
    metadata=COALESCE(email_providers.metadata,'{}'::jsonb) || EXCLUDED.metadata,
    updated_at=NOW();

-- Preserve any already-configured Sonjj credential/billing values.
UPDATE email_providers
SET credential_ref = CASE
        WHEN credential_ref='' THEN 'env:EMAIL_SONJJ_API_KEY'
        ELSE credential_ref
    END
WHERE code='sonjj';

-- Channel 1 becomes the free public provider. Existing historical orders still
-- reference the same channel row but keep their original provider_id snapshot.
UPDATE email_channels
SET provider_id=(SELECT id FROM email_providers WHERE code='temp_tf'),
    public_name='渠道1',
    role='primary',
    email_type='public_temporary',
    privacy_level='public_temporary',
    enabled=TRUE,
    visible=TRUE,
    healthy=TRUE,
    sale_price=0,
    capture_policy='on_target_email_received',
    refund_policy='refund_if_no_message',
    order_ttl_seconds=900,
    polling_backoff='[3,5,8,12,20,30,45,60]'::jsonb,
    max_provider_requests_per_order=45,
    metadata=COALESCE(metadata,'{}'::jsonb)
        || '{"force_free":true,"address_group":"public"}'::jsonb,
    updated_at=NOW()
WHERE code='email_channel_1';

INSERT INTO email_channels (
    code, public_name, role, provider_id, email_type, privacy_level,
    enabled, visible, healthy, sale_price, order_ttl_seconds,
    capture_policy, refund_policy, polling_backoff,
    max_provider_requests_per_order, sort_order, metadata
)
SELECT
    'email_channel_2', '渠道2', 'primary', id, 'private_provider',
    'private_api', FALSE, TRUE, FALSE, 0.05, 900,
    'on_target_email_received', 'refund_if_no_message',
    '[3,5,8,12,20,30,45,60]'::jsonb, 60, 2,
    '{"address_group":"private","base_markup":1.0,"minimum_profit":0.01}'::jsonb
FROM email_providers
WHERE code='sonjj'
ON CONFLICT (code) DO UPDATE SET
    provider_id=EXCLUDED.provider_id,
    public_name='渠道2',
    email_type=EXCLUDED.email_type,
    privacy_level=EXCLUDED.privacy_level,
    visible=TRUE,
    sort_order=2,
    metadata=COALESCE(email_channels.metadata,'{}'::jsonb) || EXCLUDED.metadata,
    updated_at=NOW();

INSERT INTO email_channels (
    code, public_name, role, provider_id, email_type, privacy_level,
    enabled, visible, healthy, sale_price, order_ttl_seconds,
    capture_policy, refund_policy, sort_order, metadata
)
SELECT
    'email_channel_3', '渠道3', 'backup', id, 'temporary_gmail',
    'public_temporary', FALSE, FALSE, FALSE, 0.30, 900,
    'on_target_email_received', 'refund_if_no_message', 99,
    '{"beta":true,"address_group":"fallback"}'::jsonb
FROM email_providers
WHERE code='emailnator'
ON CONFLICT (code) DO UPDATE SET
    provider_id=EXCLUDED.provider_id,
    role='backup',
    visible=FALSE,
    metadata=COALESCE(email_channels.metadata,'{}'::jsonb) || '{"beta":true}'::jsonb,
    updated_at=NOW();

INSERT INTO settings (key,value) VALUES
    ('email_free_daily_limit','20'),
    ('email_free_active_limit','3'),
    ('email_free_generation_interval_seconds','5')
ON CONFLICT (key) DO NOTHING;
