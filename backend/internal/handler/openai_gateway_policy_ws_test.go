package handler

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type wsPolicyQuotaRepo struct {
	mu           sync.Mutex
	next         int
	reservations []service.PolicyQuotaReservationRequest
	finalized    []int64
	released     []string
}

func (r *wsPolicyQuotaRepo) Reserve(_ context.Context, req service.PolicyQuotaReservationRequest) (*service.PolicyQuotaReservation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	r.reservations = append(r.reservations, req)
	return &service.PolicyQuotaReservation{
		ID:              fmt.Sprintf("ws-policy-%d", r.next),
		RequestID:       req.RequestID,
		APIKeyID:        req.APIKeyID,
		WorkspaceID:     req.WorkspaceID,
		ProjectID:       req.ProjectID,
		EstimatedTokens: req.EstimatedTokens,
		RequestUnits:    req.RequestUnits,
		Status:          service.PolicyQuotaReservationPending,
	}, nil
}

func (r *wsPolicyQuotaRepo) Finalize(_ context.Context, _ string, tokens int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finalized = append(r.finalized, tokens)
	return nil
}

func (r *wsPolicyQuotaRepo) Release(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.released = append(r.released, id)
	return nil
}

func TestCheckOpenAIWSPolicyChecksMappedModelAndPlatform(t *testing.T) {
	allowed := []string{"gpt-*"}
	platforms := []string{"openai"}
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: &domain.Policy{
		Scope:            domain.PolicyScopeWorkspace,
		ScopeID:          11,
		AllowedModels:    allowed,
		AllowedPlatforms: platforms,
	}})
	require.NoError(t, err)

	require.NoError(t, checkOpenAIWSPolicy(policy, "gpt-5.4", "openai"))
	require.Error(t, checkOpenAIWSPolicy(policy, "claude-4", "openai"))
	require.Error(t, checkOpenAIWSPolicy(policy, "gpt-5.4", "anthropic"))
}

func TestAdmitOpenAIWSPolicyTurnCreatesOneReservationPerLogicalTurn(t *testing.T) {
	limit := int64(10)
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: &domain.Policy{
		Scope:             domain.PolicyScopeWorkspace,
		ScopeID:           11,
		DailyRequestLimit: &limit,
	}})
	require.NoError(t, err)
	repo := &wsPolicyQuotaRepo{}
	quota := service.NewPolicyQuotaService(repo)
	ctx := service.WithPolicyQuotaService(context.Background(), quota)
	key := &service.APIKey{ID: 9, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}

	first, err := admitOpenAIWSPolicyTurn(ctx, key, policy, "ws:req:1", []byte(`{"model":"gpt-5.4"}`))
	require.NoError(t, err)
	second, err := admitOpenAIWSPolicyTurn(ctx, key, policy, "ws:req:2", []byte(`{"model":"gpt-5.4"}`))
	require.NoError(t, err)
	require.NotNil(t, first.handle)
	require.NotNil(t, second.handle)
	require.NotEqual(t, first.handle.ID(), second.handle.ID())
	require.NotEqual(t, service.PolicyQuotaReservationFromContext(first.ctx).ID(), service.PolicyQuotaReservationFromContext(second.ctx).ID())

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.reservations, 2)
}

func TestOpenAIWSPolicyTurnFailoverReusesSnapshotHandle(t *testing.T) {
	repo := &wsPolicyQuotaRepo{}
	quota := service.NewPolicyQuotaService(repo)
	limit := int64(20)
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: &domain.Policy{
		Scope:             domain.PolicyScopeWorkspace,
		ScopeID:           11,
		DailyRequestLimit: &limit,
	}})
	require.NoError(t, err)
	key := &service.APIKey{ID: 9, Tenant: &service.TenantContext{WorkspaceID: 11, ProjectID: 21}}
	parent := service.WithPolicyQuotaService(context.Background(), quota)
	snapshot, err := admitOpenAIWSPolicyTurn(parent, key, policy, "ws:logical-turn:3", []byte(`{"model":"gpt-5.4"}`))
	require.NoError(t, err)
	require.NotNil(t, snapshot.handle)
	snapshot.handle.MarkProviderStarted()
	snapshot.handle.Preserve()

	turns := map[int]openAIWSPolicyTurnSnapshot{3: snapshot}
	reused, ok := reuseOpenAIWSPolicyTurnSnapshot(turns, 3)
	require.True(t, ok)
	require.Same(t, snapshot.handle, reused.handle)
	require.Same(t, snapshot.ctx, reused.ctx)
	require.True(t, reused.handle.Durable())
	require.NoError(t, reused.handle.Finalize(context.Background(), 7))

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Len(t, repo.reservations, 1, "failover must reuse the admitted logical turn")
	require.Equal(t, []int64{7}, repo.finalized)
	require.Empty(t, repo.released)
}
