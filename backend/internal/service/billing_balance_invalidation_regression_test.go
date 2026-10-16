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

type balanceInvalidationRegressionCache struct {
	*memInflightCache
	invalidations atomic.Int64
	failuresLeft  atomic.Int64
	invalidated   atomic.Bool
	onInvalidate  func(context.Context) error
}

func (c *balanceInvalidationRegressionCache) BeginUserBalanceLoad(context.Context, int64) (string, error) {
	return "fill-generation", nil
}

func (c *balanceInvalidationRegressionCache) SetUserBalanceIfUnchanged(context.Context, int64, float64, string) error {
	return nil
}

func (c *balanceInvalidationRegressionCache) InvalidateUserBalance(ctx context.Context, _ int64) error {
	c.invalidations.Add(1)
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.onInvalidate != nil {
		return c.onInvalidate(ctx)
	}
	if c.failuresLeft.Add(-1) >= 0 {
		return errors.New("invalidation failed before Redis executed it")
	}
	c.invalidated.Store(true)
	return nil
}

type admissionBalanceRepoStub struct {
	mockUserRepo
	balance float64
	err     error
	reads   atomic.Int64
}

func (r *admissionBalanceRepoStub) GetBalance(context.Context, int64) (float64, error) {
	r.reads.Add(1)
	return r.balance, r.err
}

func (r *admissionBalanceRepoStub) GetByID(context.Context, int64) (*User, error) {
	return nil, errors.New("admission must use the narrow primary balance query")
}

func TestBalanceInvalidationRetriesAndRecoversBeforeReturning(t *testing.T) {
	cache := &balanceInvalidationRegressionCache{memInflightCache: newMemInflightCache(1)}
	cache.failuresLeft.Store(2)
	svc := newBalanceInvalidationRegressionService(t, cache, &admissionBalanceRepoStub{balance: 1})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, invalidateUserBalanceCache(ctx, svc, 42))
	require.Equal(t, int64(3), cache.invalidations.Load())
	require.True(t, cache.invalidated.Load())
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), &User{ID: 42}, nil, nil, nil, ""))
	require.Equal(t, int64(3), cache.invalidations.Load(), "confirmed success must clear pending invalidation")
	require.Zero(t, cache.deducts.Load())
}

func TestBalanceInvalidationExhaustionBlocksLocalReadsUntilRecovery(t *testing.T) {
	ctx := context.Background()
	cache := &balanceInvalidationRegressionCache{memInflightCache: newMemInflightCache(1)}
	cache.failuresLeft.Store(100)
	primary := &balanceLoadUserRepoStub{balance: 2}
	svc := newBalanceInvalidationRegressionService(t, cache, primary)
	require.Error(t, svc.InvalidateUserBalance(ctx, 42))
	require.Equal(t, int64(3), cache.invalidations.Load())
	user := &User{ID: 42}
	_, err := svc.GetUserBalance(ctx, user.ID)
	require.Error(t, err, "a display read must not return the residual cache value")
	require.ErrorIs(t, svc.CheckBillingEligibility(ctx, user, nil, nil, nil, ""), ErrBillingServiceUnavailable)
	_, err = svc.reserveInflight(ctx, user, nil, nil, 0.2, false, false)
	require.ErrorIs(t, err, ErrBillingServiceUnavailable)
	_, err = svc.ReserveFixedCharge(ctx, user, nil, nil, nil, 0.2)
	require.ErrorIs(t, err, ErrBillingServiceUnavailable)
	require.Zero(t, primary.calls.Load(), "the known failed invalidation must remain fail-closed")
	cache.failuresLeft.Store(0)
	balance, err := svc.GetUserBalance(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 2.0, balance)
	require.True(t, cache.invalidated.Load())
	require.NoError(t, svc.CheckBillingEligibility(ctx, user, nil, nil, nil, ""))
}

func TestAdmissionRejectsPrimaryReadFailureDespitePositiveCache(t *testing.T) {
	for _, missingRepo := range []bool{false, true} {
		name := "primary-error"
		if missingRepo {
			name = "missing-primary"
		}
		t.Run(name, func(t *testing.T) {
			var primary UserRepository = &admissionBalanceRepoStub{err: errors.New("primary unavailable")}
			if missingRepo {
				primary = nil
			}
			cache := newMemInflightCache(100)
			svc := newBalanceInvalidationRegressionService(t, cache, primary)
			ctx := context.Background()
			user := &User{ID: 42}
			require.ErrorIs(t, svc.CheckBillingEligibility(ctx, user, nil, nil, nil, ""), ErrBillingServiceUnavailable)
			reservation, err := svc.reserveInflight(ctx, user, nil, nil, 1, false, false)
			require.ErrorIs(t, err, ErrBillingServiceUnavailable)
			require.Nil(t, reservation)
			reservation, err = svc.ReserveFixedCharge(ctx, user, nil, nil, nil, 1)
			require.ErrorIs(t, err, ErrBillingServiceUnavailable)
			require.Nil(t, reservation)
			require.Zero(t, cache.count(), "no reservation may be created from an unverified balance")
		})
	}
}

func TestFirstInflightReservationRejectsExhaustedPrimaryBalance(t *testing.T) {
	for _, balance := range []float64{-1, 0, 0.005} {
		cache := newMemInflightCache(100)
		primary := &admissionBalanceRepoStub{balance: balance}
		svc := newBalanceInvalidationRegressionService(t, cache, primary)
		svc.cfg.Billing.MinimumBalanceReserve = 0.01
		reservation, err := svc.reserveInflight(context.Background(), &User{ID: 42}, nil, nil, 0.2, false, false)
		require.ErrorIs(t, err, ErrInsufficientBalance, "primary balance=%v must fail before the first-reservation exception", balance)
		require.Nil(t, reservation)
		require.Zero(t, cache.count())
	}
}

func TestPendingBalanceInvalidationDoesNotAffectOtherUsersCircuitBreaker(t *testing.T) {
	for _, halfOpen := range []bool{false, true} {
		name := "does-not-open-global-breaker"
		if halfOpen {
			name = "does-not-consume-half-open-probe"
		}
		t.Run(name, func(t *testing.T) {
			cache := &balanceInvalidationRegressionCache{memInflightCache: newMemInflightCache(1)}
			cache.failuresLeft.Store(100)
			primary := &admissionBalanceRepoStub{balance: 2}
			svc := newBalanceInvalidationRegressionService(t, cache, primary)
			svc.circuitBreaker = newBillingCircuitBreaker(config.CircuitBreakerConfig{
				Enabled: true, FailureThreshold: 2, ResetTimeoutSeconds: 60, HalfOpenRequests: 1,
			})
			if halfOpen {
				svc.circuitBreaker.state = billingCircuitHalfOpen
				svc.circuitBreaker.halfOpenRemaining = 1
			}
			ctx := context.Background()
			require.Error(t, svc.InvalidateUserBalance(ctx, 42))
			for i := 0; i < 3; i++ {
				require.ErrorIs(t, svc.CheckBillingEligibility(ctx, &User{ID: 42}, nil, nil, nil, ""), ErrBillingServiceUnavailable)
			}
			require.NoError(t, svc.CheckBillingEligibility(ctx, &User{ID: 43}, nil, nil, nil, ""), "a per-user pending invalidation cannot block a healthy user")
			require.Equal(t, int64(1), primary.reads.Load())
			svc.circuitBreaker.mu.Lock()
			state, failures := svc.circuitBreaker.state, svc.circuitBreaker.failures
			svc.circuitBreaker.mu.Unlock()
			require.Equal(t, billingCircuitClosed, state)
			require.Zero(t, failures)
		})
	}
}

func TestOlderInvalidationSuccessCannotClearNewerFailure(t *testing.T) {
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int64
	cache := &balanceInvalidationRegressionCache{memInflightCache: newMemInflightCache(1)}
	cache.onInvalidate = func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			close(firstStarted)
			// Model an old successful Redis execution whose reply is delayed.
			select {
			case <-releaseFirst:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return errors.New("newer invalidation failed")
	}
	svc := newBalanceInvalidationRegressionService(t, cache, &admissionBalanceRepoStub{balance: 2})
	olderDone := make(chan error, 1)
	go func() { olderDone <- svc.InvalidateUserBalance(context.Background(), 42) }()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("older invalidation did not start")
	}
	require.Error(t, svc.InvalidateUserBalance(context.Background(), 42))
	close(releaseFirst)
	require.NoError(t, <-olderDone)
	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 42}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrBillingServiceUnavailable, "old confirmation must not erase the newer failed generation")
}

func (c *balanceInvalidationRegressionCache) GetUserBalance(ctx context.Context, id int64) (float64, error) {
	if c.invalidated.Load() {
		return 0, errors.New("cache miss")
	}
	return c.memInflightCache.GetUserBalance(ctx, id)
}

func newBalanceInvalidationRegressionService(t *testing.T, cache BillingCache, repo UserRepository) *BillingCacheService {
	t.Helper()
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	return svc
}

func TestBalanceInvalidationFailureCannotAdmitFromAnotherInstance(t *testing.T) {
	ctx := context.Background()
	cache := &balanceInvalidationRegressionCache{memInflightCache: newMemInflightCache(1)}
	cache.failuresLeft.Store(100)
	primary := &balanceLoadUserRepoStub{balance: -1}
	writer := newBalanceInvalidationRegressionService(t, cache, primary)
	reader := newBalanceInvalidationRegressionService(t, cache, primary)
	newBalance := -1.0
	require.Error(t, syncBalanceCacheAfterDeduction(ctx, &postUsageBillingParams{
		Cost: &CostBreakdown{ActualCost: 2}, User: &User{ID: 42},
	}, &billingDeps{billingCacheService: writer}, &UsageBillingApplyResult{Applied: true, NewBalance: &newBalance}))
	require.False(t, cache.invalidated.Load(), "the stale key must survive the injected failure")
	user := &User{ID: 42}
	t.Run("eligibility", func(t *testing.T) {
		require.ErrorIs(t, reader.CheckBillingEligibility(ctx, user, nil, nil, nil, ""), ErrInsufficientBalance)
	})
	// Exercise reservation arithmetic, rather than its intentional first-request exception.
	_, _, err := cache.ReserveInflightBalance(ctx, user.ID, "already-pending", 0.1, 1, time.Minute)
	require.NoError(t, err)
	t.Run("inflight", func(t *testing.T) {
		reservation, err := reader.reserveInflight(ctx, user, nil, nil, 0.2, false, false)
		if reservation != nil {
			reservation.HandlerDone()
		}
		require.ErrorIs(t, err, ErrInsufficientBalance)
	})
	t.Run("fixed-charge", func(t *testing.T) {
		reservation, err := reader.ReserveFixedCharge(ctx, user, nil, nil, nil, 0.2)
		if reservation != nil {
			reservation.HandlerDone()
		}
		require.ErrorIs(t, err, ErrInsufficientBalance)
	})
	require.Zero(t, cache.deducts.Load(), "invalidation retries must never repeat the debit")
}
