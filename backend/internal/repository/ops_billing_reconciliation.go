package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Bounded to protect the primary database. An incomplete scan explicitly
// produces Unknown; it cannot close alerts or claim that billing is healthy.
const opsBillingReconciliationLimit = 10000

const reconcileBillingUsageSQL = `
WITH fresh AS MATERIALIZED (
    SELECT d.id, d.created_at FROM usage_billing_dedup d
    WHERE d.expected_usage_cost IS NOT NULL AND (d.created_at, d.id) > ($1, $2)
      AND d.created_at < $3
    ORDER BY d.created_at, d.id LIMIT $4
), pending AS MATERIALIZED (
    SELECT settlement_id AS id FROM ops_billing_reconciliation_findings
    WHERE resolved_at IS NULL ORDER BY last_seen_at, id LIMIT $4
), selected AS MATERIALIZED (
    SELECT id FROM fresh ORDER BY created_at, id LIMIT $5
), evidence AS MATERIALIZED (
    SELECT d.id, d.request_id, d.expected_usage_cost, u.id AS usage_id,
           ROUND(u.actual_cost, 8) AS actual_cost
    FROM usage_billing_dedup d
    LEFT JOIN usage_logs u ON u.request_id = d.request_id AND u.api_key_id = d.api_key_id
    WHERE d.id IN (SELECT id FROM selected UNION SELECT id FROM pending)
), violations AS (
    INSERT INTO ops_billing_reconciliation_findings
        (settlement_id, request_id, kind, expected_cost, actual_cost, first_seen_at, last_seen_at)
    SELECT id, request_id, CASE WHEN usage_id IS NULL THEN 'missing_usage_log' ELSE 'amount_mismatch' END,
           expected_usage_cost, actual_cost, $6, $6
    FROM evidence WHERE usage_id IS NULL OR actual_cost IS DISTINCT FROM expected_usage_cost
    ON CONFLICT (settlement_id) DO UPDATE SET
        kind = EXCLUDED.kind, actual_cost = EXCLUDED.actual_cost,
        last_seen_at = EXCLUDED.last_seen_at, resolved_at = NULL
    RETURNING id
), recovered AS (
    UPDATE ops_billing_reconciliation_findings f SET resolved_at = $6, last_seen_at = $6
    FROM evidence e WHERE f.settlement_id = e.id AND f.resolved_at IS NULL
        AND e.usage_id IS NOT NULL AND e.actual_cost = e.expected_usage_cost
    RETURNING f.id
)
SELECT (SELECT COUNT(*) FROM fresh), (SELECT COUNT(*) FROM pending),
       (SELECT created_at FROM fresh WHERE id IN (SELECT id FROM selected) ORDER BY created_at DESC, id DESC LIMIT 1),
       (SELECT id FROM fresh WHERE id IN (SELECT id FROM selected) ORDER BY created_at DESC, id DESC LIMIT 1)`

func (r *opsRepository) ReconcileBillingUsage(ctx context.Context, now time.Time) error {
	r.FlushBillingRecordingGaps(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize the checkpoint even when a maintenance call overlaps the leader.
	var locked bool
	if err = tx.QueryRowContext(ctx, "SELECT pg_try_advisory_xact_lock(273002)").Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	end := now.Add(-2 * time.Minute)
	start := end.Add(-24 * time.Hour)
	cursorAt, cursorID := start, int64(0)
	var savedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT window_start, cursor_created_at, cursor_id
        FROM ops_billing_reconciliation_snapshot WHERE id = 1`).Scan(&start, &savedAt, &cursorID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if savedAt.Valid {
		cursorAt = savedAt.Time
	}
	var scanned, pending int64
	var lastAt sql.NullTime
	var lastID sql.NullInt64
	if err = tx.QueryRowContext(ctx, reconcileBillingUsageSQL, cursorAt, cursorID, end,
		opsBillingReconciliationLimit+1, opsBillingReconciliationLimit, now).
		Scan(&scanned, &pending, &lastAt, &lastID); err != nil {
		return err
	}
	truncated := scanned > opsBillingReconciliationLimit || pending > opsBillingReconciliationLimit
	if lastAt.Valid && lastID.Valid {
		cursorAt, cursorID = lastAt.Time, lastID.Int64
	}
	if scanned <= opsBillingReconciliationLimit {
		// Revisit the tail after catching up to include delayed transaction commits.
		cursorAt, cursorID = end.Add(-time.Minute), 0
	}
	// Read in a second statement so PostgreSQL sees the CTE writes. Open issues
	// do not silently recover merely because their settlement aged out of a window.
	var missing, mismatched int64
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE kind = 'missing_usage_log'),
        COUNT(*) FILTER (WHERE kind = 'amount_mismatch')
        FROM ops_billing_reconciliation_findings WHERE resolved_at IS NULL`).Scan(&missing, &mismatched); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ops_billing_reconciliation_snapshot
        (id, checked_at, window_start, window_end, missing_usage_logs, amount_mismatches, scanned, truncated, cursor_created_at, cursor_id)
        VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (id) DO UPDATE SET checked_at = EXCLUDED.checked_at,
        window_start = EXCLUDED.window_start, window_end = EXCLUDED.window_end,
        missing_usage_logs = EXCLUDED.missing_usage_logs, amount_mismatches = EXCLUDED.amount_mismatches,
        scanned = EXCLUDED.scanned, truncated = EXCLUDED.truncated,
        cursor_created_at = EXCLUDED.cursor_created_at, cursor_id = EXCLUDED.cursor_id`,
		now, start, end, missing, mismatched, scanned, truncated, cursorAt, cursorID)
	if err != nil {
		return err
	}
	if err = r.reconcileBillingOperations(ctx, tx, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *opsRepository) GetBillingReconciliation(ctx context.Context) (*service.OpsBillingReconciliation, error) {
	result := &service.OpsBillingReconciliation{State: "Unknown", Findings: []service.OpsBillingFinding{}}
	err := r.db.QueryRowContext(ctx, `SELECT checked_at, window_start, window_end, missing_usage_logs,
        amount_mismatches, scanned, truncated FROM ops_billing_reconciliation_snapshot WHERE id = 1`).
		Scan(&result.CheckedAt, &result.WindowStart, &result.WindowEnd, &result.MissingUsageLogs,
			&result.AmountMismatches, &result.Scanned, &result.Truncated)
	if err == sql.ErrNoRows {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, settlement_id, request_id, kind, expected_cost::text,
        actual_cost::text, first_seen_at, last_seen_at, resolved_at
        FROM ops_billing_reconciliation_findings ORDER BY last_seen_at DESC, id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var finding service.OpsBillingFinding
		if err = rows.Scan(&finding.ID, &finding.SettlementID, &finding.RequestID, &finding.Kind,
			&finding.ExpectedCost, &finding.ActualCost, &finding.FirstSeenAt, &finding.LastSeenAt, &finding.ResolvedAt); err != nil {
			return nil, err
		}
		result.Findings = append(result.Findings, finding)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if err = r.readBillingOperations(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}
