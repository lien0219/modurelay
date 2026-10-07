//go:build integration

package repository

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func runEnterpriseIdentityConcurrentActions(actions ...func() error) []error {
	start := make(chan struct{})
	results := make([]error, len(actions))
	var workers sync.WaitGroup
	for index, action := range actions {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			results[index] = action()
		}()
	}
	close(start)
	workers.Wait()
	return results
}

func requireEnterpriseIdentitySingleWinner(t *testing.T, results []error, loser error) {
	t.Helper()
	winners := 0
	for _, err := range results {
		if err == nil {
			winners++
		} else {
			require.ErrorIs(t, err, loser)
		}
	}
	require.Equal(t, 1, winners)
}

func TestEnterpriseIdentityConcurrentDomainClaimsHaveOneOwner(t *testing.T) {
	ctx, repo, owner, workspace, _, hash := enterpriseIdentityFixture(t)
	other, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).CreateOrganization(ctx, owner.ID, "Other", fmt.Sprintf("concurrent-domain-%d", owner.ID))
	require.NoError(t, err)
	domain := fmt.Sprintf("concurrent-%d.example.com", workspace.ID)
	results := runEnterpriseIdentityConcurrentActions(
		func() error { _, err := repo.CreateDomain(ctx, workspace.ID, owner.ID, domain, "", hash); return err },
		func() error { _, err := repo.CreateDomain(ctx, other.ID, owner.ID, domain, "", hash); return err },
	)
	requireEnterpriseIdentitySingleWinner(t, results, service.ErrDomainAlreadyClaimed)
	var claims int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_domains WHERE normalized_domain=$1 AND status<>'revoked'`, domain).Scan(&claims))
	require.Equal(t, 1, claims)
}

func TestEnterpriseIdentityConcurrentProviderEditsRejectStaleRevision(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	first, second := enterpriseProviderInput(provider), enterpriseProviderInput(provider)
	first.Name, second.Name = "First edit", "Second edit"
	results := runEnterpriseIdentityConcurrentActions(
		func() error {
			_, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, first, nil, provider.Scopes)
			return err
		},
		func() error {
			_, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, second, nil, provider.Scopes)
			return err
		},
	)
	requireEnterpriseIdentitySingleWinner(t, results, service.ErrWorkspaceConflict)
	saved, secret, err := repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	require.Equal(t, provider.Revision+1, saved.Revision)
	require.Contains(t, []string{first.Name, second.Name}, saved.Name)
	require.Equal(t, "cipher:fixture-secret", secret)
	var events int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='workspace.identity_provider.updated'`, workspace.ID).Scan(&events))
	require.Equal(t, 1, events)
}

func TestEnterpriseIdentityConcurrentStateConsumptionIsSingleUse(t *testing.T) {
	ctx, repo, _, workspace, provider, _ := enterpriseIdentityFixture(t)
	state, err := service.NewOIDCState(time.Now().UTC(), 5*time.Minute, workspace.ID, provider.ID, "/workspaces", "login")
	require.NoError(t, err)
	state.ProviderRevision = provider.Revision
	require.NoError(t, repo.CreateOIDCState(ctx, state))
	consume := func() error {
		_, err := repo.ConsumeOIDCState(ctx, state.Hash, state.BrowserSessionHash, time.Now().UTC())
		return err
	}
	requireEnterpriseIdentitySingleWinner(t, runEnterpriseIdentityConcurrentActions(consume, consume, consume, consume), service.ErrOIDCStateNotFound)
	var consumed int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_identity_auth_states WHERE workspace_id=$1 AND consumed_at IS NOT NULL`, workspace.ID).Scan(&consumed))
	require.Equal(t, 1, consumed)
}

func TestEnterpriseIdentityConcurrentSubjectLinksCannotChangeGlobalUser(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	other := workspaceJoin(t, ctx, service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)), owner.ID, workspace.ID, "developer")
	first := enterpriseProvisionInput(workspace.ID, provider, "concurrent-subject", owner.Email)
	second := enterpriseProvisionInput(workspace.ID, provider, "concurrent-subject", other.Email)
	first.LinkUserID, second.LinkUserID = &owner.ID, &other.ID
	results := runEnterpriseIdentityConcurrentActions(
		func() error { _, err := repo.CompleteIdentityLogin(ctx, first); return err },
		func() error { _, err := repo.CompleteIdentityLogin(ctx, second); return err },
	)
	requireEnterpriseIdentitySingleWinner(t, results, service.ErrOIDCAccountLinkRequired)
	var userID int64
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*),min(user_id) FROM workspace_user_identities WHERE workspace_id=$1 AND provider_id=$2 AND subject='concurrent-subject'`, workspace.ID, provider.ID).Scan(&count, &userID))
	require.Equal(t, 1, count)
	require.Contains(t, []int64{owner.ID, other.ID}, userID)
}

func TestEnterpriseIdentityConcurrentRoleAndTeamReconciliationIsCoherent(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	workspaces := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	firstTeam, err := workspaces.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "First", Slug: "concurrent-first"})
	require.NoError(t, err)
	secondTeam, err := workspaces.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Second", Slug: "concurrent-second"})
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, provider.ID, service.OIDCMappings{
		Roles: []service.OIDCRoleMapping{{ClaimValue: "first", Role: "admin", Priority: 10}, {ClaimValue: "second", Role: "developer", Priority: 20}},
		Teams: []service.OIDCTeamMapping{{ClaimValue: "first", TeamID: firstTeam.ID}, {ClaimValue: "second", TeamID: secondTeam.ID}},
	}))
	provider, _, err = repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	email := fmt.Sprintf("concurrent-mapping@enterprise-%d.example.com", workspace.ID)
	first := enterpriseProvisionInput(workspace.ID, provider, "concurrent-mapping", email)
	second := enterpriseProvisionInput(workspace.ID, provider, "concurrent-mapping", email)
	first.Claims.Groups, second.Claims.Groups = []string{"first"}, []string{"second"}
	first.Claims.GroupsPresent, first.Claims.GroupsComplete = true, true
	second.Claims.GroupsPresent, second.Claims.GroupsComplete = true, true
	results := runEnterpriseIdentityConcurrentActions(
		func() error { _, err := repo.CompleteIdentityLogin(ctx, first); return err },
		func() error { _, err := repo.CompleteIdentityLogin(ctx, second); return err },
	)
	for _, err := range results {
		require.NoError(t, err)
	}
	var role string
	var teamID int64
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT m.role, count(tm.team_id),min(tm.team_id) FROM workspace_members m JOIN users u ON u.id=m.user_id JOIN workspace_team_members tm ON tm.workspace_id=m.workspace_id AND tm.workspace_member_id=m.id WHERE m.workspace_id=$1 AND u.email=$2 GROUP BY m.role`, workspace.ID, email).Scan(&role, &count, &teamID))
	require.Equal(t, 1, count)
	if role == "admin" {
		require.Equal(t, firstTeam.ID, teamID)
	} else {
		require.Equal(t, "developer", role)
		require.Equal(t, secondTeam.ID, teamID)
	}
}
