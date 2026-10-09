package repository

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCompletedAnomalyBucketHonorsGrace(t *testing.T) {
	bucket, ready := completedAnomalyBucket(time.Date(2026, 10, 8, 12, 3, 0, 0, time.UTC), 5*time.Minute)
	require.True(t, ready)
	require.Equal(t, time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC), bucket)
	bucket, ready = completedAnomalyBucket(time.Date(2026, 10, 8, 12, 7, 0, 0, time.UTC), 5*time.Minute)
	require.True(t, ready)
	require.Equal(t, time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC), bucket)
}

func TestAnomalyConfigAndQueriesStayBounded(t *testing.T) {
	cfg := anomalyConfigWithDefaults(service.FinOpsAnomalyConfig{CandidateCap: 100000, MaxRollupRows: 100000, ScanWorkspaceBatch: 100000})
	require.Equal(t, 1000, cfg.CandidateCap)
	require.Equal(t, 10000, cfg.MaxRollupRows)
	require.Equal(t, 1000, cfg.ScanWorkspaceBatch)
	where, args, _ := anomalyWhere(service.FinOpsScope{WorkspaceID: 7, WorkspaceOnly: true}, service.FinOpsAnomalyFilter{Status: service.AnomalyStatusOpen, Page: 1, PageSize: 50}, 1)
	require.Contains(t, where, "f.project_id IS NULL")
	require.Contains(t, where, "f.status=$2")
	require.Len(t, args, 2)
	source, err := os.ReadFile("workspace_finops_anomaly.go")
	require.NoError(t, err)
	require.NotContains(t, strings.ToLower(string(source)), "from usage_logs")
}

func TestAnomalyWorkspaceCursorUsesDetectorVersionAndWaitsForPendingWork(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	bucket := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT workspace_id FROM usage_tenant_hourly_rollups")).
		WithArgs(bucket, 25, service.FinOpsAnomalyDetectorVersion).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(int64(7)))
	repo := &workspaceRepository{db: db}
	ids, err := repo.listAnomalyWorkspaceIDs(context.Background(), bucket, service.FinOpsAnomalyDetectorVersion, 25)
	require.NoError(t, err)
	require.Equal(t, []int64{7}, ids)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS")).
		WithArgs(bucket, service.FinOpsAnomalyDetectorVersion).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	pending, err := repo.hasPendingAnomalyWorkspace(context.Background(), bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, pending)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScanAnomalyWorkspacePreservesProjectAttributionForScopedDimensions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	bucket := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery("WITH tenant_rollups").
		WithArgs(int64(7), bucket, 100, 10000).
		WillReturnRows(sqlmock.NewRows([]string{"scope_type", "scope_id", "dimension_type", "dimension_value", "requests", "spend", "service_account", "project_id", "overflow"}).
			AddRow("api_key", int64(19), "api_key", "19", int64(30), 2.5, false, int64(31), false).
			AddRow("service_account", int64(23), "service_account", "23", int64(40), 3.5, true, int64(32), false))

	repo := &workspaceRepository{db: db}
	candidates, err := repo.scanAnomalyWorkspace(context.Background(), 7, bucket, service.DefaultFinOpsAnomalyConfig())
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, int64(31), candidates[0].projectID)
	require.Equal(t, int64(32), candidates[1].projectID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAnomalyLeaseCompletionRequiresClaimToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	bucket := time.Date(2026, 10, 8, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery("INSERT INTO finops_anomaly_detection_leases").
		WithArgs(int64(7), bucket, service.FinOpsAnomalyDetectorVersion, sqlmock.AnyArg(), float64(120)).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(int64(7)))
	repo := &workspaceRepository{db: db}
	claimed, token, err := repo.claimAnomalyLease(context.Background(), 7, bucket, service.FinOpsAnomalyDetectorVersion)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NotEmpty(t, token)

	mock.ExpectExec("UPDATE finops_anomaly_detection_leases SET completed_at").
		WithArgs(int64(7), bucket, service.FinOpsAnomalyDetectorVersion, token).
		WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.finishAnomalyLease(context.Background(), 7, bucket, service.FinOpsAnomalyDetectorVersion, token, nil))
	require.NoError(t, mock.ExpectationsWereMet())
}
