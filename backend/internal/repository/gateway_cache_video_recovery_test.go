//go:build unit

package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestVideoRecoverySnapshotSurvivesExtendedRechargeWait(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewGatewayCache(rdb).(*gatewayCache)
	ctx := context.Background()
	key := "10:20:seedance:pending-top-up"
	require.NoError(t, cache.SetGrokVideoPendingBilling(ctx, key, []byte(`{"request_id":"seedance:pending-top-up"}`), 24*time.Hour))
	require.NoError(t, cache.ScheduleGrokVideoRecovery(ctx, key, time.Now(), 24*time.Hour))
	server.FastForward(23*time.Hour + 59*time.Minute)
	require.NoError(t, cache.ScheduleGrokVideoRecovery(ctx, key, time.Now().Add(-time.Minute), 24*time.Hour))
	server.FastForward(30 * 24 * time.Hour)
	due, err := cache.ListDueGrokVideoRecovery(ctx, time.Now(), 50)
	require.NoError(t, err)
	require.Contains(t, due, key)
	payload, err := cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.NotEmpty(t, payload, "pending money events must survive retries and a long recovery outage")
}

func TestVideoSettlementCacheKeepsFirstCommandUnderConcurrency(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewGatewayCache(rdb).(*gatewayCache)
	ctx := context.Background()
	key := "10:20:seedance:race"
	var workers sync.WaitGroup
	results := make(chan []byte, 12)
	errors := make(chan error, 12)
	for i := 0; i < 12; i++ {
		payload := []byte(`{"request_id":"seedance:race","settlement":{"command":{"BalanceCost":1.25}}}`)
		if i%2 == 0 {
			payload = []byte(`{"request_id":"seedance:race","settlement":{"command":{"BalanceCost":9.5}}}`)
		}
		workers.Add(1)
		go func(payload []byte) {
			defer workers.Done()
			stored, err := cache.PrepareGrokVideoSettlement(ctx, key, payload)
			results <- stored
			errors <- err
		}(payload)
	}
	workers.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	stored, err := cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	for result := range results {
		require.Equal(t, stored, result)
	}
	server.FastForward(60 * 24 * time.Hour)
	storedAfterWait, err := cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.Equal(t, stored, storedAfterWait)
	due, err := cache.ListDueGrokVideoRecovery(ctx, time.Now().Add(time.Minute), 50)
	require.NoError(t, err)
	require.Contains(t, due, key)
	require.NoError(t, cache.CompleteGrokVideoRecovery(ctx, key, 24*time.Hour))
	due, err = cache.ListDueGrokVideoRecovery(ctx, time.Now().Add(time.Minute), 50)
	require.NoError(t, err)
	require.NotContains(t, due, key)
	server.FastForward(25 * time.Hour)
	storedAfterWait, err = cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.Empty(t, storedAfterWait, "terminal snapshots must be cleaned up")
}

func TestVideoRecoveryAdoptsLegacyLongClaimAsShortLease(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewGatewayCache(rdb).(*gatewayCache)
	ctx := context.Background()
	key := "10:20:seedance:crashed-worker"
	require.NoError(t, rdb.Set(ctx, grokVideoBilledPrefix+key, "1", 48*time.Hour).Err())
	claimed, err := cache.ClaimGrokVideoBilled(ctx, key, 2*time.Minute)
	require.NoError(t, err)
	require.False(t, claimed, "a live legacy worker must keep its claim")
	server.FastForward(3 * time.Minute)
	claimed, err = cache.ClaimGrokVideoBilled(ctx, key, 2*time.Minute)
	require.NoError(t, err)
	require.True(t, claimed, "an old crashed worker must not block recovery for 48 hours")
}

func TestVideoSettlementSurvivesLateCreateSnapshot(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewGatewayCache(rdb).(*gatewayCache)
	ctx := context.Background()
	key := "10:20:seedance:late-create"
	frozen := []byte(`{"request_id":"seedance:late-create","account_id":1,"settlement":{"command":{"BalanceCost":1.25}}}`)
	_, err := cache.PrepareGrokVideoSettlement(ctx, key, frozen)
	require.NoError(t, err)
	// The create response can reach the caller before its pending snapshot is
	// saved. A completed status poll must keep the first settlement authoritative.
	require.NoError(t, cache.SetGrokVideoPendingBilling(ctx, key, []byte(`{"request_id":"seedance:late-create","account_id":2}`), 24*time.Hour))
	stored, err := cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.Equal(t, frozen, stored)
	server.FastForward(30 * 24 * time.Hour)
	stored, err = cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.Equal(t, frozen, stored, "late creates must not restore a short expiry either")
}

func TestVideoCancellationKeepsAlreadyObservedSettlement(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewGatewayCache(rdb).(*gatewayCache)
	ctx := context.Background()
	key := "10:20:seedance:completed-before-delete"
	frozen := []byte(`{"request_id":"seedance:completed-before-delete","settlement":{"command":{"BalanceCost":1.25}}}`)
	_, err := cache.PrepareGrokVideoSettlement(ctx, key, frozen)
	require.NoError(t, err)
	require.NoError(t, cache.DeleteGrokVideoPendingBilling(ctx, key))
	stored, err := cache.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.Equal(t, frozen, stored, "deleting the upstream asset cannot waive an observed payable completion")
	due, err := cache.ListDueGrokVideoRecovery(ctx, time.Now().Add(time.Minute), 50)
	require.NoError(t, err)
	require.Contains(t, due, key)
}
