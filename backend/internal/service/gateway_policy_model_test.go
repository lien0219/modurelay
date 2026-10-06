package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestGatewayModelSelectionRejectsAliasToPolicyDeniedUpstreamModel(t *testing.T) {
	allowed := domain.Policy{Scope: domain.PolicyScopeWorkspace, ScopeID: 11, AllowedModels: []string{"public-gpt"}}
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: &allowed})
	if err != nil {
		t.Fatalf("resolve policy: %v", err)
	}
	ctx := WithEffectivePolicy(context.Background(), policy)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"public-gpt": "private-claude"},
		},
	}

	if (&GatewayService{}).isModelSupportedByAccountWithContext(ctx, account, "public-gpt") {
		t.Fatal("alias mapped to a policy-denied upstream model must not remain a candidate")
	}
}

func TestGatewayModelSelectionAllowsMappedModelWhenPolicyAllowsFinalModel(t *testing.T) {
	allowed := domain.Policy{Scope: domain.PolicyScopeWorkspace, ScopeID: 11, AllowedModels: []string{"public-gpt", "gpt-6"}}
	policy, err := domain.ResolveEffectivePolicy(domain.PolicyLayers{Workspace: &allowed})
	if err != nil {
		t.Fatalf("resolve policy: %v", err)
	}
	ctx := WithEffectivePolicy(context.Background(), policy)
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"public-gpt": "gpt-6"},
		},
	}

	if !(&GatewayService{}).isModelSupportedByAccountWithContext(ctx, account, "public-gpt") {
		t.Fatal("alias mapped to an allowed upstream model should remain a candidate")
	}
}
