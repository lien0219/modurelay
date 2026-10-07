package service

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeEnterpriseDomainCanonicalizesIDNA(t *testing.T) {
	got, err := NormalizeEnterpriseDomain("  EXAMPLE.COM. ")
	require.NoError(t, err)
	require.Equal(t, "example.com", got)

	got, err = NormalizeEnterpriseDomain("bücher.example")
	require.NoError(t, err)
	require.Equal(t, "xn--bcher-kva.example", got)
}

func TestNormalizeEnterpriseDomainRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"", "localhost", "https://example.com", "bad..example.com", "example.com/path", "127.0.0.1"} {
		_, err := NormalizeEnterpriseDomain(input)
		require.Error(t, err, input)
	}
}

func TestValidateEnterpriseReturnToOnlyAllowsRelativePaths(t *testing.T) {
	for _, input := range []string{"/workspaces/7/security", "/workspaces", "/dashboard?next=%2F", "/profile#fragment"} {
		require.NoError(t, ValidateEnterpriseReturnTo(input), input)
	}
	for _, input := range []string{"https://evil.example", "//evil.example", "/\\evil.example", "", "workspaces/7"} {
		require.Error(t, ValidateEnterpriseReturnTo(input), input)
	}
}

func TestConsumeOIDCStateIsSingleUse(t *testing.T) {
	store := &memoryEnterpriseIdentityStore{states: make(map[string]memoryOIDCState)}
	now := time.Unix(100, 0)
	state, err := NewOIDCState(now, 10*time.Minute, 7, 9, "/workspaces/7", "login")
	require.NoError(t, err)
	store.states[state.Hash] = memoryOIDCState{Hash: state.Hash, ExpiresAt: state.ExpiresAt}

	got, err := store.ConsumeOIDCState(state.Hash, now.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, state.Hash, got.Hash)
	_, err = store.ConsumeOIDCState(state.Hash, now.Add(2*time.Minute))
	require.ErrorIs(t, err, ErrOIDCStateConsumed)
}

func TestMapOIDCRoleNeverGrantsOwnerOrOverwritesManualRole(t *testing.T) {
	role, managed := MapOIDCRole([]string{"owner", "admin"}, []OIDCRoleMapping{{ClaimValue: "owner", Role: WorkspaceRoleOwner, Priority: 100}, {ClaimValue: "admin", Role: WorkspaceRoleAdmin, Priority: 10}}, WorkspaceRoleViewer, false)
	require.Equal(t, WorkspaceRoleAdmin, role)
	require.True(t, managed)

	role, managed = MapOIDCRole([]string{"admin"}, []OIDCRoleMapping{{ClaimValue: "admin", Role: WorkspaceRoleAdmin, Priority: 10}}, WorkspaceRoleViewer, true)
	require.Equal(t, WorkspaceRoleViewer, role)
	require.False(t, managed)
}

func TestReconcileOIDCTeamsPreservesManualAndMissingClaims(t *testing.T) {
	existing := []OIDCTeamMembership{{TeamID: 1, Source: MembershipSourceManual}, {TeamID: 2, Source: MembershipSourceOIDC}, {TeamID: 3, Source: MembershipSourceSCIM}}
	got := ReconcileOIDCTeams(existing, nil, false)
	require.ElementsMatch(t, existing, got, "missing group claim must preserve all sources")

	got = ReconcileOIDCTeams(existing, []int64{4}, true)
	require.ElementsMatch(t, []OIDCTeamMembership{{TeamID: 1, Source: MembershipSourceManual}, {TeamID: 3, Source: MembershipSourceSCIM}, {TeamID: 4, Source: MembershipSourceOIDC}}, got)
}

func TestRequireWorkspaceAssuranceKeepsPersonalAndMachinePathsUnaffected(t *testing.T) {
	policy := WorkspaceSecurityPolicy{RequireSSO: true}
	require.NoError(t, RequireWorkspaceAssurance(policy, WorkspaceTypePersonal, PrincipalHuman, WorkspaceAssurance{}))
	require.NoError(t, RequireWorkspaceAssurance(policy, WorkspaceTypeOrganization, PrincipalAPIKey, WorkspaceAssurance{}))
	require.NoError(t, RequireWorkspaceAssurance(policy, WorkspaceTypeOrganization, PrincipalServiceAccount, WorkspaceAssurance{}))
	require.ErrorIs(t, RequireWorkspaceAssurance(policy, WorkspaceTypeOrganization, PrincipalHuman, WorkspaceAssurance{}), ErrSSORequired)
	require.NoError(t, RequireWorkspaceAssurance(policy, WorkspaceTypeOrganization, PrincipalHuman, WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, AuthenticatedAt: time.Now()}))
}

type memoryOIDCState struct {
	Hash      string
	Consumed  bool
	ExpiresAt time.Time
}

type memoryEnterpriseIdentityStore struct {
	states map[string]memoryOIDCState
}

func (m *memoryEnterpriseIdentityStore) ConsumeOIDCState(hash string, now time.Time) (memoryOIDCState, error) {
	s, ok := m.states[hash]
	if !ok {
		return memoryOIDCState{}, ErrOIDCStateNotFound
	}
	if s.Consumed {
		return memoryOIDCState{}, ErrOIDCStateConsumed
	}
	if !now.Before(s.ExpiresAt) {
		return memoryOIDCState{}, ErrOIDCStateExpired
	}
	s.Consumed = true
	m.states[hash] = s
	return s, nil
}

func TestEnterpriseIdentitySentinelErrorsAreDistinct(t *testing.T) {
	require.False(t, errors.Is(ErrOIDCStateConsumed, ErrOIDCStateExpired))
}
