package repository

// Legacy cleanup must exclude every immutable tenant attribution, even for
// placeholders or records created before allocation snapshots were introduced.
// A snapshot protects an otherwise unassigned row too.
const legacyUsageRetentionPredicate = `workspace_id IS NULL AND project_id IS NULL
	AND billing_principal_user_id IS NULL AND budget_reservation_id IS NULL
	AND service_account_id IS NULL AND NOT EXISTS (
		SELECT 1 FROM usage_allocation_snapshots s WHERE s.usage_log_id=usage_logs.id
	)`
