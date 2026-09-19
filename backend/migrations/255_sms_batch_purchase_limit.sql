-- Add the administrator-controlled SMS batch purchase limit to existing
-- pricing settings without overwriting any values already configured.
UPDATE settings
SET value = jsonb_set(value::jsonb, '{batch_purchase_limit}', '5'::jsonb, true)::text
WHERE key = 'sms_pricing_settings'
  AND NOT (value::jsonb ? 'batch_purchase_limit');
