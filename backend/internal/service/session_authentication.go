package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// SessionAuthentication describes the original human authentication, never
// the time a refresh token was rotated. Legacy sessions retain a zero time.
type SessionAuthentication struct {
	AuthMethod      string
	AuthenticatedAt time.Time
	MFASatisfied    bool
	// MFAEnrolled is a live hint for recovery UI, never authentication proof.
	MFAEnrolled bool
}

type sessionAuthenticationContextKey struct{}

func WithSessionAuthentication(ctx context.Context, auth SessionAuthentication) context.Context {
	return context.WithValue(ctx, sessionAuthenticationContextKey{}, auth)
}

func SessionAuthenticationFromContext(ctx context.Context) (SessionAuthentication, bool) {
	if ctx == nil {
		return SessionAuthentication{}, false
	}
	auth, ok := ctx.Value(sessionAuthenticationContextKey{}).(SessionAuthentication)
	return auth, ok
}

func (auth SessionAuthentication) Recent(now time.Time, maxAge time.Duration) bool {
	return maxAge > 0 && auth.AuthMethod != "" && !auth.AuthenticatedAt.IsZero() && !auth.AuthenticatedAt.After(now.Add(time.Minute)) && now.Sub(auth.AuthenticatedAt) <= maxAge
}

var ErrRecentAuthenticationRequired = infraerrors.Forbidden("RECENT_AUTH_REQUIRED", "recent authentication is required for this operation")

type recentAuthenticationContextKey struct{}

// RecentAuthenticationProof records verified sensitive-operation evidence.
type RecentAuthenticationProof struct {
	VerifiedAt   time.Time
	MFASatisfied bool
}

// WithRecentAuthentication carries proof checked by a sensitive-operation
// handler. It does not upgrade session MFA or change original authentication.
// Omitting MFA strength records only primary authentication proof.
func WithRecentAuthentication(ctx context.Context, verifiedAt time.Time, mfaSatisfied ...bool) context.Context {
	proof := RecentAuthenticationProof{VerifiedAt: verifiedAt, MFASatisfied: len(mfaSatisfied) > 0 && mfaSatisfied[0]}
	return context.WithValue(ctx, recentAuthenticationContextKey{}, proof)
}

func RecentAuthenticationProofFromContext(ctx context.Context) (RecentAuthenticationProof, bool) {
	if ctx == nil {
		return RecentAuthenticationProof{}, false
	}
	proof, ok := ctx.Value(recentAuthenticationContextKey{}).(RecentAuthenticationProof)
	return proof, ok
}

func RequireRecentAuthentication(ctx context.Context, now time.Time, maxAge time.Duration) error {
	if maxAge <= 0 || ctx == nil {
		return ErrRecentAuthenticationRequired
	}
	auth, _ := SessionAuthenticationFromContext(ctx)
	if auth.Recent(now, maxAge) && (!auth.MFAEnrolled || auth.MFASatisfied) {
		switch auth.AuthMethod {
		case "password", "oidc", "saml", "passkey":
			return nil
		}
	}
	if proof, ok := RecentAuthenticationProofFromContext(ctx); ok && !proof.VerifiedAt.IsZero() && !proof.VerifiedAt.After(now.Add(time.Minute)) && now.Sub(proof.VerifiedAt) <= maxAge && (!auth.MFAEnrolled || proof.MFASatisfied) {
		return nil
	}
	return ErrRecentAuthenticationRequired
}
