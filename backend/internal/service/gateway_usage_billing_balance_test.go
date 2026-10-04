//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type insufficientBalanceUsageBillingRepoStub struct {
	UsageBillingRepository
}

func (insufficientBalanceUsageBillingRepoStub) Apply(context.Context, *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	return nil, ErrInsufficientBalance
}

func TestApplyUsageBilling_InvalidatesBalanceCacheAfterInsufficientBalance(t *testing.T) {
	cache := &balanceEligibilityCacheStub{}
	cfg := &config.Config{}
	billingCache := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCache.Stop)

	_, err := applyUsageBilling(context.Background(), "grok-video:task-1", nil, &postUsageBillingParams{
		Cost:    &CostBreakdown{ActualCost: 1},
		User:    &User{ID: 7},
		APIKey:  &APIKey{ID: 13},
		Account: &Account{ID: 9},
	}, &billingDeps{billingCacheService: billingCache}, insufficientBalanceUsageBillingRepoStub{})

	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Equal(t, int64(1), cache.invalidateCalls.Load())
}
