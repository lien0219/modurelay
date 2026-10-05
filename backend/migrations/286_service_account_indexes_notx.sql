CREATE INDEX CONCURRENTLY IF NOT EXISTS api_keys_service_account_id
 ON api_keys(service_account_id,id) WHERE service_account_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS usage_logs_service_account_created
 ON usage_logs(service_account_id,created_at) WHERE service_account_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS budget_reservations_service_account_pending
 ON budget_reservations(service_account_id) WHERE service_account_id IS NOT NULL AND status='pending';
CREATE INDEX CONCURRENTLY IF NOT EXISTS batch_image_jobs_service_account_created
 ON batch_image_jobs(service_account_id,created_at) WHERE service_account_id IS NOT NULL;
