package service

import "context"

type policyActorContextKey struct{}

// WithPolicyActor attaches the authenticated human actor to policy mutations.
// Actor attribution is consumed only by the SQL policy repository and never
// comes from a request field.
func WithPolicyActor(ctx context.Context, actorID int64) context.Context {
	return context.WithValue(ctx, policyActorContextKey{}, actorID)
}

func PolicyActorID(ctx context.Context) int64 {
	actorID, _ := ctx.Value(policyActorContextKey{}).(int64)
	return actorID
}
