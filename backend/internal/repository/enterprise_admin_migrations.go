package repository

import "context"

// Only migration303's fixed index allowlist is eligible for invalid-build
// recovery. Valid indexes are untouched; the existing runner owns locking.
func prepareEnterpriseAdminDiagnosticsIndexes(ctx context.Context, db migrationConnection) error {
	for _, name := range []string{
		"enterprise_admin_workspace_name_prefix",
		"enterprise_admin_workspace_slug_prefix",
		"enterprise_admin_workspace_name_order",
		"enterprise_admin_workspace_status",
		"enterprise_admin_workspace_type",
		"enterprise_admin_workspace_owner",
		"enterprise_admin_workspace_created",
		"enterprise_admin_workspace_updated",
		"enterprise_admin_export_state_created",
		"enterprise_admin_export_attention",
		"enterprise_admin_export_success",
		"enterprise_admin_purge_state_created",
		"enterprise_admin_purge_attention",
		"enterprise_admin_purge_success",
		"enterprise_admin_webhook_scope_created",
		"enterprise_admin_scim_expiry",
		"enterprise_admin_billing_recovered",
		"enterprise_admin_objects_scope",
		"enterprise_admin_active_holds",
		"enterprise_admin_image_attention_global",
		"enterprise_admin_image_attention_scope",
		"enterprise_admin_image_errors_global",
		"enterprise_admin_outbox_claims",
		"enterprise_admin_outbox_retry",
		"enterprise_admin_outbox_pending_created",
		"enterprise_admin_webhook_claims",
		"enterprise_admin_webhook_pending_created",
		"enterprise_admin_webhook_dead_scope",
		"enterprise_admin_finops_pending_scope",
		"enterprise_admin_finops_errors_global",
		"enterprise_admin_export_claims",
		"enterprise_admin_export_pending_created",
		"enterprise_admin_export_errors_global",
		"enterprise_admin_purge_claims",
		"enterprise_admin_purge_pending_created",
		"enterprise_admin_purge_errors_global",
		"enterprise_admin_scim_errors_global",
		"enterprise_admin_members_scope_state",
		"enterprise_admin_members_state",
		"enterprise_admin_projects_scope_state",
		"enterprise_admin_projects_state",
		"enterprise_admin_service_accounts_scope_state",
		"enterprise_admin_service_accounts_state",
		"enterprise_admin_identity_state",
		"enterprise_admin_scim_state",
		"enterprise_admin_scim_scope_state",
		"enterprise_admin_budget_pending_scope",
		"enterprise_admin_webhook_scope_due",
		"enterprise_admin_webhook_success_scope",
		"enterprise_admin_purge_eligibility",
		"enterprise_admin_purge_cooling",
		"enterprise_admin_billing_state",
		"enterprise_admin_image_success_global",
		"enterprise_admin_budget_reserved",
		"enterprise_admin_findings_scope_state",
	} {
		if e := dropInvalidIndexIfPresent(ctx, db, name); e != nil {
			return e
		}
	}
	return nil
}
