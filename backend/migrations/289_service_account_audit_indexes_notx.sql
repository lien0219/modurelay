CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_content_moderation_logs_service_account_created
  ON content_moderation_logs(service_account_id, created_at DESC)
  WHERE service_account_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prompt_audit_jobs_service_account_created
  ON prompt_audit_jobs(service_account_id, created_at DESC)
  WHERE service_account_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_prompt_audit_events_service_account_created
  ON prompt_audit_events(service_account_id, created_at DESC)
  WHERE service_account_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_service_account_created
  ON ops_error_logs(service_account_id, created_at DESC)
  WHERE service_account_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_system_logs_service_account_created
  ON ops_system_logs(service_account_id, created_at DESC)
  WHERE service_account_id IS NOT NULL;
