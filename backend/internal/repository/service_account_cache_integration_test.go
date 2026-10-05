//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestServiceAccountResponsesOwnerRedisRoundTrip(t *testing.T) {
	ctx := context.Background()
	cache := NewGatewayCache(testRedis(t))
	writer := service.NewOpenAIWSStateStore(cache)
	require.NoError(t, writer.BindHTTPResponseOwner(ctx, 4, "resp_machine", -9, 5, time.Minute))
	reader := service.NewOpenAIWSStateStore(cache)
	owner, key, found, err := reader.GetHTTPResponseOwner(ctx, 4, "resp_machine")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(-9), owner)
	require.Equal(t, int64(5), key)
	_, _, found, err = reader.GetHTTPResponseOwner(ctx, 7, "resp_machine")
	require.ErrorIs(t, err, service.ErrStickySessionNotFound)
	require.False(t, found)
}
