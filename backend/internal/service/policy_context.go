package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type effectivePolicyContextKey struct{}

// An authenticated request carries the resolver used by gateway admission so
// long-lived transports can re-evaluate policy without trusting client tenant
// metadata. The pointer is immutable and its repository is safe to share.
type effectivePolicyResolverContextKey struct{}

func WithEffectivePolicyResolver(ctx context.Context, resolver *domain.EffectivePolicyResolver) context.Context {
	if resolver == nil {
		return ctx
	}
	return context.WithValue(ctx, effectivePolicyResolverContextKey{}, resolver)
}

func EffectivePolicyResolverFromContext(ctx context.Context) (*domain.EffectivePolicyResolver, bool) {
	if ctx == nil {
		return nil, false
	}
	resolver, ok := ctx.Value(effectivePolicyResolverContextKey{}).(*domain.EffectivePolicyResolver)
	return resolver, ok && resolver != nil
}

func WithEffectivePolicy(ctx context.Context, policy domain.EffectivePolicy) context.Context {
	return context.WithValue(ctx, effectivePolicyContextKey{}, policy)
}

func EffectivePolicyFromContext(ctx context.Context) (domain.EffectivePolicy, bool) {
	if ctx == nil {
		return domain.EffectivePolicy{}, false
	}
	policy, ok := ctx.Value(effectivePolicyContextKey{}).(domain.EffectivePolicy)
	return policy, ok
}

func EffectivePolicyPlatformFilter(ctx context.Context) func(string) bool {
	policy, ok := EffectivePolicyFromContext(ctx)
	if !ok {
		return nil
	}
	for _, layer := range []*domain.Policy{
		policy.Layers.Group,
		policy.Layers.Workspace,
		policy.Layers.Project,
		policy.Layers.ServiceAccount,
		policy.Layers.Credential,
	} {
		if layer != nil && layer.AllowedPlatforms != nil {
			return policy.AllowsPlatform
		}
	}
	return nil
}
