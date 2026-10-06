package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveEffectivePolicyUsesTriStatePredicatesAndNumericMinimum(t *testing.T) {
	rpmWorkspace := int64(100)
	rpmProject := int64(50)
	rpmCredential := int64(10)
	policies := PolicyLayers{
		Group: &Policy{
			Scope:            PolicyScopeGroup,
			ScopeID:          1,
			Revision:         4,
			AllowedModels:    []string{"gpt-6"},
			AllowedPlatforms: []string{"openai", "anthropic"},
		},
		Workspace: &Policy{
			Scope:            PolicyScopeWorkspace,
			ScopeID:          2,
			Revision:         7,
			AllowedModels:    nil, // inherit; it must not become deny-all.
			AllowedPlatforms: []string{"openai"},
			RPMLimit:         &rpmWorkspace,
		},
		Project: &Policy{
			Scope:         PolicyScopeProject,
			ScopeID:       3,
			Revision:      9,
			AllowedModels: []string{"gpt-6", "claude-4"},
			RPMLimit:      &rpmProject,
		},
		Credential: &Policy{
			Scope:    PolicyScopeCredential,
			ScopeID:  5,
			Revision: 2,
			RPMLimit: &rpmCredential,
		},
	}

	effective, err := ResolveEffectivePolicy(policies)
	if err != nil {
		t.Fatalf("resolve policy: %v", err)
	}
	if !effective.AllowsModel("gpt-6") {
		t.Fatal("gpt-6 should be allowed by every policy layer")
	}
	if effective.AllowsModel("claude-4") {
		t.Fatal("project allowance must not broaden the group model restriction")
	}
	if !effective.AllowsPlatform("openai") {
		t.Fatal("openai should be allowed by every policy layer")
	}
	if effective.AllowsPlatform("anthropic") {
		t.Fatal("workspace platform restriction must be applied as an AND predicate")
	}
	if got := effective.RPMLimitValue(); got != 10 {
		t.Fatalf("effective RPM = %d, want 10", got)
	}
	if got := effective.Revision(PolicyScopeWorkspace); got != 7 {
		t.Fatalf("workspace revision = %d, want 7", got)
	}
}

func TestResolveEffectivePolicyRejectsMismatchedLayerScope(t *testing.T) {
	_, err := ResolveEffectivePolicy(PolicyLayers{
		Workspace: &Policy{Scope: PolicyScopeProject, ScopeID: 2},
	})
	if !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("mismatched layer error = %v, want ErrInvalidPolicy", err)
	}
}

func TestPolicyValidationRejectsInvalidNumericAndListValues(t *testing.T) {
	zero := int64(0)
	if err := (Policy{RPMLimit: &zero}).Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("zero RPM validation error = %v, want ErrInvalidPolicy", err)
	}
	if err := (Policy{AllowedModels: []string{"  "}}).Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("blank model validation error = %v, want ErrInvalidPolicy", err)
	}
	if err := (Policy{AllowedPlatforms: []string{"openai", ""}}).Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("blank platform validation error = %v, want ErrInvalidPolicy", err)
	}
	if err := (Policy{AllowedModels: make([]string, 1001)}).Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("over-limit model allowlist error = %v, want ErrInvalidPolicy", err)
	}
	if err := (Policy{AllowedPlatforms: make([]string, 257)}).Validate(); !errors.Is(err, ErrInvalidPolicy) {
		t.Fatalf("over-limit platform allowlist error = %v, want ErrInvalidPolicy", err)
	}
}

func TestMemoryPolicyStoreUsesOptimisticRevision(t *testing.T) {
	store := NewMemoryPolicyStore()
	ref := PolicyRef{Scope: PolicyScopeWorkspace, ScopeID: 22}
	initial := Policy{AllowedModels: []string{"gpt-6"}}

	created, err := store.UpdatePolicy(context.Background(), ref, 0, initial)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	if created.Revision != 1 {
		t.Fatalf("created revision = %d, want 1", created.Revision)
	}
	if _, err = store.UpdatePolicy(context.Background(), ref, 0, initial); !errors.Is(err, ErrPolicyRevisionConflict) {
		t.Fatalf("stale update error = %v, want ErrPolicyRevisionConflict", err)
	}
	updated, err := store.UpdatePolicy(context.Background(), ref, 1, Policy{AllowedModels: []string{"gpt-6", "claude-4"}})
	if err != nil {
		t.Fatalf("current update: %v", err)
	}
	if updated.Revision != 2 {
		t.Fatalf("updated revision = %d, want 2", updated.Revision)
	}
	got, err := store.GetPolicy(context.Background(), ref)
	if err != nil {
		t.Fatalf("get policy: %v", err)
	}
	if got == nil || len(got.AllowedModels) != 2 {
		t.Fatalf("stored policy = %#v, want two allowed models", got)
	}
}

func TestMemoryPolicyStorePreservesEmptyAllowlistAsDenyAll(t *testing.T) {
	store := NewMemoryPolicyStore()
	ref := PolicyRef{Scope: PolicyScopeWorkspace, ScopeID: 44}
	if _, err := store.UpdatePolicy(context.Background(), ref, 0, Policy{AllowedModels: []string{}}); err != nil {
		t.Fatalf("create deny-all policy: %v", err)
	}
	got, err := store.GetPolicy(context.Background(), ref)
	if err != nil {
		t.Fatalf("get deny-all policy: %v", err)
	}
	if got == nil || got.AllowedModels == nil {
		t.Fatalf("empty allowlist lost tri-state semantics: %#v", got)
	}
	if got.AllowsModel("gpt-6") {
		t.Fatal("non-nil empty allowlist must deny every model")
	}
}

func TestPolicyAllowsModelPreservesWildcardSemantics(t *testing.T) {
	policy := Policy{AllowedModels: []string{"gpt-*", "*sonnet*"}}
	if !policy.AllowsModel("gpt-6") {
		t.Fatal("expected prefix wildcard to allow model")
	}
	if !policy.AllowsModel("claude-sonnet-4") {
		t.Fatal("expected infix wildcard to allow model")
	}
	if policy.AllowsModel("gemini-2.5") {
		t.Fatal("unexpected wildcard match")
	}
}

func TestEffectivePolicyResolverLoadsLayersAndTreatsMissingAsInherit(t *testing.T) {
	store := NewMemoryPolicyStore()
	rpm := int64(25)
	ref := PolicyRef{Scope: PolicyScopeProject, ScopeID: 33}
	if _, err := store.UpdatePolicy(context.Background(), ref, 0, Policy{RPMLimit: &rpm}); err != nil {
		t.Fatalf("seed project policy: %v", err)
	}

	resolver := NewEffectivePolicyResolver(store)
	effective, err := resolver.Resolve(context.Background(), PolicyContext{WorkspaceID: 11, ProjectID: 33})
	if err != nil {
		t.Fatalf("resolve layers: %v", err)
	}
	if effective.RPMLimitValue() != 25 {
		t.Fatalf("effective RPM = %d, want 25", effective.RPMLimitValue())
	}
	if !effective.AllowsModel("any-model") || !effective.AllowsPlatform("any-platform") {
		t.Fatal("missing policies must inherit unrestricted model/platform access")
	}
}

func TestDirectAPIKeyPolicyInheritanceExcludesServiceAccountLayer(t *testing.T) {
	store := NewMemoryPolicyStore()
	serviceAccountRPM := int64(1)
	workspaceRPM := int64(25)
	projectRPM := int64(10)

	// A service-account policy may exist for the same tenant while this key is
	// still an ordinary user credential. It must not enter this resolution.
	_, err := store.UpdatePolicy(context.Background(), PolicyRef{Scope: PolicyScopeServiceAccount, ScopeID: 31}, 0, Policy{
		AllowedModels: []string{},
		RPMLimit:      &serviceAccountRPM,
	})
	require.NoError(t, err)
	_, err = store.UpdatePolicy(context.Background(), PolicyRef{Scope: PolicyScopeWorkspace, ScopeID: 11}, 0, Policy{RPMLimit: &workspaceRPM})
	require.NoError(t, err)
	_, err = store.UpdatePolicy(context.Background(), PolicyRef{Scope: PolicyScopeProject, ScopeID: 21}, 0, Policy{RPMLimit: &projectRPM})
	require.NoError(t, err)

	resolver := NewEffectivePolicyResolver(store)
	effective, err := resolver.Resolve(context.Background(), PolicyContext{
		WorkspaceID: 11,
		ProjectID:   21,
		APIKeyID:    51,
	})
	require.NoError(t, err)
	require.Nil(t, effective.Layers.ServiceAccount)
	require.Nil(t, effective.Layers.Credential)
	require.True(t, effective.AllowsModel("gpt-6"), "missing policy fields must inherit unrestricted model access")
	require.True(t, effective.AllowsPlatform("openai"), "missing policy fields must inherit unrestricted platform access")
	require.Equal(t, int64(10), effective.RPMLimitValue(), "workspace/project limits still compose for a direct key")
}

func TestDirectAPIKeyExplicitPolicyLayersComposeWithoutMachinePolicy(t *testing.T) {
	groupRPM := int64(100)
	workspaceRPM := int64(50)
	projectRPM := int64(20)
	credentialRPM := int64(5)
	serviceAccountRPM := int64(1)

	effective, err := ResolveEffectivePolicy(PolicyLayers{
		Group:          &Policy{Scope: PolicyScopeGroup, ScopeID: 7, AllowedModels: []string{"gpt-6"}, RPMLimit: &groupRPM},
		Workspace:      &Policy{Scope: PolicyScopeWorkspace, ScopeID: 11, RPMLimit: &workspaceRPM},
		Project:        &Policy{Scope: PolicyScopeProject, ScopeID: 21, AllowedPlatforms: []string{"openai"}, RPMLimit: &projectRPM},
		Credential:     &Policy{Scope: PolicyScopeCredential, ScopeID: 51, RPMLimit: &credentialRPM},
		ServiceAccount: &Policy{Scope: PolicyScopeServiceAccount, ScopeID: 31, RPMLimit: &serviceAccountRPM},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), effective.RPMLimitValue(), "a service-account layer would incorrectly narrow this direct key")
	// The resolver itself is a general composition primitive; the direct-key
	// compatibility path is responsible for omitting the machine layer before
	// calling it. Re-resolve the user path to assert the intended four layers.
	withoutMachine, err := ResolveEffectivePolicy(PolicyLayers{
		Group:      effective.Layers.Group,
		Workspace:  effective.Layers.Workspace,
		Project:    effective.Layers.Project,
		Credential: effective.Layers.Credential,
	})
	require.NoError(t, err)
	require.Nil(t, withoutMachine.Layers.ServiceAccount)
	require.Equal(t, int64(5), withoutMachine.RPMLimitValue())
	require.True(t, withoutMachine.AllowsModel("gpt-6"))
	require.False(t, withoutMachine.AllowsModel("claude-4"))
	require.True(t, withoutMachine.AllowsPlatform("openai"))
	require.False(t, withoutMachine.AllowsPlatform("anthropic"))
}
