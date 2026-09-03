//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type refreshTokenConsumeCacheStub struct {
	refreshTokenCacheStub
	data         *RefreshTokenData
	consume      bool
	consumeErr   error
	consumeCalls int
	storeCalls   int
}

type nonAtomicRefreshTokenCacheStub struct {
	refreshTokenCacheStub
	data *RefreshTokenData
}

func (s *nonAtomicRefreshTokenCacheStub) GetRefreshToken(context.Context, string) (*RefreshTokenData, error) {
	cloned := *s.data
	return &cloned, nil
}

func (s *refreshTokenConsumeCacheStub) GetRefreshToken(context.Context, string) (*RefreshTokenData, error) {
	if s.data == nil {
		return nil, ErrRefreshTokenNotFound
	}
	cloned := *s.data
	return &cloned, nil
}

func (s *refreshTokenConsumeCacheStub) ConsumeRefreshToken(context.Context, string) (bool, error) {
	s.consumeCalls++
	return s.consume, s.consumeErr
}

func (s *refreshTokenConsumeCacheStub) StoreRefreshToken(context.Context, string, *RefreshTokenData, time.Duration) error {
	s.storeCalls++
	return nil
}

func newRefreshTokenConsumeAuthService(cache RefreshTokenCache) *AuthService {
	user := &User{
		ID:                   42,
		Email:                "refresh@example.com",
		Role:                 RoleUser,
		Status:               StatusActive,
		TokenVersion:         7,
		TokenVersionResolved: true,
	}
	cfg := &config.Config{JWT: config.JWTConfig{
		Secret:                   "refresh-test-secret",
		ExpireHour:               1,
		AccessTokenExpireMinutes: 60,
		RefreshTokenExpireDays:   7,
	}}
	return NewAuthService(nil, &userRepoStub{user: user}, nil, cache, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
}

func validRefreshTokenData() *RefreshTokenData {
	return &RefreshTokenData{
		UserID:       42,
		TokenVersion: 7,
		FamilyID:     "family-1",
		CreatedAt:    time.Now().Add(-time.Minute),
		ExpiresAt:    time.Now().Add(time.Hour),
	}
}

func TestRefreshTokenPairRejectsAlreadyConsumedToken(t *testing.T) {
	cache := &refreshTokenConsumeCacheStub{data: validRefreshTokenData()}
	svc := newRefreshTokenConsumeAuthService(cache)

	result, err := svc.RefreshTokenPair(context.Background(), "rt_already-consumed")

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrRefreshTokenReused)
	require.Equal(t, 1, cache.consumeCalls)
	require.Zero(t, cache.storeCalls)
}

func TestRefreshTokenPairFailsClosedWhenConsumptionFails(t *testing.T) {
	cache := &refreshTokenConsumeCacheStub{
		data:       validRefreshTokenData(),
		consumeErr: errors.New("redis unavailable"),
	}
	svc := newRefreshTokenConsumeAuthService(cache)

	result, err := svc.RefreshTokenPair(context.Background(), "rt_redis-error")

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.Equal(t, 1, cache.consumeCalls)
	require.Zero(t, cache.storeCalls)
}

func TestRefreshTokenPairFailsClosedWithoutAtomicConsumer(t *testing.T) {
	cache := &nonAtomicRefreshTokenCacheStub{data: validRefreshTokenData()}
	svc := newRefreshTokenConsumeAuthService(cache)

	result, err := svc.RefreshTokenPair(context.Background(), "rt_non-atomic-cache")

	require.Nil(t, result)
	require.ErrorIs(t, err, ErrServiceUnavailable)
}

func TestRefreshTokenPairRotatesAfterAtomicConsumption(t *testing.T) {
	cache := &refreshTokenConsumeCacheStub{data: validRefreshTokenData(), consume: true}
	svc := newRefreshTokenConsumeAuthService(cache)

	result, err := svc.RefreshTokenPair(context.Background(), "rt_valid")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.AccessToken)
	require.NotEmpty(t, result.RefreshToken)
	require.Equal(t, 1, cache.consumeCalls)
	require.Equal(t, 1, cache.storeCalls)
}
