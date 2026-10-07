package service

import (
	"context"
	"time"
)

// SessionAuthentication describes the original human authentication, never
// the time a refresh token was rotated. Legacy sessions retain a zero time.
type SessionAuthentication struct {
	AuthMethod      string
	AuthenticatedAt time.Time
	MFASatisfied    bool
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
	return auth.AuthMethod != "" && !auth.AuthenticatedAt.IsZero() && !auth.AuthenticatedAt.After(now.Add(time.Minute)) && now.Sub(auth.AuthenticatedAt) <= maxAge
}
