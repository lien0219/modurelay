package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newRedisLoginAbuseStoreTest(t *testing.T) (*RedisLoginAbuseStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisLoginAbuseStore(client), server
}

func TestRedisLoginAbuseStoreRequestCounterIsAtomicAndAuditsOneThrottle(t *testing.T) {
	store, _ := newRedisLoginAbuseStoreTest(t)
	const attempts = 100
	var blocked atomic.Int64
	var audited atomic.Int64
	var wait sync.WaitGroup
	errs := make(chan error, attempts)
	for range attempts {
		wait.Add(1)
		go func() {
			defer wait.Done()
			count, _, audit, err := store.IncrementRequest(context.Background(), "req:ip", "req:ip:audit", time.Minute, 20)
			if err != nil {
				errs <- err
				return
			}
			if count > 20 {
				blocked.Add(1)
			}
			if audit {
				audited.Add(1)
			}
		}()
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 80, blocked.Load())
	require.EqualValues(t, 1, audited.Load())
}

func TestRedisLoginAbuseStoreFailureThresholdBlockAndTTL(t *testing.T) {
	store, server := newRedisLoginAbuseStoreTest(t)
	ctx := context.Background()
	for attempt := 1; attempt <= 5; attempt++ {
		count, blocked, err := store.IncrementFailure(ctx, "account-ip:fail", "account-ip:block", 30*time.Minute, 5, 30*time.Minute)
		require.NoError(t, err)
		require.EqualValues(t, attempt, count)
		require.Equal(t, attempt == 5, blocked)
	}
	accountIPTTL, accountTTL, err := store.CredentialBlockTTLs(ctx, "account-ip:block", "account:block")
	require.NoError(t, err)
	require.Positive(t, accountIPTTL)
	require.Zero(t, accountTTL)
	require.Positive(t, server.TTL("account-ip:fail"))

	server.FastForward(30*time.Minute + time.Second)
	accountIPTTL, accountTTL, err = store.CredentialBlockTTLs(ctx, "account-ip:block", "account:block")
	require.NoError(t, err)
	require.Zero(t, accountIPTTL)
	require.Zero(t, accountTTL)
	count, blocked, err := store.IncrementFailure(ctx, "account-ip:fail", "account-ip:block", 30*time.Minute, 5, 30*time.Minute)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.False(t, blocked)
}

func TestRedisLoginAbuseStoreSuccessDeletesOnlySourceCounter(t *testing.T) {
	store, _ := newRedisLoginAbuseStoreTest(t)
	protector := service.NewLoginAbuseProtector(store, nil)
	ctx := context.Background()

	_, err := protector.RecordPasswordFailure(ctx, "user@example.com", "203.0.113.12")
	require.NoError(t, err)
	require.NoError(t, protector.RecordSuccess(ctx, "user@example.com", "203.0.113.12"))
	outcome, err := protector.RecordPasswordFailure(ctx, "user@example.com", "203.0.113.12")
	require.NoError(t, err)
	require.EqualValues(t, 1, outcome.AccountIPCount)
	require.EqualValues(t, 2, outcome.AccountCount)
}

func TestRedisLoginAbuseStoreFailsClosedOnRedisError(t *testing.T) {
	store, server := newRedisLoginAbuseStoreTest(t)
	server.Close()
	ctx := context.Background()

	_, _, _, err := store.IncrementRequest(ctx, "req:ip", "req:ip:audit", time.Minute, 20)
	require.Error(t, err)
	_, _, err = store.CredentialBlockTTLs(ctx, "account-ip:block", "account:block")
	require.Error(t, err)
	_, _, err = store.IncrementFailure(ctx, "account-ip:fail", "account-ip:block", 30*time.Minute, 5, 30*time.Minute)
	require.Error(t, err)
	require.Error(t, store.DeleteCounter(ctx, "account-ip:fail"))
}
