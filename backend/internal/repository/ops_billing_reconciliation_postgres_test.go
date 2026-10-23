package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

// Uses an isolated schema on an explicitly selected disposable PostgreSQL. No
// Docker or production database is required. See deploy/CLUSTER_MONITORING.md.
func monitoringTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MODURELAY_MONITORING_TEST_DSN")
	if dsn == "" {
		t.Skip("set MODURELAY_MONITORING_TEST_DSN to run real PostgreSQL monitoring checks")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	schema := fmt.Sprintf("monitoring_test_%d", time.Now().UnixNano())
	_, err = db.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	scopedDSN := dsn + " search_path=" + schema
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, parseErr := url.Parse(dsn)
		require.NoError(t, parseErr)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		scopedDSN = u.String()
	}
	db, err = sql.Open("postgres", scopedDSN)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _, _ = db.Exec("DROP SCHEMA " + schema + " CASCADE"); _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE usage_billing_dedup (
 id BIGSERIAL PRIMARY KEY, request_id VARCHAR(255) NOT NULL, api_key_id BIGINT NOT NULL,
 request_fingerprint VARCHAR(64) NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), UNIQUE(request_id,api_key_id));
 CREATE TABLE usage_billing_dedup_archive (request_id VARCHAR(255), api_key_id BIGINT, request_fingerprint VARCHAR(64),
 created_at TIMESTAMPTZ, PRIMARY KEY(request_id,api_key_id));
 CREATE TABLE usage_logs (id BIGSERIAL PRIMARY KEY,request_id VARCHAR(255),api_key_id BIGINT,actual_cost NUMERIC(20,10), UNIQUE(request_id,api_key_id));`)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("273_ops_billing_reconciliation.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	return db
}

func TestMonitoringBillingReconciliationPostgres(t *testing.T) {
	db := monitoringTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := db.Exec(`INSERT INTO usage_billing_dedup (request_id,api_key_id,request_fingerprint,created_at,expected_usage_cost) VALUES
 ('matched',1,'f',$1,1.12345678),('missing',1,'f',$1,2.5),('mismatch',1,'f',$1,3),
 ('legacy',1,'f',$1,NULL),('too-new',1,'f',$2,1);
 `, now.Add(-5*time.Minute), now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs (request_id,api_key_id,actual_cost) VALUES ('matched',1,1.123456781),('mismatch',1,2)`)
	require.NoError(t, err)
	repo := &opsRepository{db: db}
	require.NoError(t, repo.ReconcileBillingUsage(ctx, now))
	result, err := repo.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, *result.MissingUsageLogs)
	require.EqualValues(t, 1, *result.AmountMismatches)
	require.Len(t, result.Findings, 2)
	require.False(t, result.Truncated)
	// A restart has no in-memory state; missing logs that arrive later recover.
	_, err = db.Exec(`INSERT INTO usage_logs (request_id,api_key_id,actual_cost) VALUES ('missing',1,2.5); UPDATE usage_logs SET actual_cost=3 WHERE request_id='mismatch'`)
	require.NoError(t, err)
	require.NoError(t, (&opsRepository{db: db}).ReconcileBillingUsage(ctx, now.Add(time.Minute)))
	result, err = repo.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Zero(t, *result.MissingUsageLogs)
	require.Zero(t, *result.AmountMismatches)
	require.Len(t, result.Findings, 2)
	for _, f := range result.Findings {
		require.NotNil(t, f.ResolvedAt)
	}
}

func TestMonitoringBillingBacklogAndPersistencePostgres(t *testing.T) {
	db := monitoringTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := db.Exec(`INSERT INTO usage_billing_dedup (request_id,api_key_id,request_fingerprint,created_at,expected_usage_cost)
	 SELECT 'request-'||n,1,'f',$1,1 FROM generate_series(1,10002) n`, now.Add(-10*time.Minute))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs(request_id,api_key_id,actual_cost) SELECT 'request-'||n,1,1 FROM generate_series(1,10002) n`)
	require.NoError(t, err)
	repo := &opsRepository{db: db}
	require.NoError(t, repo.ReconcileBillingUsage(ctx, now))
	result, err := repo.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.True(t, result.Truncated)
	require.NoError(t, repo.ReconcileBillingUsage(ctx, now.Add(time.Minute)))
	result, err = repo.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.False(t, result.Truncated)
	require.Zero(t, *result.MissingUsageLogs)
}

func TestMonitoringBillingEvidenceAtomicClaimPostgres(t *testing.T) {
	db := monitoringTestDB(t)
	ctx := context.Background()
	repo := &usageBillingRepository{db: db}
	expected := 2.5
	cmd := &service.UsageBillingCommand{RequestID: "proof", APIKeyID: 1, ExpectedUsageCost: &expected, BalanceCost: 2.5}
	cmd.Normalize()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	applied, err := repo.claimUsageBillingKey(ctx, tx, cmd)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM usage_billing_dedup").Scan(&count))
	require.Zero(t, count)
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	applied, err = repo.claimUsageBillingKey(ctx, tx, cmd)
	require.NoError(t, err)
	require.True(t, applied)
	require.NoError(t, tx.Commit())
	var actual, delta string
	require.NoError(t, db.QueryRow("SELECT expected_usage_cost::text,balance_cost::text FROM usage_billing_dedup").Scan(&actual, &delta))
	require.Equal(t, "2.50000000", actual)
	require.Equal(t, actual, delta)
	// Replays retain the original evidence and do not apply a second debit.
	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	applied, err = repo.claimUsageBillingKey(ctx, tx, cmd)
	require.NoError(t, err)
	require.False(t, applied)
	require.NoError(t, tx.Rollback())
}

func TestMonitoringBillingOpenFindingSurvivesArchivePostgres(t *testing.T) {
	db := monitoringTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := db.Exec(`INSERT INTO usage_billing_dedup
		(request_id,api_key_id,request_fingerprint,created_at,expected_usage_cost)
		VALUES ('missing-before-archive',1,'f',$1,2.5),('matched-before-archive',1,'f',$1,3)`, now.Add(-10*time.Minute))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs(request_id,api_key_id,actual_cost) VALUES ('matched-before-archive',1,3)`)
	require.NoError(t, err)
	repo := &opsRepository{db: db}
	require.NoError(t, repo.ReconcileBillingUsage(ctx, now))
	cleanup := newDashboardAggregationRepositoryWithSQL(db)
	require.NoError(t, cleanup.CleanupUsageBillingDedup(ctx, now))
	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id='missing-before-archive'`).Scan(&count))
	require.Equal(t, 1, count, "retain evidence while its anomaly is unresolved")
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup_archive WHERE request_id='matched-before-archive'`).Scan(&count))
	require.Equal(t, 1, count, "healthy evidence keeps its existing archival behavior")
	_, err = db.Exec(`INSERT INTO usage_logs(request_id,api_key_id,actual_cost) VALUES ('missing-before-archive',1,2.5)`)
	require.NoError(t, err)
	require.NoError(t, repo.ReconcileBillingUsage(ctx, now.Add(time.Minute)))
	result, err := repo.GetBillingReconciliation(ctx)
	require.NoError(t, err)
	require.Zero(t, *result.MissingUsageLogs)
	require.NotNil(t, result.Findings[0].ResolvedAt)
	require.NoError(t, cleanup.CleanupUsageBillingDedup(ctx, now))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM usage_billing_dedup`).Scan(&count))
	require.Zero(t, count, "resolved evidence can be archived on the next cleanup")
}
