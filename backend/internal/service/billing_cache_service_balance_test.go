//go:build unit

package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type balanceEligibilityCacheStub struct {
	billingCacheWorkerStub

	balance                  float64
	cacheMissAfterInvalidate bool
	invalidated              atomic.Bool
	deductCalls              atomic.Int64
	invalidateCalls          atomic.Int64
}

func (s *balanceEligibilityCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	if s.cacheMissAfterInvalidate && s.invalidated.Load() {
		return 0, errors.New("cache miss")
	}
	return s.balance, nil
}

func (s *balanceEligibilityCacheStub) DeductUserBalance(context.Context, int64, float64) error {
	s.deductCalls.Add(1)
	return nil
}

func (s *balanceEligibilityCacheStub) InvalidateUserBalance(context.Context, int64) error {
	s.invalidateCalls.Add(1)
	s.invalidated.Store(true)
	return nil
}

func TestCheckBillingEligibility_RejectsBalanceBelowMinimumReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.005}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, &balanceLoadUserRepoStub{balance: 0.005}, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestCheckBillingEligibility_AllowsBalanceAtMinimumReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.01}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, &balanceLoadUserRepoStub{balance: 0.01}, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.NoError(t, err)
}

func TestSyncBalanceCacheAfterDeduction_InvalidatesExhaustedBalance(t *testing.T) {
	cache := &balanceEligibilityCacheStub{
		balance:                  0.50,
		cacheMissAfterInvalidate: true,
	}
	userRepo := &balanceLoadUserRepoStub{balance: -0.25}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := -0.25
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.75},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{
		NewBalance:         &newBalance,
		BalanceOverdrafted: true,
	})

	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Equal(t, int64(0), cache.deductCalls.Load())

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Equal(t, int64(1), userRepo.calls.Load())
}

func TestSyncBalanceCacheAfterDeduction_InvalidatesWhenBalanceFallsBelowReserve(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0.50}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := 0.005
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.495},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})

	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Equal(t, int64(0), cache.deductCalls.Load())
}

func TestSyncBalanceCacheAfterDeduction_QueuesDeductWhenBalanceStillEligible(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 1}
	cfg := &config.Config{}
	cfg.Billing.MinimumBalanceReserve = 0.01
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)

	newBalance := 0.75
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 0.25},
		User: &User{ID: 1},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})

	require.Equal(t, int64(0), cache.invalidateCalls.Load())
	require.Eventually(t, func() bool {
		return cache.deductCalls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
}

type fencedBalanceCacheStub struct {
	balanceEligibilityCacheStub
	filledToken chan string
	beginCalled atomic.Bool
}

func (s *fencedBalanceCacheStub) BeginUserBalanceLoad(context.Context, int64) (string, error) {
	s.beginCalled.Store(true)
	return "generation-before-db-read", nil
}

func (s *fencedBalanceCacheStub) SetUserBalanceIfUnchanged(_ context.Context, _ int64, _ float64, token string) error {
	s.filledToken <- token
	return nil
}

func (s *fencedBalanceCacheStub) InvalidateUserBalance(ctx context.Context, userID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.balanceEligibilityCacheStub.InvalidateUserBalance(ctx, userID)
}

func TestSyncBalanceCacheAfterDeduction_FencedCacheInvalidatesSynchronously(t *testing.T) {
	cache := &fencedBalanceCacheStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	newBalance := 9.0
	syncBalanceCacheAfterDeduction(context.Background(), &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 1}, User: &User{ID: 42},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{NewBalance: &newBalance})
	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Zero(t, cache.deductCalls.Load())
}

func TestSyncBalanceCacheAfterDeduction_FencesAfterBillingContextCanceled(t *testing.T) {
	cache := &fencedBalanceCacheStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	syncBalanceCacheAfterDeduction(ctx, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 1}, User: &User{ID: 42},
	}, &billingDeps{billingCacheService: svc}, &UsageBillingApplyResult{Applied: true})
	require.Equal(t, int64(1), cache.invalidateCalls.Load())
	require.Zero(t, cache.deductCalls.Load())
}

type fencedBalanceUserRepoStub struct {
	balanceLoadUserRepoStub
	cache *fencedBalanceCacheStub
}

func (s *fencedBalanceUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if !s.cache.beginCalled.Load() {
		return nil, errors.New("balance fence must be obtained before the database read")
	}
	return s.balanceLoadUserRepoStub.GetByID(ctx, id)
}

func TestGetUserBalancePropagatesFenceFromBeforeDatabaseRead(t *testing.T) {
	cache := &fencedBalanceCacheStub{filledToken: make(chan string, 1)}
	cache.cacheMissAfterInvalidate = true
	cache.invalidated.Store(true)
	repo := &fencedBalanceUserRepoStub{balanceLoadUserRepoStub: balanceLoadUserRepoStub{balance: 9}, cache: cache}
	svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	balance, err := svc.GetUserBalance(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 9.0, balance)
	select {
	case token := <-cache.filledToken:
		require.Equal(t, "generation-before-db-read", token)
	case <-time.After(time.Second):
		t.Fatal("fenced balance fill was not queued")
	}
}
