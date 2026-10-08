//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type sessionMFAUpgradeCache struct {
	refreshTokenConsumeCacheStub
	stored *RefreshTokenData
}

func (c *sessionMFAUpgradeCache) StoreRefreshToken(_ context.Context, _ string, data *RefreshTokenData, _ time.Duration) error {
	copy := *data
	c.stored = &copy
	return nil
}

func (c *sessionMFAUpgradeCache) ConsumeRefreshToken(context.Context, string) (bool, error) {
	if c.consumeErr != nil {
		return false, c.consumeErr
	}
	if !c.consume {
		return false, nil
	}
	c.consume = false
	return true, nil
}

type sessionMFAUpgradeVerifier struct {
	err error
}

func (v sessionMFAUpgradeVerifier) VerifyStepUp(_ context.Context, userID int64, sessionKey, code string) (time.Duration, error) {
	if userID != 42 || sessionKey != "family-1" || code != "246810" {
		return 0, ErrTotpInvalidCode
	}
	return StepUpGrantTTL, v.err
}

func newSessionMFAUpgradeFixture(t *testing.T) (*AuthService, *sessionMFAUpgradeCache, string) {
	t.Helper()
	data := validRefreshTokenData()
	data.AuthMethod = "saml"
	data.AuthenticatedAt = time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	data.OIDCAuthenticatedAt = data.AuthenticatedAt.Add(-time.Hour)
	data.OIDCProviderID, data.OIDCProviderRevision, data.OIDCWorkspaceID = 9, 4, 7
	data.OIDCValidUntil = time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	cache := &sessionMFAUpgradeCache{refreshTokenConsumeCacheStub: refreshTokenConsumeCacheStub{data: data, consume: true}}
	svc := newRefreshTokenConsumeAuthService(cache)
	svc.SetSessionMFAVerifier(sessionMFAUpgradeVerifier{})
	ctx := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: data.AuthMethod, AuthenticatedAt: data.AuthenticatedAt})
	ctx = WithAuthenticationAssurance(ctx, WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, ProviderRevision: 4, AuthMethod: "saml", AuthenticatedAt: data.OIDCAuthenticatedAt, ValidUntil: data.OIDCValidUntil})
	user, err := svc.userRepo.GetByID(ctx, 42)
	require.NoError(t, err)
	access, err := svc.generateAccessToken(ctx, user, "family-1", "")
	require.NoError(t, err)
	return svc, cache, access
}

func TestSessionMFAUpgradePreservesFamilyAndOriginalAssurance(t *testing.T) {
	svc, cache, access := newSessionMFAUpgradeFixture(t)
	pair, err := svc.UpgradeSessionMFA(context.Background(), access, "rt_original", "246810")
	require.NoError(t, err)
	claims, err := svc.ValidateToken(pair.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "family-1", claims.SessionID)
	require.True(t, claims.MFASatisfied)
	require.Equal(t, "saml", claims.AuthMethod)
	require.Equal(t, cache.data.AuthenticatedAt, claims.AuthenticatedAt)
	require.Equal(t, cache.data.OIDCAuthenticatedAt, claims.OIDCAuthenticatedAt)
	require.Equal(t, cache.data.OIDCValidUntil, claims.OIDCValidUntil)
	require.EqualValues(t, 7, claims.OIDCWorkspaceID)
	require.EqualValues(t, 9, claims.OIDCProviderID)
	require.EqualValues(t, 4, claims.OIDCProviderRevision)
	require.NotNil(t, cache.stored)
	require.True(t, cache.stored.MFASatisfied)
	require.Equal(t, cache.data.AuthenticatedAt, cache.stored.AuthenticatedAt)
	require.Equal(t, cache.data.OIDCValidUntil, cache.stored.OIDCValidUntil)
	_, err = svc.UpgradeSessionMFA(context.Background(), access, "rt_original", "246810")
	require.ErrorIs(t, err, ErrSessionMFAUpgradeReused)
}

func TestSessionMFAUpgradeRejectsMismatchedEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*RefreshTokenData)
	}{
		{"different family", func(d *RefreshTokenData) { d.FamilyID = "another-family" }},
		{"different user", func(d *RefreshTokenData) { d.UserID = 43 }},
		{"revoked version", func(d *RefreshTokenData) { d.TokenVersion = 8 }},
		{"changed authentication time", func(d *RefreshTokenData) { d.AuthenticatedAt = d.AuthenticatedAt.Add(time.Minute) }},
		{"different workspace", func(d *RefreshTokenData) { d.OIDCWorkspaceID = 8 }},
		{"changed provider revision", func(d *RefreshTokenData) { d.OIDCProviderRevision++ }},
		{"changed assertion expiry", func(d *RefreshTokenData) { d.OIDCValidUntil = time.Time{} }},
		{"expired refresh", func(d *RefreshTokenData) { d.ExpiresAt = time.Now().Add(-time.Second) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cache, access := newSessionMFAUpgradeFixture(t)
			tc.mutate(cache.data)
			pair, err := svc.UpgradeSessionMFA(context.Background(), access, "rt_original", "246810")
			require.Nil(t, pair)
			require.ErrorIs(t, err, ErrSessionMFAUpgradeInvalid)
			require.Nil(t, cache.stored)
			require.True(t, cache.consume)
		})
	}
}

func TestSessionMFAUpgradeNeedsVerifiedCodeAndAtomicConsumption(t *testing.T) {
	for _, tc := range []struct {
		name       string
		code       string
		verifyErr  error
		consumeErr error
		want       error
	}{
		{"wrong code", "000000", nil, nil, ErrTotpInvalidCode},
		{"verification unavailable", "246810", ErrServiceUnavailable, nil, ErrServiceUnavailable},
		{"consumption unavailable", "246810", nil, errors.New("redis unavailable"), ErrServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cache, access := newSessionMFAUpgradeFixture(t)
			svc.SetSessionMFAVerifier(sessionMFAUpgradeVerifier{err: tc.verifyErr})
			cache.consumeErr = tc.consumeErr
			pair, err := svc.UpgradeSessionMFA(context.Background(), access, "rt_original", tc.code)
			require.Nil(t, pair)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, cache.stored)
		})
	}
}
