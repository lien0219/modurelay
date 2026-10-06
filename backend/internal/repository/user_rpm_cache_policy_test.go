package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestUserRPMCacheAdmitMultiScopeRPMIsAtomic(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewUserRPMCache(client)
	multi, ok := cache.(service.MultiScopeRPMCache)
	require.True(t, ok, "production RPM cache must implement multi-scope admission")

	counters := []service.RPMCounter{
		{Key: "u:71", Scope: "user", Limit: 5},
		{Key: "policy:workspace:11", Scope: "workspace", Limit: 100},
	}
	const requests = 32
	results := make(chan service.RPMAdmissionResult, requests)
	errs := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := multi.AdmitMultiScopeRPM(context.Background(), counters)
			if err != nil {
				errs <- err
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	allowed := 0
	for result := range results {
		if result.Allowed {
			allowed++
			continue
		}
		require.Equal(t, "user", result.Scope)
		require.EqualValues(t, 5, result.Limit)
	}
	require.Equal(t, 5, allowed)
	userKeys, err := client.Keys(context.Background(), "rpm:u:71:*").Result()
	require.NoError(t, err)
	require.Len(t, userKeys, 1)
	userCount, err := client.Get(context.Background(), userKeys[0]).Int()
	require.NoError(t, err)
	workspaceKeys, err := client.Keys(context.Background(), "rpm:policy:workspace:11:*").Result()
	require.NoError(t, err)
	require.Len(t, workspaceKeys, 1)
	workspaceCount, err := client.Get(context.Background(), workspaceKeys[0]).Int()
	require.NoError(t, err)
	require.Equal(t, requests, userCount, "rejected attempts count consistently across scopes")
	require.Equal(t, requests, workspaceCount)
}
