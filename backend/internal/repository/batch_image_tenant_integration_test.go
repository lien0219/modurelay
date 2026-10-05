//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func batchImageTenantFixture(t *testing.T, estimate, hold float64) (context.Context, *service.BatchImageJob, service.UsageBillingRepository, *service.User, *service.User) {
	t.Helper()
	ctx, workspaces, payer, workspace := workspaceFixture(t)
	actor := workspaceJoin(t, ctx, workspaces, payer.ID, workspace.ID, "developer")
	project, err := workspaces.CreateProject(ctx, payer.ID, workspace.ID, service.ProjectInput{Name: "Batch", Slug: "batch"})
	require.NoError(t, err)
	key, err := testEntClient(t).APIKey.Create().SetUserID(actor.ID).SetProjectID(project.ID).SetKey("batch-tenant-" + uuid.NewString()).SetName("Batch").SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id IN ($1,$2)`, actor.ID, payer.ID)
	require.NoError(t, err)
	batchID := "imgbatch_" + uuid.NewString()
	reservation, err := NewBudgetRepository(integrationDB).Reserve(ctx, service.BudgetAttribution{WorkspaceID: workspace.ID, ProjectID: project.ID, ActorUserID: actor.ID, APIKeyID: key.ID, BillingPrincipalUserID: payer.ID}, service.BatchImageHoldRequestID(batchID), estimate)
	require.NoError(t, err)
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "batch-tenant-" + uuid.NewString(), Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey})
	job, err := NewBatchImageRepository(integrationDB).CreateBatchImageJob(ctx, service.CreateBatchImageJobParams{
		BatchID: batchID, UserID: actor.ID, APIKeyID: &key.ID, AccountID: &account.ID,
		WorkspaceID: &workspace.ID, ProjectID: &project.ID, BillingPrincipalUserID: &payer.ID,
		BudgetReservationID: &reservation.ID, Provider: service.BatchImageProviderGeminiAPI,
		Model: "gemini-2.5-flash-image", ItemCount: 2, EstimatedCost: estimate, HoldAmount: &hold,
		PricingSnapshotVersion: 1, BillableUnitPrice: estimate / 2, GroupRateMultiplier: 1, AccountRateMultiplier: 1, BatchDiscountMultiplier: 1,
	})
	require.NoError(t, err)
	return ctx, job, NewUsageBillingRepository(testEntClient(t), integrationDB), actor, payer
}

func batchImageTenantHoldCommand(job *service.BatchImageJob, requestID string, actual float64) *service.BatchImageBalanceHoldCommand {
	return &service.BatchImageBalanceHoldCommand{RequestID: requestID, BatchID: job.BatchID, UserID: job.UserID, APIKeyID: *job.APIKeyID, WorkspaceID: *job.WorkspaceID, ProjectID: *job.ProjectID, BillingPrincipalUserID: *job.BillingPrincipalUserID, BudgetReservationID: *job.BudgetReservationID, HoldAmount: *job.HoldAmount, ActualAmount: actual}
}

func TestBatchImageTenantCaptureUsesFrozenPayerAndFinalizesOnce(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, job, billing, actor, payer := batchImageTenantFixture(t, 1, 1.2)
	_, err := billing.ReserveBatchImageBalance(ctx, batchImageTenantHoldCommand(job, service.BatchImageHoldRequestID(job.BatchID), 0))
	require.NoError(t, err)
	var actorBalance, payerBalance, frozen float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, actor.ID).Scan(&actorBalance))
	require.Equal(t, float64(100), actorBalance)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, payer.ID).Scan(&payerBalance, &frozen))
	require.InDelta(t, 98.8, payerBalance, 1e-8)
	require.InDelta(t, 1.2, frozen, 1e-8)
	// Membership/key changes after admission must not change settlement ownership.
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspaces SET billing_owner_user_id=$2 WHERE id=$1`, *job.WorkspaceID, actor.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND user_id=$2`, *job.WorkspaceID, actor.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE api_keys SET status='disabled' WHERE id=$1`, *job.APIKeyID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE batch_image_jobs SET status='settling',success_count=1 WHERE batch_id=$1`, job.BatchID)
	require.NoError(t, err)
	cmd := batchImageTenantHoldCommand(job, service.BatchImageCaptureRequestID(job.BatchID), .5)
	first, err := billing.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err := billing.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.False(t, second.Applied)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance,frozen_balance FROM users WHERE id=$1`, payer.ID).Scan(&payerBalance, &frozen))
	require.InDelta(t, 99.5, payerBalance, 1e-8)
	require.Zero(t, frozen)
	var status string
	var actual, reserved, spent float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status,actual FROM budget_reservations WHERE id=$1`, *job.BudgetReservationID).Scan(&status, &actual))
	require.Equal(t, "finalized", status)
	require.Equal(t, .5, actual)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT reserved,spent FROM budget_counters WHERE scope_type='workspace' AND scope_id=$1`, *job.WorkspaceID).Scan(&reserved, &spent))
	require.Zero(t, reserved)
	require.Equal(t, .5, spent)
	loaded, err := NewBatchImageRepository(integrationDB).GetBatchImageJobByBatchIDForOwner(ctx, actor.ID, *job.APIKeyID, job.BatchID)
	require.NoError(t, err)
	require.Equal(t, job.BillingPrincipalUserID, loaded.BillingPrincipalUserID)
	_, err = NewBatchImageRepository(integrationDB).GetBatchImageJobByBatchIDForOwner(ctx, payer.ID, *job.APIKeyID, job.BatchID)
	require.ErrorIs(t, err, service.ErrBatchImageJobNotFound)
}

func TestBatchImageTenantReleaseWithoutWalletHoldReleasesBudgetOnce(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, job, billing, _, _ := batchImageTenantFixture(t, 1, 0)
	_, err := integrationDB.ExecContext(ctx, `UPDATE batch_image_jobs SET status='failed' WHERE batch_id=$1`, job.BatchID)
	require.NoError(t, err)
	cmd := batchImageTenantHoldCommand(job, service.BatchImageReleaseRequestID(job.BatchID), 0)
	first, err := billing.ReleaseBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.True(t, first.Applied)
	second, err := billing.ReleaseBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	require.False(t, second.Applied)
	var status string
	var reserved, spent float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM budget_reservations WHERE id=$1`, *job.BudgetReservationID).Scan(&status))
	require.Equal(t, "released", status)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT reserved,spent FROM budget_counters WHERE scope_type='project' AND scope_id=$1`, *job.ProjectID).Scan(&reserved, &spent))
	require.Zero(t, reserved)
	require.Zero(t, spent)
}

func TestBatchImageTenantCapturePersistsSummaryBeforeStatusWrite(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, job, billing, _, _ := batchImageTenantFixture(t, 1, 1)
	_, err := billing.ReserveBatchImageBalance(ctx, batchImageTenantHoldCommand(job, service.BatchImageHoldRequestID(job.BatchID), 0))
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE batch_image_jobs SET status='settling',success_count=1 WHERE batch_id=$1`, job.BatchID)
	require.NoError(t, err)
	cmd := batchImageTenantHoldCommand(job, service.BatchImageCaptureRequestID(job.BatchID), .5)
	_, err = billing.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	_, err = billing.CaptureBatchImageBalance(ctx, cmd)
	require.NoError(t, err)
	var logs int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM usage_logs WHERE request_id=$1 AND api_key_id=$2 AND user_id=$3 AND workspace_id=$4 AND project_id=$5 AND billing_principal_user_id=$6 AND budget_reservation_id=$7 AND image_count=1 AND actual_cost=.5`, cmd.RequestID, cmd.APIKeyID, cmd.UserID, cmd.WorkspaceID, cmd.ProjectID, cmd.BillingPrincipalUserID, cmd.BudgetReservationID).Scan(&logs))
	require.Equal(t, 1, logs, "the aggregate capture must persist its FinOps summary before MarkSettled")
	var status string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM batch_image_jobs WHERE batch_id=$1`, job.BatchID).Scan(&status))
	require.Equal(t, service.BatchImageJobStatusSettling, status)
}
