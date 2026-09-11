package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAPIKeyHealthCacheTripsWithinRollingWindow(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, ok := NewTempUnschedCache(client).(service.OpenAIAPIKeyHealthCache)
	require.True(t, ok)

	ctx := context.Background()
	for attempt := 1; attempt <= 3; attempt++ {
		count, tripped, err := store.RecordOpenAIAPIKeyHealthFailure(ctx, 42, 1, 3)
		require.NoError(t, err)
		require.EqualValues(t, attempt, count)
		require.Equal(t, attempt == 3, tripped)
	}
}

func TestOpenAIAPIKeyHealthCacheDropsFailuresOutsideRollingWindow(t *testing.T) {
	server := miniredis.RunT(t)
	now := time.Unix(1_700_000_000, 0)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, ok := NewTempUnschedCache(client).(service.OpenAIAPIKeyHealthCache)
	require.True(t, ok)

	ctx := context.Background()
	count, tripped, err := store.RecordOpenAIAPIKeyHealthFailure(ctx, 42, 1, 3)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.False(t, tripped)

	server.SetTime(now.Add(61 * time.Second))
	count, tripped, err = store.RecordOpenAIAPIKeyHealthFailure(ctx, 42, 1, 3)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.False(t, tripped)
}

func testAccountHealthPolicy() service.AccountHealthPolicy {
	return service.AccountHealthPolicy{
		MinimumSamples:      10,
		DegradedScore:       80,
		OpenScore:           45,
		ConsecutiveFailures: 3,
		BaseCooldown:        30 * time.Second,
		MaxCooldown:         10 * time.Minute,
		HalfOpenMaxProbes:   1,
		StateTTL:            24 * time.Hour,
		LocalCacheTTL:       500 * time.Millisecond,
	}
}

func TestAccountHealthCacheCircuitBreakerLifecycle(t *testing.T) {
	server := miniredis.RunT(t)
	now := time.Unix(1_700_000_000, 0)
	server.SetTime(now)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, ok := NewTempUnschedCache(client).(service.AccountHealthCache)
	require.True(t, ok)

	ctx := context.Background()
	policy := testAccountHealthPolicy()
	var snapshot *service.AccountHealthSnapshot
	for attempt := 1; attempt <= policy.ConsecutiveFailures; attempt++ {
		var err error
		snapshot, err = store.Record(ctx, service.AccountHealthEvent{
			AccountID:     77,
			FailureReason: "upstream_http_502",
		}, policy)
		require.NoError(t, err)
	}
	require.Equal(t, service.AccountHealthStateOpen, snapshot.State)
	require.Equal(t, policy.ConsecutiveFailures, snapshot.ConsecutiveFailures)
	require.Equal(t, 1, snapshot.OpenCount)
	require.Equal(t, now.Add(policy.BaseCooldown).Unix(), snapshot.OpenUntilUnix)
	require.Equal(t, "upstream_http_502", snapshot.LastFailureReason)

	server.FastForward(policy.BaseCooldown + time.Second)
	snapshot, err := store.Record(ctx, service.AccountHealthEvent{
		AccountID:     77,
		FailureReason: "transport_timeout",
	}, policy)
	require.NoError(t, err)
	require.Equal(t, service.AccountHealthStateOpen, snapshot.State)
	require.Equal(t, 2, snapshot.OpenCount)
	require.Equal(t, now.Add(2*policy.BaseCooldown).Unix(), snapshot.OpenUntilUnix)

	server.FastForward(2*policy.BaseCooldown + time.Second)
	snapshot, err = store.Record(ctx, service.AccountHealthEvent{
		AccountID: 77,
		Success:   true,
		LatencyMs: 250,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, service.AccountHealthStateWarming, snapshot.State)
	require.Equal(t, 100.0, snapshot.Score)
	require.Zero(t, snapshot.ConsecutiveFailures)
	require.Zero(t, snapshot.OpenCount)
	require.Zero(t, snapshot.OpenUntilUnix)
	require.Empty(t, snapshot.LastFailureReason)
}

func TestAccountHealthCacheLimitsHalfOpenProbesAcrossCallers(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewTempUnschedCache(client).(service.AccountHealthCache)
	ctx := context.Background()

	firstToken, acquired, err := store.AcquireProbe(ctx, 88, 1, 30*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotEmpty(t, firstToken)

	_, acquired, err = store.AcquireProbe(ctx, 88, 1, 30*time.Second)
	require.NoError(t, err)
	require.False(t, acquired)

	require.NoError(t, store.ReleaseProbe(ctx, 88, firstToken))
	_, acquired, err = store.AcquireProbe(ctx, 88, 1, 30*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)
}

func TestAccountHealthCacheGetBatchSkipsMissingAccounts(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewTempUnschedCache(client).(service.AccountHealthCache)
	ctx := context.Background()

	_, err := store.Record(ctx, service.AccountHealthEvent{AccountID: 91, Success: true}, testAccountHealthPolicy())
	require.NoError(t, err)
	snapshots, err := store.GetBatch(ctx, []int64{90, 91})
	require.NoError(t, err)
	require.NotContains(t, snapshots, int64(90))
	require.Contains(t, snapshots, int64(91))
}
