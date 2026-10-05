package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestTenantEstimateUsesBillingPrincipalRate(t *testing.T) {
	groupID := int64(9)
	key := &APIKey{UserID: 4, User: &User{ID: 4}, GroupID: &groupID, Group: &Group{ID: 9, RateMultiplier: 1}, Tenant: &TenantContext{BillingPrincipalUserID: 3}}
	var pricedUser int64
	d := inflightEstimateDeps{userGroupRate: func(_ context.Context, userID, _ int64, _ float64) float64 { pricedUser = userID; return 2 }}
	text, _ := d.rates(context.Background(), key)
	require.Equal(t, int64(3), pricedUser, "the actor's discount must not change the payer's charge")
	require.Equal(t, 2.0, text)
}

func TestTenantSimpleEstimateRepresentsNoCharge(t *testing.T) {
	d := inflightEstimateDeps{cfg: &config.Config{RunMode: config.RunModeSimple}}
	estimate, priced := d.estimate(context.Background(), &APIKey{User: &User{ID: 1}}, InflightEstimateRequest{Model: "unknown", Kind: InflightEstimateToken})
	require.True(t, priced)
	require.Zero(t, estimate)
}

type tenantLifecycleBillingRepo struct {
	UsageBillingRepository
	err error
}

func (r tenantLifecycleBillingRepo) Apply(context.Context, *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &UsageBillingApplyResult{Applied: true}, nil
}
func TestTenantBudgetBillingCleanupDoesNotReleaseCommittedOrUncertainUsage(t *testing.T) {
	for _, failure := range []error{nil, errors.New("commit outcome unavailable")} {
		f := &budgetRepoFake{}
		h := NewBudgetReservationHandle(NewBudgetService(f), "r1")
		ctx := WithBudgetReservation(context.Background(), h)
		p := &postUsageBillingParams{Cost: &CostBreakdown{}, User: &User{ID: 1}, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3}, SimpleModeKeyRateLimitOnly: true}
		_, err := applyPreparedUsageBilling(ctx, &UsageBillingCommand{RequestID: "req", APIKeyID: 2}, p, &billingDeps{deferredService: &DeferredService{}}, tenantLifecycleBillingRepo{err: failure})
		if failure != nil {
			require.ErrorIs(t, err, failure)
		} else {
			require.NoError(t, err)
		}
		require.NoError(t, h.Release(context.Background()))
		require.Zero(t, f.released)
	}
}
