-- Publish only the advanced capabilities whose financial and recovery contract
-- is implemented. Multi-service and add-service remain fail-closed until their
-- provider charging semantics pass real-account validation.
UPDATE sms_providers
SET capabilities = COALESCE(capabilities, '{}'::jsonb) ||
    '{
        "supports_rental_constraints": true,
        "supports_rental_restore": false,
        "supports_rental_multi_service": false,
        "supports_rental_add_service": false,
        "supports_refund": false,
        "supports_refund_status": false,
        "supports_webhook": false
    }'::jsonb,
    updated_at = NOW()
WHERE code = 'smspva';
