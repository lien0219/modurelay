package repository

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"strings"
	"time"
)

var _ service.EnterpriseAdminRepository = (*workspaceRepository)(nil)

const adminRunbook = "docs/ENTERPRISE_ADMIN_DIAGNOSTICS_RUNBOOK.md"

// Each entry point validates the live global actor inside its bounded snapshot.
// No tenant access helper or bootstrap is involved.
func (r *workspaceRepository) adminReadTx(ctx context.Context, a int64) (*sql.Tx, time.Time, error) {
	if a <= 0 {
		return nil, time.Time{}, service.ErrWorkspaceForbidden
	}
	tx, e := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if e != nil {
		return nil, time.Time{}, e
	}
	fail := func(e error) (*sql.Tx, time.Time, error) { _ = tx.Rollback(); return nil, time.Time{}, e }
	if _, e = tx.ExecContext(ctx, `SET LOCAL statement_timeout='2000ms'`); e != nil {
		return fail(e)
	}
	if _, e = tx.ExecContext(ctx, `SET LOCAL lock_timeout='250ms'`); e != nil {
		return fail(e)
	}
	if e = requireWorkspaceGlobalAdmin(ctx, tx, a); e != nil {
		return fail(e)
	}
	var now time.Time
	if e = tx.QueryRowContext(ctx, `SELECT now()`).Scan(&now); e != nil {
		return fail(e)
	}
	return tx, now, nil
}

// A source-local SQL failure must not abort the remaining diagnostic sections.
// Savepoints retain one repeatable-read snapshot and sanitize failure output.
func adminSection(ctx context.Context, tx *sql.Tx, read func() error) error {
	if _, e := tx.ExecContext(ctx, `SAVEPOINT admin_diagnostic_section`); e != nil {
		return e
	}
	e := read()
	if e != nil {
		if _, rollback := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT admin_diagnostic_section`); rollback != nil {
			return rollback
		}
	}
	_, release := tx.ExecContext(ctx, `RELEASE SAVEPOINT admin_diagnostic_section`)
	if e != nil {
		return e
	}
	return release
}
func adminIssue(code, scope, reason string, now time.Time) service.AdminIssue {
	return service.AdminIssue{Code: code, Severity: "warning", Scope: scope, ObservedAt: now, Reason: reason, RecommendedAction: "Review the diagnostic coverage and operator runbook.", Runbook: adminRunbook}
}
func adminCount(ctx context.Context, tx *sql.Tx, now time.Time, source, coverage, query string, args ...any) service.AdminCount {
	c := service.AdminCount{ObservedAt: now, Source: source, Coverage: coverage, Freshness: "unknown"}
	var n int64
	if e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT count(*) FROM (`+query+` LIMIT 10001) admin_bounded`, args...).Scan(&n)
	}); e != nil {
		return c
	}
	c.Value = &n
	c.Available = true
	c.Capped = n > 10000
	c.Freshness = "fresh"
	return c
}
func adminSearchWhere(f service.AdminWorkspaceFilter) (string, []any) {
	clauses := []string{"TRUE"}
	args := []any{}
	add := func(expr string, v any) {
		args = append(args, v)
		clauses = append(clauses, fmt.Sprintf(expr, len(args)))
	}
	if f.WorkspaceID > 0 {
		add("id=$%d", f.WorkspaceID)
	}
	if f.OwnerUserID > 0 {
		add("owner_user_id=$%d", f.OwnerUserID)
	}
	if f.BillingOwnerUserID > 0 {
		add("billing_owner_user_id=$%d", f.BillingOwnerUserID)
	}
	if f.NamePrefix != "" {
		add(`lower(name) LIKE $%d ESCAPE '\'`, f.EscapedNamePrefix())
	}
	if f.SlugPrefix != "" {
		add(`lower(slug) LIKE $%d ESCAPE '\'`, f.EscapedSlugPrefix())
	}
	if f.Status != "" {
		add("status=$%d", f.Status)
	}
	if f.Type != "" {
		add("type=$%d", f.Type)
	}
	for _, v := range []struct{ expr, value string }{{"created_at >= $%d::timestamptz", f.CreatedFrom}, {"created_at <= $%d::timestamptz", f.CreatedTo}, {"updated_at >= $%d::timestamptz", f.UpdatedFrom}, {"updated_at <= $%d::timestamptz", f.UpdatedTo}} {
		if v.value != "" {
			add(v.expr, v.value)
		}
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
func (r *workspaceRepository) AdminSearchWorkspaces(ctx context.Context, a int64, f service.AdminWorkspaceFilter) (*service.AdminWorkspacePage, error) {
	if e := f.Validate(); e != nil {
		return nil, e
	}
	f = f.Normalized()
	tx, now, e := r.adminReadTx(ctx, a)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	where, args := adminSearchWhere(f)
	out := &service.AdminWorkspacePage{Items: []service.Workspace{}, Page: f.Page, PageSize: f.PageSize, ObservedAt: now}
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM (SELECT id FROM workspaces`+where+` LIMIT 10001) bounded`, args...).Scan(&out.Total); e != nil {
		return nil, e
	}
	out.TotalCapped = out.Total > 10000
	out.TotalPages = (out.Total + int64(f.PageSize) - 1) / int64(f.PageSize)
	order := f.Sort + " " + f.Direction
	if f.Sort != "id" {
		order += ",id " + f.Direction
	}
	queryArgs := append(append([]any{}, args...), f.PageSize, (f.Page-1)*f.PageSize)
	rows, e := tx.QueryContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces`+where+` ORDER BY `+order+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2), queryArgs...)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		w, err := scanWorkspace(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		w.Permissions = []string{}
		out.Items = append(out.Items, *w)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, e
	}
	return out, tx.Commit()
}

type adminInventoryQuery struct{ name, source, query string }

var adminOverviewInventory = []adminInventoryQuery{
	{"workspaces", "workspaces", "SELECT id FROM workspaces"},
	{"projects", "projects", "SELECT id FROM projects"},
	{"service_accounts", "service_accounts", "SELECT id FROM service_accounts"},
	{"identity_providers", "workspace_identity_providers", "SELECT id FROM workspace_identity_providers"},
	{"scim_connectors", "workspace_scim_connectors", "SELECT id FROM workspace_scim_connectors"},
	{"open_findings", "finops_anomaly_findings", "SELECT id FROM finops_anomaly_findings WHERE status='open'"},
}

func (r *workspaceRepository) AdminDiagnosticsOverview(ctx context.Context, a int64) (*service.AdminDiagnosticsOverview, error) {
	tx, now, e := r.adminReadTx(ctx, a)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	out := &service.AdminDiagnosticsOverview{State: "unknown", ObservedAt: now, Counts: map[string]service.AdminCount{}, Issues: []service.AdminIssue{}}
	for _, q := range adminOverviewInventory {
		out.Counts[q.name] = adminCount(ctx, tx, now, q.source, "inventory", q.query)
	}
	for _, s := range []string{"active", "suspended", "archived", "pending_deletion", "purging", "deleted"} {
		out.Counts["workspaces_"+s] = adminCount(ctx, tx, now, "workspaces", "inventory", `SELECT id FROM workspaces WHERE status=$1`, s)
	}
	adminResourceStates(ctx, tx, now, 0, out.Counts)
	out.Jobs = adminJobsTx(ctx, tx, now, 0)
	out.Identity = adminIdentityTx(ctx, tx, now, 0)
	var exceptions, unknown int64
	for _, worker := range out.Jobs.Workers {
		if worker.State == "degraded" || worker.State == "blocked" {
			exceptions++
		}
		if worker.Liveness == "unknown" {
			unknown++
		}
	}
	out.WorkerExceptions = service.AdminCount{Value: &exceptions, Available: true, ObservedAt: now, Source: "nine_worker_diagnostics", Coverage: "proven_blocked_or_degraded_families; unknown_liveness_excluded", Freshness: "fresh"}
	out.UnknownWorkerLiveness = service.AdminCount{Value: &unknown, Available: true, ObservedAt: now, Source: "nine_worker_diagnostics", Coverage: "families_without_heartbeat_evidence; not_proven_failures", Freshness: "fresh"}
	out.State = out.Jobs.State
	for name, c := range out.Counts {
		if !c.Available {
			out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", name, "This inventory source is unavailable.", now))
			out.State = "unknown"
		}
	}
	return out, tx.Commit()
}
func (r *workspaceRepository) AdminWorkspaceDiagnostics(ctx context.Context, a, w int64) (*service.AdminWorkspaceDiagnostics, error) {
	if w <= 0 {
		return nil, service.ErrWorkspaceInvalid
	}
	tx, now, e := r.adminReadTx(ctx, a)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	ws, e := scanWorkspace(tx.QueryRowContext(ctx, `SELECT `+workspaceColumns+` FROM workspaces WHERE id=$1`, w))
	if e != nil {
		return nil, e
	}
	ws.Permissions = []string{}
	out := &service.AdminWorkspaceDiagnostics{State: "unknown", ObservedAt: now, Workspace: ws, Counts: map[string]service.AdminCount{}, RecentAudit: []service.AdminAuditSummary{}, Retention: []service.AdminRetentionSummary{}, Issues: []service.AdminIssue{}, AllowedOperations: []string{}}
	checks := []adminInventoryQuery{
		{"members", "workspace_members", `SELECT id FROM workspace_members WHERE workspace_id=$1`},
		{"projects", "projects", `SELECT id FROM projects WHERE workspace_id=$1`},
		{"service_accounts", "service_accounts", `SELECT id FROM service_accounts WHERE workspace_id=$1`},
		{"identity_providers", "workspace_identity_providers", `SELECT id FROM workspace_identity_providers WHERE workspace_id=$1`},
		{"scim_connectors", "workspace_scim_connectors", `SELECT id FROM workspace_scim_connectors WHERE workspace_id=$1`},
		{"open_findings", "finops_anomaly_findings", `SELECT id FROM finops_anomaly_findings WHERE workspace_id=$1 AND status='open'`},
		{"pending_reservations", "budget_reservations", `SELECT id FROM budget_reservations WHERE workspace_id=$1 AND status='pending'`},
		{"pending_quota_reservations", "policy_quota_reservations", `SELECT id FROM policy_quota_reservations WHERE workspace_id=$1 AND status='pending'`},
		{"pending_batch_images", "batch_image_jobs", `SELECT id FROM batch_image_jobs WHERE workspace_id=$1 AND (status NOT IN ('completed','failed','cancelled','output_deleted') OR last_error_code='SUBMIT_OUTCOME_UNKNOWN')`},
		{"pending_exports", "workspace_export_jobs", `SELECT id FROM workspace_export_jobs WHERE workspace_id=$1 AND state IN ('pending','running')`},
		{"holds", "workspace_lifecycle_holds", `SELECT code FROM workspace_lifecycle_holds WHERE workspace_id=$1 AND active`},
		{"failed_batch_images", "batch_image_jobs", `SELECT id FROM batch_image_jobs WHERE workspace_id=$1 AND status='failed'`},
		{"pending_webhook_deliveries", "workspace_webhook_deliveries", `SELECT id FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND status IN ('pending','retrying','delivering')`},
		{"reserved_budget_counters", "budget_counters", `SELECT scope_id FROM budget_counters WHERE scope_type='workspace' AND scope_id=$1 AND reserved<>0 UNION ALL SELECT c.scope_id FROM (SELECT id FROM projects WHERE workspace_id=$1 LIMIT 10001) p JOIN LATERAL (SELECT scope_id FROM budget_counters WHERE scope_type='project' AND scope_id=p.id AND reserved<>0 LIMIT 10001 OFFSET 0) c ON TRUE`},
	}
	for _, q := range checks {
		out.Counts[q.name] = adminCount(ctx, tx, now, q.source, "workspace_inventory", q.query, w)
		if !out.Counts[q.name].Available {
			out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", q.name, "This workspace inventory source is unavailable.", now))
		}
	}
	adminResourceStates(ctx, tx, now, w, out.Counts)
	if out.Counts["projects"].Capped {
		c := out.Counts["reserved_budget_counters"]
		c.Capped = true
		c.Coverage += "; bounded_first_10001_project_sources"
		out.Counts["reserved_budget_counters"] = c
	}
	var valid bool
	var ownerValid bool
	e = adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL),EXISTS(SELECT 1 FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.role='owner' AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL)`, w, ws.BillingOwnerUserID).Scan(&valid, &ownerValid)
	})
	if e == nil {
		out.BillingOwnerValid = &valid
		out.OwnerValid = &ownerValid
	} else {
		out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", "billing_owner", "Billing-owner eligibility is unavailable.", now))
	}
	p := &service.AdminSecuritySummary{}
	e = adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT require_sso,require_mfa,session_max_age_seconds,invitation_policy,allow_external_members,workspace_jit_enabled,approved_identity_provider_mode FROM workspace_security_policies WHERE workspace_id=$1`, w).Scan(&p.RequireSSO, &p.RequireMFA, &p.SessionMaxAgeSeconds, &p.InvitationPolicy, &p.AllowExternalMembers, &p.WorkspaceJITEnabled, &p.ApprovedIdentityProviderMode)
	})
	if e == nil {
		out.SecurityPolicy = p
	} else {
		out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", "security_policy", "Security-policy evidence is unavailable.", now))
	}
	e = adminSection(ctx, tx, func() error {
		rows, e := tx.QueryContext(ctx, `SELECT id,action,target_type,target_id,created_at FROM workspace_audit_logs WHERE workspace_id=$1 ORDER BY id DESC LIMIT 20`, w)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v service.AdminAuditSummary
			if e = rows.Scan(&v.ID, &v.Action, &v.TargetType, &v.TargetID, &v.CreatedAt); e != nil {
				return e
			}
			out.RecentAudit = append(out.RecentAudit, v)
		}
		return rows.Err()
	})
	if e != nil {
		out.RecentAudit = []service.AdminAuditSummary{}
		out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", "audit", "Recent audit evidence is unavailable.", now))
	}
	e = adminSection(ctx, tx, func() error {
		rows, e := tx.QueryContext(ctx, `SELECT category,minimum_days,effective_lifecycle_retention_days($1,category),(protected OR effective_lifecycle_retention_days($1,category)=0) FROM platform_retention_policies ORDER BY category LIMIT 32`, w)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v service.AdminRetentionSummary
			if e = rows.Scan(&v.Category, &v.MinimumDays, &v.EffectiveDays, &v.Protected); e != nil {
				return e
			}
			out.Retention = append(out.Retention, v)
		}
		return rows.Err()
	})
	if e != nil {
		out.Retention = []service.AdminRetentionSummary{}
		out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", "retention", "Retention evidence is unavailable.", now))
	}
	out.Jobs = adminJobsTx(ctx, tx, now, w)
	out.Identity = adminIdentityTx(ctx, tx, now, w)
	for _, worker := range out.Jobs.Workers {
		if worker.Worker == "outbox" {
			q, args := adminScopedSQL("o.event_id", `domain_event_outbox o JOIN domain_events e ON e.id=o.event_id`, `o.delivered_at IS NULL AND e.event_type NOT LIKE 'workspace.deletion.%'`, "e.workspace_id", w)
			c := adminCount(ctx, tx, now, "domain_event_outbox", "bounded_recent_scoped_domain_sources; lifecycle_self_events_excluded", q, args...)
			qualifier := service.AdminWorkerDiagnostics{Counts: map[string]service.AdminCount{"pending_outbox": c}}
			adminQualifyCounts(&qualifier, worker.Counts["pending"], "pending_outbox")
			out.Counts["pending_outbox"] = qualifier.Counts["pending_outbox"]
		}
		if worker.Worker == "billing_recovery" {
			out.Counts["unresolved_alerts"] = worker.Counts["pending_alerts"]
		}
	}
	out.Purge = adminCurrentPurge(ctx, tx, now, out)
	out.State = out.Jobs.State
	if (out.BillingOwnerValid != nil && !*out.BillingOwnerValid) || ws.Status != "active" {
		out.State = "blocked"
	}
	if out.State != "blocked" && (out.Identity.Providers.State == "degraded" || out.Identity.SCIM.State == "degraded") {
		out.State = "degraded"
	}
	if ws.Status == "active" {
		out.AllowedOperations = append(out.AllowedOperations, "suspend")
	}
	if ws.Status == "suspended" && out.BillingOwnerValid != nil && *out.BillingOwnerValid && out.OwnerValid != nil && *out.OwnerValid {
		out.AllowedOperations = append(out.AllowedOperations, "resume")
	}
	return out, tx.Commit()
}

func adminResourceStates(ctx context.Context, tx *sql.Tx, now time.Time, w int64, counts map[string]service.AdminCount) {
	for _, q := range []struct {
		name, table string
		states      []string
	}{{"members", "workspace_members", []string{"active", "suspended"}}, {"projects", "projects", []string{"active", "archived"}}, {"service_accounts", "service_accounts", []string{"active", "disabled"}}, {"identity_providers", "workspace_identity_providers", []string{"active", "disabled"}}, {"scim_connectors", "workspace_scim_connectors", []string{"active", "disabled"}}, {"findings", "finops_anomaly_findings", []string{"open", "acknowledged", "resolved"}}} {
		for _, state := range q.states {
			query := `SELECT id FROM ` + q.table + ` WHERE status=$1`
			args := []any{state}
			if w > 0 {
				query += ` AND workspace_id=$2`
				args = append(args, w)
			}
			counts[q.name+"_"+state] = adminCount(ctx, tx, now, q.table, "resource_state_inventory", query, args...)
		}
	}
}
