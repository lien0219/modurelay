//go:build integration

package repository

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func samlRepositoryInput(key string) service.EnterpriseIdentityProviderInput {
	return service.EnterpriseIdentityProviderInput{Type: "saml", ProviderKey: key, Name: "SAML Identity", SAML: &service.SAMLProviderConfig{IDPEntityID: "https://idp.example.com/entity", SSOURL: "https://idp.example.com/sso", SigningCertificates: []string{"public-idp-certificate"}, SPCertificate: "public-sp-certificate", AuthnRequestsSigned: true, EmailAttribute: "email"}, ClaimMapping: map[string]any{}, JITConfig: service.JITConfig{Enabled: true, DefaultRole: "viewer"}}
}

func TestEnterpriseSAMLRepositoryProtocolPersistenceAndRevision(t *testing.T) {
	ctx, repo, owner, workspace, oidc, _ := enterpriseIdentityFixture(t)
	before := enterpriseHistorySnapshot(t, integrationDB, "workspace_identity_providers", false)
	body, err := migrations.FS.ReadFile("295_enterprise_identity_saml.sql")
	require.NoError(t, err)
	_, err = integrationDB.Exec(string(body))
	require.NoError(t, err, "raw migration rerun must be idempotent")
	_, err = integrationDB.Exec(string(body))
	require.NoError(t, err, "applying migration 295 twice must preserve OIDC rows")
	require.JSONEq(t, before, enterpriseHistorySnapshot(t, integrationDB, "workspace_identity_providers", false))
	in := samlRepositoryInput("saml")
	in.IsDefault = true
	in.SAML.MetadataXML = "do not persist imported XML"
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, in, "cipher:sp-private-keys", nil)
	require.NoError(t, err)
	require.Len(t, saml.PublicID, 43)
	require.Empty(t, saml.IssuerURL)
	require.Empty(t, saml.ClientID)
	require.False(t, saml.HasClientSecret)
	require.Empty(t, saml.SAML.MetadataXML)
	var oauthNull bool
	require.NoError(t, integrationDB.QueryRow(`SELECT issuer_url IS NULL AND client_id IS NULL AND token_auth_method IS NULL AND scopes IS NULL AND discovery_enabled IS NULL AND encrypted_client_secret IS NULL FROM workspace_identity_providers WHERE workspace_id=$1 AND id=$2`, workspace.ID, saml.ID).Scan(&oauthNull))
	require.True(t, oauthNull)
	old, oldSecret, err := repo.GetProvider(ctx, workspace.ID, owner.ID, oidc.ID)
	require.NoError(t, err)
	require.False(t, old.IsDefault)
	require.Equal(t, "cipher:fixture-secret", oldSecret)
	public, keys, err := repo.GetSAMLProviderByPublicID(ctx, saml.PublicID)
	require.NoError(t, err)
	require.Equal(t, saml.ID, public.ID)
	require.Equal(t, "cipher:sp-private-keys", keys)
	discovered, err := repo.DiscoverSSO(ctx, fmt.Sprintf("enterprise-%d.example.com", workspace.ID))
	require.NoError(t, err)
	require.Len(t, discovered, 2)
	require.Equal(t, "saml", discovered[0].Type)
	require.Equal(t, saml.PublicID, discovered[0].SAMLPublicID)
	require.Equal(t, "oidc", discovered[1].Type)
	require.Empty(t, discovered[1].SAMLPublicID)
	encoded, err := json.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-keys")
	_, _, err = repo.GetProvider(ctx, workspace.ID+100000, owner.ID, saml.ID)
	require.Error(t, err)
	in.Revision = saml.Revision
	in.SAML.SPCertificate = "client-attempted-replacement"
	updated, err := repo.UpdateProvider(ctx, workspace.ID, owner.ID, saml.ID, in, nil, nil)
	require.NoError(t, err)
	require.Equal(t, saml.PublicID, updated.PublicID)
	require.Equal(t, "public-sp-certificate", updated.SAML.SPCertificate)
	_, preservedKeys, err := repo.GetProvider(ctx, workspace.ID, 0, updated.ID)
	require.NoError(t, err)
	require.Equal(t, "cipher:sp-private-keys", preservedKeys)
	in.Revision = updated.Revision
	in.Type = "oidc"
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, saml.ID, in, nil, nil)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	in.Type = "saml"
	input := enterpriseProvisionInput(workspace.ID, updated, "saml-stable", fmt.Sprintf("saml@enterprise-%d.example.com", workspace.ID))
	input.Protocol = "saml"
	userID, err := repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var source, signup string
	require.NoError(t, integrationDB.QueryRow(`SELECT m.membership_source,u.signup_source FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.user_id=$2`, workspace.ID, userID).Scan(&source, &signup))
	require.Equal(t, "saml", source)
	require.Equal(t, "saml", signup)
	input.Protocol = "oidc"
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.ErrorIs(t, err, service.ErrOIDCStateSessionMismatch)
	in.SAML.IDPEntityID = "https://another-idp.example.com/entity"
	_, err = repo.UpdateProvider(ctx, workspace.ID, owner.ID, saml.ID, in, nil, nil)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	require.NoError(t, repo.DisableProvider(ctx, workspace.ID, owner.ID, saml.ID))
	_, _, err = repo.GetSAMLProviderByPublicID(ctx, saml.PublicID)
	require.Error(t, err)
}

func TestEnterpriseSAMLReplayAtomicConcurrentAndTenantScoped(t *testing.T) {
	ctx, repo, owner, workspace, oidc, _ := enterpriseIdentityFixture(t)
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, samlRepositoryInput("saml"), "cipher:keys", nil)
	require.NoError(t, err)
	expires := time.Now().UTC().Add(time.Hour)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- repo.UseSAMLResponse(ctx, workspace.ID, saml.ID, "response", "assertion", expires)
		}()
	}
	wg.Wait()
	close(errs)
	success, replay := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrSAMLReplayDetected)
			replay++
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 7, replay)
	err = repo.UseSAMLResponse(ctx, workspace.ID, saml.ID, "response2", "assertion", expires)
	require.ErrorIs(t, err, service.ErrSAMLReplayDetected)
	require.NoError(t, repo.UseSAMLResponse(ctx, workspace.ID, saml.ID, "response2", "assertion2", expires), "failed pair must leave no partial replay record")
	err = repo.UseSAMLResponse(ctx, workspace.ID+100000, saml.ID, "tenant-response", "tenant-assertion", expires)
	require.Error(t, err)
	err = repo.UseSAMLResponse(ctx, workspace.ID, oidc.ID, "oidc-response", "oidc-assertion", expires)
	require.Error(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_identity_saml_replays WHERE workspace_id=$1 AND provider_id=$2`, workspace.ID, saml.ID).Scan(&count))
	require.Equal(t, 4, count)
	for _, invalidExpiry := range []time.Time{time.Now().Add(-time.Minute), time.Now().Add(25 * time.Hour)} {
		require.ErrorIs(t, repo.UseSAMLResponse(ctx, workspace.ID, saml.ID, "bad-response", "bad-assertion", invalidExpiry), service.ErrEnterpriseIdentityInvalid)
	}
	_, err = integrationDB.Exec(`INSERT INTO workspace_identity_saml_replays(workspace_id,provider_id,id_hash,created_at,expires_at) SELECT $1,$2,decode(md5('stale-'||n)||md5('stale-'||n),'hex'),now()-interval '48 hours',now()-interval '24 hours' FROM generate_series(1,150) n`, workspace.ID, saml.ID)
	require.NoError(t, err)
	require.NoError(t, repo.UseSAMLResponse(ctx, workspace.ID, saml.ID, "bounded-response", "bounded-assertion", expires))
	var stale, live int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FILTER(WHERE expires_at<now()),count(*) FILTER(WHERE expires_at>=now()) FROM workspace_identity_saml_replays WHERE workspace_id=$1 AND provider_id=$2`, workspace.ID, saml.ID).Scan(&stale, &live))
	require.Equal(t, 50, stale, "cleanup removes at most 100 expired IDs")
	require.Equal(t, 6, live, "cleanup never removes valid replay IDs")
}

func TestEnterpriseSAMLDatabaseRejectsMalformedConfigAndOAuthFields(t *testing.T) {
	ctx, repo, owner, workspace, oidc, _ := enterpriseIdentityFixture(t)
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, samlRepositoryInput("saml"), "cipher:keys", nil)
	require.NoError(t, err)
	for _, config := range []string{
		`null`, `{}`, `[]`,
		`{"idp_entity_id":"issuer","sso_url":"https://idp.example.com/sso","signing_certificates":[],"sp_certificate":"cert","authn_requests_signed":true,"allow_unspecified_name_id":false}`,
		`{"idp_entity_id":"issuer","sso_url":"https://idp.example.com/sso","signing_certificates":[4],"sp_certificate":"cert","authn_requests_signed":true,"allow_unspecified_name_id":false}`,
		`{"idp_entity_id":"issuer","sso_url":"https://idp.example.com/sso","signing_certificates":["cert"],"sp_certificate":"cert","authn_requests_signed":false,"allow_unspecified_name_id":false}`,
		`{"idp_entity_id":"issuer","sso_url":"https://idp.example.com/sso","signing_certificates":["cert"],"sp_certificate":"cert","authn_requests_signed":true,"allow_unspecified_name_id":false,"metadata_xml":"raw XML"}`,
		`{"idp_entity_id":"issuer","sso_url":"https://idp.example.com/sso","signing_certificates":["cert"],"sp_certificate":"cert","authn_requests_signed":true,"allow_unspecified_name_id":false,"private_key":"secret"}`,
	} {
		_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET saml_config=$3::jsonb WHERE workspace_id=$1 AND id=$2`, workspace.ID, saml.ID, config)
		require.Error(t, err, config)
	}
	for _, statement := range []string{
		`UPDATE workspace_identity_providers SET issuer_url='dummy-issuer' WHERE id=$1`,
		`UPDATE workspace_identity_providers SET client_id='dummy-client' WHERE id=$1`,
		`UPDATE workspace_identity_providers SET scopes=ARRAY['openid'] WHERE id=$1`,
		`UPDATE workspace_identity_providers SET discovery_enabled=false WHERE id=$1`,
		`UPDATE workspace_identity_providers SET encrypted_client_secret='dummy-secret' WHERE id=$1`,
		`UPDATE workspace_identity_providers SET encrypted_saml_sp_keys=NULL WHERE id=$1`,
		`UPDATE workspace_identity_providers SET saml_public_id=NULL WHERE id=$1`,
	} {
		_, err = integrationDB.Exec(statement, saml.ID)
		require.Error(t, err)
	}
	_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET encrypted_saml_sp_keys='dummy' WHERE id=$1`, oidc.ID)
	require.Error(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET saml_config='{}'::jsonb WHERE id=$1`, oidc.ID)
	require.Error(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_identity_providers SET issuer_url=NULL WHERE id=$1`, oidc.ID)
	require.Error(t, err)
}

func TestEnterpriseSAMLStatesAndCompletionsBindProtocol(t *testing.T) {
	ctx, repo, owner, workspace, _, _ := enterpriseIdentityFixture(t)
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, samlRepositoryInput("saml"), "cipher:keys", nil)
	require.NoError(t, err)
	now := time.Now().UTC()
	state, err := service.NewOIDCState(now, time.Minute, workspace.ID, saml.ID, "/workspaces", "login")
	require.NoError(t, err)
	state.Protocol, state.RequestID, state.ProviderRevision = "saml", "_request", saml.Revision
	require.NoError(t, repo.CreateOIDCState(ctx, state))
	var noOAuth bool
	require.NoError(t, integrationDB.QueryRow(`SELECT nonce_hash IS NULL AND nonce_ciphertext IS NULL AND pkce_verifier_ciphertext IS NULL FROM workspace_identity_auth_states WHERE workspace_id=$1 AND provider_id=$2`, workspace.ID, saml.ID).Scan(&noOAuth))
	require.True(t, noOAuth)
	consumed, err := repo.ConsumeOIDCState(ctx, state.Hash, state.BrowserSessionHash, now)
	require.NoError(t, err)
	require.Equal(t, "saml", consumed.Protocol)
	require.Equal(t, "_request", consumed.RequestID)
	require.Empty(t, consumed.PKCEVerifier)
	require.Empty(t, consumed.Nonce)
	_, err = repo.ConsumeOIDCState(ctx, state.Hash, state.BrowserSessionHash, now)
	require.Error(t, err)
	a := service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: saml.ID, ProviderRevision: saml.Revision, AuthMethod: "saml", AuthenticatedAt: now.Add(-time.Hour)}
	login := &service.OIDCLoginResult{User: owner, Workspace: workspace.ID, ProviderID: saml.ID, ReturnTo: "/workspaces", Assurance: a}
	token, browser := service.HashEnterpriseToken("completion"), service.HashEnterpriseToken("browser")
	require.NoError(t, repo.CreateLoginCompletion(ctx, login, token, browser, now.Add(time.Minute)))
	preview, id, err := repo.PreviewLoginCompletion(ctx, token, browser, now)
	require.NoError(t, err)
	require.Equal(t, owner.ID, id)
	require.Equal(t, "saml", preview.Assurance.AuthMethod)
	require.WithinDuration(t, a.AuthenticatedAt, preview.Assurance.AuthenticatedAt, time.Microsecond)
	login.Assurance.AuthMethod = "oidc"
	require.ErrorIs(t, repo.CreateLoginCompletion(ctx, login, service.HashEnterpriseToken("mismatch"), browser, now.Add(time.Minute)), service.ErrOIDCStateSessionMismatch)
	require.NoError(t, repo.DisableProvider(ctx, workspace.ID, owner.ID, saml.ID))
	_, _, err = repo.ConsumeLoginCompletion(ctx, token, browser, now)
	require.Error(t, err)
}

func TestEnterpriseSAMLMixedProviderTeamGrantsPreserveManualRole(t *testing.T) {
	ctx, repo, owner, workspace, oidc, _ := enterpriseIdentityFixture(t)
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, samlRepositoryInput("saml"), "cipher:keys", nil)
	require.NoError(t, err)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	team, err := ws.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Mixed identity", Slug: "mixed-identity"})
	require.NoError(t, err)
	manual, err := ws.CreateTeam(ctx, owner.ID, workspace.ID, service.WorkspaceTeamInput{Name: "Manual identity", Slug: "manual-identity"})
	require.NoError(t, err)
	mappings := service.OIDCMappings{Roles: []service.OIDCRoleMapping{{ClaimValue: "group", Role: "admin", Priority: 10}}, Teams: []service.OIDCTeamMapping{{ClaimValue: "group", TeamID: team.ID}}}
	for _, provider := range []*service.EnterpriseIdentityProvider{oidc, saml} {
		require.NoError(t, repo.ReplaceMappings(ctx, workspace.ID, owner.ID, provider.ID, mappings))
		current, _, err := repo.GetProvider(ctx, workspace.ID, 0, provider.ID)
		require.NoError(t, err)
		*provider = *current
	}
	var memberID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, owner.ID).Scan(&memberID))
	require.NoError(t, ws.AddTeamMember(ctx, owner.ID, workspace.ID, manual.ID, memberID))
	for _, provider := range []*service.EnterpriseIdentityProvider{saml, oidc} {
		input := enterpriseProvisionInput(workspace.ID, provider, provider.Type+"-owner", owner.Email)
		input.Protocol = provider.Type
		_, err := repo.CompleteIdentityLogin(ctx, input)
		require.ErrorIs(t, err, service.ErrOIDCAccountLinkRequired, "existing email alone never adopts the Owner")
		input.Protocol, input.LinkUserID = provider.Type, &owner.ID
		input.Claims.Groups, input.Claims.GroupsPresent, input.Claims.GroupsComplete = []string{"group"}, true, true
		_, err = repo.CompleteIdentityLogin(ctx, input)
		require.NoError(t, err)
	}
	var role, source string
	var providerID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT role,membership_source FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, owner.ID).Scan(&role, &source))
	require.Equal(t, "owner", role)
	require.Equal(t, "manual", source)
	input := enterpriseProvisionInput(workspace.ID, oidc, "oidc-owner", owner.Email)
	input.Claims.Groups, input.Claims.GroupsPresent, input.Claims.GroupsComplete = []string{}, true, true
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT membership_source,membership_provider_id FROM workspace_team_members WHERE workspace_id=$1 AND team_id=$2 AND workspace_member_id=$3`, workspace.ID, team.ID, memberID).Scan(&source, &providerID))
	require.Equal(t, "saml", source)
	require.Equal(t, saml.ID, providerID)
	input = enterpriseProvisionInput(workspace.ID, saml, "saml-owner", owner.Email)
	input.Protocol = "saml"
	input.Claims.Groups, input.Claims.GroupsPresent, input.Claims.GroupsComplete = []string{}, true, true
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND workspace_member_id=$2`, workspace.ID, memberID).Scan(&count))
	require.Equal(t, 1, count, "manual Team membership survives removing both protocols")
}

func TestEnterpriseSAMLPolicyUsesLiveMatchingOwnerAssurance(t *testing.T) {
	ctx, repo, owner, workspace, _, _ := enterpriseIdentityFixture(t)
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, samlRepositoryInput("saml"), "cipher:keys", nil)
	require.NoError(t, err)
	input := enterpriseProvisionInput(workspace.ID, saml, "owner", owner.Email)
	input.Protocol, input.LinkUserID = "saml", &owner.ID
	_, err = repo.CompleteIdentityLogin(ctx, input)
	require.NoError(t, err)
	assurance := service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: saml.ID, ProviderRevision: saml.Revision, AuthMethod: "saml", AuthenticatedAt: time.Now().Add(-time.Hour)}
	policy, err := repo.UpdatePolicy(service.WithAuthenticationAssurance(ctx, assurance), workspace.ID, owner.ID, true, nil)
	require.NoError(t, err)
	require.True(t, policy.RequireSSO)
	assurance.AuthMethod = "oidc"
	_, err = repo.UpdatePolicy(service.WithAuthenticationAssurance(ctx, assurance), workspace.ID, owner.ID, true, nil)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	assurance.AuthMethod = "saml"
	assurance.ProviderRevision++
	_, err = repo.UpdatePolicy(service.WithAuthenticationAssurance(ctx, assurance), workspace.ID, owner.ID, true, nil)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
}

func TestEnterpriseSAMLKeyMutationRequiresTenantRevisionAndRBAC(t *testing.T) {
	ctx, repo, owner, workspace, oidc, _ := enterpriseIdentityFixture(t)
	saml, err := repo.CreateProvider(ctx, workspace.ID, owner.ID, samlRepositoryInput("saml"), "cipher:keys", nil)
	require.NoError(t, err)
	_, err = repo.UpdateSAMLKeys(ctx, workspace.ID, owner.ID, oidc.ID, oidc.Revision, "cipher:changed", "new", "next")
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	_, err = repo.UpdateSAMLKeys(ctx, workspace.ID+100000, owner.ID, saml.ID, saml.Revision, "cipher:changed", "new", "next")
	require.Error(t, err)
	_, err = repo.UpdateSAMLKeys(ctx, workspace.ID, owner.ID+100000, saml.ID, saml.Revision, "cipher:changed", "new", "next")
	require.Error(t, err)
	changed, err := repo.UpdateSAMLKeys(ctx, workspace.ID, owner.ID, saml.ID, saml.Revision, "cipher:changed", "new", "next")
	require.NoError(t, err)
	require.Equal(t, saml.Revision+1, changed.Revision)
	require.Equal(t, saml.PublicID, changed.PublicID)
	require.Equal(t, "new", changed.SAML.SPCertificate)
	require.Equal(t, "next", changed.SAML.NextSPCertificate)
	_, ciphertext, err := repo.GetProvider(ctx, workspace.ID, 0, saml.ID)
	require.NoError(t, err)
	require.Equal(t, "cipher:changed", ciphertext)
	_, err = repo.UpdateSAMLKeys(ctx, workspace.ID, owner.ID, saml.ID, saml.Revision, "cipher:stale", "stale", "")
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	var audit, events, outbox int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='identity.saml_keys_rotated'`, workspace.ID).Scan(&audit))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND payload->'data'->>'provider_revision'=$2`, workspace.ID, fmt.Sprint(changed.Revision)).Scan(&events))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox o JOIN domain_events e ON e.id=o.event_id WHERE e.workspace_id=$1`, workspace.ID).Scan(&outbox))
	require.Equal(t, 1, audit)
	require.Positive(t, events)
	require.Positive(t, outbox)
}
