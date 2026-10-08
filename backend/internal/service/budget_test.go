package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

type budgetRepoFake struct{ reserved, finalized, released int }

type allocationCaptureRepo struct{ allocation *AllocationSnapshot }

func (r *allocationCaptureRepo) Reserve(_ context.Context, a BudgetAttribution, _ string, _ float64) (*BudgetReservation, error) {
	if a.Allocation != nil {
		copy := cloneAllocationSnapshot(*a.Allocation)
		r.allocation = &copy
	}
	return &BudgetReservation{ID: "allocation-reservation"}, nil
}
func (r *allocationCaptureRepo) Finalize(context.Context, string, float64) error { return nil }
func (r *allocationCaptureRepo) Release(context.Context, string) error           { return nil }

func TestBudgetServiceFreezesAdmissionAllocation(t *testing.T) {
	center := int64(42)
	tags := map[string]string{"team": "edge"}
	repo := &allocationCaptureRepo{}
	allocation := &AllocationSnapshot{CostCenterID: &center, Environment: AllocationEnvironmentProduction, Tags: tags, PolicyRevision: 3}
	_, err := NewBudgetService(repo).Reserve(context.Background(), BudgetAttribution{
		WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, ActorUserID: 4, APIKeyID: 5, Allocation: allocation,
	}, "request", 1)
	require.NoError(t, err)
	center = 99
	tags["team"] = "mutated"
	require.Equal(t, int64(42), *repo.allocation.CostCenterID)
	require.Equal(t, "edge", repo.allocation.Tags["team"])
}

func TestBudgetPolicyUsesFrontendJSONContract(t *testing.T) {
	policy := BudgetPolicy{WorkspaceID: 1, ProjectID: 2, Amount: 25, HardLimit: true, Enabled: true, Timezone: "Asia/Shanghai"}
	payload, err := json.Marshal(BudgetView{Policy: policy})
	require.NoError(t, err)
	var result struct {
		Policy map[string]any `json:"policy"`
	}
	require.NoError(t, json.Unmarshal(payload, &result))
	require.Equal(t, float64(25), result.Policy["amount"])
	require.Equal(t, true, result.Policy["hard_limit"])
	require.Equal(t, true, result.Policy["enabled"])
	require.Equal(t, "Asia/Shanghai", result.Policy["timezone"])
	require.NotContains(t, result.Policy, "Amount")
}

func (f *budgetRepoFake) Reserve(context.Context, BudgetAttribution, string, float64) (*BudgetReservation, error) {
	f.reserved++
	return &BudgetReservation{ID: "r1"}, nil
}

func TestBudgetReservationRequiresStorage(t *testing.T) {
	_, err := NewBudgetService(nil).Reserve(context.Background(), BudgetAttribution{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, ActorUserID: 4, APIKeyID: 5}, "request", 1)
	require.Error(t, err, "a tenant request must not proceed without a durable reservation")
}

func TestBudgetEligibilityRejectsNonFiniteCost(t *testing.T) {
	for _, amount := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1} {
		require.Error(t, NewBudgetService(nil).CheckEligibility(context.Background(), BudgetAttribution{}, amount, true))
	}
}

func TestBudgetHandleCleanupIsIdempotent(t *testing.T) {
	f := &budgetRepoFake{}
	h := NewBudgetReservationHandle(NewBudgetService(f), "r1")
	require.NoError(t, h.Release(context.Background()))
	require.NoError(t, h.Release(context.Background()))
	require.Equal(t, 1, f.released)
}

func TestBudgetHandleProviderRejectionReleasesAfterProviderAttempt(t *testing.T) {
	f := &budgetRepoFake{}
	h := NewBudgetReservationHandle(NewBudgetService(f), "r1")
	h.MarkProviderStarted()
	h.MarkProviderRejected()
	h.PreserveIfProviderStarted()
	require.NoError(t, h.Release(context.Background()))
	require.Equal(t, 1, f.released, "a definite provider rejection must release the reservation")
}

func TestBudgetHandleProviderAttemptIsPreservedWhenOutcomeIsUncertain(t *testing.T) {
	f := &budgetRepoFake{}
	h := NewBudgetReservationHandle(NewBudgetService(f), "r1")
	h.MarkProviderStarted()
	h.PreserveIfProviderStarted()
	require.NoError(t, h.Release(context.Background()))
	require.Zero(t, f.released, "an uncertain provider outcome must remain recoverable")
}

func (f *budgetRepoFake) Finalize(context.Context, string, float64) error { f.finalized++; return nil }
func (f *budgetRepoFake) Release(context.Context, string) error           { f.released++; return nil }

func TestBudgetServiceDelegatesDurableTransitions(t *testing.T) {
	f := &budgetRepoFake{}
	s := NewBudgetService(f)
	if _, err := s.Reserve(context.Background(), BudgetAttribution{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, ActorUserID: 4, APIKeyID: 5}, "req", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Finalize(context.Background(), "r1", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.Release(context.Background(), "r1"); err != nil {
		t.Fatal(err)
	}
	if f.reserved != 1 || f.finalized != 1 || f.released != 1 {
		t.Fatalf("transitions=%+v", f)
	}
}

func TestBudgetEligibilityRejectsUnpricedHardPath(t *testing.T) {
	s := NewBudgetService(nil)
	if err := s.CheckEligibility(context.Background(), BudgetAttribution{}, 0, false); err != ErrBudgetUnpriced {
		t.Fatalf("got %v", err)
	}
}
