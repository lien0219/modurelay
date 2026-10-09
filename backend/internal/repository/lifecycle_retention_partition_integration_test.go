//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLifecycleRetentionPostgresLockedPartitionDropKeepsFinancialRollups(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, _, workspace := workspaceFixture(t)
	var project int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM projects WHERE workspace_id=$1 AND is_default`, workspace.ID).Scan(&project))
	// Retain the original migrated graph. The shadow partitioned table tests
	// repository DDL against a real DB executor, without relying on row guards.
	_, err := integrationDB.ExecContext(ctx, `ALTER TABLE usage_logs RENAME TO retained_usage_source;
	CREATE TABLE usage_logs (id bigint,created_at timestamptz,workspace_id bigint,project_id bigint,billing_principal_user_id bigint,budget_reservation_id uuid,service_account_id bigint) PARTITION BY RANGE(created_at);
	CREATE TABLE usage_logs_202606 PARTITION OF usage_logs FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');
	CREATE TABLE usage_logs_202607 PARTITION OF usage_logs FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
	INSERT INTO usage_logs(id,created_at) VALUES(1,'2026-06-02'),(2,'2026-07-02');`)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO usage_logs(id,created_at,workspace_id,project_id) VALUES(3,'2026-07-03',$1,$2);`, workspace.ID, project)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO usage_tenant_hourly_rollups(workspace_id,project_id,bucket_start,api_key_id,resolved_platform,model,request_count,actual_cost) VALUES($1,$2,'2026-06-02',123,'openai','protected',1,10),($1,$2,'2026-07-03',123,'openai','protected',1,20)`, workspace.ID, project)
	require.NoError(t, err)
	repo := newDashboardAggregationRepositoryWithSQL(integrationDB)
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, repo.dropUsageLogsPartitions(ctx, cutoff))
	var juneExists, julyExists bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT to_regclass('public.usage_logs_202606') IS NOT NULL,to_regclass('public.usage_logs_202607') IS NOT NULL`).Scan(&juneExists, &julyExists))
	require.False(t, juneExists, "expired legacy-only partition can still be dropped")
	require.True(t, julyExists, "a mixed partition retains all of its data until bounded row cleanup")
	require.NoError(t, repo.cleanupUsageLogsBatches(ctx, cutoff))
	var retained int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs`).Scan(&retained))
	require.Equal(t, 1, retained)
	var requests int64
	var spend float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT sum(request_count),sum(actual_cost) FROM usage_tenant_hourly_rollups WHERE workspace_id=$1`, workspace.ID).Scan(&requests, &spend))
	require.Equal(t, int64(2), requests, "historical tenant rollups outlive legacy source partitions")
	require.Equal(t, float64(30), spend)
}

func TestLifecycleRetentionPostgresPartitionDropCannotRaceWriter(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, `ALTER TABLE usage_logs RENAME TO retained_usage_source;
	CREATE TABLE usage_logs (id bigint,created_at timestamptz,workspace_id bigint,project_id bigint,billing_principal_user_id bigint,budget_reservation_id uuid,service_account_id bigint) PARTITION BY RANGE(created_at);
	CREATE TABLE usage_logs_202607 PARTITION OF usage_logs FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');`)
	require.NoError(t, err)
	writer, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer writer.Rollback()
	_, err = writer.ExecContext(ctx, `INSERT INTO usage_logs(id,created_at,workspace_id) VALUES(1,'2026-07-02',42)`)
	require.NoError(t, err)
	// The uncommitted writer holds a conflicting relation lock. Cleanup must
	// time out before checking eligibility or deleting the partition.
	err = newDashboardAggregationRepositoryWithSQL(integrationDB).dropUsageLogsPartitions(ctx, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	require.Error(t, err)
	require.NoError(t, writer.Commit())
	require.NoError(t, newDashboardAggregationRepositoryWithSQL(integrationDB).dropUsageLogsPartitions(ctx, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)))
	var retained int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs`).Scan(&retained))
	require.Equal(t, 1, retained, "a committed tenant writer is never lost to retention DDL")
}
