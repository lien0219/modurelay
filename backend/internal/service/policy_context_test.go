package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestEffectivePolicyResolverContextRoundTrip(t *testing.T) {
	resolver := domain.NewEffectivePolicyResolver(domain.NewMemoryPolicyStore())
	ctx := WithEffectivePolicyResolver(context.Background(), resolver)

	got, ok := EffectivePolicyResolverFromContext(ctx)
	require.True(t, ok)
	require.Same(t, resolver, got)
}
