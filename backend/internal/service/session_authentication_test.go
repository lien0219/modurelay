package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRequireRecentAuthenticationKeepsOriginalTimeAndTrustedProofSeparate(t *testing.T) {
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name            string
		authenticatedAt time.Time
		verifiedAt      time.Time
		allowed         bool
	}{
		{"missing", time.Time{}, time.Time{}, false},
		{"original recent", now.Add(-9 * time.Minute), time.Time{}, true},
		{"original stale", now.Add(-11 * time.Minute), time.Time{}, false},
		{"trusted step-up", now.Add(-6 * time.Hour), now.Add(-time.Minute), true},
		{"expired step-up", now.Add(-6 * time.Hour), now.Add(-11 * time.Minute), false},
		{"future proof", time.Time{}, now.Add(2 * time.Minute), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: "password", AuthenticatedAt: tc.authenticatedAt})
			ctx = WithRecentAuthentication(ctx, tc.verifiedAt)
			err := RequireRecentAuthentication(ctx, now, 10*time.Minute)
			if tc.allowed {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrRecentAuthenticationRequired)
			}
			auth, _ := SessionAuthenticationFromContext(ctx)
			require.Equal(t, tc.authenticatedAt, auth.AuthenticatedAt)
			require.False(t, auth.MFASatisfied)
		})
	}
}

func TestRequireRecentAuthenticationRejectsWeakPrimaryAndEnrolledWithoutMFA(t *testing.T) {
	now := time.Now()
	for _, auth := range []SessionAuthentication{
		{AuthMethod: "other", AuthenticatedAt: now},
		{AuthMethod: "password", AuthenticatedAt: now, MFAEnrolled: true},
	} {
		ctx := WithSessionAuthentication(context.Background(), auth)
		require.ErrorIs(t, RequireRecentAuthentication(ctx, now, 10*time.Minute), ErrRecentAuthenticationRequired)
		require.NoError(t, RequireRecentAuthentication(WithRecentAuthentication(ctx, now, true), now, 10*time.Minute))
	}
}

func TestRequireRecentAuthenticationTimeOnlyProofDoesNotBecomeMFA(t *testing.T) {
	now := time.Now()
	ctx := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-time.Hour), MFAEnrolled: true})
	ctx = WithRecentAuthentication(ctx, now)
	require.ErrorIs(t, RequireRecentAuthentication(ctx, now, 10*time.Minute), ErrRecentAuthenticationRequired, "a password-only proof cannot satisfy the factor enrolled after the handler read")
	auth, _ := SessionAuthenticationFromContext(ctx)
	require.False(t, auth.MFASatisfied)
	require.Equal(t, now.Add(-time.Hour), auth.AuthenticatedAt)
	ctx = WithRecentAuthentication(ctx, now, true)
	require.NoError(t, RequireRecentAuthentication(ctx, now, 10*time.Minute), "verified recent TOTP remains separate from Session MFA")
	auth, _ = SessionAuthenticationFromContext(ctx)
	require.False(t, auth.MFASatisfied)
	require.Equal(t, now.Add(-time.Hour), auth.AuthenticatedAt)
}
