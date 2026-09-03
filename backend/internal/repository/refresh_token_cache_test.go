//go:build unit

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

func TestRefreshTokenCacheConsumeRefreshTokenAllowsOneConcurrentWinner(t *testing.T) {
	t.Parallel()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })

	cache := &refreshTokenCache{rdb: client}
	ctx := context.Background()
	const tokenHash = "concurrent-token"
	require.NoError(t, cache.StoreRefreshToken(ctx, tokenHash, &service.RefreshTokenData{
		UserID:    42,
		FamilyID:  "family-1",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour),
	}, time.Hour))

	var winners atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			consumed, err := cache.ConsumeRefreshToken(ctx, tokenHash)
			if err != nil {
				errs <- err
				return
			}
			if consumed {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(1), winners.Load())
	_, err := cache.GetRefreshToken(ctx, tokenHash)
	require.ErrorIs(t, err, service.ErrRefreshTokenNotFound)
}
