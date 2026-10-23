package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func monitoringFinancialTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := monitoringTestDB(t)
	_, err := db.Exec(`CREATE TABLE users(id BIGINT PRIMARY KEY,balance NUMERIC(20,8) NOT NULL DEFAULT 0,frozen_balance NUMERIC(20,8) NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ);
 CREATE TABLE user_subscriptions(id BIGINT PRIMARY KEY,daily_usage_usd NUMERIC(20,10) NOT NULL DEFAULT 0,weekly_usage_usd NUMERIC(20,10) NOT NULL DEFAULT 0,monthly_usage_usd NUMERIC(20,10) NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ,group_id BIGINT);
 INSERT INTO users(id,balance) VALUES(1,10); INSERT INTO user_subscriptions(id) VALUES(1);`)
	require.NoError(t, err)
	for _, name := range []string{"278_ops_billing_operations.sql", "279_ops_account_ledger.sql", "280_ops_account_projection_isolation.sql"} {
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		tx, err := db.Begin()
		require.NoError(t, err)
		_, err = tx.Exec(string(raw))
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
	}
	return db
}

func TestMonitoringBillingFailedIntentSurvivesRollbackPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	ctx := context.Background()
	billing := NewUsageBillingRepository(nil, db)
	cmd := &service.UsageBillingCommand{RequestID: "failed-after-debit", APIKeyID: 1, UserID: 1, BalanceCost: 2, APIKeyQuotaCost: 2}
	_, err := billing.Apply(ctx, cmd) // missing api_keys table fails after the debit
	require.Error(t, err)
	var balance string
	var count int
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "10.00000000", balance)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ops_account_ledger_entries`).Scan(&count))
	require.Zero(t, count, "transactional ledger rolls back with debit")
	var state string
	var taskID int64
	require.NoError(t, db.QueryRow(`SELECT o.id,i.state FROM ops_billing_compensation_outbox o JOIN ops_billing_intents i ON i.id=o.intent_id`).Scan(&taskID, &state))
	require.Equal(t, "failed", state)
	ops := &opsRepository{db: db}
	require.NoError(t, ops.RetryBillingOperation(ctx, taskID, 42, "check after failure"))
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now().Add(3*time.Minute)))
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "10.00000000", balance, "recheck must never debit")
	require.NoError(t, db.QueryRow(`SELECT state FROM ops_billing_compensation_outbox WHERE id=$1`, taskID).Scan(&state))
	require.Equal(t, "manual_required", state)
	_, err = db.Exec(`CREATE TABLE api_keys(id BIGINT PRIMARY KEY,quota_used NUMERIC NOT NULL DEFAULT 0,quota NUMERIC NOT NULL DEFAULT 100,status TEXT DEFAULT 'active',updated_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ);INSERT INTO api_keys(id) VALUES(1)`)
	require.NoError(t, err)
	result, err := billing.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	result, err = billing.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "8.00000000", balance)
	require.NoError(t, (&opsRepository{db: db}).ReconcileBillingUsage(ctx, time.Now()))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ops_account_ledger_entries WHERE observation_kind='aggregate' AND field='balance'`).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.QueryRow(`SELECT state FROM ops_billing_compensation_outbox WHERE id=$1`, taskID).Scan(&state))
	require.Equal(t, "resolved", state)
}

func TestMonitoringBillingLedgerCoversHoldRefundAndDirectSQLPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	ctx := context.Background()
	billing := NewUsageBillingRepository(nil, db)
	makeCmd := func(request, batch string, hold, actual float64) *service.BatchImageBalanceHoldCommand {
		return &service.BatchImageBalanceHoldCommand{RequestID: request, BatchID: batch, APIKeyID: 1, UserID: 1, HoldAmount: hold, ActualAmount: actual}
	}
	_, err := billing.ReserveBatchImageBalance(ctx, makeCmd(service.BatchImageHoldRequestID("a"), "a", 4, 0))
	require.NoError(t, err)
	_, err = billing.CaptureBatchImageBalance(ctx, makeCmd(service.BatchImageCaptureRequestID("a"), "a", 4, 3))
	require.NoError(t, err)
	var captureEvidence string
	require.NoError(t, db.QueryRow(`SELECT expected_usage_cost::text FROM usage_billing_dedup WHERE request_id=$1`, service.BatchImageCaptureRequestID("a")).Scan(&captureEvidence))
	require.Equal(t, "3.00000000", captureEvidence)
	_, err = billing.ReserveBatchImageBalance(ctx, makeCmd(service.BatchImageHoldRequestID("b"), "b", 2, 0))
	require.NoError(t, err)
	_, err = billing.ReleaseBatchImageBalance(ctx, makeCmd(service.BatchImageReleaseRequestID("b"), "b", 2, 0))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE users SET balance=balance+5 WHERE id=1;UPDATE user_subscriptions SET daily_usage_usd=4,weekly_usage_usd=4,monthly_usage_usd=4 WHERE id=1;UPDATE user_subscriptions SET daily_usage_usd=0,weekly_usage_usd=0,monthly_usage_usd=0 WHERE id=1`)
	require.NoError(t, err)
	var windowRevision int
	require.NoError(t, db.QueryRow(`SELECT ops_ledger_revision FROM user_subscriptions WHERE id=1`).Scan(&windowRevision))
	require.Equal(t, 2, windowRevision, "all three window increments and resets advance bounded source projection")
	ops := &opsRepository{db: db}
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	result, err := ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Equal(t, "Available", result.OperationalState)
	require.Zero(t, *result.LedgerMismatches)
	require.Zero(t, *result.CompensationBacklog)
	var balance, frozen string
	require.NoError(t, db.QueryRow(`SELECT balance::text,frozen_balance::text FROM users WHERE id=1`).Scan(&balance, &frozen))
	require.Equal(t, "12.00000000", balance)
	require.Equal(t, "0.00000000", frozen)
	var operations int
	require.NoError(t, db.QueryRow(`SELECT COUNT(DISTINCT operation) FROM ops_billing_intents`).Scan(&operations))
	require.Equal(t, 3, operations)
	// A missing observation is detected independently; no moving window can
	// silently clear it as the account/settlement becomes older.
	_, err = db.Exec(`ALTER TABLE users DISABLE TRIGGER ops_account_ledger_user; UPDATE users SET balance=balance-1 WHERE id=1; ALTER TABLE users ENABLE TRIGGER ops_account_ledger_user`)
	require.NoError(t, err)
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now().Add(48*time.Hour)))
	result, err = ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, *result.LedgerMismatches)
	require.Len(t, result.LedgerFindings, 1)
	require.EqualValues(t, 1, *result.MissingUsageLogs, "capture has a real log obligation; reserve/release do not")
}

func TestMonitoringBrokenObservationDoesNotChangeBillingPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	ctx := context.Background()
	billing := NewUsageBillingRepository(nil, db)
	_, err := db.Exec(`ALTER TABLE ops_billing_intents RENAME TO broken_intents; ALTER TABLE ops_account_ledger_entries RENAME TO broken_entries`)
	require.NoError(t, err)
	cmd := &service.UsageBillingCommand{RequestID: "observation-outage", APIKeyID: 1, UserID: 1, BalanceCost: 2}
	result, err := billing.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	result, err = billing.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	var balance string
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "8.00000000", balance)
	_, err = db.Exec(`ALTER TABLE broken_intents RENAME TO ops_billing_intents;ALTER TABLE broken_entries RENAME TO ops_account_ledger_entries`)
	require.NoError(t, err)
	ops := &opsRepository{db: db}
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	snapshot, err := ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Zero(t, *snapshot.LedgerMismatches)
	require.Equal(t, "Unknown", snapshot.OperationalState, "recording gap persists, but delayed projection catches up without a false discrepancy")
}

func TestMonitoringBillingCrashRecoveryDoesNotReplayPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	ctx := context.Background()
	observer := &usageBillingRepository{db: db, observeBilling: true}
	committed := &service.UsageBillingCommand{RequestID: "crash-after-commit", APIKeyID: 1, UserID: 1, BalanceCost: 2}
	committed.Normalize()
	require.Positive(t, observer.beginBillingObservation(committed.RequestID, 1, committed.RequestFingerprint, "usage", 1, committed))
	_, err := (&usageBillingRepository{db: db}).Apply(ctx, committed)
	require.NoError(t, err)
	uncommitted := &service.UsageBillingCommand{RequestID: "crash-before-commit", APIKeyID: 1, UserID: 1, BalanceCost: 3}
	uncommitted.Normalize()
	require.Positive(t, observer.beginBillingObservation(uncommitted.RequestID, 1, uncommitted.RequestFingerprint, "usage", 1, uncommitted))
	ops := &opsRepository{db: db}
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now().Add(3*time.Minute)))
	var resolved, pending int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FILTER(WHERE state='resolved'),COUNT(*) FILTER(WHERE state='manual_required') FROM ops_billing_compensation_outbox`).Scan(&resolved, &pending))
	require.Equal(t, 1, resolved)
	require.Equal(t, 1, pending)
	var balance string
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "8.00000000", balance)
	result, err := ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, *result.FailedSettlements)
	require.EqualValues(t, 1, *result.CompensationBacklog)
}

func TestMonitoringBillingPanicNeverMarksCommittedPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	repo, ok := NewUsageBillingRepository(nil, db).(*usageBillingRepository)
	require.True(t, ok)
	cmd := &service.BatchImageBalanceHoldCommand{RequestID: "panic-in-transaction", BatchID: "panic", APIKeyID: 1, UserID: 1, HoldAmount: 2}
	require.Panics(t, func() {
		_, _ = repo.applyBatchImageBalanceHold(context.Background(), cmd, "image_reserve", func(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
			_, err := reserveUsageBillingBatchImageBalance(ctx, tx, cmd)
			require.NoError(t, err)
			panic("settlement panic")
		})
	})
	var intent, task, balance string
	require.NoError(t, db.QueryRow(`SELECT i.state,o.state FROM ops_billing_intents i JOIN ops_billing_compensation_outbox o ON o.intent_id=i.id`).Scan(&intent, &task))
	require.Equal(t, "pending", intent)
	require.Equal(t, "queued", task)
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "10.00000000", balance)
	ops := &opsRepository{db: db}
	require.NoError(t, ops.ReconcileBillingUsage(context.Background(), time.Now().Add(3*time.Minute)))
	require.NoError(t, db.QueryRow(`SELECT state FROM ops_billing_compensation_outbox`).Scan(&task))
	require.Equal(t, "manual_required", task)
}

func TestMonitoringBillingObservationTimeoutCircuitAndGapPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	repo, ok := NewUsageBillingRepository(nil, db).(*usageBillingRepository)
	require.True(t, ok)
	ops := &opsRepository{db: db}
	t.Setenv("OPS_BILLING_OBSERVATION_TIMEOUT_MS", "50")
	_, err := db.Exec(`CREATE FUNCTION slow_observation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.4);RETURN NEW;END $$;CREATE TRIGGER slow_observation BEFORE INSERT ON ops_billing_intents FOR EACH ROW EXECUTE FUNCTION slow_observation()`)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	cmd := &service.UsageBillingCommand{RequestID: "observer-timeout", APIKeyID: 1, UserID: 1, BalanceCost: 2}
	started := time.Now()
	result, err := repo.Apply(ctx, cmd)
	firstElapsed := time.Since(started)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Less(t, firstElapsed, 300*time.Millisecond)
	started = time.Now()
	result, err = repo.Apply(ctx, cmd)
	circuitElapsed := time.Since(started)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Less(t, circuitElapsed, 200*time.Millisecond)
	require.Greater(t, repo.observationRetryAfter.Load(), time.Now().UnixNano())
	_, err = db.Exec(`DROP TRIGGER slow_observation ON ops_billing_intents`)
	require.NoError(t, err)
	require.NoError(t, ops.ReconcileBillingUsage(context.Background(), time.Now()))
	state, err := ops.GetBillingReconciliation(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Unknown", state.OperationalState)
	require.Zero(t, state.ObservationUnpersisted)
	require.Len(t, state.RecordingGaps, 1)
	gap := state.RecordingGaps[0]
	require.EqualValues(t, 2, gap.MissedCount)
	require.Zero(t, gap.AcknowledgedCount)
	require.Error(t, ops.AcknowledgeBillingRecordingGap(context.Background(), gap.InstanceID, 42, "  "))
	require.Error(t, ops.AcknowledgeBillingRecordingGap(context.Background(), gap.InstanceID, 42, strings.Repeat("核", 1001)))
	require.NoError(t, ops.AcknowledgeBillingRecordingGap(context.Background(), gap.InstanceID, 42, strings.Repeat("复核", 500)))
	require.NoError(t, ops.ReconcileBillingUsage(context.Background(), time.Now()))
	state, err = ops.GetBillingReconciliation(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Available", state.OperationalState)
	// A later gap reopens Unknown; acknowledgement never clears cumulative evidence.
	repo.missedBillingObservation()
	require.NoError(t, ops.ReconcileBillingUsage(context.Background(), time.Now()))
	state, err = ops.GetBillingReconciliation(context.Background())
	require.NoError(t, err)
	require.Equal(t, "Unknown", state.OperationalState)
	require.EqualValues(t, 3, state.RecordingGaps[0].MissedCount)
	require.EqualValues(t, 2, state.RecordingGaps[0].AcknowledgedCount)
	var auditCount int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ops_billing_recording_gap_audit WHERE actor_id=42 AND note<>''`).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
	var balance string
	require.NoError(t, db.QueryRow(`SELECT balance::text FROM users WHERE id=1`).Scan(&balance))
	require.Equal(t, "8.00000000", balance)
	t.Logf("50ms observer budget: first failure=%s circuit-open replay=%s; balance deducted once", firstElapsed, circuitElapsed)
}

func TestMonitoringBillingObservationLatencyPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	repo, ok := NewUsageBillingRepository(nil, db).(*usageBillingRepository)
	require.True(t, ok)
	const count = 20
	elapsed := func(observe bool, prefix string) time.Duration {
		repo.observeBilling = observe
		started := time.Now()
		for i := range count {
			result, err := repo.Apply(context.Background(), &service.UsageBillingCommand{RequestID: fmt.Sprintf("%s-%d", prefix, i), APIKeyID: 1, UserID: 1, BalanceCost: 0.01})
			require.NoError(t, err)
			require.True(t, result.Applied)
		}
		return time.Since(started) / count
	}
	baseline := elapsed(false, "baseline")
	observed := elapsed(true, "observed")
	t.Logf("local PG sequential sample, n=%d each: existing settlement+source projection=%s; settlement+source projection+intents=%s; incremental=%s", count, baseline, observed, observed-baseline)
}

func TestMonitoringBillingProjectionIgnoresSlowAndLockedObserverPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	db.SetMaxOpenConns(3)
	ctx := context.Background()
	_, err := db.Exec(`CREATE FUNCTION slow_ledger_observation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.4);RETURN NEW;END $$;
 CREATE TRIGGER slow_ledger_observation BEFORE INSERT ON ops_account_ledger_entries FOR EACH ROW EXECUTE FUNCTION slow_ledger_observation()`)
	require.NoError(t, err)
	mutate := func(label string) {
		t.Helper()
		requestCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		started := time.Now()
		tx, err := db.BeginTx(requestCtx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		_, err = tx.ExecContext(requestCtx, `SET LOCAL statement_timeout='100ms'`)
		require.NoError(t, err)
		_, err = tx.ExecContext(requestCtx, `UPDATE users SET balance=balance-1 WHERE id=1;UPDATE user_subscriptions SET daily_usage_usd=daily_usage_usd+1,weekly_usage_usd=weekly_usage_usd+1,monthly_usage_usd=monthly_usage_usd+1 WHERE id=1`)
		require.NoError(t, err, label)
		require.NoError(t, tx.Commit(), label)
		t.Logf("%s: monetary commit under Go/server100ms deadlines=%s", label, time.Since(started))
	}
	mutate("observer INSERT trigger sleeps400ms")
	for _, table := range []string{"ops_account_ledger_entries", "ops_account_ledger_heads", "ops_billing_recording_gaps"} {
		blocker, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		_, err = blocker.Exec(`LOCK TABLE ` + table + ` IN ACCESS EXCLUSIVE MODE`)
		require.NoError(t, err)
		mutate("locked " + table)
		require.NoError(t, blocker.Rollback())
	}
	var balance, expected string
	require.NoError(t, db.QueryRow(`SELECT balance::text,ops_balance_expected::text FROM users WHERE id=1`).Scan(&balance, &expected))
	require.Equal(t, "6.00000000", balance)
	require.Equal(t, "6.0000000000", expected)
	// Slow observation cancels only the independent background transaction.
	ops := &opsRepository{db: db}
	auditCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	err = ops.ReconcileBillingUsage(auditCtx, time.Now())
	cancel()
	require.Error(t, err)
	_, err = db.Exec(`DROP TRIGGER slow_ledger_observation ON ops_account_ledger_entries`)
	require.NoError(t, err)
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	var entries int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ops_account_ledger_entries WHERE observation_kind='aggregate'`).Scan(&entries))
	require.Equal(t, 5, entries)
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	var repeated int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM ops_account_ledger_entries`).Scan(&repeated))
	require.Equal(t, entries, repeated, "same source revision must not append twice")
	state, err := ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Equal(t, "Available", state.OperationalState)
	require.Zero(t, *state.LedgerMismatches)
}

func TestMonitoringBillingProjectionDeletionIsUnknownPostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	ctx := context.Background()
	ops := &opsRepository{db: db}
	_, err := db.Exec(`INSERT INTO users(id,balance) VALUES(2,7);INSERT INTO user_subscriptions(id,daily_usage_usd) VALUES(2,3)`)
	require.NoError(t, err)
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	before, err := ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Equal(t, "Available", before.OperationalState)
	_, err = db.Exec(`DELETE FROM users WHERE id=2;DELETE FROM user_subscriptions WHERE id=2`)
	require.NoError(t, err)
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	after, err := ops.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Equal(t, "Unknown", after.OperationalState)
	require.EqualValues(t, 5, *after.LedgerMismatches)
	for _, finding := range after.LedgerFindings {
		require.Nil(t, finding.ActualAmount, "physical deletion is missing evidence, never zero balance")
	}
}

func TestMonitoringBillingProjectionConcurrentRecheckDoesNotBlockOrOverwritePostgres(t *testing.T) {
	db := monitoringFinancialTestDB(t)
	db.SetMaxOpenConns(3)
	ctx := context.Background()
	_, err := db.Exec(`UPDATE users SET balance=balance-1 WHERE id=1`)
	require.NoError(t, err)
	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = blocker.Exec(`LOCK TABLE ops_account_ledger_entries IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	done := make(chan error, 1)
	ops := &opsRepository{db: db}
	go func() {
		auditCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		done <- ops.ReconcileBillingUsage(auditCtx, time.Now())
	}()
	require.Eventually(t, func() bool {
		var waiting int
		err := db.QueryRow(`SELECT COUNT(*) FROM pg_locks WHERE relation='ops_account_ledger_entries'::regclass AND NOT granted`).Scan(&waiting)
		return err == nil && waiting > 0
	}, time.Second, 10*time.Millisecond)
	businessCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err = db.ExecContext(businessCtx, `UPDATE users SET balance=balance-1 WHERE id=1`)
	cancel()
	require.NoError(t, err, "background observer must hold no account lock")
	require.NoError(t, blocker.Rollback())
	require.NoError(t, <-done)
	require.NoError(t, ops.ReconcileBillingUsage(ctx, time.Now()))
	var expected, actual string
	var source, observed int64
	require.NoError(t, db.QueryRow(`SELECT h.expected_amount::text,u.balance::text,u.ops_ledger_revision,h.source_revision FROM users u JOIN ops_account_ledger_heads h ON h.account_type='user' AND h.account_id=u.id AND h.field='balance' WHERE u.id=1`).Scan(&expected, &actual, &source, &observed))
	require.Equal(t, "8.0000000000", expected)
	require.Equal(t, "8.00000000", actual)
	require.EqualValues(t, 2, source)
	require.Equal(t, source, observed)
	var sum string
	require.NoError(t, db.QueryRow(`SELECT SUM(delta)::text FROM ops_account_ledger_entries WHERE account_type='user' AND account_id=1 AND field='balance'`).Scan(&sum))
	require.Equal(t, "-2.0000000000", sum)
}
