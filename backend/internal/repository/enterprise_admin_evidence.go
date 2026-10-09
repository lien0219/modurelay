package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"time"
)

func adminUnknownTime(now time.Time, source, coverage string) service.AdminTimeEvidence {
	return service.AdminTimeEvidence{ObservedAt: now, Source: source, Coverage: coverage, Freshness: "unknown"}
}
func adminDueSample(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics, table, predicate string) {
	q := `SELECT 1 FROM ` + table + ` WHERE ` + predicate
	args := []any{}
	if w > 0 {
		q += ` AND workspace_id=$1`
		args = append(args, w)
	}
	sample := adminCount(ctx, tx, now, table, "bounded_first_10001_time_eligible_sources", q, args...)
	if sample.Capped || !sample.Available {
		c := worker.Counts["due"]
		c.Capped = sample.Capped
		c.Coverage = "source_sample_cap10001; not_exact_matching_inventory"
		if !sample.Available {
			c.Value = nil
			c.Available = false
			c.Freshness = "unknown"
		}
		worker.Counts["due"] = c
		worker.DueLagSeconds = nil
		worker.Issues = append(worker.Issues, adminIssue("DUE_SOURCE_SAMPLE_PARTIAL", worker.Worker, "Eligible work is a bounded source sample; empty matches do not certify an empty queue.", now))
	}
}
func adminTimeProbe(ctx context.Context, tx *sql.Tx, now time.Time, w int64, from, predicate, scope, clock, direction, coverage string) service.AdminTimeEvidence {
	out := adminUnknownTime(now, clock, coverage)
	q, args := adminScopedSQL(clock, from, predicate, scope, w)
	var at *time.Time
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT (`+q+` ORDER BY `+clock+` `+direction+` LIMIT 1)`, args...).Scan(&at)
	})
	if e != nil {
		return out
	}
	if at != nil && at.After(now) {
		return out
	}
	out.Available = true
	out.Freshness = "fresh"
	out.Value = at
	if at != nil {
		age := int64(now.Sub(*at).Seconds())
		out.AgeSeconds = &age
	}
	return out
}
func adminIdentityTx(ctx context.Context, tx *sql.Tx, now time.Time, w int64) service.AdminIdentityDiagnostics {
	result := service.AdminIdentityDiagnostics{}
	for _, family := range []string{"providers", "scim"} {
		source := "workspace_identity_providers"
		if family == "scim" {
			source = "workspace_scim_connectors"
		}
		section := service.AdminIdentitySection{State: "unknown", ObservedAt: now, Source: source, Coverage: "latest_20_connections; health_counts_from_first_10001_scoped_sources; validation_or_client_sync_not_worker_liveness", Freshness: "unknown", Items: []service.AdminIdentityConnection{}, Counts: map[string]service.AdminCount{}, Issues: []service.AdminIssue{}}
		selectCols := `id,workspace_id,type,status,last_validated_at,COALESCE(last_validation_code,'')`
		if family == "scim" {
			selectCols = `id,workspace_id,'scim',status,last_sync_at,COALESCE(last_error_code,''),failure_count`
		}
		q, args := adminScopedSQL(selectCols, source, "TRUE", "workspace_id", w)
		e := adminSection(ctx, tx, func() error {
			rows, e := tx.QueryContext(ctx, q+` ORDER BY id DESC LIMIT 21`, args...)
			if e != nil {
				return e
			}
			defer func() { _ = rows.Close() }()
			for rows.Next() {
				v := service.AdminIdentityConnection{ObservedAt: now, Source: source, Coverage: section.Coverage, State: "unknown", Issues: []service.AdminIssue{}}
				var at *time.Time
				var code string
				var failures int64
				if family == "scim" {
					e = rows.Scan(&v.ID, &v.WorkspaceID, &v.Type, &v.Status, &at, &code, &failures)
					v.ConsecutiveFailures = &failures
					v.LastErrorCode = adminFamilyFailureCode("scim", code)
				} else {
					e = rows.Scan(&v.ID, &v.WorkspaceID, &v.Type, &v.Status, &at, &code)
					v.LastValidationCode = adminValidationCode(code)
				}
				if e != nil {
					return e
				}
				v.Freshness = service.AdminEvidenceFreshness(at, now, 24*time.Hour)
				if at != nil && !at.After(now) {
					if family == "scim" {
						v.LastSyncAt = at
					} else {
						v.LastValidatedAt = at
					}
				}
				if v.Status == "disabled" {
					v.State = "blocked"
				} else if v.Freshness != "unknown" {
					if (family == "scim" && failures > 0) || (family == "providers" && code != "" && code != "SUCCESS") {
						v.State = "degraded"
					} else if v.Freshness == "fresh" && (family == "scim" || code == "SUCCESS") {
						v.State = "healthy"
					}
				}
				if family == "scim" && failures > 0 {
					v.State = "degraded"
					v.Issues = append(v.Issues, adminIssue("SCIM_CLIENT_SYNC_ERRORS", "scim", "Connector state records consecutive client-sync errors; failure occurrence time and monitor liveness are unavailable.", now))
				}
				if v.Freshness != "fresh" {
					v.Issues = append(v.Issues, adminIssue("IDENTITY_EVIDENCE_STALE_OR_UNKNOWN", family, "Validation or client-sync observations are old, missing or future.", now))
				}
				if family == "providers" && v.State == "degraded" {
					v.Issues = append(v.Issues, adminIssue("IDENTITY_VALIDATION_FAILED", family, "The retained provider validation reported a known failure.", now))
				}
				section.Items = append(section.Items, v)
			}
			return rows.Err()
		})
		if e != nil {
			section.Items = []service.AdminIdentityConnection{}
			section.Issues = append(section.Issues, adminIssue("SOURCE_UNAVAILABLE", family, "Identity connection evidence is unavailable.", now))
		} else {
			section.Available = true
			section.Freshness = "fresh"
			if len(section.Items) > 20 {
				section.Capped = true
				section.Items = section.Items[:20]
			}
			if len(section.Items) > 0 {
				section.State = "healthy"
				for _, v := range section.Items {
					if v.State == "degraded" {
						section.State = "degraded"
					} else if v.State == "blocked" && section.State != "degraded" {
						section.State = "blocked"
					} else if v.State == "unknown" && section.State == "healthy" {
						section.State = "unknown"
					}
				}
			}
			if section.Capped && section.State == "healthy" {
				section.State = "unknown"
			}
		}
		adminIdentityHealth(ctx, tx, now, w, family, source, &section)
		if family == "providers" {
			result.Providers = section
		} else {
			result.SCIM = section
		}
	}
	return result
}

func adminIdentityHealth(ctx context.Context, tx *sql.Tx, now time.Time, w int64, family, source string, section *service.AdminIdentitySection) {
	clock, success, failure := "last_validated_at", "last_validation_code='SUCCESS'", "COALESCE(last_validation_code,'') NOT IN ('','SUCCESS')"
	cols := "status,last_validated_at,last_validation_code"
	if family == "scim" {
		clock, success, failure = "last_sync_at", "failure_count=0", "failure_count>0"
		cols = "status,last_sync_at,failure_count"
	}
	known := clock + " IS NOT NULL AND " + clock + "<=now()"
	fresh := clock + ">=now()-interval '24 hours' AND " + known
	conditions := []string{"TRUE", "status='active'", "status='disabled'", "status='active' AND " + fresh + " AND " + success, "status='active' AND " + known + " AND " + failure, "status='active' AND " + clock + "<now()-interval '24 hours'", "status='active' AND (" + clock + " IS NULL OR " + clock + ">now())"}
	if family == "providers" {
		conditions[6] += " OR (status='active' AND COALESCE(last_validation_code,'')='')"
	}
	query := "SELECT "
	for i, p := range conditions {
		if i > 0 {
			query += ","
		}
		query += "count(*) FILTER (WHERE " + p + ")"
	}
	query += " FROM (SELECT " + cols + " FROM " + source
	var args []any
	if w > 0 {
		query += " WHERE workspace_id=$1"
		args = []any{w}
	}
	query += " LIMIT 10001) bounded_identity_sources"
	var values [7]int64
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, query, args...).Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6])
	})
	names := []string{"total", "active", "disabled", "healthy_observation", "failed_validation", "stale_observation", "unknown_observation"}
	if family == "scim" {
		names[4] = "client_sync_errors"
	}
	for i, name := range names {
		c := adminUnknownCount(now, source, "bounded_first_10001_identity_sources; not_worker_liveness")
		if e == nil {
			n := values[i]
			c.Value = &n
			c.Available = true
			c.Capped = values[0] > 10000
			c.Freshness = "fresh"
		}
		section.Counts[name] = c
	}
	if e != nil {
		section.State = "unknown"
		section.Issues = append(section.Issues, adminIssue("IDENTITY_HEALTH_UNAVAILABLE", family, "The bounded identity health inventory is unavailable.", now))
		return
	}
	// The recent list is independently capped. Health covers a larger bounded
	// source population, and positive failures remain proven even when capped.
	if values[4] > 0 {
		section.State = "degraded"
	} else if values[2] > 0 {
		section.State = "blocked"
	} else if values[0] > 10000 || values[5] > 0 || values[6] > 0 || values[0] == 0 {
		section.State = "unknown"
	} else {
		section.State = "healthy"
	}
}
func adminValidationCode(code string) string {
	switch code {
	case "", "SUCCESS", "DISCOVERY_FAILED", "ISSUER_MISMATCH", "ENDPOINT_INVALID", "CONFIGURATION_INVALID", "VALIDATION_FAILED", "PROVIDER_DISABLED", "SAML_CONFIGURATION_INVALID", "SAML_METADATA_INVALID", "SAML_CERTIFICATE_INVALID":
		return code
	default:
		return "VALIDATION_FAILED"
	}
}
func adminFamilyFailureCode(family, code string) string {
	if code == "" {
		return ""
	}
	switch family {
	case "outbox":
		switch code {
		case "dispatch_canceled", "dispatch_timeout", "outbox_lease_lost", "dispatch_failed", "notification_enqueue_failed", "webhook_enqueue_failed":
			return code
		}
		return "DISPATCH_FAILURE"
	case "webhook":
		switch code {
		case "delivery_terminal", "delivery_retryable", "delivery_lease_lost", "repository_update_failed":
			return code
		}
		return "JOB_FAILURE"
	case "finops":
		switch code {
		case "canceled", "timeout", "database_error":
			return code
		}
		return "DETECTOR_FAILURE"
	case "scim":
		switch code {
		case "invalidValue", "invalidPath", "uniqueness", "mutability", "invalidFilter", "tooLarge", "not_found", "precondition", "sync_failed", "security_conflict":
			return code
		}
		return "SYNC_FAILURE"
	case "export", "purge":
		switch code {
		case "EXPORT_BUILD_FAILED", "EXPORT_LIMIT_EXCEEDED", "LEASE_RETRIES_EXHAUSTED", "PURGE_BATCH_FAILED":
			return code
		}
		if safe, ok := adminBlockerReason(code); ok {
			_ = safe
			return code
		}
		return "LIFECYCLE_FAILURE"
	case "async_settlement":
		switch code {
		case "SUBMIT_OUTCOME_UNKNOWN", "SUBMIT_FAILED", "POLL_FAILED", "SETTLEMENT_FAILED", "BILLING_HOLD_FAILED", "PROVIDER_JOB_FAILED", "INPUT_UPLOAD_FAILED":
			return code
		}
		return "ASYNC_TASK_FAILURE"
	}
	return "JOB_FAILURE"
}
func adminBlockerReason(code string) (string, bool) {
	switch code {
	case "PERSONAL_WORKSPACE_PROTECTED":
		return "Personal Workspace cleanup requires its separate account lifecycle.", true
	case "WORKSPACE_STATE_CONFLICT":
		return "Workspace state does not permit business cleanup.", true
	case "BILLING_OWNER_UNAVAILABLE":
		return "The billing owner is not an active eligible member.", true
	case "PENDING_BUDGET_RESERVATIONS":
		return "Accepted budget holds remain pending.", true
	case "UNSETTLED_BUDGET_COUNTERS":
		return "Reserved budget counters remain nonzero.", true
	case "PENDING_QUOTA_RESERVATIONS":
		return "Accepted quota holds remain pending.", true
	case "UNSETTLED_BILLING":
		return "Billing recovery has pending alert evidence.", true
	case "PENDING_ASYNC_MEDIA":
		return "Accepted image tasks or uncertain submission outcomes require recovery.", true
	case "PENDING_EXPORTS":
		return "Export jobs remain pending or running.", true
	case "PENDING_WEBHOOK_DELIVERIES":
		return "Webhook delivery work remains nonterminal.", true
	case "PENDING_DOMAIN_EVENTS":
		return "Domain dispatch work remains undelivered.", true
	case "LEGAL_HOLD", "BUSINESS_HOLD":
		return "An active operator hold protects Workspace business resources.", true
	case "RETENTION_PROTECTED", "RETENTION_INDEFINITE":
		return "The effective retention policy protects retained records; this does not prohibit nonprotected business closure.", true
	}
	return "Stored blocker classification is unavailable.", false
}
func adminStoredBlockers(raw []byte, now, at time.Time) []service.AdminBlocker {
	// PostgreSQL projects only scalar codes from at most32 JSON array entries;
	// freeform reason/count data never crosses the SQL boundary.
	var codes []string
	if json.Unmarshal(raw, &codes) != nil {
		return []service.AdminBlocker{{Code: "UNKNOWN_STORED_BLOCKER", Reason: "Stored blocker classification is unavailable.", ObservedAt: now, SourceTime: &at, Source: "workspace_deletion_jobs.blocking_reasons", Coverage: "historical_job_checkpoint", Freshness: service.AdminEvidenceFreshness(&at, now, 5*time.Minute), Runbook: adminRunbook}}
	}
	out := []service.AdminBlocker{}
	for _, code := range codes {
		reason, ok := adminBlockerReason(code)
		if !ok {
			code = "UNKNOWN_STORED_BLOCKER"
		}
		out = append(out, service.AdminBlocker{Code: code, Reason: reason, ObservedAt: now, SourceTime: &at, Source: "workspace_deletion_jobs.blocking_reasons", Coverage: "historical_job_checkpoint", Freshness: service.AdminEvidenceFreshness(&at, now, 5*time.Minute), Runbook: adminRunbook})
	}
	return out
}
func adminCurrentPurge(ctx context.Context, tx *sql.Tx, now time.Time, d *service.AdminWorkspaceDiagnostics) service.AdminPurgeDiagnostics {
	out := service.AdminPurgeDiagnostics{State: "unknown", Available: true, ObservedAt: now, Coverage: "current_bounded_preflight", Blockers: []service.AdminBlocker{}, Issues: []service.AdminIssue{}}
	add := func(code string, count *service.AdminCount) {
		reason, _ := adminBlockerReason(code)
		out.Blockers = append(out.Blockers, service.AdminBlocker{Code: code, Reason: reason, Count: count, ObservedAt: now, SourceTime: &now, Source: "current_bounded_preflight", Coverage: out.Coverage, Freshness: "fresh", Runbook: adminRunbook})
		out.State = "blocked"
	}
	if d.Workspace.Type == "personal" {
		add("PERSONAL_WORKSPACE_PROTECTED", nil)
	}
	if d.Workspace.Status != "active" && d.Workspace.Status != "archived" && d.Workspace.Status != "pending_deletion" && d.Workspace.Status != "purging" {
		add("WORKSPACE_STATE_CONFLICT", nil)
	}
	if d.BillingOwnerValid == nil {
		out.Available = false
		out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", "purge", "Billing-owner admission evidence is unavailable.", now))
	} else if !*d.BillingOwnerValid {
		add("BILLING_OWNER_UNAVAILABLE", nil)
	}
	for _, q := range []struct{ name, code string }{{"pending_reservations", "PENDING_BUDGET_RESERVATIONS"}, {"pending_quota_reservations", "PENDING_QUOTA_RESERVATIONS"}, {"unresolved_alerts", "UNSETTLED_BILLING"}, {"pending_batch_images", "PENDING_ASYNC_MEDIA"}, {"pending_exports", "PENDING_EXPORTS"}, {"pending_webhook_deliveries", "PENDING_WEBHOOK_DELIVERIES"}, {"pending_outbox", "PENDING_DOMAIN_EVENTS"}, {"reserved_budget_counters", "UNSETTLED_BUDGET_COUNTERS"}} {
		c := d.Counts[q.name]
		if !c.Available || c.Capped {
			out.Available = false
			out.Issues = append(out.Issues, adminIssue("SOURCE_PARTIAL_OR_UNAVAILABLE", "purge", "A blocker source is unavailable or a bounded lower-bound inventory.", now))
		}
		if c.Value != nil && *c.Value > 0 {
			add(q.code, &c)
		}
	}
	e := adminSection(ctx, tx, func() error {
		rows, e := tx.QueryContext(ctx, `SELECT code FROM workspace_lifecycle_holds WHERE workspace_id=$1 AND active ORDER BY code LIMIT 32`, d.Workspace.ID)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var code string
			if e = rows.Scan(&code); e != nil {
				return e
			}
			if _, ok := adminBlockerReason(code); ok {
				add(code, nil)
			}
		}
		return rows.Err()
	})
	if e != nil {
		out.Available = false
		out.Issues = append(out.Issues, adminIssue("SOURCE_UNAVAILABLE", "purge", "Operator hold evidence is unavailable.", now))
	}
	// Retention protection describes retained categories, not a fabricated block
	// of business closure. Do not classify an indefinite financial floor as a
	// failed purge: the existing worker deliberately retains those records.
	if len(d.Retention) == 0 {
		out.Available = false
	}
	if out.State != "blocked" && out.Available {
		out.State = "healthy"
	}
	return out
}
