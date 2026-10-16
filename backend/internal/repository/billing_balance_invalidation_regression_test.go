package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type balanceInvalidationFaultHook struct {
	failuresLeft atomic.Int64
	failed       atomic.Int64
}

func (h *balanceInvalidationFaultHook) DialHook(next redis.DialHook) redis.DialHook { return next }
func (h *balanceInvalidationFaultHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}
func (h *balanceInvalidationFaultHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		args := cmd.Args()
		if cmd.Name() == "evalsha" && len(args) > 1 && args[1] == invalidateBalanceScript.Hash() && h.failuresLeft.Add(-1) >= 0 {
			h.failed.Add(1)
			return errors.New("injected failure before balance invalidation execution")
		}
		return next(ctx, cmd)
	}
}

type balanceInvalidationPrimaryStub struct {
	service.UserRepository
	balance float64
}

func (r *balanceInvalidationPrimaryStub) GetBalance(context.Context, int64) (float64, error) {
	return r.balance, nil
}
func (r *balanceInvalidationPrimaryStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Balance: r.balance}, nil
}

func TestBalanceInvalidationRetryRejectsDelayedCrossInstanceFill(t *testing.T) {
	mr := miniredis.RunT(t)
	writerRedis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	readerRedis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = writerRedis.Close(); _ = readerRedis.Close() })
	hook := &balanceInvalidationFaultHook{}
	hook.failuresLeft.Store(2)
	writerRedis.AddHook(hook)
	writerCache, readerCache := &billingCache{rdb: writerRedis}, &billingCache{rdb: readerRedis}
	writer := service.NewBillingCacheService(writerCache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(writer.Stop)
	ctx := context.Background()
	require.NoError(t, readerCache.SetUserBalance(ctx, 42, 10))
	staleToken, err := readerCache.BeginUserBalanceLoad(ctx, 42)
	require.NoError(t, err)
	require.NoError(t, writer.InvalidateUserBalance(ctx, 42))
	require.Equal(t, int64(2), hook.failed.Load())
	require.False(t, mr.Exists(billingBalanceKey(42)), "success must confirm the atomic fence + DEL executed")
	require.NoError(t, readerCache.SetUserBalanceIfUnchanged(ctx, 42, 10, staleToken))
	_, err = readerCache.GetUserBalance(ctx, 42)
	require.ErrorIs(t, err, redis.Nil, "a delayed old fill must remain fenced after retries")
}

func TestBalanceInvalidationExhaustionCannotAdmitAnotherRedisClient(t *testing.T) {
	mr := miniredis.RunT(t)
	writerRedis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	readerRedis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = writerRedis.Close(); _ = readerRedis.Close() })
	hook := &balanceInvalidationFaultHook{}
	hook.failuresLeft.Store(100)
	writerRedis.AddHook(hook)
	writerCache, readerCache := &billingCache{rdb: writerRedis}, &billingCache{rdb: readerRedis}
	cfg := &config.Config{}
	cfg.Billing.InflightReservation.Enabled = true
	primary := &balanceInvalidationPrimaryStub{balance: -1}
	writer := service.NewBillingCacheService(writerCache, primary, nil, nil, nil, nil, cfg, nil)
	reader := service.NewBillingCacheService(readerCache, primary, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(writer.Stop)
	t.Cleanup(reader.Stop)
	ctx := context.Background()
	require.NoError(t, readerCache.SetUserBalance(ctx, 42, 1))
	require.Error(t, writer.InvalidateUserBalance(ctx, 42))
	require.Equal(t, int64(3), hook.failed.Load())
	staleBalance, err := readerCache.GetUserBalance(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, 1.0, staleBalance)
	user := &service.User{ID: 42}
	require.ErrorIs(t, reader.CheckBillingEligibility(ctx, user, nil, nil, nil, ""), service.ErrInsufficientBalance)
	reservation, err := reader.ReserveInflight(ctx, user, nil, nil, 0.2)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	require.Nil(t, reservation)
	reservation, err = reader.ReserveFixedCharge(ctx, user, nil, nil, nil, 0.2)
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	require.Nil(t, reservation)
}

func TestParallelBalanceInvalidationsBothFenceOldFills(t *testing.T) {
	mr := miniredis.RunT(t)
	firstRedis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	secondRedis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = firstRedis.Close(); _ = secondRedis.Close() })
	first, second := &billingCache{rdb: firstRedis}, &billingCache{rdb: secondRedis}
	ctx := context.Background()
	require.NoError(t, first.SetUserBalance(ctx, 42, 10))
	oldToken, err := first.BeginUserBalanceLoad(ctx, 42)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for _, cache := range []*billingCache{first, second} {
		wg.Add(1)
		go func(cache *billingCache) {
			defer wg.Done()
			errCh <- cache.InvalidateUserBalance(ctx, 42)
		}(cache)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	require.NoError(t, first.SetUserBalanceIfUnchanged(ctx, 42, 10, oldToken))
	_, err = second.GetUserBalance(ctx, 42)
	require.ErrorIs(t, err, redis.Nil)
}
