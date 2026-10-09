package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Counts deliberately stop at 10001. Eligibility uses existence, never a
// full-table count or a membership-derived Global User resource sweep.
func lifecyclePreflightTx(ctx context.Context, tx *sql.Tx, w *service.Workspace) (*service.LifecyclePreflight, error) {
	out := &service.LifecyclePreflight{
		Eligible: true, BlockingReasons: []service.LifecycleBlocker{}, ResourceCounts: map[string]int64{}, ProtectedRecords: map[string]int64{},
		EstimatedPurgeScope: []string{"credential_values", "invitations", "project_access_grants", "allocation_configuration", "non_owner_membership_access", "business_resource_descriptions"},
		RetainedScope:       []string{"global_users_and_wallets", "workspace_project_credential_identifiers", "usage_and_reservations", "settlement_refund_dedup", "immutable_allocation_anomaly_evidence", "audit_and_policy_protected_metadata", "policy_ineligible_business_configuration", "global_user_owned_canvas_media"}, ProtectedEvidenceRetained: true,
	}
	if err := tx.QueryRowContext(ctx, `SELECT now()+deletion_grace_days*interval '1 day' FROM lifecycle_platform_settings WHERE id=1`).Scan(&out.EarliestPurgeAt); err != nil {
		return nil, err
	}
	blocker := func(code, reason string, count int64) {
		out.BlockingReasons = append(out.BlockingReasons, service.LifecycleBlocker{Code: code, Reason: reason, Count: count})
		out.Eligible = false
	}
	if w.Type != "organization" {
		blocker("PERSONAL_WORKSPACE_PROTECTED", "Personal workspace deletion requires the separate global account lifecycle.", 1)
	}
	if w.Status != "active" && w.Status != "archived" && w.Status != "pending_deletion" && w.Status != "purging" {
		blocker("WORKSPACE_STATE_CONFLICT", "The workspace state does not permit business cleanup.", 1)
	}
	var billingReady bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL)`, w.ID, w.BillingOwnerUserID).Scan(&billingReady); err != nil {
		return nil, err
	}
	if !billingReady {
		blocker("BILLING_OWNER_UNAVAILABLE", "The current billing owner is not an active global user and member.", 1)
	}
	checks := []struct {
		name, query, code, reason string
		protected                 bool
	}{
		{"pending_budget_reservations", `SELECT id FROM budget_reservations WHERE workspace_id=$1 AND status='pending'`, "PENDING_BUDGET_RESERVATIONS", "Budget holds must settle or release before irreversible cleanup.", false},
		{"reserved_budget_counters", `SELECT scope_id FROM budget_counters WHERE reserved<>0 AND (workspace_scope_id=$1 OR project_scope_id IN (SELECT id FROM projects WHERE workspace_id=$1))`, "UNSETTLED_BUDGET_COUNTERS", "Workspace or project reserved budget counters remain nonzero.", false},
		{"pending_quota_reservations", `SELECT id FROM policy_quota_reservations WHERE workspace_id=$1 AND status='pending'`, "PENDING_QUOTA_RESERVATIONS", "Accepted requests still hold durable quota reservations.", false},
		{"pending_billing_settlements", `SELECT b.reservation_id FROM billing_settlement_alerts b JOIN budget_reservations r ON r.id=b.reservation_id WHERE r.workspace_id=$1 AND b.state='pending'`, "UNSETTLED_BILLING", "Billing recovery has not completed.", false},
		{"pending_async_images", `SELECT id FROM batch_image_jobs WHERE workspace_id=$1 AND (status NOT IN ('completed','failed','cancelled','output_deleted') OR last_error_code='SUBMIT_OUTCOME_UNKNOWN')`, "PENDING_ASYNC_MEDIA", "Accepted image/video tasks or unknown submission outcomes require settlement or recovery.", false},
		{"pending_exports", `SELECT id FROM workspace_export_jobs WHERE workspace_id=$1 AND state IN ('pending','running')`, "PENDING_EXPORTS", "Cancel or complete the pending export before business cleanup.", false},
		{"pending_webhook_deliveries", `SELECT id FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND status IN ('pending','retrying','delivering')`, "PENDING_WEBHOOK_DELIVERIES", "Pending webhook deliveries must reach a terminal outcome.", false},
		{"pending_outbox", `SELECT o.event_id FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND o.delivered_at IS NULL AND e.event_type NOT LIKE 'workspace.deletion.%'`, "PENDING_DOMAIN_EVENTS", "Pre-existing domain event delivery is not complete.", false},
		{"usage_logs", `SELECT id FROM usage_logs WHERE workspace_id=$1`, "", "", true},
		{"budget_reservations", `SELECT id FROM budget_reservations WHERE workspace_id=$1`, "", "", true},
		{"allocation_snapshots", `SELECT usage_log_id FROM usage_allocation_snapshots WHERE workspace_id=$1`, "", "", true},
		{"reservation_allocation_snapshots", `SELECT reservation_id FROM budget_reservation_allocation_snapshots WHERE workspace_id=$1`, "", "", true},
		{"anomaly_evidence", `SELECT id FROM finops_anomaly_snapshots WHERE workspace_id=$1`, "", "", true},
		{"audit_logs", `SELECT id FROM workspace_audit_logs WHERE workspace_id=$1`, "", "", true},
		{"projects", `SELECT id FROM projects WHERE workspace_id=$1`, "", "", false},
		{"credentials", `SELECT k.id FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1`, "", "", false},
		{"service_accounts", `SELECT id FROM service_accounts WHERE workspace_id=$1`, "", "", false},
		{"members", `SELECT id FROM workspace_members WHERE workspace_id=$1`, "", "", false},
	}
	for _, check := range checks {
		var count int64
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM (`+check.query+` LIMIT 10001) bounded`, w.ID).Scan(&count); err != nil {
			return nil, err
		}
		if count > 10000 {
			out.CountsCapped = true
		}
		if check.protected {
			out.ProtectedRecords[check.name] = count
		} else {
			out.ResourceCounts[check.name] = count
		}
		if check.code != "" && count > 0 {
			blocker(check.code, check.reason, count)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT code,reason FROM workspace_lifecycle_holds WHERE workspace_id=$1 AND active ORDER BY code LIMIT 32`, w.ID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var code, reason string
		if err = rows.Scan(&code, &reason); err != nil {
			return nil, err
		}
		blocker(code, reason, 1)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	var scheduled *time.Time
	if err = tx.QueryRowContext(ctx, `SELECT min(earliest_purge_at) FROM workspace_deletion_jobs WHERE workspace_id=$1 AND state IN ('pending','running','blocked','failed')`, w.ID).Scan(&scheduled); err != nil {
		return nil, err
	}
	if scheduled != nil {
		out.EarliestPurgeAt = *scheduled
	}
	return out, nil
}

func (r *workspaceRepository) LifecyclePreflight(ctx context.Context, a, w int64) (*service.LifecyclePreflight, error) {
	tx, ac, err := r.lifecycleTx(ctx, a, w, "lifecycle.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Detailed retention/count evidence is Owner-only even when admins can view
	// the general policy; global administration never acquires this capability.
	if ac.Member.Role != "owner" {
		return nil, service.ErrWorkspaceForbidden
	}
	out, err := lifecyclePreflightTx(ctx, tx, ac.Workspace)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
