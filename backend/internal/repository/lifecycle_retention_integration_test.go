//go:build integration

package repository

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type lifecycleFinancialRows struct {
	user, workspace, project, key, account, usage int64
	reservation                                   string
}

func TestLifecycleRetentionPostgresTenantSettlementDedupPreserved(t *testing.T) {
	tx := testTx(t)
	f := lifecycleFinancialFixture(t, tx, true)
	request := "request-" + f.reservation
	_, err := tx.Exec(`INSERT INTO usage_billing_dedup(request_id,api_key_id,request_fingerprint,created_at) VALUES($1,$2,$3,'2026-01-02')`, request, f.key, strings.Repeat("a", 64))
	require.NoError(t, err)
	require.NoError(t, newDashboardAggregationRepositoryWithSQL(tx).CleanupUsageBillingDedup(context.Background(), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)))
	var retained int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, request, f.key).Scan(&retained))
	require.Equal(t, 1, retained, "tenant settlement markers remain with their original reservation graph")
	lifecycleRejectSQL(t, tx, `DELETE FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, request, f.key)
	_, err = tx.Exec(`INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at) VALUES($1,$2,$3,'2026-01-02')`, request, f.key, strings.Repeat("a", 64))
	require.NoError(t, err)
	lifecycleRejectSQL(t, tx, `DELETE FROM usage_billing_dedup_archive WHERE request_id=$1 AND api_key_id=$2`, request, f.key)
}

// All rows, including registration's personal Workspace, roll back together.
// Evidence is never deleted to tear down these tests.
func lifecycleFinancialFixture(t *testing.T, tx *sql.Tx, withUsage bool) lifecycleFinancialRows {
	t.Helper()
	ctx := context.Background()
	var f lifecycleFinancialRows
	f.reservation = uuid.NewString()
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,balance) VALUES($1,'test','user','active',100) RETURNING id`, "lifecycle-"+uuid.NewString()+"@example.com").Scan(&f.user))
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT w.id,p.id FROM workspaces w JOIN projects p ON p.workspace_id=w.id AND p.is_default WHERE w.owner_user_id=$1 AND w.type='personal'`, f.user).Scan(&f.workspace, &f.project))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,project_id,key,name,status) VALUES($1,$2,$3,'Lifecycle','active') RETURNING id`, f.user, f.project, "sk-"+uuid.NewString()).Scan(&f.key))
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO accounts(name,platform,type) VALUES($1,'openai','apikey') RETURNING id`, "Lifecycle-"+uuid.NewString()).Scan(&f.account))
	_, err := tx.ExecContext(ctx, `INSERT INTO budget_reservations(id,request_id,actor_user_id,api_key_id,workspace_id,project_id,billing_principal_user_id,period_start,period_end,project_period_start,project_period_end,estimate,actual,status,finalized_at) VALUES($1,$2,$3,$4,$5,$6,$3,'2026-01-01','2026-02-01','2026-01-01','2026-02-01',2,1,'finalized',now())`, f.reservation, "request-"+f.reservation, f.user, f.key, f.workspace, f.project)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO budget_reservation_allocation_snapshots(reservation_id,workspace_id,project_id,billing_principal_user_id,environment) VALUES($1,$2,$3,$4,'unallocated')`, f.reservation, f.workspace, f.project, f.user)
	require.NoError(t, err)
	if withUsage {
		require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,workspace_id,project_id,billing_principal_user_id,budget_reservation_id,resolved_platform,actual_cost,total_cost,created_at) VALUES($1,$2,$3,$4,'gpt-5',$5,$6,$1,$7,'openai',1,1,'2026-01-02') RETURNING id`, f.user, f.key, f.account, "usage-"+f.reservation, f.workspace, f.project, f.reservation).Scan(&f.usage))
	}
	return f
}

func lifecycleRejectSQL(t *testing.T, tx *sql.Tx, query string, args ...any) {
	t.Helper()
	_, err := tx.Exec(`SAVEPOINT protected_record`)
	require.NoError(t, err)
	_, err = tx.Exec(query, args...)
	require.Error(t, err, "protected financial evidence must reject destructive SQL")
	_, err = tx.Exec(`ROLLBACK TO SAVEPOINT protected_record`)
	require.NoError(t, err)
}

func TestLifecycleRetentionPostgresAllocationDeleteAndParentCascadeRejected(t *testing.T) {
	for _, table := range []string{"budget_reservation_allocation_snapshots", "usage_allocation_snapshots", "budget_reservations", "usage_logs"} {
		t.Run(table, func(t *testing.T) {
			tx := testTx(t)
			f := lifecycleFinancialFixture(t, tx, table == "usage_logs" || table == "usage_allocation_snapshots")
			if table == "budget_reservations" || table == "budget_reservation_allocation_snapshots" {
				column := "id"
				if table == "budget_reservation_allocation_snapshots" {
					column = "reservation_id"
				}
				lifecycleRejectSQL(t, tx, `DELETE FROM `+table+` WHERE `+column+`=$1`, f.reservation)
			} else {
				column := "id"
				if table == "usage_allocation_snapshots" {
					column = "usage_log_id"
				}
				lifecycleRejectSQL(t, tx, `DELETE FROM `+table+` WHERE `+column+`=$1`, f.usage)
			}
			var count int
			require.NoError(t, tx.QueryRow(`SELECT count(*) FROM budget_reservation_allocation_snapshots WHERE reservation_id=$1`, f.reservation).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestLifecycleRetentionPostgresFinancialTruncateRejected(t *testing.T) {
	for _, table := range []string{"budget_reservations", "usage_logs", "budget_reservation_allocation_snapshots", "usage_allocation_snapshots", "finops_anomaly_snapshots"} {
		t.Run(table, func(t *testing.T) {
			tx := testTx(t)
			lifecycleFinancialFixture(t, tx, true)
			lifecycleRejectSQL(t, tx, `TRUNCATE TABLE `+table+` CASCADE`)
		})
	}
}

func TestLifecycleRetentionPostgresLegacyCleanupPreservesTenantUsage(t *testing.T) {
	for _, cleanup := range []string{"requested", "automatic"} {
		t.Run(cleanup, func(t *testing.T) {
			tx := testTx(t)
			f := lifecycleFinancialFixture(t, tx, true)
			_, err := tx.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost,created_at) VALUES($1,$2,$3,$4,'legacy',0,'2026-01-02')`, f.user, f.key, f.account, "legacy-"+uuid.NewString())
			require.NoError(t, err)
			if cleanup == "requested" {
				deleted, err := newUsageCleanupRepositoryWithSQL(nil, tx).DeleteUsageLogsBatch(context.Background(), service.UsageCleanupFilters{StartTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndTime: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}, 100)
				require.NoError(t, err)
				require.Equal(t, int64(1), deleted, "only the unassigned legacy row may be cleaned")
			} else {
				require.NoError(t, newDashboardAggregationRepositoryWithSQL(tx).cleanupUsageLogsBatches(context.Background(), time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)))
			}
			var count int
			var spend float64
			require.NoError(t, tx.QueryRow(`SELECT count(*),COALESCE(sum(actual_cost),0) FROM usage_logs WHERE workspace_id=$1`, f.workspace).Scan(&count, &spend))
			require.Equal(t, 1, count)
			require.Equal(t, float64(1), spend)
			require.NoError(t, tx.QueryRow(`SELECT count(*) FROM usage_allocation_snapshots WHERE usage_log_id=$1`, f.usage).Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

func TestLifecycleRetentionPostgresMixedPartitionNeverDropped(t *testing.T) {
	tx := testTx(t)
	_, err := tx.Exec(`CREATE TEMP TABLE usage_logs (id bigint,created_at timestamptz,workspace_id bigint,project_id bigint,billing_principal_user_id bigint,budget_reservation_id uuid,service_account_id bigint) PARTITION BY RANGE(created_at);
	CREATE TEMP TABLE usage_logs_202607 PARTITION OF usage_logs FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');
	INSERT INTO usage_logs(id,created_at,workspace_id) VALUES(1,'2026-07-01',NULL),(2,'2026-07-02',42);`)
	require.NoError(t, err)
	repo := newDashboardAggregationRepositoryWithSQL(tx)
	require.NoError(t, repo.dropUsageLogsPartitions(context.Background(), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)))
	var retained int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM usage_logs WHERE id=2`).Scan(&retained))
	require.Equal(t, 1, retained, "partition DDL must not erase protected tenant rows")
}

func TestLifecycleRetentionPostgresFloorsFailClosed(t *testing.T) {
	tx := testTx(t)
	f := lifecycleFinancialFixture(t, tx, false)
	var exists bool
	require.NoError(t, tx.QueryRow(`SELECT to_regclass('platform_retention_policies') IS NOT NULL`).Scan(&exists))
	require.True(t, exists, "platform retention policies must enforce tenant floors")
	for _, category := range []string{"operational", "security", "temporary", "financial", "audit"} {
		lifecycleRejectSQL(t, tx, `INSERT INTO workspace_retention_policies(workspace_id,category,retention_days) VALUES($1,$2,1)`, f.workspace, category)
	}
	_, err := tx.Exec(`INSERT INTO workspace_retention_policies(workspace_id,category,retention_days) VALUES($1,'operational',365)`, f.workspace)
	require.NoError(t, err)
	var days int
	require.NoError(t, tx.QueryRow(`SELECT effective_lifecycle_retention_days($1,'operational')`, f.workspace).Scan(&days))
	require.Equal(t, 365, days)
	require.NoError(t, tx.QueryRow(`SELECT effective_lifecycle_retention_days($1,'financial')`, f.workspace).Scan(&days))
	require.Zero(t, days, "financial evidence is always retained indefinitely")
	lifecycleRejectSQL(t, tx, `UPDATE platform_retention_policies SET protected=false,minimum_days=1,default_days=1 WHERE category='financial'`)
	_, err = tx.Exec(`UPDATE platform_retention_policies SET minimum_days=730,default_days=730 WHERE category='operational'`)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRow(`SELECT effective_lifecycle_retention_days($1,'operational')`, f.workspace).Scan(&days))
	require.Equal(t, 730, days)
	_, err = tx.Exec(`UPDATE platform_retention_policies SET minimum_days=0,default_days=0 WHERE category='operational'`)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRow(`SELECT effective_lifecycle_retention_days($1,'operational')`, f.workspace).Scan(&days))
	require.Zero(t, days, "indefinite platform protection wins over an old finite tenant policy")
	require.NoError(t, tx.QueryRow(`SELECT effective_lifecycle_retention_days($1,'unknown')`, f.workspace).Scan(&days))
	require.Zero(t, days, "unknown classifications fail closed")
}

func TestLifecycleRetentionPostgresMigrationReplayPreservesProtection(t *testing.T) {
	tx := testTx(t)
	var version string
	require.NoError(t, tx.QueryRow(`SHOW server_version`).Scan(&version))
	t.Logf("actual PostgreSQL server_version=%s", version)
	f := lifecycleFinancialFixture(t, tx, true)
	migration, err := migrations.FS.ReadFile("300_workspace_retention_financial_protection.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.Exec(string(migration))
		require.NoError(t, err)
	}
	lifecycleRejectSQL(t, tx, `DELETE FROM usage_allocation_snapshots WHERE usage_log_id=$1`, f.usage)
	lifecycleRejectSQL(t, tx, `DELETE FROM budget_reservation_allocation_snapshots WHERE reservation_id=$1`, f.reservation)
	var retained int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM usage_allocation_snapshots WHERE usage_log_id=$1`, f.usage).Scan(&retained))
	require.Equal(t, 1, retained)
}

func TestLifecycleRetentionPostgresFixtureIsolationIdempotentAndNested(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	parentDB, parentClient := integrationDB, integrationEntClient
	tx := testTx(t)
	parent := lifecycleFinancialFixture(t, tx, true)
	require.NoError(t, tx.Commit())
	isolateWorkspaceTestFixtures(t)
	require.Same(t, parentDB, integrationDB)
	require.Same(t, parentClient, integrationEntClient)
	var childName string
	t.Run("independent_committed_child", func(t *testing.T) {
		isolateWorkspaceTestFixtures(t)
		require.NotSame(t, parentDB, integrationDB)
		require.NoError(t, integrationDB.QueryRow(`SELECT current_database()`).Scan(&childName))
		var existing int
		require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_allocation_snapshots`).Scan(&existing))
		require.Zero(t, existing, "a child starts from the pristine migrated template")
		tx := testTx(t)
		lifecycleFinancialFixture(t, tx, true)
		require.NoError(t, tx.Commit())
	})
	require.Same(t, parentDB, integrationDB)
	require.Same(t, parentClient, integrationEntClient)
	var retained int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_allocation_snapshots WHERE usage_log_id=$1`, parent.usage).Scan(&retained))
	require.Equal(t, 1, retained, "child teardown preserves the parent's committed financial graph")
	var childExists bool
	require.NoError(t, integrationAdminDB.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)`, childName).Scan(&childExists))
	require.False(t, childExists, "only the child disposable database was dropped")
}
