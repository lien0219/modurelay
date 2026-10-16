package repository

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type waitLeaseTestScope struct {
	key, legacyKey, index string
	acquire               func(context.Context, int64, int, string) (bool, error)
	release               func(context.Context, int64, string) error
	count                 func(context.Context, int64) (int, error)
	batchCount            func(context.Context, int64) (int, error)
}

func waitLeaseScope(c *concurrencyCache, scope string, id int64) waitLeaseTestScope {
	if scope == "account" {
		return waitLeaseTestScope{
			key: c.accountWaitKey(id), legacyKey: c.clusterKey(legacyAccountWaitKey(id)), index: accountActiveIndexKey,
			acquire: c.IncrementAccountWaitCount, release: c.DecrementAccountWaitCount,
			count: c.GetAccountWaitingCount,
			batchCount: func(ctx context.Context, id int64) (int, error) {
				loads, err := c.GetAccountsLoadBatch(ctx, []service.AccountWithConcurrency{{ID: id, MaxConcurrency: 2}})
				if err != nil {
					return 0, err
				}
				return loads[id].WaitingCount, nil
			},
		}
	}
	count := func(ctx context.Context, id int64) (int, error) {
		loads, err := c.GetUsersLoadBatch(ctx, []service.UserWithConcurrency{{ID: id, MaxConcurrency: 2}})
		if err != nil {
			return 0, err
		}
		return loads[id].WaitingCount, nil
	}
	return waitLeaseTestScope{
		key: c.waitQueueKey(id), legacyKey: c.clusterKey(legacyWaitQueueKey(id)), index: userActiveIndexKey,
		acquire: c.IncrementWaitCount, release: c.DecrementWaitCount, count: count, batchCount: count,
	}
}

func newWaitLeaseTestCache(t *testing.T) (*miniredis.Miniredis, *redis.Client, *concurrencyCache, func(time.Duration)) {
	t.Helper()
	server := miniredis.RunT(t)
	now := time.Unix(2_000_000_000, 0)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewConcurrencyCache(client, 15, 10).(*concurrencyCache)
	advance := func(d time.Duration) {
		now = now.Add(d)
		server.FastForward(d)
		server.SetTime(now)
	}
	return server, client, cache, advance
}

func TestWaitLeaseCrashedWaiterExpiresDuringContinuousTraffic(t *testing.T) {
	for _, scope := range []string{"account", "user"} {
		t.Run(scope, func(t *testing.T) {
			_, _, cache, advance := newWaitLeaseTestCache(t)
			ctx := context.Background()
			queue := waitLeaseScope(cache, scope, 1)
			ok, err := queue.acquire(ctx, 1, 2, "crashed-pod")
			require.NoError(t, err)
			require.True(t, ok)
			for i := 0; i < 4; i++ {
				advance(9 * time.Second)
				id := fmt.Sprintf("live-%d", i)
				ok, err = queue.acquire(ctx, 1, 2, id)
				require.NoError(t, err)
				require.True(t, ok)
				require.NoError(t, queue.release(ctx, 1, id))
				require.NoError(t, cache.CleanupStaleProcessSlots(ctx, "restarted-pod"))
			}
			count, err := queue.count(ctx, 1)
			require.NoError(t, err)
			require.Zero(t, count, "new arrivals must not extend a crashed waiter's expiry")
			for _, id := range []string{"first-real-waiter", "second-real-waiter"} {
				ok, err = queue.acquire(ctx, 1, 2, id)
				require.NoError(t, err)
				require.True(t, ok, "both real waiters must fit after the crashed waiter expires")
			}
		})
	}
}

func TestWaitLeaseLateAndDuplicateReleasePreserveOtherInstance(t *testing.T) {
	for _, scope := range []string{"account", "user"} {
		t.Run(scope, func(t *testing.T) {
			_, client, cache, advance := newWaitLeaseTestCache(t)
			peer := NewConcurrencyCache(client, 15, 10).(*concurrencyCache)
			ctx := context.Background()
			queue := waitLeaseScope(cache, scope, 1)
			other := waitLeaseScope(peer, scope, 1)
			ok, err := queue.acquire(ctx, 1, 2, "old-pod-waiter")
			require.NoError(t, err)
			require.True(t, ok)
			advance(9 * time.Second)
			ok, err = other.acquire(ctx, 1, 2, "new-pod-waiter")
			require.NoError(t, err)
			require.True(t, ok)
			advance(2 * time.Second)
			// Force an index sweep while the newer peer's waiter remains live.
			now, err := cache.redisUnixSeconds(ctx)
			require.NoError(t, err)
			require.NoError(t, client.ZAdd(ctx, queue.index, redis.Z{Score: float64(now - 1), Member: "1"}).Err())
			require.NoError(t, peer.CleanupStaleProcessSlots(ctx, "new-pod"))
			for i := 0; i < 2; i++ {
				require.NoError(t, queue.release(ctx, 1, "old-pod-waiter"))
				count, err := other.count(ctx, 1)
				require.NoError(t, err)
				require.Equal(t, 1, count)
			}
			ok, err = queue.acquire(ctx, 1, 2, "replacement")
			require.NoError(t, err)
			require.True(t, ok)
			ok, err = other.acquire(ctx, 1, 2, "over-limit")
			require.NoError(t, err)
			require.False(t, ok)
			require.NoError(t, other.release(ctx, 1, "new-pod-waiter"))
			require.NoError(t, queue.release(ctx, 1, "replacement"))
			count, err := queue.count(ctx, 1)
			require.NoError(t, err)
			require.Zero(t, count)
			require.ErrorIs(t, client.ZScore(ctx, queue.index, "1").Err(), redis.Nil)
		})
	}
}

func TestWaitLeaseReadsAndCleanupReapIndividualExpiries(t *testing.T) {
	for _, scope := range []string{"account", "user"} {
		t.Run(scope, func(t *testing.T) {
			_, client, cache, _ := newWaitLeaseTestCache(t)
			ctx := context.Background()
			queue := waitLeaseScope(cache, scope, 1)
			now, err := cache.redisUnixSeconds(ctx)
			require.NoError(t, err)
			for _, read := range []func(context.Context, int64) (int, error){queue.count, queue.batchCount} {
				require.NoError(t, client.ZAdd(ctx, queue.key,
					redis.Z{Score: float64(now - 1), Member: "expired"},
					redis.Z{Score: float64(now + 10), Member: "live"}).Err())
				count, err := read(ctx, 1)
				require.NoError(t, err)
				require.Equal(t, 1, count)
				require.ErrorIs(t, client.ZScore(ctx, queue.key, "expired").Err(), redis.Nil)
			}
			require.NoError(t, client.ZAdd(ctx, queue.key, redis.Z{Score: float64(now - 1), Member: "live"}).Err())
			require.NoError(t, client.ZAdd(ctx, queue.index, redis.Z{Score: float64(now - 1), Member: strconv.FormatInt(1, 10)}).Err())
			require.NoError(t, cache.CleanupExpiredAccountSlotKeys(ctx))
			require.Zero(t, client.ZCard(ctx, queue.key).Val())
			require.ErrorIs(t, client.ZScore(ctx, queue.index, "1").Err(), redis.Nil)
		})
	}
}

func TestWaitLeaseLegacyCountersExpireWithoutBeingRefreshed(t *testing.T) {
	for _, scope := range []string{"account", "user"} {
		t.Run(scope, func(t *testing.T) {
			server, client, cache, advance := newWaitLeaseTestCache(t)
			ctx := context.Background()
			queue := waitLeaseScope(cache, scope, 1)
			require.NoError(t, client.Set(ctx, queue.legacyKey, 1, 10*time.Second).Err())
			advance(9 * time.Second)
			ok, err := queue.acquire(ctx, 1, 2, "new-version-waiter")
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, time.Second, server.TTL(queue.legacyKey))
			count, err := queue.batchCount(ctx, 1)
			require.NoError(t, err)
			require.Equal(t, 2, count, "new instances must still account for legacy waiters")
			ok, err = queue.acquire(ctx, 1, 2, "blocked")
			require.NoError(t, err)
			require.False(t, ok)
			require.NoError(t, queue.release(ctx, 1, "unknown-waiter"))
			require.Equal(t, "1", client.Get(ctx, queue.legacyKey).Val())
			advance(2 * time.Second)
			ok, err = queue.acquire(ctx, 1, 2, "now-allowed")
			require.NoError(t, err)
			require.True(t, ok, "legacy residue must disappear even while v2 traffic continues")
			require.False(t, server.Exists(queue.legacyKey))
		})
	}
}

func TestWaitLeaseRetryDoesNotExtendExpiryAndShorterPeerDoesNotExpireLongLease(t *testing.T) {
	server, client, cache, advance := newWaitLeaseTestCache(t)
	ctx := context.Background()
	ok, err := cache.IncrementAccountWaitCount(ctx, 1, 2, "long-waiter")
	require.NoError(t, err)
	require.True(t, ok)
	advance(2 * time.Second)
	ok, err = cache.IncrementAccountWaitCount(ctx, 1, 2, "long-waiter")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 8*time.Second, server.TTL(cache.accountWaitKey(1)))
	shorter := NewConcurrencyCache(client, 15, 3)
	ok, err = shorter.IncrementAccountWaitCount(ctx, 1, 2, "short-waiter")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 8*time.Second, server.TTL(cache.accountWaitKey(1)))
	advance(4 * time.Second)
	count, err := cache.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	advance(5 * time.Second)
	count, err = cache.GetAccountWaitingCount(ctx, 1)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestWaitLeaseFactoryCoversHandlerDeadlines(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sticky, fallback time.Duration
		wantSeconds      int
	}{
		{name: "short account waits still cover user queue", sticky: time.Second, fallback: 2 * time.Second, wantSeconds: 31},
		{name: "fractional account timeout rounds up", sticky: 120500 * time.Millisecond, fallback: 30 * time.Second, wantSeconds: 122},
		{name: "longer fallback wins", sticky: 30 * time.Second, fallback: 180 * time.Second, wantSeconds: 181},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Gateway.Scheduling.StickySessionWaitTimeout = tc.sticky
			cfg.Gateway.Scheduling.FallbackWaitTimeout = tc.fallback
			cache := ProvideConcurrencyCache(nil, cfg).(*concurrencyCache)
			require.Equal(t, tc.wantSeconds, cache.waitQueueTTLSeconds)
		})
	}
}
