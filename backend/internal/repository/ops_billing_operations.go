package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/billingobserve"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

const billingObservationBackoff = 30 * time.Second

func billingObservationTimeout() time.Duration {
	n, err := strconv.Atoi(os.Getenv("OPS_BILLING_OBSERVATION_TIMEOUT_MS"))
	if err != nil || n <= 0 {
		n = 50
	}
	if n > 200 {
		n = 200
	}
	return time.Duration(n) * time.Millisecond
}
func (r *usageBillingRepository) missedBillingObservation() {
	billingobserve.Failed(r.observationInstance)
}

// The separate transaction survives a later settlement rollback. Recording is
// bounded and fail-open: observation failure never changes the billing result.
func (r *usageBillingRepository) beginBillingObservation(requestID string, apiKeyID int64, fingerprint, operation string, userID int64, payload any) int64 {
	if !r.observeBilling {
		return 0
	}
	r.observationIdentity.Do(func() { r.observationInstance = uuid.NewString() })
	if time.Now().UnixNano() < r.observationRetryAfter.Load() {
		r.missedBillingObservation()
		return 0
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		r.missedBillingObservation()
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), billingObservationTimeout())
	defer cancel()
	var id int64
	gap := billingobserve.Snapshot(r.observationInstance)
	err = r.db.QueryRowContext(ctx, `WITH gap AS (
 INSERT INTO ops_billing_recording_gaps(instance_id,missed_count,first_missed_at,last_missed_at)
 SELECT $7,$8,$9,$10 WHERE $8::bigint>0
 ON CONFLICT(instance_id) DO UPDATE SET missed_count=GREATEST(ops_billing_recording_gaps.missed_count,EXCLUDED.missed_count),last_missed_at=GREATEST(ops_billing_recording_gaps.last_missed_at,EXCLUDED.last_missed_at)
), intent AS (
 INSERT INTO ops_billing_intents(request_id,api_key_id,request_fingerprint,operation,user_id,payload)
 VALUES($1,$2,$3,$4,$5,$6::jsonb)
 ON CONFLICT(request_id,api_key_id,operation,request_fingerprint) DO UPDATE SET updated_at=NOW()
 RETURNING id
), task AS (
 INSERT INTO ops_billing_compensation_outbox(intent_id) SELECT id FROM intent
 ON CONFLICT(intent_id) DO NOTHING
) SELECT id FROM intent`, requestID, apiKeyID, fingerprint, operation, userID, string(raw), r.observationInstance, gap.MissedCount, gap.FirstMissedAt, gap.LastMissedAt).Scan(&id)
	if err != nil {
		r.missedBillingObservation()
		r.observationRetryAfter.Store(time.Now().Add(billingObservationBackoff).UnixNano())
		slog.Warn("billing observation unavailable", "operation", operation, "instance_id", r.observationInstance, "missed_count", billingobserve.Snapshot(r.observationInstance).MissedCount)
		return 0
	}
	billingobserve.Persisted(r.observationInstance, gap.MissedCount)
	return id
}

func (r *usageBillingRepository) finishBillingObservation(id int64, billingErr error) {
	if id == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), billingObservationTimeout())
	defer cancel()
	state, code := "committed", ""
	if billingErr != nil {
		state, code = "failed", "settlement_failed"
	}
	// A successful replay wins over an earlier concurrent failure. Actual commit
	// proof is still rechecked against the immutable dedup table by the worker.
	_, err := r.db.ExecContext(ctx, `WITH changed AS (
 UPDATE ops_billing_intents SET state=$2,error_code=$3,updated_at=NOW()
 WHERE id=$1 AND (state<>'committed' OR $2='committed') RETURNING id,state
), task AS (
 UPDATE ops_billing_compensation_outbox o SET
 state=CASE WHEN c.state='committed' THEN 'resolved' ELSE 'manual_required' END,
 resolved_at=CASE WHEN c.state='committed' THEN NOW() ELSE NULL END,
 checked_at=NOW()
 FROM changed c WHERE o.intent_id=c.id RETURNING o.id,o.state
) INSERT INTO ops_billing_operation_audit(outbox_id,action) SELECT id,'settlement_'||state FROM task`, id, state, code)
	if err != nil {
		r.missedBillingObservation()
		r.observationRetryAfter.Store(time.Now().Add(billingObservationBackoff).UnixNano())
		slog.Warn("billing observation outcome unavailable", "instance_id", r.observationInstance, "missed_count", billingobserve.Snapshot(r.observationInstance).MissedCount)
	}
}

func (r *opsRepository) AcknowledgeBillingRecordingGap(ctx context.Context, instanceID string, actorID int64, note string) error {
	if len(instanceID) > 100 || instanceID == "" || actorID <= 0 || strings.TrimSpace(note) == "" || utf8.RuneCountInString(note) > 1000 {
		return errors.New("an administrator and review note are required")
	}
	result, err := r.db.ExecContext(ctx, `WITH reviewed AS (
 UPDATE ops_billing_recording_gaps SET acknowledged_count=missed_count
 WHERE instance_id=$1 AND acknowledged_count<missed_count RETURNING instance_id,acknowledged_count
) INSERT INTO ops_billing_recording_gap_audit(instance_id,actor_id,acknowledged_count,note)
 SELECT instance_id,$2,acknowledged_count,$3 FROM reviewed`, instanceID, actorID, note)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// The marker is only correlation metadata. A savepoint prevents a monitoring
// statement failure from poisoning the pre-existing billing transaction.
func bindBillingObservation(ctx context.Context, tx *sql.Tx, id int64) {
	if id == 0 {
		return
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT billing_observation"); err != nil {
		return
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('modurelay.billing_intent',$1,true)`, strconv.FormatInt(id, 10)); err != nil {
		_, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT billing_observation")
	}
	_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT billing_observation")
}

// Reconciliation retries only inspect existing durable evidence. They never
// execute the stored billing command or automatically debit an account.
func (r *opsRepository) RetryBillingOperation(ctx context.Context, id, actorID int64, note string) error {
	if id <= 0 || actorID <= 0 || utf8.RuneCountInString(note) > 1000 {
		return errors.New("invalid billing reconciliation retry")
	}
	result, err := r.db.ExecContext(ctx, `WITH task AS (
 UPDATE ops_billing_compensation_outbox SET state='queued',next_check_at=NOW(),resolved_at=NULL
 WHERE id=$1 AND state='manual_required' RETURNING id
) INSERT INTO ops_billing_operation_audit(outbox_id,actor_id,action,note)
 SELECT id,$2,'recheck_requested',$3 FROM task`, id, actorID, note)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *opsRepository) reconcileBillingOperations(ctx context.Context, tx *sql.Tx, now time.Time) error {
	var available bool
	if err := tx.QueryRowContext(ctx, `SELECT to_regclass('ops_billing_intents') IS NOT NULL AND to_regclass('ops_account_ledger_heads') IS NOT NULL`).Scan(&available); err != nil {
		return err
	}
	if !available {
		return nil
	}
	_, err := tx.ExecContext(ctx, `WITH candidates AS MATERIALIZED (
 SELECT o.id,o.intent_id,o.state,
 EXISTS(SELECT 1 FROM usage_billing_dedup d WHERE d.request_id=i.request_id AND d.api_key_id=i.api_key_id AND d.request_fingerprint=i.request_fingerprint
 UNION ALL SELECT 1 FROM usage_billing_dedup_archive d WHERE d.request_id=i.request_id AND d.api_key_id=i.api_key_id AND d.request_fingerprint=i.request_fingerprint) AS committed
 FROM ops_billing_compensation_outbox o JOIN ops_billing_intents i ON i.id=o.intent_id
 WHERE o.state<>'resolved' AND o.next_check_at<=$1
 ORDER BY o.next_check_at,o.id LIMIT 1000
), intents AS (
 UPDATE ops_billing_intents i SET state=CASE WHEN c.committed OR i.state='committed' THEN 'committed' ELSE 'failed' END,
 error_code=CASE WHEN c.committed OR i.state='committed' THEN '' ELSE 'settlement_not_committed' END,updated_at=$1
 FROM candidates c WHERE i.id=c.intent_id RETURNING i.id,i.state
), tasks AS (
 UPDATE ops_billing_compensation_outbox o SET state=CASE WHEN i.state='committed' THEN 'resolved' ELSE 'manual_required' END,
 attempts=o.attempts+1,checked_at=$1,next_check_at=$1+INTERVAL '5 minutes',
 resolved_at=CASE WHEN i.state='committed' THEN $1 ELSE NULL END
 FROM candidates c JOIN intents i ON i.id=c.intent_id WHERE o.id=c.id RETURNING o.id,o.state
) INSERT INTO ops_billing_operation_audit(outbox_id,action)
 SELECT t.id,'evidence_'||t.state FROM tasks t JOIN candidates c ON c.id=t.id WHERE c.state<>t.state`, now)
	if err != nil {
		return err
	}
	// A fixed source-row projection is independent of observer-table writes.
	if err = reconcileAccountProjections(ctx, tx, now); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ops_billing_operational_snapshot(id,checked_at,failed_settlements,compensation_backlog,ledger_mismatches,coverage_incomplete)
 SELECT 1,$1,(SELECT COUNT(*) FROM ops_billing_intents WHERE state='failed'),
 (SELECT COUNT(*) FROM ops_billing_compensation_outbox WHERE state<>'resolved'),
 (SELECT COUNT(*) FROM ops_account_ledger_heads WHERE mismatch),
 EXISTS(SELECT 1 FROM ops_account_ledger_heads WHERE coverage_incomplete OR checked_at IS NULL OR checked_at<$1::timestamptz-INTERVAL '1 hour')
 OR EXISTS(SELECT 1 FROM ops_billing_compensation_outbox WHERE state='queued' AND next_check_at<=$1)
 OR EXISTS(SELECT 1 FROM ops_billing_recording_gaps WHERE missed_count>acknowledged_count)
 OR EXISTS(SELECT 1 FROM ops_account_ledger_heads h LEFT JOIN users u ON h.account_type='user' AND h.account_id=u.id LEFT JOIN user_subscriptions s ON h.account_type='subscription' AND h.account_id=s.id
 WHERE COALESCE(u.ops_ledger_revision,s.ops_ledger_revision,h.source_revision)>h.source_revision)
 OR EXISTS(SELECT 1 FROM users u WHERE (SELECT COUNT(*) FROM ops_account_ledger_heads h WHERE h.account_type='user' AND h.account_id=u.id)<2)
 OR EXISTS(SELECT 1 FROM user_subscriptions s WHERE (SELECT COUNT(*) FROM ops_account_ledger_heads h WHERE h.account_type='subscription' AND h.account_id=s.id)<3)
 ON CONFLICT(id) DO UPDATE SET checked_at=EXCLUDED.checked_at,failed_settlements=EXCLUDED.failed_settlements,
 compensation_backlog=EXCLUDED.compensation_backlog,ledger_mismatches=EXCLUDED.ledger_mismatches,coverage_incomplete=EXCLUDED.coverage_incomplete`, now)
	return err
}

func (r *opsRepository) readBillingOperations(ctx context.Context, result *service.OpsBillingReconciliation) error {
	result.OperationalState = "Unknown"
	result.Operations = []service.OpsBillingOperation{}
	result.LedgerFindings = []service.OpsAccountLedgerFinding{}
	result.RecordingGaps = []service.OpsBillingRecordingGap{}
	result.ObservationUnpersisted = billingobserve.Pending()
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT to_regclass('ops_billing_operational_snapshot') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	var incomplete bool
	err := r.db.QueryRowContext(ctx, `SELECT checked_at,failed_settlements,compensation_backlog,ledger_mismatches,coverage_incomplete FROM ops_billing_operational_snapshot WHERE id=1`).Scan(&result.OperationalCheckedAt, &result.FailedSettlements, &result.CompensationBacklog, &result.LedgerMismatches, &incomplete)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !incomplete && result.ObservationUnpersisted == 0 {
		result.OperationalState = "Available"
	}
	rows, err := r.db.QueryContext(ctx, `SELECT o.id,i.id,i.request_id,i.operation,o.state,i.error_code,o.attempts,i.created_at,o.checked_at
 FROM ops_billing_compensation_outbox o JOIN ops_billing_intents i ON i.id=o.intent_id
 ORDER BY (o.state='resolved'),i.updated_at DESC,o.id DESC LIMIT 100`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.OpsBillingOperation
		if err = rows.Scan(&item.ID, &item.IntentID, &item.RequestID, &item.Operation, &item.State, &item.ErrorCode, &item.Attempts, &item.CreatedAt, &item.CheckedAt); err != nil {
			rows.Close()
			return err
		}
		result.Operations = append(result.Operations, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT account_type,account_id,field,opening_amount::text,expected_amount::text,actual_amount::text FROM ops_account_ledger_heads WHERE mismatch ORDER BY account_type,account_id,field LIMIT 100`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item service.OpsAccountLedgerFinding
		if err = rows.Scan(&item.AccountType, &item.AccountID, &item.Field, &item.OpeningAmount, &item.ExpectedAmount, &item.ActualAmount); err != nil {
			rows.Close()
			return err
		}
		result.LedgerFindings = append(result.LedgerFindings, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT instance_id,missed_count,acknowledged_count,first_missed_at,last_missed_at FROM ops_billing_recording_gaps ORDER BY (missed_count>acknowledged_count) DESC,last_missed_at DESC LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var gap service.OpsBillingRecordingGap
		if err = rows.Scan(&gap.InstanceID, &gap.MissedCount, &gap.AcknowledgedCount, &gap.FirstMissedAt, &gap.LastMissedAt); err != nil {
			return err
		}
		result.RecordingGaps = append(result.RecordingGaps, gap)
		if gap.MissedCount > gap.AcknowledgedCount {
			result.OperationalState = "Unknown"
		}
	}
	return rows.Err()
}

// Flush from the periodic reconciler as well as from a recovered billing path;
// an idle process must not wait for another charge to persist its outage.
func (r *opsRepository) FlushBillingRecordingGaps(ctx context.Context) {
	gaps := billingobserve.PendingGaps()
	if len(gaps) == 0 {
		return
	}
	raw, err := json.Marshal(gaps)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, billingObservationTimeout())
	defer cancel()
	_, err = r.db.ExecContext(ctx, `INSERT INTO ops_billing_recording_gaps(instance_id,missed_count,first_missed_at,last_missed_at)
 SELECT instance_id,missed_count,first_missed_at,last_missed_at FROM jsonb_to_recordset($1::jsonb) AS x(instance_id text,missed_count bigint,first_missed_at timestamptz,last_missed_at timestamptz)
 ON CONFLICT(instance_id) DO UPDATE SET missed_count=GREATEST(ops_billing_recording_gaps.missed_count,EXCLUDED.missed_count),last_missed_at=GREATEST(ops_billing_recording_gaps.last_missed_at,EXCLUDED.last_missed_at)`, string(raw))
	if err == nil {
		for _, gap := range gaps {
			billingobserve.Persisted(gap.InstanceID, gap.MissedCount)
		}
	}
}
