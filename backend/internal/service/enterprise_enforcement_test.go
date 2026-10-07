package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type enterpriseEnforcementRepo struct {
	EnterpriseIdentityRepository
	policy      WorkspaceIdentityPolicy
	provider    EnterpriseIdentityProvider
	policyCalls int
}

func (r *enterpriseEnforcementRepo) GetPolicy(context.Context, int64, int64) (*WorkspaceIdentityPolicy, error) {
	r.policyCalls++
	return &r.policy, nil
}
func (r *enterpriseEnforcementRepo) GetProvider(_ context.Context, workspaceID, _, providerID int64) (*EnterpriseIdentityProvider, string, error) {
	if workspaceID != r.provider.WorkspaceID || providerID != r.provider.ID {
		return nil, "", ErrWorkspaceNotFound
	}
	return &r.provider, "", nil
}

func TestEnterpriseEnforcementRejectsOtherWorkspaceStaleOrDisabledAssurance(t *testing.T) {
	now := time.Now().UTC()
	repo := &enterpriseEnforcementRepo{policy: WorkspaceIdentityPolicy{WorkspaceID: 7, RequireSSO: true}, provider: EnterpriseIdentityProvider{ID: 9, WorkspaceID: 7, Status: "active"}}
	svc := NewEnterpriseIdentityService(repo, nil, nil, nil)
	svc.SetClock(func() time.Time { return now })
	valid := WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, AuthMethod: "oidc", AuthenticatedAt: now.Add(-time.Hour)}
	require.NoError(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, valid))
	for _, assurance := range []WorkspaceAssurance{{}, {WorkspaceID: 8, ProviderID: 9, AuthMethod: "oidc", AuthenticatedAt: now}, {WorkspaceID: 7, ProviderID: 9, AuthMethod: "password", AuthenticatedAt: now}, {WorkspaceID: 7, ProviderID: 9, AuthMethod: "oidc", AuthenticatedAt: now.Add(-13 * time.Hour)}, {WorkspaceID: 7, ProviderID: 9, AuthMethod: "oidc", AuthenticatedAt: now.Add(2 * time.Minute)}} {
		require.ErrorIs(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, assurance), ErrSSORequired)
	}
	repo.provider.Status = "disabled"
	require.ErrorIs(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, valid), ErrSSORequired)
	for _, principal := range []string{PrincipalAPIKey, PrincipalServiceAccount} {
		require.NoError(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, principal, WorkspaceAssurance{}))
	}
	require.NoError(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypePersonal, PrincipalHuman, WorkspaceAssurance{}))
}

func TestEnterpriseEnforcementGraceEndsAtBoundary(t *testing.T) {
	now := time.Now().UTC()
	repo := &enterpriseEnforcementRepo{policy: WorkspaceIdentityPolicy{WorkspaceID: 7, RequireSSO: true, SSOGraceUntil: &now}}
	svc := NewEnterpriseIdentityService(repo, nil, nil, nil)
	svc.SetClock(func() time.Time { return now.Add(-time.Second) })
	require.NoError(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, WorkspaceAssurance{}))
	svc.SetClock(func() time.Time { return now })
	require.ErrorIs(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, WorkspaceAssurance{}), ErrSSORequired)
}
