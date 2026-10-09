package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// A completed cursor alone is insufficient: a manual repair or a conflicting
// writer could have recreated credentials or grants behind it. Every terminal
// business invariant is checked under the Workspace lock before closure.
func lifecycleVerifyPurge(ctx context.Context, tx *sql.Tx, w int64) error {
	queries := []string{
		`SELECT EXISTS(SELECT 1 FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1 AND (k.status<>'inactive' OR k.deleted_at IS NULL OR k.key NOT LIKE 'lifecycle-revoked-%'))`,
		`SELECT EXISTS(SELECT 1 FROM service_accounts WHERE workspace_id=$1 AND status<>'disabled')`,
		`SELECT EXISTS(SELECT 1 FROM workspace_invitations WHERE workspace_id=$1 AND (revoked_at IS NULL OR email NOT LIKE 'deleted-%@invalid.local'))`,
		`SELECT EXISTS(SELECT 1 FROM project_access_grants WHERE workspace_id=$1 AND effective_lifecycle_retention_days($1,'security')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'security')*interval '1 day')`,
		`SELECT EXISTS(SELECT 1 FROM project_cost_allocations WHERE workspace_id=$1 AND effective_lifecycle_retention_days($1,'operational')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'operational')*interval '1 day')`,
		`SELECT EXISTS(SELECT 1 FROM api_key_allocation_overrides WHERE workspace_id=$1 AND effective_lifecycle_retention_days($1,'operational')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'operational')*interval '1 day')`,
		`SELECT EXISTS(SELECT 1 FROM service_account_allocation_overrides WHERE workspace_id=$1 AND effective_lifecycle_retention_days($1,'operational')>0 AND updated_at<now()-effective_lifecycle_retention_days($1,'operational')*interval '1 day')`,
		`SELECT EXISTS(SELECT 1 FROM workspace_identity_providers WHERE workspace_id=$1 AND (status<>'disabled' OR is_default OR (encrypted_client_secret IS NOT NULL AND encrypted_client_secret<>'destroyed') OR (encrypted_saml_sp_keys IS NOT NULL AND encrypted_saml_sp_keys<>'destroyed')))`,
		`SELECT EXISTS(SELECT 1 FROM workspace_scim_tokens WHERE workspace_id=$1 AND (status<>'revoked' OR revoked_at IS NULL))`,
		`SELECT EXISTS(SELECT 1 FROM workspace_scim_connectors WHERE workspace_id=$1 AND status<>'disabled')`,
		`SELECT EXISTS(SELECT 1 FROM workspace_webhooks WHERE workspace_id=$1 AND (enabled OR secret_current_encrypted<>'destroyed' OR secret_previous_encrypted IS NOT NULL))`,
		`SELECT EXISTS(SELECT 1 FROM workspace_members WHERE workspace_id=$1 AND role<>'owner' AND user_id<>(SELECT billing_owner_user_id FROM workspaces WHERE id=$1) AND (status<>'suspended' OR NOT administratively_suspended OR NOT administratively_removed OR effective_membership_source_id IS NOT NULL))`,
		`SELECT EXISTS(SELECT 1 FROM workspace_teams WHERE workspace_id=$1 AND status<>'archived')`,
		`SELECT EXISTS(SELECT 1 FROM projects WHERE workspace_id=$1 AND status<>'archived')`,
	}
	for _, query := range queries {
		var unsafe bool
		if err := tx.QueryRowContext(ctx, query, w).Scan(&unsafe); err != nil {
			return err
		}
		if unsafe {
			return service.ErrWorkspaceConflict
		}
	}
	return nil
}
