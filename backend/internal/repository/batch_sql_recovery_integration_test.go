//go:build integration

package repository

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBatchSQLRecoveryFindsAcceptedJobsAndFailedUnreleasedHolds(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, job, billing, _, payer := batchImageTenantFixture(t, 1, 1.2)
	repo, ok := NewBatchImageRepository(integrationDB).(service.BatchImageSQLRecovery)
	require.True(t, ok, "batch recovery must reconstruct accepted work from SQL when Redis enqueue fails")
	_, err := billing.ReserveBatchImageBalance(ctx, batchImageTenantHoldCommand(job, service.BatchImageHoldRequestID(job.BatchID), 0))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE batch_image_jobs SET status='failed',last_error_code='SUBMIT_STALE_BEFORE_PROVIDER',updated_at=$2 WHERE batch_id=$1`, job.BatchID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	jobs, err := repo.ListRecoverableBatchImageJobs(ctx, 10)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	_, err = billing.ReleaseBatchImageBalance(ctx, batchImageTenantHoldCommand(job, service.BatchImageReleaseRequestID(job.BatchID), 0))
	require.NoError(t, err)
	jobs, err = repo.ListRecoverableBatchImageJobs(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, jobs, "a release receipt closes its durable retry obligation")
	var balance, frozen float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance,frozen_balance FROM users WHERE id=$1`, payer.ID).Scan(&balance, &frozen))
	require.Equal(t, float64(100), balance)
	require.Zero(t, frozen)
	_, err = integrationDB.ExecContext(ctx, `UPDATE batch_image_jobs SET status='submitted',provider_job_name='provider/job',updated_at=$2 WHERE batch_id=$1`, job.BatchID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	jobs, err = repo.ListRecoverableBatchImageJobs(ctx, 1)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	_, err = integrationDB.ExecContext(ctx, `UPDATE batch_image_jobs SET status='failed',provider_job_name=NULL,provider_create_started_at=now(),last_error_code='SUBMIT_OUTCOME_UNKNOWN' WHERE batch_id=$1`, job.BatchID)
	require.NoError(t, err)
	jobs, err = repo.ListRecoverableBatchImageJobs(ctx, 10)
	require.NoError(t, err)
	require.Empty(t, jobs, "unknown provider starts cannot enter resubmission or refund recovery")
}
