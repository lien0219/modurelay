-- Recovery evidence for add-service operations whose provider response is
-- ambiguous. The baseline contains active provider order ids observed before
-- mutation so reconciliation can only bind a genuinely new matching order.
ALTER TABLE sms_rental_service_charges
    ADD COLUMN IF NOT EXISTS recovery_baseline JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS reconciliation_status TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_sms_rental_service_charges_reconcile
    ON sms_rental_service_charges(status, reconciliation_status, updated_at);
