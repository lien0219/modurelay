-- Preserve execution-principal attribution in security, moderation, and Ops evidence.
-- Existing human rows remain NULL and retain their user_id snapshots.
ALTER TABLE content_moderation_logs ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE prompt_audit_jobs ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE prompt_audit_events ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE ops_error_logs ADD COLUMN IF NOT EXISTS service_account_id BIGINT;
ALTER TABLE ops_system_logs ADD COLUMN IF NOT EXISTS service_account_id BIGINT;

-- No historical scan or backfill. Restrict deletion of referenced identities
-- so retained evidence does not lose its machine principal.
ALTER TABLE content_moderation_logs ADD CONSTRAINT content_moderation_logs_service_account_fk
  FOREIGN KEY(service_account_id) REFERENCES service_accounts(id) NOT VALID;
ALTER TABLE prompt_audit_jobs ADD CONSTRAINT prompt_audit_jobs_service_account_fk
  FOREIGN KEY(service_account_id) REFERENCES service_accounts(id) NOT VALID;
ALTER TABLE prompt_audit_events ADD CONSTRAINT prompt_audit_events_service_account_fk
  FOREIGN KEY(service_account_id) REFERENCES service_accounts(id) NOT VALID;
ALTER TABLE ops_error_logs ADD CONSTRAINT ops_error_logs_service_account_fk
  FOREIGN KEY(service_account_id) REFERENCES service_accounts(id) NOT VALID;
ALTER TABLE ops_system_logs ADD CONSTRAINT ops_system_logs_service_account_fk
  FOREIGN KEY(service_account_id) REFERENCES service_accounts(id) NOT VALID;

ALTER TABLE content_moderation_logs ADD CONSTRAINT content_moderation_logs_machine_actor
  CHECK (service_account_id IS NULL OR (user_id IS NULL AND user_email='')) NOT VALID;
ALTER TABLE prompt_audit_jobs ADD CONSTRAINT prompt_audit_jobs_machine_actor
  CHECK (service_account_id IS NULL OR (user_id IS NULL AND username_snapshot='' AND user_email_snapshot='')) NOT VALID;
ALTER TABLE prompt_audit_events ADD CONSTRAINT prompt_audit_events_machine_actor
  CHECK (service_account_id IS NULL OR (user_id IS NULL AND username_snapshot='' AND user_email_snapshot='')) NOT VALID;
ALTER TABLE ops_error_logs ADD CONSTRAINT ops_error_logs_machine_actor
  CHECK (service_account_id IS NULL OR user_id IS NULL) NOT VALID;
ALTER TABLE ops_system_logs ADD CONSTRAINT ops_system_logs_machine_actor
  CHECK (service_account_id IS NULL OR user_id IS NULL) NOT VALID;
