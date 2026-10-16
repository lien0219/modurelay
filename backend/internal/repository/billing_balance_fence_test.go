package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBillingBalanceFenceRejectsStaleCrossInstanceFill(t *testing.T) {
	for _, mutate := range []string{"deduct", "invalidate", "replace", "deduct_missing", "expire"} {
		t.Run(mutate, func(t *testing.T) {
			mr := miniredis.RunT(t)
			firstClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			secondClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = firstClient.Close(); _ = secondClient.Close() })
			first := &billingCache{rdb: firstClient}
			second := &billingCache{rdb: secondClient}
			ctx := context.Background()
			if mutate != "deduct_missing" {
				require.NoError(t, first.SetUserBalance(ctx, 42, 10))
			}
			// Pod A has just read balance=10 from the primary and queues its fill.
			token, err := first.BeginUserBalanceLoad(ctx, 42)
			require.NoError(t, err)
			switch mutate {
			case "deduct", "deduct_missing":
				require.NoError(t, second.DeductUserBalance(ctx, 42, 3))
			case "invalidate":
				require.NoError(t, second.InvalidateUserBalance(ctx, 42))
			case "replace":
				require.NoError(t, second.SetUserBalance(ctx, 42, 6))
			case "expire":
				mr.FastForward(billingCacheTTL + time.Second)
				// Re-created generations must not have an ABA reset to the old one.
				newToken, err := second.BeginUserBalanceLoad(ctx, 42)
				require.NoError(t, err)
				require.NotEqual(t, token, newToken)
			}
			// Pod B changed the balance while Pod A's async fill was waiting.
			require.NoError(t, first.SetUserBalanceIfUnchanged(ctx, 42, 10, token))
			balance, err := second.GetUserBalance(ctx, 42)
			switch mutate {
			case "deduct":
				require.NoError(t, err)
				require.Equal(t, 7.0, balance)
			case "replace":
				require.NoError(t, err)
				require.Equal(t, 6.0, balance)
			default:
				require.ErrorIs(t, err, redis.Nil)
			}
		})
	}
}

func TestBillingBalanceFenceAcceptsCurrentGeneration(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := &billingCache{rdb: rdb}
	ctx := context.Background()
	token, err := cache.BeginUserBalanceLoad(ctx, 42)
	require.NoError(t, err)
	require.NoError(t, cache.SetUserBalanceIfUnchanged(ctx, 42, 8, token))
	balance, err := cache.GetUserBalance(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, 8.0, balance)
	require.NoError(t, cache.SetUserBalanceIfUnchanged(ctx, 42, 100, ""))
	balance, err = cache.GetUserBalance(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, 8.0, balance)
}
