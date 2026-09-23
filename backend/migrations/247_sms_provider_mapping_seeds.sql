-- Seed only provider identifiers that are stable and verified. Provider
-- identifiers are deliberately kept in the mapping tables instead of being
-- inferred from the platform codes: 5SIM uses country slugs, while SMSPool
-- uses numeric service and country identifiers. Providers without a verified
-- catalog remain administrator-configurable but receive no guessed mappings.

INSERT INTO sms_provider_service_mappings (
    provider_id,
    service_id,
    provider_service_code,
    provider_service_name,
    temporary_supported,
    rental_supported,
    enabled
)
SELECT p.id,
       s.id,
       v.provider_service_code,
       v.provider_service_name,
       TRUE,
       FALSE,
       TRUE
FROM (VALUES
    ('5sim', 'google', 'google', 'Google'),
    ('5sim', 'openai', 'openai', 'OpenAI'),
    ('5sim', 'telegram', 'telegram', 'Telegram'),
    ('5sim', 'discord', 'discord', 'Discord'),
    ('5sim', 'whatsapp', 'whatsapp', 'WhatsApp'),
    ('smspool', 'google', '395', 'Google/Gmail'),
    ('smspool', 'openai', '671', 'OpenAI/ChatGPT'),
    ('smspool', 'telegram', '907', 'Telegram'),
    ('smspool', 'discord', '273', 'Discord'),
    ('smspool', 'whatsapp', '1012', 'WhatsApp')
) AS v(provider_code, service_code, provider_service_code, provider_service_name)
JOIN sms_providers p ON p.code = v.provider_code
JOIN sms_services s ON s.code = v.service_code
ON CONFLICT (provider_id, service_id) DO NOTHING;

INSERT INTO sms_provider_country_mappings (
    provider_id,
    country_id,
    provider_country_id,
    provider_country_code
)
SELECT p.id,
       c.id,
       v.provider_country_id,
       c.iso2
FROM (VALUES
    ('5sim', 'US', 'usa'),
    ('5sim', 'GB', 'england'),
    ('5sim', 'DE', 'germany'),
    ('smspool', 'US', '1'),
    ('smspool', 'GB', '2'),
    ('smspool', 'DE', '24')
) AS v(provider_code, iso2, provider_country_id)
JOIN sms_providers p ON p.code = v.provider_code
JOIN sms_countries c ON c.iso2 = v.iso2
ON CONFLICT (provider_id, country_id) DO NOTHING;
