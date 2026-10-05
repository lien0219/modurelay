//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchImageHoldFingerprintBindsFrozenPayer(t *testing.T) {
	first := &BatchImageBalanceHoldCommand{UserID: 11, APIKeyID: 22, BatchID: "imgbatch_payer", HoldAmount: 1, BillingPrincipalUserID: 31}
	second := *first
	second.BillingPrincipalUserID = 32
	first.Normalize()
	second.Normalize()
	require.NotEqual(t, first.RequestFingerprint, second.RequestFingerprint, "a retry must not move a hold to a different payer")
}

func TestBatchImagePublicServiceUncertainSubmitPreservesHold(t *testing.T) {
	svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
	provider.submitErr = context.DeadlineExceeded
	billing := svc.BillingRepo.(*fakeBatchImageBillingRepo)

	_, err := svc.Submit(context.Background(), testBatchImageOwner(), validBatchImageSubmitRequest(), "")
	require.Error(t, err)
	require.Len(t, billing.reserves, 1)
	require.Empty(t, billing.releases, "a timeout can occur after the provider accepted the job")
	for _, job := range repo.jobs {
		require.Equal(t, BatchImageJobStatusUploading, job.Status)
		require.Equal(t, "SUBMIT_OUTCOME_UNKNOWN", batchImageDerefString(job.LastErrorCode))
		require.Nil(t, job.UserDeletedAt)
	}
}

func TestBatchImageSettlementUsesFrozenPayerAttribution(t *testing.T) {
	job := testSettlingBatchImageJob("imgbatch_frozen_tenant")
	workspaceID, projectID, principalID := int64(41), int64(42), int64(43)
	reservationID := "e1386f15-9507-4cce-84a5-cff62b039cc4"
	job.WorkspaceID, job.ProjectID, job.BillingPrincipalUserID, job.BudgetReservationID = &workspaceID, &projectID, &principalID, &reservationID
	repo := newFakeBatchImageRepository()
	repo.jobs[job.BatchID] = job
	billing := &fakeBatchImageBillingRepo{}
	logs := &openAIRecordUsageLogRepoStub{}
	cache := &fakeBatchImageAuthCacheInvalidator{}
	svc := &BatchImageSettlementService{Repo: repo, BillingRepo: billing, Pricing: &fakeBatchImagePricingResolver{unitPrice: .25}, UsageLogRepo: logs, AuthCache: cache}

	_, err := svc.Settle(context.Background(), job.BatchID)
	require.NoError(t, err)
	require.Len(t, billing.captures, 1)
	require.Equal(t, job.UserID, billing.captures[0].UserID)
	require.Equal(t, principalID, billing.captures[0].BillingPrincipalUserID)
	require.Equal(t, &workspaceID, logs.lastLog.WorkspaceID)
	require.Equal(t, &projectID, logs.lastLog.ProjectID)
	require.Equal(t, &principalID, logs.lastLog.BillingPrincipalUserID)
	require.Equal(t, &reservationID, logs.lastLog.BudgetReservationID)
	require.Contains(t, cache.userIDs, principalID)
}

func TestBatchImageReleaseZeroWalletHoldStillReleasesTenantBudget(t *testing.T) {
	job := testSettlingBatchImageJob("imgbatch_zero_hold_budget")
	zero := float64(0)
	job.HoldAmount, job.EstimatedCost = &zero, 0
	workspaceID, projectID, principalID := int64(41), int64(42), int64(43)
	reservationID := "e1386f15-9507-4cce-84a5-cff62b039cc4"
	job.WorkspaceID, job.ProjectID, job.BillingPrincipalUserID, job.BudgetReservationID = &workspaceID, &projectID, &principalID, &reservationID
	billing := &fakeBatchImageBillingRepo{}

	require.NoError(t, releaseBatchImageBalanceHold(context.Background(), billing, job, ""))
	require.Len(t, billing.releases, 1)
	require.Equal(t, reservationID, billing.releases[0].BudgetReservationID)
	require.Equal(t, principalID, billing.releases[0].BillingPrincipalUserID)
}

func TestBatchImageProviderCreateUncertainty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     error
		unknown bool
	}{
		{"timeout", context.DeadlineExceeded, true},
		{"server_failure", &GeminiAPIError{StatusCode: 503}, true},
		{"rejected", &GeminiAPIError{StatusCode: 403}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := NewGeminiAPIBatchImageProvider(&fakeGeminiBatchClient{createErr: tc.err})
			_, err := provider.Submit(context.Background(), nil, geminiAPIKeyAccount("sk-unit"), validGeminiBatchInput())
			require.Error(t, err)
			require.Equal(t, tc.unknown, batchImageSubmitOutcomeUncertain(err))
		})
	}
}

func TestBatchImagePublicServiceCannotRefundUnknownSubmitOnCancel(t *testing.T) {
	svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
	provider.submitErr = context.DeadlineExceeded
	billing := svc.BillingRepo.(*fakeBatchImageBillingRepo)
	_, err := svc.Submit(context.Background(), testBatchImageOwner(), validBatchImageSubmitRequest(), "")
	require.Error(t, err)
	for _, job := range repo.jobs {
		_, err = svc.Cancel(context.Background(), testBatchImageOwner(), job.BatchID)
		require.ErrorIs(t, err, ErrBatchImageProviderSubmitUncertain)
		require.Equal(t, BatchImageJobStatusUploading, job.Status)
	}
	require.Empty(t, billing.releases)
}

func TestBatchImageTenantAdmissionPersistsReservationBeforeProvider(t *testing.T) {
	svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
	budget := &batchImageTenantBudgetRepo{}
	svc.Budget = NewBudgetService(budget)
	billing := svc.BillingRepo.(*fakeBatchImageBillingRepo)
	owner := BatchImageOwner{UserID: 11, APIKeyID: 22, WorkspaceID: 41, ProjectID: 42, BillingPrincipalUserID: 43}
	svc.ProviderRegistry = NewBatchImageProviderRegistry(&batchImageTenantObservingProvider{BatchImageProvider: provider, beforeSubmit: func(job *BatchImageJob) {
		require.NotNil(t, job.BudgetReservationID)
		require.Equal(t, budget.reservation.ID, *repo.jobs[job.BatchID].BudgetReservationID)
		require.NotNil(t, job.ProviderCreateStartedAt)
		require.Len(t, billing.reserves, 1)
		require.Equal(t, int64(43), billing.reserves[0].BillingPrincipalUserID)
	}})
	req := validBatchImageSubmitRequest()
	req.TaskName = "Frozen admission"
	created, err := svc.Submit(context.Background(), owner, req, "tenant-idempotency")
	require.NoError(t, err)
	require.Equal(t, owner.UserID, budget.attribution.ActorUserID)
	require.Equal(t, owner.APIKeyID, budget.attribution.APIKeyID)
	require.Equal(t, owner.WorkspaceID, budget.attribution.WorkspaceID)
	require.Equal(t, owner.ProjectID, budget.attribution.ProjectID)
	require.Equal(t, owner.BillingPrincipalUserID, budget.attribution.BillingPrincipalUserID)
	require.Equal(t, BatchImageHoldRequestID(created.ID), budget.reservation.RequestID)
	require.Equal(t, created.EstimatedCost, budget.reservation.Estimate)
	owner.BillingPrincipalUserID = 99
	repeated, err := svc.Submit(context.Background(), owner, req, "tenant-idempotency")
	require.NoError(t, err)
	require.Equal(t, created.ID, repeated.ID)
	require.Len(t, provider.submits, 1)
	require.Equal(t, int64(43), *repo.jobs[created.ID].BillingPrincipalUserID)
	repo.jobs[created.ID].Status = BatchImageJobStatusFailed
	_, err = svc.Cancel(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Len(t, billing.releases, 1)
	require.Equal(t, int64(43), billing.releases[0].BillingPrincipalUserID)
}

func TestBatchImageTenantAdmissionRejectsBudgetBeforeProvider(t *testing.T) {
	svc, _, _, provider, _ := newTestBatchImagePublicService(true)
	svc.Budget = NewBudgetService(&batchImageTenantBudgetRepo{err: ErrProjectBudgetExceeded})
	_, err := svc.Submit(context.Background(), BatchImageOwner{UserID: 11, APIKeyID: 22, WorkspaceID: 41, ProjectID: 42, BillingPrincipalUserID: 43}, validBatchImageSubmitRequest(), "")
	require.ErrorIs(t, err, ErrProjectBudgetExceeded)
	require.Empty(t, provider.submits)
	require.Empty(t, svc.BillingRepo.(*fakeBatchImageBillingRepo).reserves)
}

type batchImageTenantObservingProvider struct {
	BatchImageProvider
	beforeSubmit func(*BatchImageJob)
}

func (p *batchImageTenantObservingProvider) Submit(ctx context.Context, job *BatchImageJob, account *Account, input BatchImageInput) (*BatchProviderJob, error) {
	p.beforeSubmit(job)
	return p.BatchImageProvider.Submit(ctx, job, account, input)
}

type batchImageTenantBudgetRepo struct {
	attribution BudgetAttribution
	reservation *BudgetReservation
	err         error
}

func (r *batchImageTenantBudgetRepo) Reserve(_ context.Context, a BudgetAttribution, requestID string, estimate float64) (*BudgetReservation, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.attribution = a
	r.reservation = &BudgetReservation{ID: "e1386f15-9507-4cce-84a5-cff62b039cc4", RequestID: requestID, ActorUserID: a.ActorUserID, APIKeyID: a.APIKeyID, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, Estimate: estimate, Status: "pending"}
	return r.reservation, nil
}
func (r *batchImageTenantBudgetRepo) Finalize(context.Context, string, float64) error { return nil }
func (r *batchImageTenantBudgetRepo) Release(context.Context, string) error           { return nil }
