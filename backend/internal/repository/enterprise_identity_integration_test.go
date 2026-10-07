//go:build integration

package repository

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func enterpriseIdentityFixture(t *testing.T) (context.Context, *enterpriseIdentityRepository, *service.User, *service.Workspace, *service.EnterpriseIdentityProvider, []byte) {
	t.Helper()
	isolateWorkspaceTestFixtures(t)
	ctx, _, owner, workspace := workspaceFixture(t)
	repo := NewEnterpriseIdentityRepository(integrationDB, enterpriseTestEncryptor{}).(*enterpriseIdentityRepository)
	domain := fmt.Sprintf("enterprise-%d.example.com", workspace.ID)
	hash := service.HashEnterpriseToken("fixture-verification")
	item, err := repo.CreateDomain(ctx, workspace.ID, owner.ID, domain, "", hash)
	require.NoError(t, err)
	_, err = repo.MarkDomainChecked(ctx, workspace.ID, owner.ID, item.ID, hash, "", true)
	require.NoError(t, err)
	enabled := true
	provider, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, service.EnterpriseIdentityProviderInput{
		ProviderKey: "test", Name: "Company Identity", IssuerURL: "https://idp.example.com", ClientID: "client", TokenAuthMethod: "client_secret_basic", DiscoveryEnabled: &enabled,
		ClaimMapping: map[string]any{}, JITConfig: service.JITConfig{Enabled: true, DefaultRole: "viewer"}, IsDefault: true,
	}, "cipher:fixture-secret", []string{"openid", "email"})
	require.NoError(t, err)
	return ctx, repo, owner, workspace, provider, hash
}

func enterpriseProvisionInput(workspaceID int64, provider *service.EnterpriseIdentityProvider, subject, email string) service.OIDCProvisionInput {
	return service.OIDCProvisionInput{WorkspaceID: workspaceID, ProviderID: provider.ID, ProviderRevision: provider.Revision, PasswordHash: "$2a$10$isolated-oidc-password", Claims: &service.OIDCClaims{Subject: subject, Email: email, EmailVerified: true}}
}

func TestEnterpriseIdentityJITIsAtomicAndConcurrent(t *testing.T) {
	ctx, repo, _, workspace, provider, _ := enterpriseIdentityFixture(t)
	email := fmt.Sprintf("new@enterprise-%d.example.com", workspace.ID)
	input := enterpriseProvisionInput(workspace.ID, provider, "stable-subject", email)
	var wg sync.WaitGroup
	ids := make(chan int64, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); id, err := repo.CompleteIdentityLogin(ctx, input); ids <- id; errs <- err }()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var userID int64
	for id := range ids {
		if userID == 0 {
			userID = id
		}
		require.Equal(t, userID, id)
	}
	var users, identities, personal, members int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM users WHERE email=$1`, email).Scan(&users))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_user_identities WHERE workspace_id=$1 AND provider_id=$2 AND subject='stable-subject'`, workspace.ID, provider.ID).Scan(&identities))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspaces WHERE type='personal' AND owner_user_id=$1`, userID).Scan(&personal))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members WHERE workspace_id=$1 AND user_id=$2 AND membership_source='oidc' AND membership_provider_id=$3`, workspace.ID, userID, provider.ID).Scan(&members))
	require.Equal(t, 1, users)
	require.Equal(t, 1, identities)
	require.Equal(t, 1, personal)
	require.Equal(t, 1, members)
	input.Claims.Subject = "different-subject"
	_, err := repo.CompleteIdentityLogin(ctx, input)
	require.ErrorIs(t, err, service.ErrOIDCAccountLinkRequired)
	input.Claims.Subject = "stable-subject"
	input.Claims.Email = "renamed@unrelated.example.com"
	input.Claims.EmailVerified = false
	again, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	require.Equal(t, userID, again)
	_, err = integrationDB.Exec(`UPDATE workspace_members SET status='suspended' WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, userID)
	require.NoError(t, err)
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.Error(t, err)
}

func TestEnterpriseIdentityExplicitLinkPreservesManualOwnerAndBilling(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	input := enterpriseProvisionInput(workspace.ID, provider, "owner-subject", owner.Email)
	_, err := repo.CompleteIdentityLogin(ctx, input)
	require.ErrorIs(t, err, service.ErrOIDCAccountLinkRequired)
	input.LinkUserID = &owner.ID
	id, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	require.Equal(t, owner.ID, id)
	var role, source string
	var ownerID, billingID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT role,membership_source FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, id).Scan(&role, &source))
	require.Equal(t, "owner", role)
	require.Equal(t, "manual", source)
	require.NoError(t, integrationDB.QueryRow(`SELECT owner_user_id,billing_owner_user_id FROM workspaces WHERE id=$1`, workspace.ID).Scan(&ownerID, &billingID))
	require.Equal(t, owner.ID, ownerID)
	require.Equal(t, owner.ID, billingID)
	outsider := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("link-outsider-%d@example.com", owner.ID)})
	input.LinkUserID = &outsider.ID
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.Error(t, err)
	input.Claims.Subject = "outsider-subject"
	input.Claims.Email = outsider.Email
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.Error(t, err)
}

func TestEnterpriseIdentityMappingReconciliationPreservesOtherSources(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	team, err := ws.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Mapped", Slug: "mapped"})
	require.NoError(t, err)
	manual, err := ws.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Manual", Slug: "manual"})
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, provider.ID, service.OIDCMappings{Roles: []service.OIDCRoleMapping{{ClaimValue: "engineering", Role: "developer", Priority: 5}, {ClaimValue: "finance", Role: "billing", Priority: 8}}, Teams: []service.OIDCTeamMapping{{ClaimValue: "engineering", TeamID: team.ID}}}))
	input := enterpriseProvisionInput(workspace.ID, provider, "mapping-subject", fmt.Sprintf("mapped@enterprise-%d.example.com", workspace.ID))
	// Mapping changes invalidate the provider revision used for an earlier state.
	provider, _, err = repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	input.ProviderRevision = provider.Revision
	input.Claims.Groups = []string{"engineering", "finance"}
	input.Claims.GroupsPresent, input.Claims.GroupsComplete = true, true
	id, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var memberID int64
	var role string
	require.NoError(t, integrationDB.QueryRow(`SELECT id,role FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, id).Scan(&memberID, &role))
	require.Equal(t, "billing", role)
	require.NoError(t, ws.AddTeamMember(ctx, owner.ID, workspace.ID, manual.ID, memberID))
	input.Claims.Groups = nil // absent claim must not remove mapped access.
	input.Claims.GroupsPresent, input.Claims.GroupsComplete = false, false
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND workspace_member_id=$2`, workspace.ID, memberID).Scan(&count))
	require.Equal(t, 2, count)
	input.Claims.Groups = []string{} // an explicit, complete empty claim revokes only this provider.
	input.Claims.GroupsPresent, input.Claims.GroupsComplete = true, true
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT role FROM workspace_members WHERE id=$1`, memberID).Scan(&role))
	require.Equal(t, "viewer", role)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND workspace_member_id=$2 AND membership_source='manual'`, workspace.ID, memberID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestEnterpriseIdentityDomainClaimsAndVerificationRace(t *testing.T) {
	ctx, repo, owner, workspace, _, hash := enterpriseIdentityFixture(t)
	var domainID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_domains WHERE workspace_id=$1`, workspace.ID).Scan(&domainID))
	item, err := repo.MarkDomainChecked(ctx, workspace.ID, owner.ID, domainID, hash, "DNS_LOOKUP_FAILED", false)
	require.NoError(t, err)
	require.Equal(t, "verified", item.Status)
	item, err = repo.MarkDomainChecked(ctx, workspace.ID, owner.ID, domainID, hash, "TXT_MISMATCH", false)
	require.NoError(t, err)
	require.Equal(t, "failed", item.Status)
	other, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).CreateOrganization(ctx, owner.ID, "Other", fmt.Sprintf("domain-other-%d", owner.ID))
	require.NoError(t, err)
	_, err = repo.CreateDomain(ctx, other.ID, owner.ID, item.NormalizedDomain, "", hash)
	require.ErrorIs(t, err, service.ErrDomainAlreadyClaimed)
	newHash := service.HashEnterpriseToken("new-token")
	_, err = repo.RegenerateDomain(ctx, workspace.ID, owner.ID, domainID, newHash)
	require.NoError(t, err)
	_, err = repo.MarkDomainChecked(ctx, workspace.ID, owner.ID, domainID, hash, "", true)
	require.Error(t, err)
	_, err = repo.RevokeDomain(ctx, workspace.ID, owner.ID, domainID)
	require.NoError(t, err)
	_, err = repo.CreateDomain(ctx, other.ID, owner.ID, item.NormalizedDomain, "", hash)
	require.NoError(t, err)
}

func TestEnterpriseIdentityBreakGlassIsOwnerOnlyAtomicAndRateLimited(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	admin := workspaceJoin(t, ctx, ws, owner.ID, workspace.ID, "admin")
	input := enterpriseProvisionInput(workspace.ID, provider, "owner-break-glass", owner.Email)
	input.LinkUserID = &owner.ID
	_, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	ctx = service.WithAuthenticationAssurance(ctx, service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthMethod: "oidc", AuthenticatedAt: time.Now().UTC()})
	_, err = repo.UpdatePolicy(ctx, workspace.ID, owner.ID, true, nil)
	require.NoError(t, err)
	_, err = repo.BreakGlass(ctx, workspace.ID, admin.ID, "recovery")
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	policy, err := repo.BreakGlass(ctx, workspace.ID, owner.ID, "lost IdP access")
	require.NoError(t, err)
	require.False(t, policy.RequireSSO)
	var audit, events, outbox int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='identity.break_glass'`, workspace.ID).Scan(&audit))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type='workspace.sso.break_glass_used'`, workspace.ID).Scan(&events))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1 AND e.event_type='workspace.sso.break_glass_used'`, workspace.ID).Scan(&outbox))
	require.Equal(t, 1, audit)
	require.Equal(t, 1, events)
	require.Equal(t, 1, outbox)
	_, err = repo.UpdatePolicy(ctx, workspace.ID, owner.ID, true, nil)
	require.NoError(t, err)
	_, err = repo.BreakGlass(ctx, workspace.ID, owner.ID, "repeated recovery")
	require.Error(t, err)
}

func TestEnterpriseIdentityMigrationIsIdempotentAndTenantBounded(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	migration, err := os.ReadFile("../../migrations/294_enterprise_identity_oidc.sql")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	other, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).CreateOrganization(ctx, owner.ID, "Other", fmt.Sprintf("identity-other-%d", owner.ID))
	require.NoError(t, err)
	team, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).CreateTeam(ctx, owner.ID, other.ID, service.WorkspaceTeamInput{Name: "Other", Slug: "other"})
	require.NoError(t, err)
	require.Error(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, provider.ID, service.OIDCMappings{Teams: []service.OIDCTeamMapping{{ClaimValue: "x", TeamID: team.ID}}}))
	_, err = integrationDB.Exec(`INSERT INTO workspace_identity_team_mappings(workspace_id,provider_id,claim_value,team_id,created_by_user_id) VALUES($1,$2,'cross-tenant',$3,$4)`, workspace.ID, provider.ID, team.ID, owner.ID)
	require.Error(t, err)
	_, _, err = repo.GetProvider(ctx, other.ID, owner.ID, provider.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	outsider := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("identity-outsider-%d@example.com", time.Now().UnixNano())})
	require.ErrorIs(t, repo.DisableProvider(ctx, workspace.ID, outsider.ID, provider.ID), service.ErrWorkspaceNotFound)
}

func enterpriseProviderInput(provider *service.EnterpriseIdentityProvider) service.EnterpriseIdentityProviderInput {
	discovery := provider.DiscoveryEnabled
	return service.EnterpriseIdentityProviderInput{ProviderKey: provider.ProviderKey, Name: provider.Name, IssuerURL: provider.IssuerURL, ClientID: provider.ClientID, TokenAuthMethod: provider.TokenAuthMethod,
		Revision: provider.Revision, IsDefault: provider.IsDefault, DiscoveryEnabled: &discovery, ClaimMapping: provider.ClaimMapping, JITConfig: provider.JITConfig, SecretAction: "preserve"}
}

func TestEnterpriseIdentityEnforcementRechecksOwnerAssuranceUnderWorkspaceLock(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	link := enterpriseProvisionInput(workspace.ID, provider, "enforcement-owner", owner.Email)
	link.LinkUserID = &owner.ID
	_, err := repo.CompleteIdentityLogin(ctx, link)
	require.NoError(t, err)
	assurance := service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthMethod: "oidc", AuthenticatedAt: time.Now().UTC()}
	staleCtx := service.WithAuthenticationAssurance(ctx, assurance)
	// The service has checked revision R, then a concurrent configuration edit
	// commits R+1 before the policy transaction obtains its workspace lock.
	input := enterpriseProviderInput(provider)
	input.Name = "Configuration changed after validation"
	updated, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.NoError(t, err)
	_, err = repo.UpdatePolicy(staleCtx, workspace.ID, owner.ID, true, nil)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict, "stale validation must not enable enforcement")
	policy, err := repo.GetPolicy(ctx, workspace.ID, owner.ID)
	require.NoError(t, err)
	require.False(t, policy.RequireSSO)
	var events int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, workspace.ID, service.EventSSOEnforcementEnabled).Scan(&events))
	require.Zero(t, events, "rejected enabling must not emit an enforcement event")
	_, err = repo.UpdatePolicy(ctx, workspace.ID, owner.ID, true, nil)
	require.ErrorIs(t, err, service.ErrSSORequired, "a binding alone is not proof of current SSO")
	assurance.ProviderRevision = updated.Revision
	currentCtx := service.WithAuthenticationAssurance(ctx, assurance)
	policy, err = repo.UpdatePolicy(currentCtx, workspace.ID, owner.ID, true, nil)
	require.NoError(t, err)
	require.True(t, policy.RequireSSO)
}

func TestEnterpriseIdentityLegacyHumanKeysRequireSSOWithoutAffectingGateway(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	client := testEntClient(t)
	wr := NewWorkspaceRepository(integrationDB)
	ws := service.NewWorkspaceService(wr)
	project, err := ws.CreateProject(ctx, owner.ID, workspace.ID, service.ProjectInput{Name: "SSO keys", Slug: "sso-keys"})
	require.NoError(t, err)
	keys := service.NewAPIKeyService(NewAPIKeyRepository(client, integrationDB), NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
	keys.ConfigureWorkspaces(wr)
	keys.SetEnterpriseIdentityService(service.NewEnterpriseIdentityService(repo, service.NewWorkspaceAccessService(wr), NewUserRepository(client, integrationDB), enterpriseTestEncryptor{}))
	organizationKey, err := keys.CreateForProject(ctx, owner.ID, workspace.ID, project.ID, service.CreateAPIKeyRequest{Name: "Organization"})
	require.NoError(t, err)
	personalKey, err := keys.Create(ctx, owner.ID, service.CreateAPIKeyRequest{Name: "Personal"})
	require.NoError(t, err)
	link := enterpriseProvisionInput(workspace.ID, provider, "legacy-key-owner", owner.Email)
	link.LinkUserID = &owner.ID
	_, err = repo.CompleteIdentityLogin(ctx, link)
	require.NoError(t, err)
	assurance := service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthMethod: "oidc", AuthenticatedAt: time.Now().UTC()}
	assuredCtx := service.WithAuthenticationAssurance(ctx, assurance)
	_, err = repo.UpdatePolicy(assuredCtx, workspace.ID, owner.ID, true, nil)
	require.NoError(t, err)
	checkVisibility := func(t *testing.T, requestCtx context.Context, allowed bool) {
		t.Helper()
		_, readErr := keys.GetForUser(requestCtx, owner.ID, organizationKey.ID)
		if allowed {
			require.NoError(t, readErr)
		} else {
			require.ErrorIs(t, readErr, service.ErrSSORequired)
		}
		expectedIDs := []int64{personalKey.ID}
		if allowed {
			expectedIDs = append(expectedIDs, organizationKey.ID)
		}
		for _, sort := range []string{"created_at", "current_concurrency"} {
			listed, page, listErr := keys.List(requestCtx, owner.ID, pagination.PaginationParams{Page: 1, PageSize: 20, SortBy: sort}, service.APIKeyListFilters{})
			require.NoError(t, listErr)
			require.EqualValues(t, len(expectedIDs), page.Total)
			ids := make([]int64, 0, len(listed))
			for _, key := range listed {
				ids = append(ids, key.ID)
			}
			require.ElementsMatch(t, expectedIDs, ids)
		}
		searched, searchErr := keys.SearchAPIKeys(requestCtx, owner.ID, "", 20)
		require.NoError(t, searchErr)
		ids := make([]int64, 0, len(searched))
		for _, key := range searched {
			ids = append(ids, key.ID)
		}
		require.ElementsMatch(t, expectedIDs, ids)
		searched, searchErr = keys.SearchAPIKeys(requestCtx, owner.ID, "Organization", 1)
		require.NoError(t, searchErr)
		if allowed {
			require.Len(t, searched, 1)
			require.Equal(t, organizationKey.ID, searched[0].ID)
			require.NotEqual(t, organizationKey.Key, searched[0].Key, "legacy search must mask organization secrets")
		} else {
			require.Empty(t, searched)
		}
	}
	checkVisibility(t, assuredCtx, true)
	for _, test := range []struct {
		name   string
		modify func(*service.WorkspaceAssurance)
	}{
		{"wrong_workspace", func(a *service.WorkspaceAssurance) { a.WorkspaceID++ }},
		{"expired_age", func(a *service.WorkspaceAssurance) { a.AuthenticatedAt = time.Now().Add(-13 * time.Hour) }},
		{"expired_deadline", func(a *service.WorkspaceAssurance) { a.ValidUntil = time.Now().Add(-time.Second) }},
		{"wrong_method", func(a *service.WorkspaceAssurance) { a.AuthMethod = "password" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := assurance
			test.modify(&invalid)
			checkVisibility(t, service.WithAuthenticationAssurance(ctx, invalid), false)
		})
	}
	grace := time.Now().UTC().Add(time.Hour)
	_, err = repo.UpdatePolicy(assuredCtx, workspace.ID, owner.ID, true, &grace)
	require.NoError(t, err)
	checkVisibility(t, ctx, true)
	// Simulate the grace deadline being reached, without a wall-clock sleep.
	_, err = integrationDB.Exec(`UPDATE workspace_security_policies SET sso_grace_until=now() WHERE workspace_id=$1`, workspace.ID)
	require.NoError(t, err)
	checkVisibility(t, ctx, false)
	_, err = keys.GetForUser(ctx, owner.ID, organizationKey.ID)
	require.ErrorIs(t, err, service.ErrSSORequired)
	name := "Changed"
	_, err = keys.Update(ctx, organizationKey.ID, owner.ID, service.UpdateAPIKeyRequest{Name: &name})
	require.ErrorIs(t, err, service.ErrSSORequired)
	params := pagination.PaginationParams{Page: 1, PageSize: 20}
	for _, sort := range []string{"created_at", "current_concurrency"} {
		params.SortBy = sort
		listed, page, err := keys.List(ctx, owner.ID, params, service.APIKeyListFilters{})
		require.NoError(t, err)
		require.EqualValues(t, 1, page.Total)
		require.Len(t, listed, 1)
		require.Equal(t, personalKey.ID, listed[0].ID)
	}
	_, err = keys.GetForUser(ctx, owner.ID, personalKey.ID)
	require.NoError(t, err)
	_, err = keys.GetByKey(ctx, organizationKey.Key)
	require.NoError(t, err, "gateway authentication must not require browser SSO")
	_, err = keys.GetForUser(assuredCtx, owner.ID, organizationKey.ID)
	require.NoError(t, err)
	listed, page, err := keys.List(assuredCtx, owner.ID, params, service.APIKeyListFilters{})
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	require.Len(t, listed, 2)
	require.ErrorIs(t, keys.Delete(ctx, organizationKey.ID, owner.ID), service.ErrSSORequired)
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, enterpriseProviderInput(provider), nil, provider.Scopes)
	require.NoError(t, err)
	_, err = keys.GetForUser(assuredCtx, owner.ID, organizationKey.ID)
	require.ErrorIs(t, err, service.ErrSSORequired)
	listed, page, err = keys.List(assuredCtx, owner.ID, params, service.APIKeyListFilters{})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total, "stale assurance must not disclose organization key rows or counts")
	require.Len(t, listed, 1)
	checkVisibility(t, assuredCtx, false)
}

func TestEnterpriseIdentityProviderAuthMethodAndSecretRemainConsistent(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	secondInput := enterpriseProviderInput(provider)
	secondInput.ProviderKey, secondInput.Name = "other-default", "Other default"
	_, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, secondInput, "cipher:other-secret", provider.Scopes)
	require.NoError(t, err)
	provider, _, err = repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	mutationSnapshot := func() string {
		t.Helper()
		var snapshot string
		require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_build_object('providers',(SELECT jsonb_agg(jsonb_build_object('id',id,'revision',revision,'default',is_default,'method',token_auth_method,'secret',encrypted_client_secret IS NOT NULL) ORDER BY id) FROM workspace_identity_providers WHERE workspace_id=$1),'audit',(SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1),'events',(SELECT count(*) FROM domain_events WHERE workspace_id=$1),'outbox',(SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1))::text`, workspace.ID).Scan(&snapshot))
		return snapshot
	}
	before := mutationSnapshot()
	input := enterpriseProviderInput(provider)
	input.TokenAuthMethod = "none"
	input.IsDefault = true
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.ErrorIs(t, err, service.ErrEnterpriseIdentityInvalid, "preserve must not attach a confidential secret to public auth")
	require.JSONEq(t, before, mutationSnapshot(), "rejected edits must preserve defaults, revisions, audit and outbox")
	unchanged, _, err := repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	require.Equal(t, provider.Revision, unchanged.Revision)
	input.SecretAction = "remove"
	public, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.NoError(t, err)
	input = enterpriseProviderInput(public)
	input.TokenAuthMethod = "client_secret_post"
	before = mutationSnapshot()
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.ErrorIs(t, err, service.ErrEnterpriseIdentityInvalid, "preserve must not enable confidential auth without a secret")
	require.JSONEq(t, before, mutationSnapshot())
	input.SecretAction = "replace"
	ciphertext := "cipher:replacement"
	confidential, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, &ciphertext, provider.Scopes)
	require.NoError(t, err)
	require.True(t, confidential.HasClientSecret)
	input = enterpriseProviderInput(confidential)
	input.ProviderKey = "invalid-public"
	input.TokenAuthMethod = "none"
	before = mutationSnapshot()
	_, err = repo.CreateProvider(ctx, workspace.ID, owner.ID, input, ciphertext, provider.Scopes)
	require.ErrorIs(t, err, service.ErrEnterpriseIdentityInvalid)
	require.JSONEq(t, before, mutationSnapshot())
	input.ProviderKey = "invalid-confidential"
	input.TokenAuthMethod = "client_secret_basic"
	_, err = repo.CreateProvider(ctx, workspace.ID, owner.ID, input, "", provider.Scopes)
	require.ErrorIs(t, err, service.ErrEnterpriseIdentityInvalid)
	require.JSONEq(t, before, mutationSnapshot())
}

func TestEnterpriseIdentityProviderSecretsAndDefaultAreAtomic(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	input := enterpriseProviderInput(provider)
	updated, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.NoError(t, err)
	require.Greater(t, updated.Revision, provider.Revision)
	_, ciphertext, err := repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	require.Equal(t, "cipher:fixture-secret", ciphertext)
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	input = enterpriseProviderInput(updated)
	input.SecretAction = "replace"
	replacement := "cipher:replacement"
	updated, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, &replacement, provider.Scopes)
	require.NoError(t, err)
	_, ciphertext, err = repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	require.Equal(t, replacement, ciphertext)
	input = enterpriseProviderInput(updated)
	input.SecretAction, input.TokenAuthMethod = "remove", "none"
	updated, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, input, nil, provider.Scopes)
	require.NoError(t, err)
	require.False(t, updated.HasClientSecret)
	_, ciphertext, err = repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	require.Empty(t, ciphertext)
	input = enterpriseProviderInput(updated)
	input.ProviderKey, input.Name = "second", "Second"
	second, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, input, "", []string{"openid"})
	require.NoError(t, err)
	require.True(t, second.IsDefault)
	first, _, err := repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	require.False(t, first.IsDefault)
	// Clearing the default and then failing insertion must roll back together.
	_, err = repo.CreateProvider(ctx, workspace.ID, owner.ID, input, "", []string{"openid"})
	require.Error(t, err)
	second, _, err = repo.GetProvider(ctx, workspace.ID, 0, second.ID)
	require.NoError(t, err)
	require.True(t, second.IsDefault)
}

func TestEnterpriseIdentityAuthStateBrowserBindingPrecedesConsumption(t *testing.T) {
	ctx, repo, _, workspace, provider, _ := enterpriseIdentityFixture(t)
	state, err := service.NewOIDCState(time.Now().UTC(), 5*time.Minute, workspace.ID, provider.ID, "/workspaces", "login")
	require.NoError(t, err)
	state.ProviderRevision = provider.Revision
	require.NoError(t, repo.CreateOIDCState(ctx, state))
	_, err = repo.ConsumeOIDCState(ctx, state.Hash, service.HashEnterpriseToken("wrong-browser"), time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateNotFound)
	consumed, err := repo.ConsumeOIDCState(ctx, state.Hash, state.BrowserSessionHash, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, state.Nonce, consumed.Nonce)
	require.Equal(t, state.PKCEVerifier, consumed.PKCEVerifier)
	_, err = repo.ConsumeOIDCState(ctx, state.Hash, state.BrowserSessionHash, time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateNotFound)
}

func TestEnterpriseIdentityLoginCompletionIsBrowserBoundOneUseAndRevisionBound(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	input := enterpriseProvisionInput(workspace.ID, provider, "completion-subject", fmt.Sprintf("completion@enterprise-%d.example.com", workspace.ID))
	userID, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	login := &service.OIDCLoginResult{User: &service.User{ID: userID}, Workspace: workspace.ID, ProviderID: provider.ID, ReturnTo: "/workspaces", Assurance: service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthMethod: "oidc", AuthenticatedAt: time.Now().UTC()}}
	tokenHash, browserHash := service.HashEnterpriseToken("completion"), service.HashEnterpriseToken("bound-browser")
	require.NoError(t, repo.CreateLoginCompletion(ctx, login, tokenHash, browserHash, time.Now().UTC().Add(2*time.Minute)))
	_, _, err = repo.PreviewLoginCompletion(ctx, tokenHash, service.HashEnterpriseToken("wrong-browser"), time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	preview, previewID, err := repo.PreviewLoginCompletion(ctx, tokenHash, browserHash, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, userID, previewID)
	require.Equal(t, provider.Revision, preview.Assurance.ProviderRevision)
	_, _, err = repo.PreviewLoginCompletion(ctx, tokenHash, browserHash, time.Now().UTC())
	require.NoError(t, err, "preview must leave the completion available for TOTP verification")
	_, _, err = repo.ConsumeLoginCompletion(ctx, tokenHash, service.HashEnterpriseToken("wrong-browser"), time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	completed, id, err := repo.ConsumeLoginCompletion(ctx, tokenHash, browserHash, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, userID, id)
	require.Equal(t, "/workspaces", completed.ReturnTo)
	require.Equal(t, login.Assurance.AuthenticatedAt.Unix(), completed.Assurance.AuthenticatedAt.Unix())
	_, _, err = repo.ConsumeLoginCompletion(ctx, tokenHash, browserHash, time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	_, _, err = repo.PreviewLoginCompletion(ctx, tokenHash, browserHash, time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	secondHash := service.HashEnterpriseToken("revision-completion")
	require.NoError(t, repo.CreateLoginCompletion(ctx, login, secondHash, browserHash, time.Now().UTC().Add(2*time.Minute)))
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, enterpriseProviderInput(provider), nil, provider.Scopes)
	require.NoError(t, err)
	_, _, err = repo.ConsumeLoginCompletion(ctx, secondHash, browserHash, time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	_, _, err = repo.PreviewLoginCompletion(ctx, secondHash, browserHash, time.Now().UTC())
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	require.ErrorIs(t, repo.CreateLoginCompletion(ctx, login, service.HashEnterpriseToken("late-completion"), browserHash, time.Now().UTC().Add(2*time.Minute)), service.ErrOIDCStateSessionMismatch)
}

func TestEnterpriseIdentityJITRollsBackUserAndPersonalWorkspaceOnAuditFailure(t *testing.T) {
	ctx, repo, _, workspace, provider, _ := enterpriseIdentityFixture(t)
	_, err := integrationDB.Exec(fmt.Sprintf(`ALTER TABLE workspace_audit_logs ADD CONSTRAINT enterprise_test_audit_failure CHECK (NOT (workspace_id=%d AND action='identity.jit_provisioned')) NOT VALID`, workspace.ID))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`ALTER TABLE workspace_audit_logs DROP CONSTRAINT IF EXISTS enterprise_test_audit_failure`)
	})
	email := fmt.Sprintf("rollback@enterprise-%d.example.com", workspace.ID)
	_, err = repo.CompleteIdentityLogin(ctx, enterpriseProvisionInput(workspace.ID, provider, "rollback-subject", email))
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM users WHERE email=$1`, email).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspaces w JOIN users u ON u.id=w.owner_user_id WHERE w.type='personal' AND u.email=$1`, email).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_user_identities WHERE workspace_id=$1 AND subject='rollback-subject'`, workspace.ID).Scan(&count))
	require.Zero(t, count)
}

func TestEnterpriseIdentitySharedTeamRetainsOtherProviderAttribution(t *testing.T) {
	ctx, repo, owner, workspace, first, _ := enterpriseIdentityFixture(t)
	team, err := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB)).CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Shared", Slug: "shared"})
	require.NoError(t, err)
	providerInput := enterpriseProviderInput(first)
	providerInput.ProviderKey, providerInput.Name, providerInput.IsDefault = "another", "Another", false
	second, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, providerInput, "cipher:secret", first.Scopes)
	require.NoError(t, err)
	mappings := service.OIDCMappings{Roles: []service.OIDCRoleMapping{{ClaimValue: "engineering", Role: "developer", Priority: 5}}, Teams: []service.OIDCTeamMapping{{ClaimValue: "engineering", TeamID: team.ID}}}
	require.NoError(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, first.ID, mappings))
	require.NoError(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, second.ID, mappings))
	first, _, err = repo.GetProvider(ctx, workspace.ID, 0, first.ID)
	require.NoError(t, err)
	second, _, err = repo.GetProvider(ctx, workspace.ID, 0, second.ID)
	require.NoError(t, err)
	input := enterpriseProvisionInput(workspace.ID, first, "first-subject", fmt.Sprintf("shared@enterprise-%d.example.com", workspace.ID))
	input.Claims.Groups, input.Claims.GroupsPresent, input.Claims.GroupsComplete = []string{"engineering"}, true, true
	userID, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	secondInput := enterpriseProvisionInput(workspace.ID, second, "second-subject", input.Claims.Email)
	secondInput.LinkUserID = &userID
	secondInput.Claims.Groups, secondInput.Claims.GroupsPresent, secondInput.Claims.GroupsComplete = []string{"engineering"}, true, true
	_, err = repo.CompleteIdentityLogin(ctx, secondInput)
	require.NoError(t, err)
	input.Claims.Groups = []string{}
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND team_id=$2`, workspace.ID, team.ID).Scan(&count))
	require.Equal(t, 1, count)
	secondInput.Claims.Groups = []string{}
	_, err = repo.CompleteIdentityLogin(ctx, secondInput)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND team_id=$2`, workspace.ID, team.ID).Scan(&count))
	require.Zero(t, count)
}

func TestEnterpriseIdentityJITRequiresVerifiedDomainAndActiveScopes(t *testing.T) {
	for _, scenario := range []string{"unverified_email", "unverified_domain", "disabled_provider", "suspended_workspace", "jit_disabled", "domain_allowlist"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, repo, _, workspace, provider, _ := enterpriseIdentityFixture(t)
			input := enterpriseProvisionInput(workspace.ID, provider, scenario, fmt.Sprintf("denied@enterprise-%d.example.com", workspace.ID))
			var err error
			switch scenario {
			case "unverified_email":
				input.Claims.EmailVerified = false
			case "unverified_domain":
				_, err = integrationDB.Exec(`UPDATE workspace_domains SET status='pending',verified_at=NULL WHERE workspace_id=$1`, workspace.ID)
			case "disabled_provider":
				_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET status='disabled' WHERE id=$1`, provider.ID)
			case "suspended_workspace":
				_, err = integrationDB.Exec(`UPDATE workspaces SET status='suspended' WHERE id=$1`, workspace.ID)
			case "jit_disabled":
				_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET jit_config='{"enabled":false,"default_role":"viewer"}' WHERE id=$1`, provider.ID)
			case "domain_allowlist":
				_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET jit_config='{"enabled":true,"default_role":"viewer","allowed_domains":["elsewhere.example.com"]}' WHERE id=$1`, provider.ID)
			}
			require.NoError(t, err)
			_, err = repo.CompleteIdentityLogin(ctx, input)
			require.Error(t, err)
			var count int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM users WHERE email=$1`, input.Claims.Email).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestEnterpriseIdentitySCIMRoleAndTeamSurviveOIDCReconciliation(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	member := workspaceJoin(t, ctx, ws, owner.ID, workspace.ID, "developer")
	_, err := integrationDB.Exec(`UPDATE workspace_members SET membership_source='scim' WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, member.ID)
	require.NoError(t, err)
	team, err := ws.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "SCIM", Slug: "scim"})
	require.NoError(t, err)
	var memberID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, member.ID).Scan(&memberID))
	_, err = integrationDB.Exec(`INSERT INTO workspace_team_members(workspace_id,team_id,workspace_member_id,membership_source) VALUES($1,$2,$3,'scim')`, workspace.ID, team.ID, memberID)
	require.NoError(t, err)
	require.NoError(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, provider.ID, service.OIDCMappings{Roles: []service.OIDCRoleMapping{{ClaimValue: "elevated", Role: "admin", Priority: 100}}, Teams: []service.OIDCTeamMapping{{ClaimValue: "elevated", TeamID: team.ID}}}))
	provider, _, err = repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
	require.NoError(t, err)
	input := enterpriseProvisionInput(workspace.ID, provider, "scim-subject", member.Email)
	input.LinkUserID = &member.ID
	input.Claims.Groups, input.Claims.GroupsPresent, input.Claims.GroupsComplete = []string{"elevated"}, true, true
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	input.Claims.Groups = []string{}
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var role, source, teamSource string
	require.NoError(t, integrationDB.QueryRow(`SELECT role,membership_source FROM workspace_members WHERE id=$1`, memberID).Scan(&role, &source))
	require.Equal(t, "developer", role)
	require.Equal(t, "scim", source)
	require.NoError(t, integrationDB.QueryRow(`SELECT membership_source FROM workspace_team_members WHERE workspace_id=$1 AND team_id=$2 AND workspace_member_id=$3`, workspace.ID, team.ID, memberID).Scan(&teamSource))
	require.Equal(t, "scim", teamSource)
}

func TestEnterpriseIdentityManualRoleAssignmentTakesPrecedenceAfterJIT(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	input := enterpriseProvisionInput(workspace.ID, provider, "manual-role-subject", fmt.Sprintf("manual@enterprise-%d.example.com", workspace.ID))
	userID, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, workspace.ID, userID, "developer", "active"))
	input.Claims.Groups, input.Claims.GroupsPresent, input.Claims.GroupsComplete = []string{}, true, true
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var role, source string
	require.NoError(t, integrationDB.QueryRow(`SELECT role,membership_source FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, userID).Scan(&role, &source))
	require.Equal(t, "developer", role)
	require.Equal(t, "manual", source)
}

func TestEnterpriseIdentityDiscoveryDoesNotDependOnUserExistence(t *testing.T) {
	ctx, repo, _, workspace, provider, _ := enterpriseIdentityFixture(t)
	items, err := repo.DiscoverSSO(ctx, fmt.Sprintf("enterprise-%d.example.com", workspace.ID))
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, provider.ID, items[0].ProviderID)
	items, err = repo.DiscoverSSO(ctx, "unknown.example.com")
	require.NoError(t, err)
	require.Empty(t, items)
	_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET status='disabled' WHERE id=$1`, provider.ID)
	require.NoError(t, err)
	items, err = repo.DiscoverSSO(ctx, fmt.Sprintf("enterprise-%d.example.com", workspace.ID))
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestEnterpriseIdentityProviderValidationCannotOverwriteNewConfiguration(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	require.NoError(t, repo.RecordProviderValidation(ctx, workspace.ID, owner.ID, provider.ID, provider.Revision, "SUCCESS"))
	validated, _, err := repo.GetProvider(ctx, workspace.ID, owner.ID, provider.ID)
	require.NoError(t, err)
	require.NotNil(t, validated.LastValidatedAt)
	require.Equal(t, "SUCCESS", validated.LastValidationCode)
	require.Equal(t, provider.Revision, validated.Revision)
	updated, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, enterpriseProviderInput(provider), nil, provider.Scopes)
	require.NoError(t, err)
	require.Nil(t, updated.LastValidatedAt)
	require.Empty(t, updated.LastValidationCode)
	require.ErrorIs(t, repo.RecordProviderValidation(ctx, workspace.ID, owner.ID, provider.ID, provider.Revision, "DISCOVERY_FAILED"), service.ErrWorkspaceConflict)
	updated, _, err = repo.GetProvider(ctx, workspace.ID, owner.ID, provider.ID)
	require.NoError(t, err)
	require.Nil(t, updated.LastValidatedAt)
	require.Empty(t, updated.LastValidationCode)
	require.ErrorIs(t, repo.RecordProviderValidation(ctx, workspace.ID, owner.ID, provider.ID, updated.Revision, "secret=https://idp.example.com?client_secret=unsafe"), service.ErrEnterpriseIdentityInvalid)
	require.NoError(t, repo.RecordProviderValidation(ctx, workspace.ID, owner.ID, provider.ID, updated.Revision, "DISCOVERY_FAILED"))
	failed, _, err := repo.GetProvider(ctx, workspace.ID, owner.ID, provider.ID)
	require.NoError(t, err)
	require.Equal(t, "DISCOVERY_FAILED", failed.LastValidationCode)
}

func TestEnterpriseIdentityExpiredAuthenticationCleanupIsBounded(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	_, err := integrationDB.Exec(`INSERT INTO workspace_identity_auth_states(state_hash,workspace_id,provider_id,provider_revision,browser_session_hash,nonce_hash,pkce_verifier_ciphertext,nonce_ciphertext,return_to,intent,expires_at,created_at)
		SELECT sha256(convert_to('expired-state-'||i::text,'UTF8')),$1,$2,$3,decode(repeat('00',32),'hex'),decode(repeat('00',32),'hex'),'cipher:pkce','cipher:nonce','/workspaces','login',now()-interval '2 days',now()-interval '2 days 5 minutes' FROM generate_series(1,105) i`, workspace.ID, provider.ID, provider.Revision)
	require.NoError(t, err)
	state, err := service.NewOIDCState(time.Now().UTC(), 5*time.Minute, workspace.ID, provider.ID, "/workspaces", "login")
	require.NoError(t, err)
	state.ProviderRevision = provider.Revision
	require.NoError(t, repo.CreateOIDCState(ctx, state))
	var expired int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_identity_auth_states WHERE workspace_id=$1 AND expires_at<now()`, workspace.ID).Scan(&expired))
	require.Equal(t, 5, expired)
	_, err = repo.ConsumeOIDCState(ctx, state.Hash, state.BrowserSessionHash, time.Now().UTC())
	require.NoError(t, err, "cleanup must preserve live authentication state")
	_, err = integrationDB.Exec(`INSERT INTO workspace_identity_login_completions(token_hash,browser_session_hash,workspace_id,provider_id,provider_revision,user_id,return_to,authenticated_at,expires_at,created_at)
		SELECT sha256(convert_to('expired-completion-'||i::text,'UTF8')),decode(repeat('00',32),'hex'),$1,$2,$3,$4,'/workspaces',now()-interval '2 days 1 minute',now()-interval '2 days',now()-interval '2 days 1 minute' FROM generate_series(1,105) i`, workspace.ID, provider.ID, provider.Revision, owner.ID)
	require.NoError(t, err)
	login := &service.OIDCLoginResult{User: owner, Workspace: workspace.ID, ProviderID: provider.ID, ReturnTo: "/workspaces", Assurance: service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthMethod: "oidc", AuthenticatedAt: time.Now().UTC()}}
	token, browser := service.HashEnterpriseToken("live-cleanup-completion"), service.HashEnterpriseToken("live-cleanup-browser")
	require.NoError(t, repo.CreateLoginCompletion(ctx, login, token, browser, time.Now().UTC().Add(2*time.Minute)))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_identity_login_completions WHERE workspace_id=$1 AND expires_at<now()`, workspace.ID).Scan(&expired))
	require.Equal(t, 5, expired)
	_, _, err = repo.ConsumeLoginCompletion(ctx, token, browser, time.Now().UTC())
	require.NoError(t, err, "cleanup must preserve a live login completion")
}

func TestEnterpriseIdentityLinkRejectsUnverifiedOrDifferentAccountEmail(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	for _, tc := range []struct {
		email    string
		verified bool
	}{{owner.Email, false}, {"someone-else@example.com", true}} {
		input := enterpriseProvisionInput(workspace.ID, provider, "mismatched-link", tc.email)
		input.LinkUserID = &owner.ID
		input.Claims.EmailVerified = tc.verified
		_, err := repo.CompleteIdentityLogin(ctx, input)
		require.ErrorIs(t, err, service.ErrOIDCAccountLinkRequired)
	}
	var bindings int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_user_identities WHERE workspace_id=$1`, workspace.ID).Scan(&bindings))
	require.Zero(t, bindings)
}

func TestEnterpriseIdentityBoundProviderCannotChangeSubjectNamespace(t *testing.T) {
	ctx, repo, owner, workspace, provider, _ := enterpriseIdentityFixture(t)
	input := enterpriseProvisionInput(workspace.ID, provider, "namespace-subject", owner.Email)
	input.LinkUserID = &owner.ID
	_, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	for _, field := range []string{"issuer", "client"} {
		changed := enterpriseProviderInput(provider)
		if field == "issuer" {
			changed.IssuerURL = "https://attacker.example.com"
		} else {
			changed.ClientID = "another-client"
		}
		_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, provider.ID, changed, nil, provider.Scopes)
		require.ErrorIs(t, err, service.ErrWorkspaceConflict, "existing subject bindings must retain their issuer/client namespace")
	}
}
