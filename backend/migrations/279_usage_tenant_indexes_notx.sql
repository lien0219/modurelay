-- Do not block gateway writes while scanning existing hot tables.
CREATE INDEX CONCURRENTLY IF NOT EXISTS api_keys_project_active_id
 ON api_keys(project_id,id DESC) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS usage_logs_workspace_created
 ON usage_logs(workspace_id,created_at) WHERE workspace_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS usage_logs_project_created
 ON usage_logs(project_id,created_at) WHERE project_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS usage_logs_principal_created
 ON usage_logs(billing_principal_user_id,created_at) WHERE billing_principal_user_id IS NOT NULL;
