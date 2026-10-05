//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMachineBatchSettlementKeepsFrozenMachineAndPayer(t *testing.T) {
	job := testSettlingBatchImageJob("imgbatch_machine_snapshot")
	machineID, workspaceID, projectID, payerID := int64(31), int64(41), int64(42), int64(43)
	reservationID := "e1386f15-9507-4cce-84a5-cff62b039cc4"
	job.UserID = payerID
	job.ServiceAccountID, job.WorkspaceID, job.ProjectID, job.BillingPrincipalUserID, job.BudgetReservationID = &machineID, &workspaceID, &projectID, &payerID, &reservationID
	repo := newFakeBatchImageRepository()
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	logs := &openAIRecordUsageLogRepoStub{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: .25}, UsageLogRepo: logs}
	_, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Len(t, billing.captures, 1)
	require.Zero(t, billing.captures[0].UserID)
	require.Equal(t, payerID, billing.captures[0].BillingPrincipalUserID)
	require.Equal(t, machineID, billing.captures[0].ServiceAccountID)
	require.Zero(t, logs.lastLog.UserID)
	require.Equal(t, &machineID, logs.lastLog.ServiceAccountID)
}
