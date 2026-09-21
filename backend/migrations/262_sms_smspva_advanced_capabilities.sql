-- Keep persisted SMSPVA capabilities aligned with runtime behavior.
-- Features that depend on unverified provider billing semantics remain disabled
-- even though adapter support exists behind the capability gate.
UPDATE sms_providers
SET capabilities = COALESCE(capabilities, '{}'::jsonb)
    || jsonb_build_object(
        'supports_rental_constraints', true,
        'supports_rental_multi_service', false,
        'supports_rental_add_service', false,
        'supports_rental_restore', true
    ),
    updated_at = NOW()
WHERE code = 'smspva';
