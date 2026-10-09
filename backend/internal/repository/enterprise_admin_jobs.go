package repository

import (
	"context"
	"database/sql"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"strconv"
	"strings"
	"time"
)

func (r *workspaceRepository) AdminOperationsJobs(ctx context.Context, a int64) (*service.AdminJobsDiagnostics, error) {
	tx, now, e := r.adminReadTx(ctx, a)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	out := adminJobsTx(ctx, tx, now, 0)
	return out, tx.Commit()
}

type adminWorkerQuery struct{ state, from, predicate, scope string }

func adminWebhookNonterminalSource(w int64) string {
	if w > 0 {
		var sources []string
		for _, status := range []string{"pending", "retrying", "delivering"} {
			// A singleton status array keeps status in the required index ordering,
			// avoiding an id-only PK scan with a residual status filter. An explicit
			// id range seeks the third key without a cross-status tuple-page walk.
			sources = append(sources, `(`+adminWebhookSourceSeek(`workspace_id=$1 AND status=ANY(ARRAY['`+status+`']::text[])`, `status,id`, `id>sample.id`)+`)`)
		}
		return strings.Join(sources, ` UNION ALL `) + ` LIMIT 10001`
	}
	return adminWebhookSourceSeek(`status IN ('pending','retrying','delivering')`, `created_at,workspace_id,id`, `(created_at,workspace_id,id)>(sample.created_at,sample.workspace_id,sample.id)`)
}

func adminWebhookSourceSeek(predicate, order, next string) string {
	columns := `id,workspace_id,webhook_id,status,created_at`
	// A large LIMIT alone can choose a full heap scan/sort on fresh pages. Seek
	// one ordered index key per step so sparse terminal history cannot make the
	// source scan unbounded; endpoint filters remain outside this source.
	return `WITH RECURSIVE sample AS (
		(SELECT ` + columns + `,1 AS ordinal FROM workspace_webhook_deliveries WHERE ` + predicate + ` ORDER BY ` + order + ` LIMIT 1)
		UNION ALL
		SELECT next.id,next.workspace_id,next.webhook_id,next.status,next.created_at,sample.ordinal+1
		FROM sample JOIN LATERAL (
			SELECT ` + columns + ` FROM workspace_webhook_deliveries WHERE ` + predicate + ` AND ` + next + ` ORDER BY ` + order + ` LIMIT 1
		) next ON TRUE WHERE sample.ordinal<10001
	) SELECT ` + columns + ` FROM sample`
}

func adminWebhookInventoryFrom(w int64) string {
	// Materialize once and deduplicate endpoint keys before looking up enablement.
	// Tiny endpoint tables may use a sequential lookup; do not repeat it for every
	// delivery of the same endpoint. Large estates use the scoped endpoint key.
	return `(WITH deliveries AS MATERIALIZED (` + adminWebhookNonterminalSource(w) + `),
		endpoints AS MATERIALIZED (
			SELECT keys.workspace_id,keys.webhook_id,h.enabled
			FROM (SELECT DISTINCT workspace_id,webhook_id FROM deliveries) keys
			JOIN LATERAL (SELECT enabled FROM workspace_webhooks WHERE workspace_id=keys.workspace_id AND id=keys.webhook_id OFFSET 0) h ON TRUE
		)
		SELECT d.*,h.enabled AS endpoint_enabled FROM deliveries d
		JOIN endpoints h ON h.workspace_id=d.workspace_id AND h.webhook_id=d.webhook_id) d`
}

func adminWebhookInventorySQL(w int64) (string, []any) {
	q := `SELECT count(*),count(*) FILTER (WHERE d.status IN ('pending','retrying')),count(*) FILTER (WHERE NOT d.endpoint_enabled) FROM ` + adminWebhookInventoryFrom(w)
	if w > 0 {
		return q, []any{w}
	}
	return q, nil
}

func adminWebhookInventory(ctx context.Context, tx *sql.Tx, now time.Time, w int64) (service.AdminCount, service.AdminCount, service.AdminCount) {
	q, args := adminWebhookInventorySQL(w)
	var total, pending, disabled int64
	if e := adminSection(ctx, tx, func() error { return tx.QueryRowContext(ctx, q, args...).Scan(&total, &pending, &disabled) }); e != nil {
		unknown := adminUnknownCount(now, "workspace_webhook_deliveries", "bounded_nonterminal_source_unavailable")
		return unknown, unknown, unknown
	}
	count := func(n int64) service.AdminCount {
		return service.AdminCount{Value: &n, Available: true, Capped: total > 10000, ObservedAt: now, Source: "workspace_webhook_deliveries", Coverage: "bounded_first_10001_nonterminal_sources; filtered_counts_are_lower_bounds_after_source_cap", Freshness: "fresh"}
	}
	return count(total), count(pending), count(disabled)
}

func adminScopedSQL(selectExpr, from, predicate, scope string, w int64) (string, []any) {
	if w > 0 && (from == "workspace_export_jobs j" || from == "workspace_deletion_jobs j") && adminLifecycleHistoricalPredicate(predicate, from) {
		from = `(SELECT * FROM ` + strings.TrimSuffix(from, " j") + ` WHERE workspace_id=$1 ORDER BY created_at DESC,id DESC LIMIT 10001) j`
	}
	if from == "finops_anomaly_detection_leases f" && w > 0 {
		if strings.Contains(predicate, "f.completed_at IS NULL") && (strings.Contains(predicate, "f.claimed_until") || strings.Contains(predicate, "f.last_error_code")) {
			from = `(SELECT * FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND completed_at IS NULL ORDER BY bucket_start LIMIT 10001) f`
		} else if strings.Contains(predicate, "f.failed_at IS NOT NULL") {
			from = `(SELECT * FROM finops_anomaly_detection_leases WHERE workspace_id=$1 AND completed_at IS NULL AND failed_at IS NOT NULL ORDER BY bucket_start LIMIT 10001) f`
		} else if strings.Contains(predicate, "f.completed_at IS NOT NULL") {
			from = `(SELECT * FROM finops_anomaly_detection_leases WHERE workspace_id=$1 ORDER BY bucket_start DESC LIMIT 10001) f`
		}
	}
	if w > 0 && from == "batch_image_jobs j" && (strings.Contains(predicate, "j.last_error_code") && !strings.Contains(predicate, "j.status NOT IN") || strings.Contains(predicate, "j.finished_at")) {
		from = `(SELECT * FROM batch_image_jobs WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT 10001) j`
	}
	if w > 0 && from == "workspace_scim_tokens t" {
		from = `(SELECT * FROM workspace_scim_tokens WHERE workspace_id=$1 LIMIT 10001) t`
	}
	if w > 0 && from == "workspace_scim_connectors c" && strings.Contains(predicate, "c.failure_count") {
		from = `(SELECT * FROM workspace_scim_connectors WHERE workspace_id=$1 ORDER BY id DESC LIMIT 10001) c`
	}
	if strings.Contains(from, "workspace_webhook_deliveries d JOIN workspace_webhooks h") {
		switch {
		case w > 0 && predicate == `d.status='succeeded' AND d.finished_at IS NOT NULL`:
			// Endpoint ownership/existence is enforced by the delivery FK. Success
			// does not depend on endpoint enablement; seek scoped completion order.
			from = `workspace_webhook_deliveries d`
		case predicate == `NOT h.enabled AND d.status IN ('pending','retrying','delivering')`:
			// Keep endpoint filtering outside the indexed delivery source limit.
			from = adminWebhookInventoryFrom(w)
			predicate = `NOT d.endpoint_enabled AND d.status IN ('pending','retrying','delivering')`
		case w == 0 && predicate == `d.status IN ('pending','retrying')`:
			from = `(` + adminWebhookNonterminalSource(w) + `) d`
		}
	}
	if w > 0 && strings.Contains(from, "workspace_webhook_deliveries d JOIN workspace_webhooks h") && strings.Contains(predicate, "d.locked_at") && !strings.Contains(predicate, "d.next_attempt_at") {
		from = `(SELECT * FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND status='delivering' LIMIT 10001) d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`
	}
	if strings.Contains(predicate, "d.next_attempt_at<=now()") && strings.Contains(from, "workspace_webhook_deliveries d JOIN workspace_webhooks h") {
		source := `SELECT * FROM workspace_webhook_deliveries WHERE status IN ('pending','retrying','delivering') AND next_attempt_at<=now()`
		if w > 0 {
			source += ` AND workspace_id=$1`
		}
		from = `(` + source + ` ORDER BY next_attempt_at,created_at LIMIT 10001) d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`
	}
	if strings.Contains(predicate, "j.available_at<=now()") && (from == "workspace_export_jobs j" || from == "workspace_deletion_jobs j") {
		table := strings.TrimSuffix(from, " j")
		source := `SELECT * FROM ` + table + ` WHERE state IN ('pending','running') AND available_at<=now()`
		if table == "workspace_deletion_jobs" {
			source = `SELECT * FROM ` + table + ` WHERE state IN ('pending','running','blocked') AND GREATEST(available_at,earliest_purge_at)<=now()`
		}
		if w > 0 {
			source += ` AND workspace_id=$1`
		}
		order := "available_at"
		if table == "workspace_deletion_jobs" {
			order = "GREATEST(available_at,earliest_purge_at)"
		}
		from = `(` + source + ` ORDER BY ` + order + ` LIMIT 10001) j`
	}
	if strings.Contains(from, "billing_settlement_alerts b JOIN budget_reservations r") {
		if w > 0 {
			from = `(SELECT id,workspace_id FROM budget_reservations WHERE workspace_id=$1 ORDER BY id LIMIT 10001) r JOIN LATERAL (SELECT * FROM billing_settlement_alerts WHERE reservation_id=r.id OFFSET 0) b ON TRUE`
		} else {
			from = "billing_settlement_alerts b"
		}
	}
	if strings.Contains(from, "domain_event_outbox o JOIN domain_events e") {
		if w > 0 {
			from = `(SELECT id,workspace_id,event_type FROM domain_events WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT 10001) e JOIN LATERAL (SELECT * FROM domain_event_outbox WHERE event_id=e.id OFFSET 0) o ON TRUE`
		} else if strings.Contains(predicate, "o.available_at<=now()") {
			from = `(SELECT * FROM domain_event_outbox WHERE delivered_at IS NULL ORDER BY available_at,created_at,event_id LIMIT 10001) o`
		} else {
			from = strings.Replace(from, ` JOIN domain_events e ON e.id=o.event_id`, "", 1)
		}
	}
	q := "SELECT " + selectExpr + " FROM " + from + " WHERE " + predicate
	if w > 0 {
		return q + " AND " + scope + "=$1", []any{w}
	}
	return q, nil
}

func adminLifecycleHistoricalPredicate(predicate, from string) bool {
	return strings.Contains(predicate, "failure_code") || strings.Contains(predicate, "completed_at") || strings.Contains(predicate, "'completed'") || strings.Contains(predicate, "'cancelled'") || strings.Contains(predicate, "'expired'") || strings.Contains(predicate, "'failed'") && (from == "workspace_export_jobs j" || strings.Contains(predicate, " IN "))
}
func adminWorker(now time.Time, name, source, coverage string) service.AdminWorkerDiagnostics {
	return service.AdminWorkerDiagnostics{Worker: name, State: "unknown", Liveness: "unknown", ObservedAt: now, Source: source, Coverage: coverage, Freshness: "fresh", Counts: map[string]service.AdminCount{}, OldestPending: adminUnknownTime(now, source, "no_pending_clock_evidence"), PendingAlertAge: adminUnknownTime(now, source, "not_an_alert_inventory"), LatestFailureTime: adminUnknownTime(now, source, "failure_occurrence_time_unavailable"), LastFailure: service.AdminFailureEvidence{ObservedAt: now, Source: source, Coverage: "failure_code_unavailable", Freshness: "unknown"}, Jobs: []service.AdminJobSummary{}, Issues: []service.AdminIssue{adminIssue("LIVENESS_UNAVAILABLE", name, "Persisted queue evidence does not establish worker process liveness.", now)}, Runbook: adminRunbook}
}
func adminUnknownCount(now time.Time, source, coverage string) service.AdminCount {
	return service.AdminCount{ObservedAt: now, Source: source, Coverage: coverage, Freshness: "unknown"}
}
func adminWorkerCounts(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics, queries []adminWorkerQuery) {
	for _, q := range queries {
		sql, args := adminScopedSQL("1", q.from, q.predicate, q.scope, w)
		c := adminCount(ctx, tx, now, worker.Source, worker.Coverage, sql, args...)
		worker.Counts[q.state] = c
		if !c.Available {
			worker.Issues = append(worker.Issues, adminIssue("SOURCE_UNAVAILABLE", worker.Worker, "This bounded worker inventory source is unavailable.", now))
		}
		if c.Capped {
			worker.Issues = append(worker.Issues, adminIssue("INVENTORY_CAPPED", worker.Worker, "Inventory is a lower bound, not an exact total.", now))
		}
		if worker.State != "blocked" && c.Value != nil && *c.Value > 0 && (q.state == "dead" || q.state == "failed" || q.state == "expired_leases" || q.state == "retry_evidence" || q.state == "connector_errors" || q.state == "pending_alerts") {
			worker.State = "degraded"
		}
		if c.Value != nil && *c.Value > 0 && (q.state == "blocked" || q.state == "disabled_endpoint_backlog") {
			worker.State = "blocked"
		}
	}
}
func adminDueLag(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics, from, predicate, scope, clock string) {
	q, args := adminScopedSQL(clock, from, predicate, scope, w)
	var at *time.Time
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT min(due_at) FROM (`+q+` ORDER BY `+clock+` LIMIT 1) oldest(due_at)`, args...).Scan(&at)
	})
	if e != nil {
		worker.Issues = append(worker.Issues, adminIssue("LAG_UNAVAILABLE", worker.Worker, "Due-work lag is unavailable.", now))
		return
	}
	lag := int64(0)
	if at != nil {
		if at.After(now) {
			return
		}
		lag = int64(now.Sub(*at).Seconds())
	}
	worker.DueLagSeconds = &lag
}

func adminJobsTx(ctx context.Context, tx *sql.Tx, now time.Time, w int64) *service.AdminJobsDiagnostics {
	out := &service.AdminJobsDiagnostics{State: "unknown", ObservedAt: now, Workers: []service.AdminWorkerDiagnostics{}, WebhookFailureTrend: []service.AdminFailureBucket{}, Issues: []service.AdminIssue{}}
	ob := adminWorker(now, "outbox", "domain_event_outbox", "shared_dispatch_queue")
	obFrom := `domain_event_outbox o JOIN domain_events e ON e.id=o.event_id`
	obScope := "e.workspace_id"
	adminWorkerCounts(ctx, tx, now, w, &ob, []adminWorkerQuery{
		{"pending", obFrom, `o.delivered_at IS NULL`, obScope},
		{"live_leases", obFrom, `o.delivered_at IS NULL AND o.locked_until>now()`, obScope},
		{"expired_leases", obFrom, `o.delivered_at IS NULL AND o.locked_until<=now()`, obScope},
		{"due", obFrom, `o.delivered_at IS NULL AND o.available_at<=now() AND (o.locked_until IS NULL OR o.locked_until<=now())`, obScope},
		{"retry_evidence", obFrom, `o.delivered_at IS NULL AND o.last_error IS NOT NULL AND o.last_error<>''`, obScope},
	})
	adminDueLag(ctx, tx, now, w, &ob, obFrom, `o.delivered_at IS NULL AND o.available_at<=now() AND (o.locked_until IS NULL OR o.locked_until<=now())`, obScope, "o.available_at")
	if w == 0 && ob.Counts["pending"].Capped {
		c := ob.Counts["due"]
		c.Capped = true
		c.Coverage = "bounded_first_10001_pending_sources; not_exact_due_total"
		ob.Counts["due"] = c
		ob.DueLagSeconds = nil
	}
	adminLatestSuccess(ctx, tx, w, &ob, obFrom, `o.delivered_at IS NOT NULL`, obScope, "o.delivered_at")
	ob.Counts["running"] = adminUnknownCount(now, "domain_event_outbox", "only_live_claims_are_persisted; execution_state_unavailable")
	ob.Counts["failed"] = adminUnknownCount(now, "domain_event_outbox", "retry_evidence_is_not_terminal_failure")
	adminWorkerCounts(ctx, tx, now, w, &ob, []adminWorkerQuery{{"completed", obFrom, `o.delivered_at IS NOT NULL`, obScope}})
	ob.OldestPending = adminTimeProbe(ctx, tx, now, w, obFrom, `o.delivered_at IS NULL`, obScope, "o.created_at", "ASC", "oldest_undelivered_creation_time")
	adminFailureProbe(ctx, tx, now, w, &ob, obFrom, `o.delivered_at IS NULL AND o.last_error IS NOT NULL AND o.last_error<>''`, obScope, "o.last_error", "o.available_at", "current_retry_code; available_at_is_scheduled_retry_not_failure_time")
	if w > 0 {
		coverage := adminCount(ctx, tx, now, "domain_events", "bounded_recent_10001_event_sample", `SELECT id FROM domain_events WHERE workspace_id=$1`, w)
		adminApplySample(&ob, coverage)
	}
	out.Workers = append(out.Workers, ob)
	notification := adminWorker(now, "notification", "domain_event_outbox", "shared_with_outbox_not_additive")
	notification.Counts["pending"] = adminUnknownCount(now, "domain_event_outbox", "shared_with_outbox_not_additive")
	for _, state := range []string{"running", "failed", "completed"} {
		notification.Counts[state] = adminUnknownCount(now, "domain_event_outbox", "shared_with_outbox_not_additive")
	}
	notification.Freshness = "unknown"
	notification.Issues = append(notification.Issues, adminIssue("SHARED_QUEUE", "notification", "Notification dispatch shares the outbox; do not add a second backlog.", now))
	out.Workers = append(out.Workers, notification)

	wh := adminWorker(now, "webhook", "workspace_webhook_deliveries", "persisted_delivery_inventory")
	whFrom := `workspace_webhook_deliveries d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`
	whScope := "d.workspace_id"
	queries := []adminWorkerQuery{
		{"running", whFrom, `d.status='delivering'`, whScope},
		{"failed", whFrom, `d.status='dead'`, whScope},
		{"completed", whFrom, `d.status='succeeded'`, whScope},
		{"live_leases", whFrom, `d.status='delivering' AND d.locked_at>now()-interval '2 minutes'`, whScope},
		{"expired_leases", whFrom, `d.status='delivering' AND d.locked_at<=now()-interval '2 minutes'`, whScope},
		{"due", whFrom, `h.enabled AND d.status IN ('pending','retrying','delivering') AND d.next_attempt_at<=now() AND (d.locked_at IS NULL OR d.locked_at<=now()-interval '2 minutes')`, whScope},
		{"dead", whFrom, `d.status='dead'`, whScope},
	}
	if w > 0 {
		queries = append(queries, adminWorkerQuery{"pending", whFrom, `d.status IN ('pending','retrying')`, whScope})
	}
	adminWorkerCounts(ctx, tx, now, w, &wh, queries)
	nonterminal, pending, disabled := adminWebhookInventory(ctx, tx, now, w)
	wh.Counts["disabled_endpoint_backlog"] = disabled
	if w == 0 {
		wh.Counts["pending"] = pending
	}
	if disabled.Value != nil && *disabled.Value > 0 {
		wh.State = "blocked"
	}
	wh.Coverage = "persisted_delivery_inventory; lease_expiry_derived_from_current_fixed_2m_worker_lease"
	adminDueLag(ctx, tx, now, w, &wh, whFrom, `h.enabled AND d.status IN ('pending','retrying','delivering') AND d.next_attempt_at<=now() AND (d.locked_at IS NULL OR d.locked_at<=now()-interval '2 minutes')`, whScope, "d.next_attempt_at")
	adminDueSample(ctx, tx, now, w, &wh, "workspace_webhook_deliveries", `status IN ('pending','retrying','delivering') AND next_attempt_at<=now()`)
	adminLatestSuccess(ctx, tx, w, &wh, whFrom, `d.status='succeeded' AND d.finished_at IS NOT NULL`, whScope, "d.finished_at")
	adminWebhookSummaries(ctx, tx, now, w, &wh)
	adminLatestWebhookFailure(ctx, tx, now, w, &wh)
	oldestFrom := whFrom
	if w > 0 {
		oldestFrom = `(SELECT * FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND status IN ('pending','retrying','delivering') LIMIT 10001) d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`
	}
	wh.OldestPending = adminTimeProbe(ctx, tx, now, w, oldestFrom, `d.status IN ('pending','retrying','delivering')`, whScope, "d.created_at", "ASC", "oldest_nonterminal_delivery_creation_time")
	if nonterminal.Capped || !nonterminal.Available {
		wh.Coverage += "; disabled_backlog_and_global_pending_use_bounded_nonterminal_source"
		wh.Issues = append(wh.Issues, adminIssue("SOURCE_SAMPLE_INCOMPLETE", "webhook", "Filtered nonterminal inventory is a lower bound when its bounded source overflows, and unknown when that source is unavailable.", now))
	}
	if w > 0 {
		if nonterminal.Capped || !nonterminal.Available {
			wh.OldestPending = adminUnknownTime(now, wh.Source, "source_sample_capped_or_unavailable")
		}
		adminQualifyCounts(&wh, wh.Counts["running"], "live_leases", "expired_leases")
	}
	out.Workers = append(out.Workers, wh)
	adminWebhookTrend(ctx, tx, now, w, out)

	fin := adminWorker(now, "finops", "finops_anomaly_detection_leases", "materialized_work_only; undiscovered_rollup_work_unknown")
	adminWorkerCounts(ctx, tx, now, w, &fin, []adminWorkerQuery{
		{"materialized_pending", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND f.failed_at IS NULL`, "f.workspace_id"},
		{"live_leases", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND f.failed_at IS NULL AND f.claimed_until>now()`, "f.workspace_id"},
		{"expired_leases", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND f.failed_at IS NULL AND f.claimed_until<=now()`, "f.workspace_id"},
		{"retry_evidence", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND COALESCE(f.last_error_code,'')<>''`, "f.workspace_id"},
		{"terminal_failed", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND f.failed_at IS NOT NULL`, "f.workspace_id"},
	})
	fin.Counts["pending"] = adminUnknownCount(now, "rollup_discovery", "undiscovered_work_not_enumerated")
	fin.Counts["running"] = adminUnknownCount(now, "finops_anomaly_detection_leases", "live_claims_are_candidates_not_process_execution")
	fin.Counts["failed"] = adminUnknownCount(now, "finops_anomaly_detection_leases", "legacy_failure_alias; use terminal_failed for durable failed_at evidence")
	adminWorkerCounts(ctx, tx, now, w, &fin, []adminWorkerQuery{{"completed", "finops_anomaly_detection_leases f", `f.completed_at IS NOT NULL`, "f.workspace_id"}})
	oldestFinFrom := "finops_anomaly_detection_leases f"
	if w == 0 {
		oldestFinFrom = `(SELECT bucket_start,workspace_id FROM finops_anomaly_detection_leases WHERE completed_at IS NULL LIMIT 10001) f`
	}
	fin.OldestPending = adminTimeProbe(ctx, tx, now, w, oldestFinFrom, `f.completed_at IS NULL AND f.failed_at IS NULL`, "f.workspace_id", "f.bucket_start", "ASC", "oldest_materialized_work_bucket_time; not_job_creation_or_undiscovered_work")
	if w > 0 {
		fin.OldestPending = adminTimeProbe(ctx, tx, now, w, "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND f.failed_at IS NULL`, "f.workspace_id", "f.bucket_start", "ASC", "oldest_scoped_materialized_bucket_time")
	}
	adminFailureProbe(ctx, tx, now, w, &fin, "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND COALESCE(f.last_error_code,'')<>''`, "f.workspace_id", "f.last_error_code", "f.updated_at", "last_materialized_error_state_update; no_failure_occurrence_clock")
	adminLatestSuccess(ctx, tx, w, &fin, "finops_anomaly_detection_leases f", `f.completed_at IS NOT NULL`, "f.workspace_id", "f.completed_at")
	fin.Issues = append(fin.Issues, adminIssue("FINOPS_LEASE_FENCING_LIMITATION", "finops", "Baseline completion evidence does not fully enforce live lease expiry; no reset or retry is supported.", now))
	if w == 0 {
		adminFinOpsStatus(ctx, tx, now, &fin)
		if fin.Counts["materialized_pending"].Capped {
			fin.OldestPending = adminUnknownTime(now, fin.Source, "bounded_first_10001_materialized_sources; globally_oldest_unknown")
		}
	} else {
		adminQualifyCounts(&fin, fin.Counts["materialized_pending"], "live_leases", "expired_leases", "retry_evidence")
		if fin.Counts["materialized_pending"].Capped {
			fin.LastFailure = service.AdminFailureEvidence{ObservedAt: now, Source: fin.Source, Coverage: "materialized_source_sample_cap10001", Freshness: "unknown"}
		}
		all := adminCount(ctx, tx, now, fin.Source, "all_scoped_materialized_sources", `SELECT workspace_id FROM finops_anomaly_detection_leases WHERE workspace_id=$1`, w)
		adminQualifyCounts(&fin, all, "completed")
		if all.Capped || !all.Available {
			fin.LastSuccessfulJobAt = nil
			fin.Coverage += "; latest_scoped_success_unknown_after_source_cap"
		}
	}
	out.Workers = append(out.Workers, fin)

	for _, family := range []string{"export", "purge"} {
		table := "workspace_export_jobs"
		if family == "purge" {
			table = "workspace_deletion_jobs"
		}
		worker := adminWorker(now, family, table, "durable_job_inventory; failure_occurrence_time_unavailable")
		queries := []adminWorkerQuery{
			{"pending", table + " j", `j.state='pending'`, "j.workspace_id"},
			{"running", table + " j", `j.state='running'`, "j.workspace_id"},
			{"unleased_running", table + " j", `j.state='running' AND j.lease_expires_at IS NULL`, "j.workspace_id"},
			{"live_leases", table + " j", `j.state='running' AND j.lease_expires_at>now()`, "j.workspace_id"},
			{"expired_leases", table + " j", `j.state='running' AND j.lease_expires_at<=now()`, "j.workspace_id"},
			{"failed", table + " j", `j.state='failed'`, "j.workspace_id"},
		}
		due := `j.state IN ('pending','running') AND j.available_at<=now() AND (j.lease_expires_at IS NULL OR j.lease_expires_at<=now())`
		if family == "purge" {
			queries = append(queries, adminWorkerQuery{"blocked", table + " j", `j.state='blocked'`, "j.workspace_id"}, adminWorkerQuery{"cooling", table + " j", `j.state='pending' AND GREATEST(j.earliest_purge_at,j.available_at)>now()`, "j.workspace_id"})
			due = `j.state IN ('pending','running','blocked') AND j.available_at<=now() AND j.earliest_purge_at<=now() AND (j.lease_expires_at IS NULL OR j.lease_expires_at<=now())`
		}
		queries = append(queries, adminWorkerQuery{"due", table + " j", due, "j.workspace_id"})
		adminWorkerCounts(ctx, tx, now, w, &worker, queries)
		adminDueLag(ctx, tx, now, w, &worker, table+" j", due, "j.workspace_id", "j.available_at")
		timeEligible := `state IN ('pending','running') AND available_at<=now()`
		if family == "purge" {
			timeEligible = `state IN ('pending','running','blocked') AND GREATEST(available_at,earliest_purge_at)<=now()`
		}
		adminDueSample(ctx, tx, now, w, &worker, table, timeEligible)
		success := `j.state='completed' AND j.completed_at IS NOT NULL`
		if family == "export" {
			success = `j.state IN ('completed','expired') AND j.completed_at IS NOT NULL`
		}
		adminLatestSuccess(ctx, tx, w, &worker, table+" j", success, "j.workspace_id", "j.completed_at")
		adminLifecycleSummaries(ctx, tx, now, w, &worker, table)
		states := []string{"completed", "cancelled"}
		if family == "export" {
			states = append(states, "expired")
		}
		for _, state := range states {
			adminWorkerCounts(ctx, tx, now, w, &worker, []adminWorkerQuery{{state, table + " j", `j.state='` + state + `'`, "j.workspace_id"}})
		}
		pending := `j.state IN ('pending','running')`
		if family == "purge" {
			pending = `j.state IN ('pending','running','blocked')`
		}
		worker.OldestPending = adminTimeProbe(ctx, tx, now, w, table+" j", pending, "j.workspace_id", "j.created_at", "ASC", "oldest_nonterminal_creation_time; independent_of_due_or_cooling")
		adminFailureProbe(ctx, tx, now, w, &worker, table+" j", `j.failure_code<>''`, "j.workspace_id", "j.failure_code", "j.updated_at", "latest_retained_error_state_update; not_failure_occurrence_time")
		if w > 0 {
			sample := adminCount(ctx, tx, now, table, "bounded_recent_10001_scoped_jobs", `SELECT id FROM `+table+` WHERE workspace_id=$1`, w)
			keys := []string{"completed", "cancelled", "expired"}
			if family == "export" {
				keys = append(keys, "failed")
			}
			adminQualifyCounts(&worker, sample, keys...)
			if sample.Capped || !sample.Available {
				worker.LastFailure = service.AdminFailureEvidence{ObservedAt: now, Source: table, Coverage: "bounded_recent_scoped_source_sample_cap10001", Freshness: "unknown"}
				worker.LastSuccessfulJobAt = nil
				worker.JobsCapped = true
				worker.Coverage += "; recent_history_source_sample_cap10001; last_success_and_last_error_unknown"
			}
		}
		out.Workers = append(out.Workers, worker)
	}
	scim := adminWorker(now, "scim", "workspace_scim_tokens/workspace_scim_connectors", "actionable_inventory; client_sync_is_not_monitor_liveness")
	adminWorkerCounts(ctx, tx, now, w, &scim, []adminWorkerQuery{{"expiring_tokens", "workspace_scim_tokens t", `t.status='active' AND t.expires_at IS NOT NULL AND t.expires_at<=now()+interval '7 days'`, "t.workspace_id"}, {"connector_errors", "workspace_scim_connectors c", `c.failure_count>0`, "c.workspace_id"}})
	scim.Counts["pending"] = adminUnknownCount(now, "scim_monitor", "no_persisted_monitor_queue_or_scan")
	for _, state := range []string{"running", "failed", "completed"} {
		scim.Counts[state] = adminUnknownCount(now, "scim_monitor", "no_persisted_monitor_queue_or_scan")
	}
	adminFailureProbe(ctx, tx, now, w, &scim, "workspace_scim_connectors c", `c.failure_count>0`, "c.workspace_id", "c.last_error_code", "c.updated_at", "retained_connector_error_state_update; not_monitor_failure_or_actual_failure_clock")
	if w > 0 {
		sample := adminCount(ctx, tx, now, "workspace_scim_tokens", "bounded_scoped_token_sources", `SELECT id FROM workspace_scim_tokens WHERE workspace_id=$1`, w)
		adminQualifyCounts(&scim, sample, "expiring_tokens")
		connectors := adminCount(ctx, tx, now, "workspace_scim_connectors", "bounded_recent_10001_scoped_connectors", `SELECT id FROM workspace_scim_connectors WHERE workspace_id=$1`, w)
		adminQualifyCounts(&scim, connectors, "connector_errors")
		if connectors.Capped || !connectors.Available {
			scim.LastFailure = service.AdminFailureEvidence{ObservedAt: now, Source: "workspace_scim_connectors", Coverage: "bounded_recent_scoped_connector_source_sample_cap10001", Freshness: "unknown"}
			scim.Coverage += "; connector_error_source_sample_cap10001; latest_error_unknown_after_cap"
		}
	}
	out.Workers = append(out.Workers, scim)
	billing := adminWorker(now, "billing_recovery", "billing_settlement_alerts", "alert_inventory_only; not_charge_authority_or_complete_recovery_queue")
	billingFrom := `billing_settlement_alerts b JOIN budget_reservations r ON r.id=b.reservation_id`
	adminWorkerCounts(ctx, tx, now, w, &billing, []adminWorkerQuery{{"pending_alerts", billingFrom, `b.state='pending'`, "r.workspace_id"}})
	billing.PendingAlertAge = adminTimeProbe(ctx, tx, now, w, billingFrom, `b.state='pending'`, "r.workspace_id", "b.pending_at", "ASC", "oldest_pending_alert_age; not_worker_due_lag")
	billing.OldestPending = billing.PendingAlertAge
	for _, state := range []string{"running", "failed"} {
		billing.Counts[state] = adminUnknownCount(now, "billing_settlement_alerts", "no_persisted_execution_or_failure_state")
	}
	adminWorkerCounts(ctx, tx, now, w, &billing, []adminWorkerQuery{{"recovered", billingFrom, `b.state='recovered'`, "r.workspace_id"}})
	adminLatestSuccess(ctx, tx, w, &billing, billingFrom, `b.state='recovered' AND b.recovered_at IS NOT NULL`, "r.workspace_id", "b.recovered_at")
	if w > 0 {
		coverage := adminCount(ctx, tx, now, "budget_reservations", "bounded_first_10001_scoped_reservations", `SELECT id FROM budget_reservations WHERE workspace_id=$1`, w)
		adminApplySample(&billing, coverage)
		if coverage.Capped || !coverage.Available {
			billing.PendingAlertAge = adminUnknownTime(now, "budget_reservations", "source_sample_capped_or_unavailable")
		}
	}
	out.Workers = append(out.Workers, billing)
	async := adminWorker(now, "async_settlement", "batch_image_jobs", "SQL_batch_image_inventory_only; Redis_video_inventory_unknown; terminal_job_is_not_settlement_proof")
	adminWorkerCounts(ctx, tx, now, w, &async, []adminWorkerQuery{{"batch_image_pending", "batch_image_jobs j", `(j.status NOT IN ('completed','failed','cancelled','output_deleted') OR j.last_error_code='SUBMIT_OUTCOME_UNKNOWN')`, "j.workspace_id"}})
	adminWorkerCounts(ctx, tx, now, w, &async, []adminWorkerQuery{{"running", "batch_image_jobs j", `j.status IN ('submitted','running','indexing','settling')`, "j.workspace_id"}, {"failed", "batch_image_jobs j", `j.status='failed'`, "j.workspace_id"}, {"completed", "batch_image_jobs j", `j.status='completed'`, "j.workspace_id"}, {"cancelled", "batch_image_jobs j", `j.status='cancelled'`, "j.workspace_id"}, {"output_deleted", "batch_image_jobs j", `j.status='output_deleted'`, "j.workspace_id"}})
	async.OldestPending = adminTimeProbe(ctx, tx, now, w, "batch_image_jobs j", `(j.status NOT IN ('completed','failed','cancelled','output_deleted') OR j.last_error_code='SUBMIT_OUTCOME_UNKNOWN')`, "j.workspace_id", "j.created_at", "ASC", "SQL_image_pending_creation_time; Redis_video_oldest_unknown")
	adminFailureProbe(ctx, tx, now, w, &async, "batch_image_jobs j", `COALESCE(j.last_error_code,'')<>''`, "j.workspace_id", "j.last_error_code", "j.updated_at", "retained_SQL_image_error_state_update; not_settlement_or_video_terminal_failure_time")
	async.Counts["video_pending"] = adminUnknownCount(now, "Redis_video", "no_bounded_cache_adapter_wired")
	async.Counts["pending"] = async.Counts["batch_image_pending"]
	for _, state := range []string{"video_running", "video_failed", "video_completed"} {
		async.Counts[state] = adminUnknownCount(now, "Redis_video", "durable_video_terminal_history_unavailable")
	}
	adminLatestSuccess(ctx, tx, w, &async, "batch_image_jobs j", `j.status='completed' AND j.finished_at IS NOT NULL`, "j.workspace_id", "j.finished_at")
	if w > 0 {
		sample := adminCount(ctx, tx, now, async.Source, "bounded_recent_scoped_image_sources", `SELECT id FROM batch_image_jobs WHERE workspace_id=$1`, w)
		if sample.Capped || !sample.Available {
			async.LastFailure = service.AdminFailureEvidence{ObservedAt: now, Source: async.Source, Coverage: "bounded_recent_source_sample_cap10001", Freshness: "unknown"}
			async.LastSuccessfulJobAt = nil
			async.Coverage += "; latest_scoped_success_and_error_unknown_after_source_cap"
		}
	}
	async.Issues = append(async.Issues, adminIssue("VIDEO_INVENTORY_UNAVAILABLE", "async_settlement", "Redis-only video recovery is unavailable; SQL image counts do not cover video or certify settlement.", now))
	out.Workers = append(out.Workers, async)
	out.ExportRotation = adminRotation(ctx, tx, now, w)
	for _, worker := range out.Workers {
		if worker.State == "blocked" {
			out.State = "blocked"
		} else if worker.State == "degraded" && out.State != "blocked" {
			out.State = "degraded"
		}
	}
	return out
}

func adminLatestSuccess(ctx context.Context, tx *sql.Tx, w int64, worker *service.AdminWorkerDiagnostics, from, predicate, scope, clock string) {
	q, args := adminScopedSQL(clock, from, predicate, scope, w)
	var at *time.Time
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT max(success_at) FROM (`+q+` ORDER BY `+clock+` DESC LIMIT 1) last_job(success_at)`, args...).Scan(&at)
	})
	if e == nil && at != nil && !at.After(worker.ObservedAt) {
		worker.LastSuccessfulJobAt = at
	} else if e != nil {
		worker.Issues = append(worker.Issues, adminIssue("SUCCESS_EVIDENCE_UNAVAILABLE", worker.Worker, "Last retained successful job evidence is unavailable.", worker.ObservedAt))
	}
}
func adminApplySample(worker *service.AdminWorkerDiagnostics, sample service.AdminCount) {
	if !sample.Available || sample.Capped {
		worker.Coverage += "; source_sample_capped_or_unavailable; matching-empty-is-unknown"
		worker.OldestPending = adminUnknownTime(sample.ObservedAt, sample.Source, "source_sample_capped_or_unavailable")
		worker.DueLagSeconds = nil
		worker.LastSuccessfulJobAt = nil
		worker.LastFailure = service.AdminFailureEvidence{ObservedAt: sample.ObservedAt, Source: sample.Source, Coverage: "source_sample_capped_or_unavailable", Freshness: "unknown"}
		for k, c := range worker.Counts {
			c.Capped = sample.Capped
			c.Coverage += "; source_sample_cap10001"
			if !sample.Available {
				c.Available = false
				c.Value = nil
				c.Freshness = "unknown"
			}
			worker.Counts[k] = c
		}
		worker.Issues = append(worker.Issues, adminIssue("SOURCE_SAMPLE_CAPPED_OR_UNAVAILABLE", worker.Worker, "A bounded source sample cannot certify an empty inventory or the globally oldest item.", sample.ObservedAt))
	}
}

// A source sample qualifies only the selected count fields. Matching rows are
// lower bounds; empty results in a capped source are not exact zeroes.
func adminQualifyCounts(worker *service.AdminWorkerDiagnostics, sample service.AdminCount, states ...string) {
	for _, state := range states {
		c, ok := worker.Counts[state]
		if !ok {
			continue
		}
		c.Capped = sample.Capped
		c.Coverage += "; source_sample_cap10001"
		if !sample.Available {
			c.Available = false
			c.Value = nil
			c.Freshness = "unknown"
		}
		worker.Counts[state] = c
	}
}
func adminFailureProbe(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics, from, predicate, scope, codeCol, timeCol, coverage string) {
	q, args := adminScopedSQL("COALESCE("+codeCol+",''),"+timeCol, from, predicate, scope, w)
	var code string
	var at *time.Time
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, q+` ORDER BY `+timeCol+` DESC LIMIT 1`, args...).Scan(&code, &at)
	})
	worker.LastFailure = service.AdminFailureEvidence{ObservedAt: now, Source: worker.Source, Coverage: coverage, Freshness: "unknown"}
	if e == sql.ErrNoRows {
		worker.LastFailure.Available = true
		worker.LastFailure.Freshness = "fresh"
		return
	}
	if e != nil || code == "" {
		return
	}
	safe := adminFamilyFailureCode(worker.Worker, code)
	worker.LastFailure = service.AdminFailureEvidence{Code: &safe, SourceTime: at, Available: true, ObservedAt: now, Source: worker.Source, Coverage: coverage, Freshness: service.AdminEvidenceFreshness(at, now, 24*time.Hour)}
	if worker.LastFailure.Freshness == "fresh" && worker.State != "blocked" {
		worker.State = "degraded"
	}
}
func adminLatestWebhookFailure(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics) {
	q, args := adminScopedSQL("d.finished_at,COALESCE(d.error_code,'')", `workspace_webhook_deliveries d`, `d.status='dead' AND d.finished_at IS NOT NULL`, "d.workspace_id", w)
	var at *time.Time
	var code string
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, q+` ORDER BY d.finished_at DESC,d.id DESC LIMIT 1`, args...).Scan(&at, &code)
	})
	worker.LastFailedAt = nil
	worker.LatestFailureTime = adminUnknownTime(now, "workspace_webhook_deliveries.finished_at", "independent_scoped_latest_terminal_failure")
	worker.LastFailure = service.AdminFailureEvidence{ObservedAt: now, Source: "workspace_webhook_deliveries.error_code", Coverage: "independent_scoped_latest_terminal_failure", Freshness: "unknown"}
	if e == sql.ErrNoRows {
		worker.LatestFailureTime.Available = true
		worker.LatestFailureTime.Freshness = "fresh"
		worker.LastFailure.Available = true
		worker.LastFailure.Freshness = "fresh"
		return
	}
	if e != nil || at == nil || at.After(now) {
		return
	}
	worker.LatestFailureTime.Value = at
	worker.LatestFailureTime.Available = true
	worker.LatestFailureTime.AgeSeconds = adminInt64Ptr(int64(now.Sub(*at).Seconds()))
	worker.LatestFailureTime.Freshness = "fresh"
	worker.LastFailedAt = at
	safe := adminFamilyFailureCode("webhook", code)
	if safe != "" {
		worker.LastFailure.Code = &safe
	}
	worker.LastFailure.Available = true
	worker.LastFailure.SourceTime = at
	worker.LastFailure.Freshness = service.AdminEvidenceFreshness(at, now, 24*time.Hour)
}
func adminInt64Ptr(v int64) *int64 { return &v }
func adminFinOpsStatus(ctx context.Context, tx *sql.Tx, now time.Time, worker *service.AdminWorkerDiagnostics) {
	var scan, updated *time.Time
	var failureCode string
	var lag int64
	e := adminSection(ctx, tx, func() error {
		return tx.QueryRowContext(ctx, `SELECT last_successful_scan,updated_at,lag_seconds,COALESCE(last_failure_code,'') FROM finops_anomaly_detector_status WHERE id=1`).Scan(&scan, &updated, &lag, &failureCode)
	})
	if e != nil {
		worker.Freshness = "unknown"
		worker.Issues = append(worker.Issues, adminIssue("SCAN_EVIDENCE_UNAVAILABLE", "finops", "Detector scan observation is unavailable.", now))
		return
	}
	worker.Freshness = service.AdminEvidenceFreshness(updated, now, 5*time.Minute)
	scanFresh := service.AdminEvidenceFreshness(scan, now, 5*time.Minute)
	if scanFresh == "unknown" {
		worker.Freshness = "unknown"
	} else if scanFresh == "stale" && worker.Freshness != "unknown" {
		worker.Freshness = "stale"
	}
	if scan != nil && !scan.After(now) {
		worker.LastSuccessfulScanAt = scan
	}
	if worker.Freshness == "fresh" {
		worker.StoredScanLagSeconds = &lag
	}
	if failureCode != "" {
		safe := adminFamilyFailureCode("finops", failureCode)
		worker.LastFailure = service.AdminFailureEvidence{Code: &safe, SourceTime: updated, Available: true, ObservedAt: now, Source: "finops_anomaly_detector_status", Coverage: "retained_scan_failure_code_may_survive_success; updated_at_is_not_failure_time", Freshness: worker.Freshness}
	}
	if worker.Freshness != "fresh" {
		worker.Issues = append(worker.Issues, adminIssue("SCAN_EVIDENCE_STALE_OR_UNKNOWN", "finops", "Detector observations are old, missing or future; stored lag is unavailable.", now))
	}
}

func adminLifecycleSummaries(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics, table string) {
	predicate := `j.state IN ('pending','running','failed')`
	if worker.Worker == "purge" {
		predicate = `j.state IN ('pending','running','failed','blocked')`
	}
	columns := `j.id::text,j.workspace_id,j.state,j.created_at,j.completed_at,j.failure_code,j.updated_at,j.progress`
	if worker.Worker == "purge" {
		columns += `,j.phase,COALESCE((SELECT jsonb_agg(COALESCE(b.value->>'code','')) FROM jsonb_array_elements(j.blocking_reasons) WITH ORDINALITY b(value,n) WHERE b.n<=32),'[]'::jsonb)`
	}
	q, args := adminScopedSQL(columns, table+" j", predicate, "j.workspace_id", w)
	e := adminSection(ctx, tx, func() error {
		rows, e := tx.QueryContext(ctx, q+` ORDER BY j.created_at DESC,j.id DESC LIMIT 21`, args...)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v service.AdminJobSummary
			var raw string
			var at time.Time
			var progress int64
			var phase string
			var blockers []byte
			values := []any{&v.ID, &v.WorkspaceID, &v.State, &v.CreatedAt, &v.FinishedAt, &raw, &at, &progress}
			if worker.Worker == "purge" {
				values = append(values, &phase, &blockers)
			}
			if e = rows.Scan(values...); e != nil {
				return e
			}
			v.FailureCode = adminFamilyFailureCode(worker.Worker, raw)
			v.ObservedAt = now
			v.SourceTime = &at
			v.Freshness = service.AdminEvidenceFreshness(&at, now, 5*time.Minute)
			v.Progress = &progress
			if worker.Worker == "purge" {
				v.Phase = "unknown"
				for _, step := range lifecyclePurgeSteps {
					if phase == step.phase {
						v.Phase = phase
						break
					}
				}
				v.Blockers = adminStoredBlockers(blockers, now, at)
			}
			worker.Jobs = append(worker.Jobs, v)
		}
		return rows.Err()
	})
	if e != nil {
		worker.Jobs = []service.AdminJobSummary{}
		worker.Issues = append(worker.Issues, adminIssue("JOB_SUMMARIES_UNAVAILABLE", worker.Worker, "Bounded job summaries are unavailable.", now))
	}
	if len(worker.Jobs) > 20 {
		worker.JobsCapped = true
		worker.Jobs = worker.Jobs[:20]
	}
}
func adminWebhookSummaries(ctx context.Context, tx *sql.Tx, now time.Time, w int64, worker *service.AdminWorkerDiagnostics) {
	predicate := `d.status='dead' AND d.finished_at IS NOT NULL`
	order := `d.finished_at DESC,d.id DESC`
	if w > 0 {
		predicate = "TRUE"
		order = `d.created_at DESC,d.id DESC`
	}
	q, args := adminScopedSQL(`d.id,d.workspace_id,d.webhook_id,d.status,d.attempts,d.last_attempt_at,h.enabled,d.created_at,d.finished_at,COALESCE(d.error_code,''),(d.lock_owner IS NULL AND d.locked_at IS NULL),ws.status='active', (SELECT count(*) FROM (SELECT 1 FROM enterprise_admin_operations o WHERE o.action='retry_webhook' AND o.target_type='webhook_delivery' AND o.workspace_id=d.workspace_id AND o.target_id=d.id LIMIT 3) bounded_retry)`, `workspace_webhook_deliveries d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id JOIN workspaces ws ON ws.id=d.workspace_id`, predicate, "d.workspace_id", w)
	e := adminSection(ctx, tx, func() error {
		rows, e := tx.QueryContext(ctx, q+` ORDER BY `+order+` LIMIT 21`, args...)
		if e != nil {
			return e
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var v service.AdminJobSummary
			var id, webhook, retries int64
			var attempts int
			var enabled, unclaimed, active bool
			var raw string
			if e = rows.Scan(&id, &v.WorkspaceID, &webhook, &v.State, &attempts, &v.LastAttemptAt, &enabled, &v.CreatedAt, &v.FinishedAt, &raw, &unclaimed, &active, &retries); e != nil {
				return e
			}
			v.ID = strconv.FormatInt(id, 10)
			v.DeliveryID = &id
			v.WebhookID = &webhook
			v.Attempts = &attempts
			v.EndpointEnabled = &enabled
			v.FailureCode = adminFamilyFailureCode("webhook", raw)
			v.ObservedAt = now
			v.Freshness = "fresh"
			v.RetryEligibility = "blocked"
			if v.State == "dead" && enabled && unclaimed && active && retries < 3 {
				v.RetryEligibility = "eligible_requires_guarded_action"
			}
			if v.State == "dead" && v.FinishedAt != nil && !v.FinishedAt.After(now) {
				v.FailedAt = v.FinishedAt
			}
			worker.Jobs = append(worker.Jobs, v)
		}
		return rows.Err()
	})
	if e != nil {
		worker.Jobs = []service.AdminJobSummary{}
		worker.LastFailedAt = nil
		worker.Issues = append(worker.Issues, adminIssue("JOB_SUMMARIES_UNAVAILABLE", "webhook", "Bounded delivery summaries are unavailable.", now))
	}
	if len(worker.Jobs) > 20 {
		worker.JobsCapped = true
		worker.Jobs = worker.Jobs[:20]
	}
}
func adminWebhookTrend(ctx context.Context, tx *sql.Tx, now time.Time, w int64, out *service.AdminJobsDiagnostics) {
	cutoff := now.UTC().Add(-24 * time.Hour)
	start := cutoff.Truncate(time.Hour)
	for i := 0; i < 25; i++ {
		hour := start.Add(time.Duration(i) * time.Hour)
		end := hour.Add(time.Hour)
		from := hour
		if from.Before(cutoff) {
			from = cutoff
		}
		if end.After(now) {
			end = now
		}
		q, _ := adminScopedSQL("d.id", "workspace_webhook_deliveries d", `d.status='dead' AND d.finished_at >= $1 AND d.finished_at < $2`, "d.workspace_id", 0)
		args := []any{from, end}
		if w > 0 {
			q += ` AND d.workspace_id=$3`
			args = append(args, w)
		}
		c := adminCount(ctx, tx, now, "workspace_webhook_deliveries.finished_at", "terminal_failure_hour", q, args...)
		out.WebhookFailureTrend = append(out.WebhookFailureTrend, service.AdminFailureBucket{Hour: hour, Count: c})
		if !c.Available {
			out.Issues = append(out.Issues, adminIssue("FAILURE_TREND_UNAVAILABLE", "webhook", "An hourly terminal-failure observation is unavailable.", now))
		}
	}
}
func adminRotation(ctx context.Context, tx *sql.Tx, now time.Time, w int64) service.AdminExportRotation {
	out := service.AdminExportRotation{State: "blocked", ObservedAt: now, Format: "MRLEX01", SingleKeyNoID: true, Capabilities: service.AdminInstanceCapabilities{Scope: "current_instance"}, Counts: map[string]service.AdminCount{}, Coverage: "partial_database_evidence_only; excludes_storage_inventory_orphans_key_custody_all_instance_consistency_and_restore_drills", Prerequisites: []string{"Phase I: approved drain or key-ring design, key custody/versioning, rollout and rollback plan.", "Phase L: private-storage restore and old/new-key readability drills including holds, orphans, interruption and lost/mismatched keys."}, Issues: []service.AdminIssue{adminIssue("SINGLE_KEY_NO_KEY_ID", "export_rotation", "Replacing the single loaded key can make retained or held ciphertext unreadable; empty jobs do not certify rotation.", now)}, Runbook: adminRunbook}
	for _, q := range []struct{ name, source, from, predicate, scope string }{{"pending_jobs", "workspace_export_jobs", "workspace_export_jobs j", `j.state IN ('pending','running')`, "j.workspace_id"}, {"object_ledger", "workspace_export_objects", "workspace_export_objects o", "TRUE", "o.workspace_id"}, {"indefinite_objects", "workspace_export_objects", "workspace_export_objects o", `o.cleanup_after IS NULL`, "o.workspace_id"}, {"holds", "workspace_lifecycle_holds", "workspace_lifecycle_holds h", `h.active`, "h.workspace_id"}} {
		sql, args := adminScopedSQL("1", q.from, q.predicate, q.scope, w)
		out.Counts[q.name] = adminCount(ctx, tx, now, q.source, "partial_database_inventory", sql, args...)
	}
	return out
}
