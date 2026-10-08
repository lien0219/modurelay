//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	rate "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/gin-gonic/gin"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func scimFixture(t *testing.T) (context.Context, *enterpriseSCIMRepository, *service.User, *service.Workspace, *service.SCIMConnector, *service.SCIMPrincipal) {
	t.Helper()
	ctx, _, owner, w, _, _ := enterpriseIdentityFixture(t)
	r := &enterpriseSCIMRepository{db: integrationDB}
	s := service.NewEnterpriseSCIMService(r)
	c, e := s.CreateConnector(ctx, w.ID, owner.ID, service.SCIMConnectorInput{Name: "Provisioning", DefaultRole: "developer"})
	require.NoError(t, e)
	token, e := s.CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	p, e := s.Authenticate(ctx, c.PublicID, token.Secret)
	require.NoError(t, e)
	return ctx, r, owner, w, c, p
}
func scimUserInput(w int64, name string) *service.SCIMUserInput {
	return &service.SCIMUserInput{Schemas: []string{service.SCIMUserSchema}, UserName: name, ExternalID: name, Emails: []service.SCIMEmail{{Value: fmt.Sprintf("%s@enterprise-%d.example.com", name, w), Primary: true}}}
}
func assertSCIMStatus(t *testing.T, e error, status int) {
	t.Helper()
	var typed *service.SCIMError
	require.True(t, errors.As(e, &typed), "%v", e)
	require.Equal(t, status, typed.Status)
}
func TestEnterpriseSCIMConcurrentCreationAndRevocation(t *testing.T) {
	ctx, r, _, w, c, p := scimFixture(t)
	in := scimUserInput(w.ID, "concurrent")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: in})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for e := range errs {
		if e == nil {
			ok++
		} else {
			assertSCIMStatus(t, e, 409)
		}
	}
	require.Equal(t, 1, ok)
	list, total, e := r.ListUsers(ctx, p, service.SCIMListQuery{StartIndex: 1, Count: 100})
	require.NoError(t, e)
	require.Equal(t, 1, total)
	require.Len(t, list, 1)
	require.NoError(t, r.RevokeToken(ctx, w.ID, w.OwnerUserID, c.ID, p.TokenID))
	_, e = r.GetUser(ctx, p, list[0].ID)
	assertSCIMStatus(t, e, 401)
}
func TestEnterpriseSCIMSourcesPreserveManualAndAdminSuspension(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	in := scimUserInput(w.ID, "manual")
	created, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, e)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	e = ws.UpdateMember(ctx, owner.ID, w.ID, created.UserID, "viewer", "active")
	require.NoError(t, e)
	inactive := false
	in.Active = &inactive
	updated, e := r.MutateUser(ctx, p, created.ID, service.SCIMUserMutation{Action: "replace", User: in, IfMatch: created.Revision})
	require.NoError(t, e)
	var status, role string
	require.NoError(t, integrationDB.QueryRow(`SELECT status,role FROM workspace_members WHERE id=$1`, created.MemberID).Scan(&status, &role))
	require.Equal(t, "active", status)
	require.Equal(t, "viewer", role)
	e = ws.UpdateMember(ctx, owner.ID, w.ID, created.UserID, "viewer", "suspended")
	require.NoError(t, e)
	active := true
	in.Active = &active
	_, e = r.MutateUser(ctx, p, created.ID, service.SCIMUserMutation{Action: "replace", User: in, IfMatch: updated.Revision})
	require.NoError(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspace_members WHERE id=$1`, created.MemberID).Scan(&status))
	require.Equal(t, "suspended", status)
	require.NoError(t, r.DisableConnector(ctx, w.ID, owner.ID, c.ID, c.Revision))
	var sources int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_membership_sources WHERE member_id=$1`, created.MemberID).Scan(&sources))
	require.Equal(t, 2, sources)
}

func TestEnterpriseSCIMGroupsPreserveManualOIDCAndSAML(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	identity := NewEnterpriseIdentityRepository(integrationDB, enterpriseTestEncryptor{}).(*enterpriseIdentityRepository)
	providers, _, e := identity.ListProviders(ctx, w.ID, owner.ID, pagination.PaginationParams{Page: 1, PageSize: 100})
	require.NoError(t, e)
	oidc := providers[0]
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	team, e := ws.CreateTeam(ctx, owner.ID, w.ID, service.WorkspaceTeamInput{Name: "Shared", Slug: "shared"})
	require.NoError(t, e)
	require.NoError(t, identity.ReplaceMappings(ctx, w.ID, owner.ID, oidc.ID, service.OIDCMappings{Teams: []service.OIDCTeamMapping{{ClaimValue: "shared", TeamID: team.ID}}, Roles: []service.OIDCRoleMapping{{ClaimValue: "shared", Role: "billing", Priority: 5}}}))
	oidcp, _, e := identity.GetProvider(ctx, w.ID, 0, oidc.ID)
	require.NoError(t, e)
	login := enterpriseProvisionInput(w.ID, oidcp, "shared", fmt.Sprintf("shared@enterprise-%d.example.com", w.ID))
	login.Claims.Groups = []string{"shared"}
	login.Claims.GroupsPresent = true
	login.Claims.GroupsComplete = true
	uid, e := identity.CompleteIdentityLogin(ctx, login)
	require.NoError(t, e)
	user, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "shared")})
	require.NoError(t, e)
	require.Equal(t, uid, user.UserID)
	group, e := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "External Name", Members: []service.SCIMMember{{Value: user.ID}}}})
	require.NoError(t, e)
	require.NoError(t, r.BindGroup(ctx, w.ID, owner.ID, c.ID, group.ID, service.SCIMGroupBindingInput{Revision: group.Revision, TeamID: &team.ID}))
	saml, e := identity.CreateProvider(ctx, w.ID, owner.ID, samlRepositoryInput("shared-saml"), "cipher:keys", nil)
	require.NoError(t, e)
	require.NoError(t, identity.ReplaceMappings(ctx, w.ID, owner.ID, saml.ID, service.OIDCMappings{Teams: []service.OIDCTeamMapping{{ClaimValue: "shared", TeamID: team.ID}}}))
	saml, _, e = identity.GetProvider(ctx, w.ID, 0, saml.ID)
	require.NoError(t, e)
	samlLogin := enterpriseProvisionInput(w.ID, saml, "shared-saml", user.Emails[0].Value)
	samlLogin.Protocol = "saml"
	samlLogin.LinkUserID = &uid
	samlLogin.Claims.Groups = []string{"shared"}
	samlLogin.Claims.GroupsPresent = true
	samlLogin.Claims.GroupsComplete = true
	_, e = identity.CompleteIdentityLogin(ctx, samlLogin)
	require.NoError(t, e)
	require.NoError(t, ws.AddTeamMember(ctx, owner.ID, w.ID, team.ID, user.MemberID))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND team_id=$3`, w.ID, user.MemberID, team.ID).Scan(&count))
	require.Equal(t, 4, count)
	// Earliest effective OIDC role retains ownership even when SCIM is changed.
	_, e = r.UpdateConnector(ctx, w.ID, owner.ID, c.ID, service.SCIMConnectorInput{Name: c.Name, DefaultRole: "admin", Revision: c.Revision})
	require.NoError(t, e)
	p.ConnectorRevision++
	var role, source string
	require.NoError(t, integrationDB.QueryRow(`SELECT role,membership_source FROM workspace_members WHERE id=$1`, user.MemberID).Scan(&role, &source))
	require.Equal(t, "billing", role)
	require.Equal(t, "oidc", source)
	group, e = r.GetGroup(ctx, p, group.ID)
	require.NoError(t, e)
	_, e = r.MutateGroup(ctx, p, group.ID, service.SCIMGroupMutation{Action: "delete", IfMatch: group.Revision})
	require.NoError(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE member_id=$1 AND team_id=$2`, user.MemberID, team.ID).Scan(&count))
	require.Equal(t, 3, count)
	require.NoError(t, ws.RemoveTeamMember(ctx, owner.ID, w.ID, team.ID, user.MemberID))
	login.Claims.Groups = []string{}
	_, e = identity.CompleteIdentityLogin(ctx, login)
	require.NoError(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_member_id=$1 AND team_id=$2`, user.MemberID, team.ID).Scan(&count))
	require.Equal(t, 1, count)
	samlLogin.LinkUserID = nil
	samlLogin.Claims.Groups = []string{}
	_, e = identity.CompleteIdentityLogin(ctx, samlLogin)
	require.NoError(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_member_id=$1 AND team_id=$2`, user.MemberID, team.ID).Scan(&count))
	require.Zero(t, count)
	var teamName string
	require.NoError(t, integrationDB.QueryRow(`SELECT name FROM workspace_teams WHERE id=$1`, team.ID).Scan(&teamName))
	require.Equal(t, "Shared", teamName)
}
func TestEnterpriseSCIMVersionIsolationAndProtectedIdentities(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	in := scimUserInput(w.ID, "protected")
	u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, e)
	same, e := r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in, IfMatch: u.Revision})
	require.NoError(t, e)
	require.Equal(t, u.Revision, same.Revision)
	in.DisplayName = "Changed"
	next, e := r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in, IfMatch: u.Revision})
	require.NoError(t, e)
	require.Equal(t, u.Revision+1, next.Revision)
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in, IfMatch: u.Revision})
	assertSCIMStatus(t, e, 412)
	in.Emails[0].Value = fmt.Sprintf("replacement@enterprise-%d.example.com", w.ID)
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	assertSCIMStatus(t, e, 400)
	in.Emails = u.Emails
	other, e := service.NewEnterpriseSCIMService(r).CreateConnector(ctx, w.ID, owner.ID, service.SCIMConnectorInput{Name: "Other", DefaultRole: "viewer"})
	require.NoError(t, e)
	token, e := service.NewEnterpriseSCIMService(r).CreateToken(ctx, w.ID, owner.ID, other.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	otherP, e := service.NewEnterpriseSCIMService(r).Authenticate(ctx, other.PublicID, token.Secret)
	require.NoError(t, e)
	_, e = r.GetUser(ctx, otherP, u.ID)
	assertSCIMStatus(t, e, 404)
	_, e = r.MutateGroup(ctx, otherP, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Bad", Members: []service.SCIMMember{{Value: u.ID}}}})
	assertSCIMStatus(t, e, 400)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	require.NoError(t, ws.ChangeBillingOwner(ctx, owner.ID, w.ID, u.UserID))
	inactive := false
	in.Active = &inactive
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	assertSCIMStatus(t, e, 409)
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "delete"})
	assertSCIMStatus(t, e, 409)
	require.NoError(t, ws.ChangeBillingOwner(ctx, owner.ID, w.ID, owner.ID))
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, u.UserID, "owner", "active"))
	active := true
	in.Active = &active
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	assertSCIMStatus(t, e, 409)
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "patch", Patch: &service.SCIMPatchRequest{Schemas: []string{service.SCIMPatchSchema}, Operations: []service.SCIMPatchOperation{{Op: "replace", Path: "displayName", Value: json.RawMessage(`"Owner update"`)}}}})
	assertSCIMStatus(t, e, 409)
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, u.UserID, "developer", "active"))
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "delete"})
	require.NoError(t, e)
	_, e = r.GetUser(ctx, p, u.ID)
	assertSCIMStatus(t, e, 404)
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "delete"})
	require.NoError(t, e)
	in.Active = &active
	_, e = r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: in})
	assertSCIMStatus(t, e, 409)
	_ = c
}
func TestEnterpriseSCIMManualTombstoneAndSSOReactivation(t *testing.T) {
	ctx, r, owner, w, _, p := scimFixture(t)
	in := scimUserInput(w.ID, "tombstone")
	u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, e)
	identity := NewEnterpriseIdentityRepository(integrationDB, enterpriseTestEncryptor{}).(*enterpriseIdentityRepository)
	providers, _, e := identity.ListProviders(ctx, w.ID, owner.ID, pagination.PaginationParams{Page: 1, PageSize: 100})
	require.NoError(t, e)
	login := enterpriseProvisionInput(w.ID, &providers[0], "bound", u.Emails[0].Value)
	login.LinkUserID = &u.UserID
	_, e = identity.CompleteIdentityLogin(ctx, login)
	require.NoError(t, e)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, u.UserID))
	inactive := false
	in.Active = &inactive
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, e)
	active := true
	in.Active = &active
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, e)
	login.LinkUserID = nil
	_, e = identity.CompleteIdentityLogin(ctx, login)
	require.ErrorIs(t, e, service.ErrWorkspaceForbidden)
	_, _, e = ws.CreateInvitation(ctx, owner.ID, w.ID, u.Emails[0].Value, "developer", time.Hour)
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	// A token issued before provisioning/removal must not resurrect a tombstone.
	token := "preexisting-invitation"
	_, e = integrationDB.Exec(`INSERT INTO workspace_invitations(workspace_id,email,role,token_hash,invited_by_user_id,expires_at) VALUES($1,$2,'developer',$3,$4,now()+interval '1 hour')`, w.ID, u.Emails[0].Value, service.HashEnterpriseToken(token), owner.ID)
	require.NoError(t, e)
	_, e = ws.AcceptInvitation(ctx, u.UserID, token)
	require.Error(t, e)
	var suspended bool
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT administratively_suspended,status FROM workspace_members WHERE id=$1`, u.MemberID).Scan(&suspended, &status))
	require.True(t, suspended)
	require.Equal(t, "suspended", status)
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, u.UserID, "viewer", "active"))
	_, e = identity.CompleteIdentityLogin(ctx, login)
	require.NoError(t, e)
	// Genuine source deprovision can reactivate a prebound identity; no admin flag.
	_, e = integrationDB.Exec(`UPDATE workspace_membership_sources SET active=false WHERE member_id=$1`, u.MemberID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_members SET status='suspended',administratively_suspended=false WHERE id=$1`, u.MemberID)
	require.NoError(t, e)
	_, e = identity.CompleteIdentityLogin(ctx, login)
	require.NoError(t, e)
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM workspace_members WHERE id=$1`, u.MemberID).Scan(&status))
	require.Equal(t, "active", status)
}
func TestEnterpriseSCIMExpiryAndFailuresAreBounded(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	expiry := time.Now().Add(time.Hour)
	token, e := service.NewEnterpriseSCIMService(r).CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{ExpiresAt: &expiry})
	require.NoError(t, e)
	require.NoError(t, r.NotifyExpiringTokens(ctx, 1))
	require.NoError(t, r.NotifyExpiringTokens(ctx, 100))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMTokenExpiring).Scan(&count))
	require.Equal(t, 1, count)
	for range 5 {
		require.NoError(t, r.RecordSyncOutcome(ctx, p, false, "sync_failed"))
	}
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMSyncFailed).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, r.RecordSyncOutcome(ctx, p, false, "uniqueness"))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMSecurityConflict).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, r.RecordSyncOutcome(ctx, p, false, "security_conflict"))
	require.NoError(t, r.RecordSyncOutcome(ctx, p, false, "security_conflict"))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMSecurityConflict).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, r.RecordSyncOutcome(ctx, p, true, ""))
	var failures int
	require.NoError(t, integrationDB.QueryRow(`SELECT failure_count FROM workspace_scim_connectors WHERE id=$1`, c.ID).Scan(&failures))
	require.Zero(t, failures)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action IN('scim.sync_failed','scim.security_conflict','scim.token_expiring') AND actor_user_id IS NULL`, w.ID).Scan(&count))
	require.Equal(t, 3, count)
	_ = token
}
func TestEnterpriseSCIMMigrationRerunPreservesNewAdministrativeState(t *testing.T) {
	t.Cleanup(func() { restoreWorkspaceSecurityMigration(t) })
	ctx, r, owner, w, _, p := scimFixture(t)
	u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "rerun")})
	require.NoError(t, e)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, u.UserID))
	tables := []string{"users", "workspace_members", "workspace_team_members", "api_keys", "service_accounts", "usage_logs", "budget_reservations", "workspace_membership_sources", "workspace_team_membership_sources", "workspace_scim_users", "workspace_scim_connectors", "workspace_scim_tokens"}
	before := map[string]string{}
	for _, table := range tables {
		before[table] = enterpriseHistorySnapshot(t, integrationDB, table, false)
	}
	body, e := migrations.FS.ReadFile("296_enterprise_identity_scim.sql")
	require.NoError(t, e)
	for range 2 {
		_, e = integrationDB.Exec(string(body))
		require.NoError(t, e)
	}
	for _, table := range tables {
		require.JSONEq(t, before[table], enterpriseHistorySnapshot(t, integrationDB, table, false), table)
	}
}

func TestEnterpriseSCIMGlobalResolutionNeverRewritesProfile(t *testing.T) {
	ctx, r, _, w, _, p := scimFixture(t)
	email := fmt.Sprintf("verified@enterprise-%d.example.com", w.ID)
	var uid int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users(email,password_hash,username) VALUES($1,'immutable-hash','Immutable Name') RETURNING id`, email).Scan(&uid))
	_, e := integrationDB.Exec(`INSERT INTO auth_identities(user_id,provider_type,provider_key,provider_subject,verified_at) VALUES($1,'email','email',$2,now())`, uid, email)
	require.NoError(t, e)
	before := enterpriseHistorySnapshot(t, integrationDB, "users", false)
	u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "verified")})
	require.NoError(t, e)
	require.Equal(t, uid, u.UserID)
	require.JSONEq(t, before, enterpriseHistorySnapshot(t, integrationDB, "users", false))
	patch := &service.SCIMPatchRequest{Schemas: []string{service.SCIMPatchSchema}, Operations: []service.SCIMPatchOperation{{Op: "replace", Path: "displayName", Value: json.RawMessage(`"Provisioned Name"`)}}}
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "patch", Patch: patch})
	require.NoError(t, e)
	require.JSONEq(t, before, enterpriseHistorySnapshot(t, integrationDB, "users", false))
	unverified := fmt.Sprintf("unverified@enterprise-%d.example.com", w.ID)
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users(email,password_hash) VALUES($1,'immutable-hash') RETURNING id`, unverified).Scan(&uid))
	_, e = r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "unverified")})
	assertSCIMStatus(t, e, 409)
	alias := scimUserInput(w.ID, "verified+tag")
	_, e = r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: alias})
	assertSCIMStatus(t, e, 409)
	external := scimUserInput(w.ID, "external")
	external.Emails[0].Value = "external@untrusted.example.com"
	_, e = r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: external})
	assertSCIMStatus(t, e, 409)
	var personal, manual int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members m JOIN workspaces w ON w.id=m.workspace_id JOIN workspace_membership_sources s ON s.workspace_id=m.workspace_id AND s.member_id=m.id AND s.source_type='manual' WHERE w.type='personal' AND m.user_id=$1`, u.UserID).Scan(&personal))
	require.Equal(t, 1, personal)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_membership_sources WHERE member_id=$1 AND source_type='manual'`, u.MemberID).Scan(&manual))
	require.Zero(t, manual)
}
func TestEnterpriseSCIMConcurrentGroupPatchAndTokenLimits(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	a, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "race-a")})
	require.NoError(t, e)
	b, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "race-b")})
	require.NoError(t, e)
	g, e := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Race"}})
	require.NoError(t, e)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{a.ID, b.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, _ := json.Marshal([]service.SCIMMember{{Value: id}})
			_, e := r.MutateGroup(ctx, p, g.ID, service.SCIMGroupMutation{Action: "patch", Patch: &service.SCIMPatchRequest{Schemas: []string{service.SCIMPatchSchema}, Operations: []service.SCIMPatchOperation{{Op: "add", Path: "members", Value: raw}}}})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	g, e = r.GetGroup(ctx, p, g.ID)
	require.NoError(t, e)
	require.Len(t, g.Members, 2)
	_, e = r.MutateGroup(ctx, p, g.ID, service.SCIMGroupMutation{Action: "delete", IfMatch: 1})
	assertSCIMStatus(t, e, 412)
	// Seven concurrent rotations fill the remaining slots without exceeding eight.
	errs = make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := service.NewEnterpriseSCIMService(r).CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{})
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		} else {
			require.ErrorIs(t, e, service.ErrWorkspaceConflict)
		}
	}
	require.Equal(t, 7, success)
	require.NoError(t, r.RevokeToken(ctx, w.ID, owner.ID, c.ID, p.TokenID))
	_, e = r.MutateGroup(ctx, p, g.ID, service.SCIMGroupMutation{Action: "delete"})
	assertSCIMStatus(t, e, 401)
}

func TestEnterpriseSCIMDatabaseTenantAndTypedProvenanceConstraints(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "schema")})
	require.NoError(t, e)
	identity := NewEnterpriseIdentityRepository(integrationDB, enterpriseTestEncryptor{}).(*enterpriseIdentityRepository)
	providers, _, e := identity.ListProviders(ctx, w.ID, owner.ID, pagination.DefaultPagination())
	require.NoError(t, e)
	oidc := providers[0]
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	other, e := ws.CreateOrganization(ctx, owner.ID, "Other", "scim-other")
	require.NoError(t, e)
	var otherMember int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, other.ID, owner.ID).Scan(&otherMember))
	_, e = integrationDB.Exec(`INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,provider_id,role) VALUES($1,$2,'saml',$3,'viewer')`, w.ID, u.MemberID, oidc.ID)
	require.Error(t, e, "source type must match actual provider")
	_, e = integrationDB.Exec(`INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,provider_id,role) VALUES($1,$2,'oidc',$3,'owner')`, w.ID, u.MemberID, oidc.ID)
	require.Error(t, e, "automated Owner forbidden")
	_, e = integrationDB.Exec(`INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,connector_id,scim_user_id,role) VALUES($1,$2,'scim',$3,$4,'viewer')`, other.ID, otherMember, c.ID, u.ID)
	require.Error(t, e, "cross-tenant connector source forbidden")
	var ownerMember int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, w.ID, owner.ID).Scan(&ownerMember))
	_, e = integrationDB.Exec(`INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,connector_id,scim_user_id,role) VALUES($1,$2,'scim',$3,$4,'viewer')`, w.ID, ownerMember, c.ID, u.ID)
	require.Error(t, e, "SCIM source must bind its own member")
	team, e := ws.CreateTeam(ctx, owner.ID, other.ID, service.WorkspaceTeamInput{Name: "Other", Slug: "other"})
	require.NoError(t, e)
	g, e := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Scoped"}})
	require.NoError(t, e)
	require.Error(t, r.BindGroup(ctx, w.ID, owner.ID, c.ID, g.ID, service.SCIMGroupBindingInput{Revision: g.Revision, TeamID: &team.ID}))
	_, e = integrationDB.Exec(`UPDATE workspace_scim_groups SET team_id=$2 WHERE id=$1`, g.ID, team.ID)
	require.Error(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_members SET effective_membership_source_id=(SELECT id FROM workspace_membership_sources WHERE workspace_id=$2 AND member_id=$3 LIMIT 1) WHERE id=$1`, u.MemberID, other.ID, otherMember)
	require.Error(t, e, "effective source must belong to exact tenant/member")
	_, _, e = ws.ListAudit(ctx, owner.ID, w.ID, pagination.DefaultPagination())
	require.NoError(t, e, "external actor NULL can be listed safely")
}

func TestEnterpriseSCIM295BackfillPreservesAllProviderSources(t *testing.T) {
	ctx := context.Background()
	container, e := tcpostgres.Run(ctx, selectDockerImage(ctx, postgresImageTag), tcpostgres.WithDatabase("scim_history"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, e := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, e)
	db, e := openSQLWithRetry(ctx, dsn, 30*time.Second)
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	historical := fstest.MapFS{}
	names, e := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, e)
	for _, name := range names {
		if name >= "296" {
			continue
		}
		body, e := migrations.FS.ReadFile(name)
		require.NoError(t, e)
		historical[name] = &fstest.MapFile{Data: body}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, historical))
	var owner, uid, w, member, team int64
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('scim-history-owner@example.com','old-hash') RETURNING id`).Scan(&owner))
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('scim-history-member@example.com','old-hash') RETURNING id`).Scan(&uid))
	require.NoError(t, db.QueryRow(`INSERT INTO workspaces(name,slug,type,owner_user_id,billing_owner_user_id) VALUES('History','scim-history','organization',$1,$1) RETURNING id`, owner).Scan(&w))
	_, e = db.Exec(`INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner')`, w, owner)
	require.NoError(t, e)
	require.NoError(t, db.QueryRow(`INSERT INTO workspace_members(workspace_id,user_id,role,status,membership_source) VALUES($1,$2,'developer','suspended','scim') RETURNING id`, w, uid).Scan(&member))
	identity := NewEnterpriseIdentityRepository(db, enterpriseTestEncryptor{}).(*enterpriseIdentityRepository)
	enabled := true
	oidc, e := identity.CreateProvider(ctx, w, owner, service.EnterpriseIdentityProviderInput{ProviderKey: "old-oidc", Name: "OIDC", IssuerURL: "https://idp.example.com", ClientID: "client", TokenAuthMethod: "client_secret_basic", DiscoveryEnabled: &enabled, ClaimMapping: map[string]any{}, JITConfig: service.JITConfig{DefaultRole: "viewer"}}, "cipher:secret", []string{"openid", "email"})
	require.NoError(t, e)
	saml, e := identity.CreateProvider(ctx, w, owner, samlRepositoryInput("old-saml"), "cipher:keys", nil)
	require.NoError(t, e)
	_, e = db.Exec(`INSERT INTO workspace_user_identities(workspace_id,provider_id,user_id,subject,email_at_link,email_verified) VALUES($1,$2,$4,'old-oidc','scim-history-member@example.com',true),($1,$3,$4,'old-saml','scim-history-member@example.com',true)`, w, oidc.ID, saml.ID, uid)
	require.NoError(t, e)
	require.NoError(t, db.QueryRow(`INSERT INTO workspace_teams(workspace_id,name,slug) VALUES($1,'History','history') RETURNING id`, w).Scan(&team))
	_, e = db.Exec(`INSERT INTO workspace_team_members(workspace_id,team_id,workspace_member_id) VALUES($1,$2,$3)`, w, team, member)
	require.NoError(t, e)
	_, e = db.Exec(`INSERT INTO workspace_identity_team_grants(workspace_id,provider_id,team_id,workspace_member_id) VALUES($1,$2,$4,$5),($1,$3,$4,$5)`, w, oidc.ID, saml.ID, team, member)
	require.NoError(t, e)
	before := enterpriseHistorySnapshot(t, db, "users", false)
	body, e := migrations.FS.ReadFile("296_enterprise_identity_scim.sql")
	require.NoError(t, e)
	_, e = db.Exec(string(body))
	require.NoError(t, e)
	require.JSONEq(t, before, enterpriseHistorySnapshot(t, db, "users", false))
	var admin bool
	require.NoError(t, db.QueryRow(`SELECT administratively_suspended FROM workspace_members WHERE id=$1`, member).Scan(&admin))
	require.True(t, admin)
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_membership_sources WHERE member_id=$1`, member).Scan(&n))
	require.Equal(t, 3, n, "legacy unbound SCIM is conservative manual plus exact OIDC/SAML bindings")
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE member_id=$1 AND team_id=$2`, member, team).Scan(&n))
	require.Equal(t, 3, n, "manual and both providers remain attributed")
	_, e = db.Exec(`DELETE FROM workspace_membership_sources WHERE member_id=$1 AND source_type='manual'`, member)
	require.NoError(t, e)
	_, e = db.Exec(`UPDATE workspace_members SET administratively_suspended=false WHERE id=$1`, member)
	require.NoError(t, e)
	_, e = db.Exec(string(body))
	require.NoError(t, e)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_membership_sources WHERE member_id=$1`, member).Scan(&n))
	require.Equal(t, 2, n)
	require.NoError(t, db.QueryRow(`SELECT administratively_suspended FROM workspace_members WHERE id=$1`, member).Scan(&admin))
	require.False(t, admin)
}

func TestEnterpriseSCIMEffectiveRoleKeepsExactConnectorOwnership(t *testing.T) {
	ctx, r, owner, w, _, p := scimFixture(t)
	in := scimUserInput(w.ID, "role-ownership")
	u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, e)
	svc := service.NewEnterpriseSCIMService(r)
	other, e := svc.CreateConnector(ctx, w.ID, owner.ID, service.SCIMConnectorInput{Name: "Billing", DefaultRole: "billing"})
	require.NoError(t, e)
	token, e := svc.CreateToken(ctx, w.ID, owner.ID, other.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	q, e := svc.Authenticate(ctx, other.PublicID, token.Secret)
	require.NoError(t, e)
	otherUser, e := r.MutateUser(ctx, q, "", service.SCIMUserMutation{Action: "create", User: in})
	require.NoError(t, e)
	role := func() string {
		var role string
		require.NoError(t, integrationDB.QueryRow(`SELECT role FROM workspace_members WHERE id=$1`, u.MemberID).Scan(&role))
		return role
	}
	require.Equal(t, "developer", role())
	inactive := false
	in.Active = &inactive
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, e)
	require.Equal(t, "billing", role())
	active := true
	in.Active = &active
	_, e = r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, e)
	require.Equal(t, "billing", role(), "reactivating earlier connector cannot silently replace active selected source")
	_, e = r.MutateUser(ctx, q, otherUser.ID, service.SCIMUserMutation{Action: "delete"})
	require.NoError(t, e)
	require.Equal(t, "developer", role())
}

func TestEnterpriseSCIMExpiryIsRecheckedAfterWorkspaceLock(t *testing.T) {
	for _, action := range []string{"read", "write", "still-valid"} {
		t.Run(action, func(t *testing.T) {
			ctx, r, _, w, _, p := scimFixture(t)
			u, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "queued")})
			require.NoError(t, e)
			held, e := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, e)
			t.Cleanup(func() { _ = held.Rollback() })
			require.NoError(t, lockWorkspace(ctx, held, w.ID, true))
			var expiry time.Time
			duration := "2 seconds"
			if action == "still-valid" {
				duration = "1 minute"
			}
			require.NoError(t, integrationDB.QueryRow(`UPDATE workspace_scim_tokens SET expires_at=clock_timestamp()+$2::interval WHERE id=$1 RETURNING expires_at`, p.TokenID, duration).Scan(&expiry))
			before := map[string]string{}
			for _, table := range []string{"workspace_scim_users", "workspace_membership_sources", "workspace_team_membership_sources", "workspace_audit_logs", "domain_events", "domain_event_outbox"} {
				before[table] = enterpriseHistorySnapshot(t, integrationDB, table, false)
			}
			result := make(chan error, 1)
			go func() {
				if action == "write" {
					in := scimUserInput(w.ID, "queued")
					in.DisplayName = "Queued update"
					_, e := r.MutateUser(ctx, p, u.ID, service.SCIMUserMutation{Action: "replace", User: in})
					result <- e
				} else {
					_, e := r.GetUser(ctx, p, u.ID)
					result <- e
				}
			}()
			require.Eventually(t, func() bool {
				var n int
				e := integrationDB.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM workspaces WHERE id=$1%'`).Scan(&n)
				return e == nil && n > 0
			}, time.Second, 10*time.Millisecond, "SCIM request must begin before expiry and wait on tenant lock")
			if action != "still-valid" {
				require.Eventually(t, func() bool {
					var expired bool
					e := integrationDB.QueryRow(`SELECT clock_timestamp()>$1`, expiry).Scan(&expired)
					return e == nil && expired
				}, 3*time.Second, 10*time.Millisecond)
			}
			require.NoError(t, held.Commit())
			e = <-result
			if action == "still-valid" {
				require.NoError(t, e)
			} else {
				assertSCIMStatus(t, e, 401)
				for table, snapshot := range before {
					require.JSONEq(t, snapshot, enterpriseHistorySnapshot(t, integrationDB, table, false), "expired queued operation must not change %s", table)
				}
			}
		})
	}
}
func TestEnterpriseSCIMExpirySweepRechecksAfterWorkspaceLock(t *testing.T) {
	ctx, r, _, w, _, p := scimFixture(t)
	held, e := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, e)
	t.Cleanup(func() { _ = held.Rollback() })
	require.NoError(t, lockWorkspace(ctx, held, w.ID, true))
	var expiry time.Time
	require.NoError(t, integrationDB.QueryRow(`UPDATE workspace_scim_tokens SET expires_at=clock_timestamp()+interval '2 seconds' WHERE id=$1 RETURNING expires_at`, p.TokenID).Scan(&expiry))
	result := make(chan error, 1)
	go func() { result <- r.NotifyExpiringTokens(ctx, 100) }()
	require.Eventually(t, func() bool {
		var n int
		e := integrationDB.QueryRow(`SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM workspaces WHERE id=$1%'`).Scan(&n)
		return e == nil && n > 0
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		var expired bool
		e := integrationDB.QueryRow(`SELECT clock_timestamp()>$1`, expiry).Scan(&expired)
		return e == nil && expired
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, held.Commit())
	require.NoError(t, <-result)
	var n int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMTokenExpiring).Scan(&n))
	require.Zero(t, n, "already expired token must not be announced as expiring after lock wait")
}
func TestEnterpriseSCIMDeletedUserRemovesGroupReferencesAndRevisions(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	a, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "deleted-a")})
	require.NoError(t, e)
	b, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "deleted-b")})
	require.NoError(t, e)
	other, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "deleted-c")})
	require.NoError(t, e)
	g, e := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Grouped", Members: []service.SCIMMember{{Value: a.ID}, {Value: b.ID}}}})
	require.NoError(t, e)
	second, e := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Other Group", Members: []service.SCIMMember{{Value: a.ID}, {Value: b.ID}}}})
	require.NoError(t, e)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	team, e := ws.CreateTeam(ctx, owner.ID, w.ID, service.WorkspaceTeamInput{Name: "Shared", Slug: "delete-shared"})
	require.NoError(t, e)
	for _, group := range []*service.SCIMGroup{g, second} {
		require.NoError(t, r.BindGroup(ctx, w.ID, owner.ID, c.ID, group.ID, service.SCIMGroupBindingInput{Revision: group.Revision, TeamID: &team.ID}))
	}
	g, e = r.GetGroup(ctx, p, g.ID)
	require.NoError(t, e)
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, a.UserID, "viewer", "active"))
	require.NoError(t, ws.AddTeamMember(ctx, owner.ID, w.ID, team.ID, a.MemberID))
	var provider int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_identity_providers WHERE workspace_id=$1 AND type='oidc' LIMIT 1`, w.ID).Scan(&provider))
	_, e = integrationDB.Exec(`INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,provider_id) VALUES($1,$2,$3,'oidc',$4)`, w.ID, a.MemberID, team.ID, provider)
	require.NoError(t, e)
	inactive := false
	in := scimUserInput(w.ID, "deleted-a")
	in.Active = &inactive
	_, e = r.MutateUser(ctx, p, a.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, e)
	deactivated, e := r.GetGroup(ctx, p, g.ID)
	require.NoError(t, e)
	require.Len(t, deactivated.Members, 2)
	require.Equal(t, g.Revision, deactivated.Revision, "deactivation retains assignments")
	active := true
	in.Active = &active
	_, e = r.MutateUser(ctx, p, a.ID, service.SCIMUserMutation{Action: "replace", User: in})
	require.NoError(t, e)
	token, e := service.NewEnterpriseSCIMService(r).CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler.NewEnterpriseSCIMHandler(service.NewEnterpriseSCIMService(r), scimIntegrationLimiter{}, "https://console.example.com/callback").Register(engine)
	request := httptest.NewRequest(http.MethodDelete, "/scim/v2/"+c.PublicID+"/Users/"+a.ID, nil)
	request.Header.Set("Authorization", "Bearer "+token.Secret)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Empty(t, response.Body.String())
	after, e := r.GetGroup(ctx, p, g.ID)
	require.NoError(t, e)
	require.Equal(t, []string{b.ID}, groupMemberIDs(after.Members))
	require.Equal(t, g.Revision+1, after.Revision)
	require.True(t, after.Meta.LastModified.After(g.Meta.LastModified))
	_, e = r.MutateUser(ctx, p, a.ID, service.SCIMUserMutation{Action: "delete"})
	require.NoError(t, e)
	repeated, e := r.GetGroup(ctx, p, g.ID)
	require.NoError(t, e)
	require.Equal(t, after.Revision, repeated.Revision, "repeat deletion does not version Groups twice")
	_, e = r.MutateGroup(ctx, p, g.ID, service.SCIMGroupMutation{Action: "delete", IfMatch: g.Revision})
	assertSCIMStatus(t, e, 412)
	patch := func(path string, value json.RawMessage) {
		_, e := r.MutateGroup(ctx, p, g.ID, service.SCIMGroupMutation{Action: "patch", Patch: &service.SCIMPatchRequest{Schemas: []string{service.SCIMPatchSchema}, Operations: []service.SCIMPatchOperation{{Op: "replace", Path: path, Value: value}}}})
		require.NoError(t, e)
	}
	patch("displayName", json.RawMessage(`"Renamed after deletion"`))
	raw, e := json.Marshal([]service.SCIMMember{{Value: b.ID}, {Value: other.ID}})
	require.NoError(t, e)
	patch("members", raw)
	raw, e = json.Marshal([]service.SCIMMember{{Value: other.ID}})
	require.NoError(t, e)
	patch("members", raw)
	var n int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_scim_group_members WHERE workspace_id=$1 AND connector_id=$2 AND user_id=$3`, w.ID, c.ID, a.ID).Scan(&n))
	require.Zero(t, n)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND source_type='manual' AND active`, w.ID, a.MemberID).Scan(&n))
	require.Equal(t, 1, n)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND scim_group_id=$3 AND active`, w.ID, b.MemberID, second.ID).Scan(&n))
	require.Equal(t, 1, n, "other Group source survives removal from first Group")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND provider_id=$3 AND active`, w.ID, a.MemberID, provider).Scan(&n))
	require.Equal(t, 1, n, "provider source survives grouped User deletion")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='scim.group_member_deleted' AND metadata->>'operation'='remove' AND (metadata->>'removed_count')::int=1 AND actor_user_id IS NULL`, w.ID).Scan(&n))
	require.Equal(t, 2, n, "both Groups have scalar removal audits")
}

func TestEnterpriseSCIMDeletedUserPreservesUnrelatedGroupOwner(t *testing.T) {
	ctx, r, owner, w, c, p := scimFixture(t)
	aInput := scimUserInput(w.ID, "deleted-ordinary")
	a, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: aInput})
	require.NoError(t, e)
	b, e := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "retained-owner")})
	require.NoError(t, e)
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	team, e := ws.CreateTeam(ctx, owner.ID, w.ID, service.WorkspaceTeamInput{Name: "SCIM only", Slug: "owner-scim"})
	require.NoError(t, e)
	shared, e := ws.CreateTeam(ctx, owner.ID, w.ID, service.WorkspaceTeamInput{Name: "Shared", Slug: "owner-shared"})
	require.NoError(t, e)
	groups := []*service.SCIMGroup{}
	for i, teamID := range []int64{team.ID, shared.ID} {
		g, e := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: fmt.Sprintf("Owner Group %d", i), Members: []service.SCIMMember{{Value: a.ID}, {Value: b.ID}}}})
		require.NoError(t, e)
		require.NoError(t, r.BindGroup(ctx, w.ID, owner.ID, c.ID, g.ID, service.SCIMGroupBindingInput{Revision: g.Revision, TeamID: &teamID}))
		g, e = r.GetGroup(ctx, p, g.ID)
		require.NoError(t, e)
		groups = append(groups, g)
	}
	// The other connector's resource and Group for the same global User survive.
	svc := service.NewEnterpriseSCIMService(r)
	other, e := svc.CreateConnector(ctx, w.ID, owner.ID, service.SCIMConnectorInput{Name: "Other", DefaultRole: "viewer"})
	require.NoError(t, e)
	otherToken, e := svc.CreateToken(ctx, w.ID, owner.ID, other.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	q, e := svc.Authenticate(ctx, other.PublicID, otherToken.Secret)
	require.NoError(t, e)
	otherUser, e := r.MutateUser(ctx, q, "", service.SCIMUserMutation{Action: "create", User: aInput})
	require.NoError(t, e)
	otherGroup, e := r.MutateGroup(ctx, q, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Other connector Group", Members: []service.SCIMMember{{Value: otherUser.ID}}}})
	require.NoError(t, e)
	require.NoError(t, r.BindGroup(ctx, w.ID, owner.ID, other.ID, otherGroup.ID, service.SCIMGroupBindingInput{Revision: otherGroup.Revision, TeamID: &shared.ID}))
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, a.UserID, "viewer", "active"))
	for _, member := range []int64{a.MemberID, b.MemberID} {
		require.NoError(t, ws.AddTeamMember(ctx, owner.ID, w.ID, shared.ID, member))
	}
	var provider int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM workspace_identity_providers WHERE workspace_id=$1 AND type='oidc' LIMIT 1`, w.ID).Scan(&provider))
	_, e = integrationDB.Exec(`INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,provider_id) VALUES($1,$2,$3,'oidc',$4)`, w.ID, a.MemberID, shared.ID, provider)
	require.NoError(t, e)
	// Use the supported manual promotion already exercised by the protection tests.
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, b.UserID, "owner", "active"))
	require.NoError(t, ws.ChangeBillingOwner(ctx, owner.ID, w.ID, b.UserID))
	var role, status string
	require.NoError(t, integrationDB.QueryRow(`SELECT role,status FROM workspace_members WHERE id=$1`, b.MemberID).Scan(&role, &status))
	require.Equal(t, "owner", role)
	require.Equal(t, "active", status)
	snapshot := func(query string, args ...any) string {
		t.Helper()
		var raw string
		require.NoError(t, integrationDB.QueryRow(query, args...).Scan(&raw))
		return raw
	}
	ownerQuery := `SELECT jsonb_build_object(
	 'member',(SELECT to_jsonb(m) FROM workspace_members m WHERE id=$1),
	 'sources',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM workspace_membership_sources s WHERE member_id=$1),
	 'team_sources',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM workspace_team_membership_sources s WHERE member_id=$1),
	 'teams',(SELECT jsonb_agg(to_jsonb(m) ORDER BY team_id) FROM workspace_team_members m WHERE workspace_member_id=$1),
	 'user',(SELECT to_jsonb(u) FROM workspace_scim_users u WHERE id=$2),
	 'groups',(SELECT jsonb_agg(to_jsonb(g) ORDER BY group_id) FROM workspace_scim_group_members g WHERE user_id=$2),
	 'billing_owner',(SELECT billing_owner_user_id FROM workspaces WHERE id=$3))::text`
	ownerBefore := snapshot(ownerQuery, b.MemberID, b.ID, w.ID)
	preservedQuery := `SELECT jsonb_build_object(
	 'sources',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM workspace_membership_sources s WHERE member_id=$1 AND connector_id IS DISTINCT FROM $2),
	 'team_sources',(SELECT jsonb_agg(to_jsonb(s) ORDER BY id) FROM workspace_team_membership_sources s WHERE member_id=$1 AND connector_id IS DISTINCT FROM $2),
	 'other_user',(SELECT to_jsonb(u) FROM workspace_scim_users u WHERE id=$3),
	 'other_group',(SELECT to_jsonb(g) FROM workspace_scim_groups g WHERE id=$4),
	 'other_assignment',(SELECT jsonb_agg(to_jsonb(g) ORDER BY group_id) FROM workspace_scim_group_members g WHERE user_id=$3))::text`
	preservedBefore := snapshot(preservedQuery, a.MemberID, c.ID, otherUser.ID, otherGroup.ID)
	globalBefore := enterpriseHistorySnapshot(t, integrationDB, "users", false)
	identitiesBefore := enterpriseHistorySnapshot(t, integrationDB, "auth_identities", false)
	var auditBoundary int64
	require.NoError(t, integrationDB.QueryRow(`SELECT max(id) FROM workspace_audit_logs WHERE workspace_id=$1`, w.ID).Scan(&auditBoundary))
	historyQuery := `SELECT jsonb_agg(to_jsonb(a) ORDER BY id)::text FROM workspace_audit_logs a WHERE workspace_id=$1 AND id<=$2`
	historyBefore := snapshot(historyQuery, w.ID, auditBoundary)
	token, e := svc.CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler.NewEnterpriseSCIMHandler(svc, scimIntegrationLimiter{}, "https://console.example.com/callback").Register(engine)
	request := func(method, resource, body, etag string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, "/scim/v2/"+c.PublicID+"/"+resource, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token.Secret)
		if body != "" {
			req.Header.Set("Content-Type", "application/scim+json")
		}
		if etag != "" {
			req.Header.Set("If-Match", etag)
		}
		res := httptest.NewRecorder()
		engine.ServeHTTP(res, req)
		return res
	}
	res := request(http.MethodDelete, "Users/"+a.ID, "", "")
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
	require.Empty(t, res.Body.String())
	for _, g := range groups {
		res = request(http.MethodGet, "Groups/"+g.ID, "", "")
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		var after service.SCIMGroup
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &after))
		require.Equal(t, []string{b.ID}, groupMemberIDs(after.Members))
		require.Equal(t, fmt.Sprintf(`W/"%d"`, g.Revision+1), res.Header().Get("ETag"))
		require.Equal(t, res.Header().Get("ETag"), after.Meta.Version)
		require.True(t, after.Meta.LastModified.After(g.Meta.LastModified))
		stored, e := r.GetGroup(ctx, p, g.ID)
		require.NoError(t, e)
		require.Equal(t, g.Revision+1, stored.Revision)
		res = request(http.MethodPatch, "Groups/"+g.ID, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"displayName","value":"Stale"}]}`, g.Meta.Version)
		require.Equal(t, http.StatusPreconditionFailed, res.Code, res.Body.String())
	}
	require.JSONEq(t, ownerBefore, snapshot(ownerQuery, b.MemberID, b.ID, w.ID))
	require.JSONEq(t, preservedBefore, snapshot(preservedQuery, a.MemberID, c.ID, otherUser.ID, otherGroup.ID))
	require.JSONEq(t, globalBefore, enterpriseHistorySnapshot(t, integrationDB, "users", false))
	require.JSONEq(t, identitiesBefore, enterpriseHistorySnapshot(t, integrationDB, "auth_identities", false))
	require.JSONEq(t, historyBefore, snapshot(historyQuery, w.ID, auditBoundary))
	res = request(http.MethodGet, "Users/"+a.ID, "", "")
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
	var active, deleted bool
	var revision int64
	require.NoError(t, integrationDB.QueryRow(`SELECT active,deleted,revision FROM workspace_scim_users WHERE id=$1`, a.ID).Scan(&active, &deleted, &revision))
	require.False(t, active)
	require.True(t, deleted)
	require.Equal(t, a.Revision+1, revision)
	var n int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND connector_id=$3`, w.ID, a.MemberID, c.ID).Scan(&n))
	require.Zero(t, n, "only deleted User's exact connector/Group sources are removed")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND workspace_member_id=$2 AND team_id=$3`, w.ID, a.MemberID, shared.ID).Scan(&n))
	require.Equal(t, 1, n, "shared Team remains through manual/provider/other connector sources")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE workspace_id=$1 AND workspace_member_id=$2 AND team_id=$3`, w.ID, a.MemberID, team.ID).Scan(&n))
	require.Zero(t, n, "SCIM-only Team access is removed")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND id>$2 AND action='scim.group_member_deleted' AND actor_user_id IS NULL AND metadata->>'operation'='remove' AND (metadata->>'removed_count')::int=1 AND metadata->>'resource_id' IN($3,$4) AND metadata-ARRAY['category','connector_id','resource_id','operation','removed_count']='{}'::jsonb`, w.ID, auditBoundary, groups[0].ID, groups[1].ID).Scan(&n))
	require.Equal(t, 2, n)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND id>$2 AND action='scim.team_reconciled' AND actor_user_id IS NULL AND (metadata->>'member_id')::bigint=$3 AND (metadata->>'team_id')::bigint=$4`, w.ID, auditBoundary, a.MemberID, team.ID).Scan(&n))
	require.Equal(t, 1, n, "one effective Team removal audit")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2 AND actor_user_id IS NULL AND payload->'data'->>'category'='scim' AND (payload->'data'->>'member_id')::bigint=$3 AND (payload->'data'->>'team_id')::bigint=$4`, w.ID, service.EventWorkspaceTeamMemberRemoved, a.MemberID, team.ID).Scan(&n))
	require.Equal(t, 1, n, "one effective Team removal event")
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2 AND payload->'data'->>'category'='scim' AND (payload->'data'->>'member_id')::bigint=$3`, w.ID, service.EventWorkspaceTeamMemberRemoved, b.MemberID).Scan(&n))
	require.Zero(t, n, "unrelated Owner has no removal event")
	// Repeated deletion leaves resources, sources, audits and events unchanged.
	beforeRepeat := map[string]string{}
	for _, table := range []string{"workspace_scim_users", "workspace_scim_groups", "workspace_scim_group_members", "workspace_members", "workspace_membership_sources", "workspace_team_membership_sources", "workspace_team_members", "workspace_audit_logs", "domain_events", "domain_event_outbox"} {
		beforeRepeat[table] = enterpriseHistorySnapshot(t, integrationDB, table, false)
	}
	res = request(http.MethodDelete, "Users/"+a.ID, "", "")
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
	for table, before := range beforeRepeat {
		require.JSONEq(t, before, enterpriseHistorySnapshot(t, integrationDB, table, false), table)
	}
	res = request(http.MethodDelete, "Users/"+b.ID, "", "")
	require.Equal(t, http.StatusConflict, res.Code, res.Body.String())
	res = request(http.MethodPatch, "Users/"+b.ID, `{"schemas":["urn:ietf:params:scim:api:messages:2.0:PatchOp"],"Operations":[{"op":"replace","path":"active","value":false}]}`, "")
	require.Equal(t, http.StatusConflict, res.Code, res.Body.String())
	require.JSONEq(t, ownerBefore, snapshot(ownerQuery, b.MemberID, b.ID, w.ID))
}

type scimIntegrationLimiter struct{}

func (scimIntegrationLimiter) Allow(context.Context, string, int, time.Duration) (rate.AllowResult, error) {
	return rate.AllowResult{Allowed: true}, nil
}
func TestEnterpriseSCIMHTTPValidationFailureThresholdAndSafeFanout(t *testing.T) {
	ctx, r, owner, w, c, _ := scimFixture(t)
	token, e := service.NewEnterpriseSCIMService(r).CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{})
	require.NoError(t, e)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler.NewEnterpriseSCIMHandler(service.NewEnterpriseSCIMService(r), scimIntegrationLimiter{}, "https://console.example.com/callback").Register(engine)
	body := `{"schemas":["unsupported"],"userName":"private-payload@example.com","emails":[{"value":"private-payload@example.com","primary":true}]}`
	for i := 1; i <= 5; i++ {
		request := httptest.NewRequest(http.MethodPost, "/scim/v2/"+c.PublicID+"/Users", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token.Secret)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code)
		var failures, events int
		require.NoError(t, integrationDB.QueryRow(`SELECT failure_count FROM workspace_scim_connectors WHERE id=$1`, c.ID).Scan(&failures))
		require.Equal(t, i, failures, "one outcome per rejected write")
		require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMSyncFailed).Scan(&events))
		if i < 3 {
			require.Zero(t, events)
		} else {
			require.Equal(t, 1, events, "threshold3 and debounce")
		}
	}
	var raw []byte
	require.NoError(t, integrationDB.QueryRow(`SELECT payload FROM domain_events WHERE workspace_id=$1 AND event_type=$2`, w.ID, service.EventSCIMSyncFailed).Scan(&raw))
	require.NotContains(t, string(raw), "private-payload@example.com")
	require.NotContains(t, string(raw), token.Secret)
	var event service.DomainEvent
	require.NoError(t, json.Unmarshal(raw, &event))
	ids, e := NewNotificationRecipientResolver(integrationDB).Resolve(ctx, &event)
	require.NoError(t, e)
	require.Equal(t, []int64{owner.ID}, ids)
	var outbox int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_event_outbox WHERE event_id=$1`, event.ID).Scan(&outbox))
	require.Equal(t, 1, outbox)
}

func TestEnterpriseSCIMHTTPEmailAddRetryIsImmutable(t *testing.T) {
	for _, pathless := range []bool{false, true} {
		t.Run(fmt.Sprintf("pathless=%t", pathless), func(t *testing.T) {
			ctx, r, owner, w, c, p := scimFixture(t)
			created, err := r.MutateUser(ctx, p, "", service.SCIMUserMutation{Action: "create", User: scimUserInput(w.ID, "email-retry")})
			require.NoError(t, err)
			ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
			team, err := ws.CreateTeam(ctx, owner.ID, w.ID, service.WorkspaceTeamInput{Name: "Retry Team", Slug: "retry-team"})
			require.NoError(t, err)
			group, err := r.MutateGroup(ctx, p, "", service.SCIMGroupMutation{Action: "create", Group: &service.SCIMGroupInput{Schemas: []string{service.SCIMGroupSchema}, DisplayName: "Retry Group", Members: []service.SCIMMember{{Value: created.ID}}}})
			require.NoError(t, err)
			require.NoError(t, r.BindGroup(ctx, w.ID, owner.ID, c.ID, group.ID, service.SCIMGroupBindingInput{Revision: group.Revision, TeamID: &team.ID}))
			svc := service.NewEnterpriseSCIMService(r)
			token, err := svc.CreateToken(ctx, w.ID, owner.ID, c.ID, service.SCIMTokenInput{})
			require.NoError(t, err)
			gin.SetMode(gin.TestMode)
			engine := gin.New()
			handler.NewEnterpriseSCIMHandler(svc, scimIntegrationLimiter{}, "https://console.example.com/callback").Register(engine)
			request := func(method, body, etag string) *httptest.ResponseRecorder {
				t.Helper()
				req := httptest.NewRequest(method, "/scim/v2/"+c.PublicID+"/Users/"+created.ID, strings.NewReader(body))
				req.Header.Set("Authorization", "Bearer "+token.Secret)
				req.Header.Set("Content-Type", "application/scim+json")
				if etag != "" {
					req.Header.Set("If-Match", etag)
				}
				res := httptest.NewRecorder()
				engine.ServeHTTP(res, req)
				return res
			}
			patchBody := func(operation service.SCIMPatchOperation) string {
				t.Helper()
				raw, err := json.Marshal(service.SCIMPatchRequest{Schemas: []string{service.SCIMPatchSchema}, Operations: []service.SCIMPatchOperation{operation}})
				require.NoError(t, err)
				return string(raw)
			}
			addBody := func(emails []service.SCIMEmail) string {
				t.Helper()
				var value any = emails
				path := "emails"
				if pathless {
					path = ""
					value = map[string]any{"emails": emails}
				}
				raw, err := json.Marshal(value)
				require.NoError(t, err)
				return patchBody(service.SCIMPatchOperation{Op: "add", Path: path, Value: raw})
			}
			// Connector outcome/last-used telemetry is intentionally outside resource snapshots.
			preservedTables := []string{"users", "auth_identities", "workspace_user_identities", "workspaces", "workspace_members", "workspace_membership_sources", "workspace_teams", "workspace_team_members", "workspace_team_membership_sources", "workspace_scim_groups", "workspace_scim_group_members", "api_keys", "service_accounts", "usage_logs"}
			preserved := map[string]string{}
			for _, table := range preservedTables {
				preserved[table] = enterpriseHistorySnapshot(t, integrationDB, table, false)
			}
			assertPreserved := func() {
				t.Helper()
				for table, before := range preserved {
					require.JSONEq(t, before, enterpriseHistorySnapshot(t, integrationDB, table, false), table)
				}
			}
			patchCount := func() int {
				t.Helper()
				var count int
				require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action='scim.user_patch'`, w.ID).Scan(&count))
				return count
			}
			initial := request(http.MethodGet, "", "")
			require.Equal(t, http.StatusOK, initial.Code, initial.Body.String())
			secondary := service.SCIMEmail{Value: "alternate@example.com", Type: "other", Display: "Alternate"}
			first := request(http.MethodPatch, addBody([]service.SCIMEmail{secondary}), "")
			require.Equal(t, http.StatusOK, first.Code, first.Body.String())
			stored, err := r.GetUser(ctx, p, created.ID)
			require.NoError(t, err)
			require.Equal(t, created.Revision+1, stored.Revision)
			require.Equal(t, []service.SCIMEmail{created.Emails[0], secondary}, stored.Emails)
			require.True(t, stored.Meta.LastModified.After(created.Meta.LastModified))
			require.Equal(t, stored.Meta.Version, first.Header().Get("ETag"))
			require.Equal(t, 1, patchCount())
			assertPreserved()
			assertRetries := func(reference *httptest.ResponseRecorder, bodies []string) {
				t.Helper()
				before := map[string]string{}
				for _, table := range []string{"workspace_scim_users", "workspace_audit_logs", "domain_events", "domain_event_outbox"} {
					before[table] = enterpriseHistorySnapshot(t, integrationDB, table, false)
				}
				for i := 0; i < 25; i++ {
					for _, body := range bodies {
						res := request(http.MethodPatch, body, "")
						require.Equal(t, http.StatusOK, res.Code, "retry %d: %s", i, res.Body.String())
						require.JSONEq(t, reference.Body.String(), res.Body.String(), "resource including emails/version/lastModified")
						require.Equal(t, reference.Header().Get("ETag"), res.Header().Get("ETag"))
						for table, snapshot := range before {
							require.JSONEq(t, snapshot, enterpriseHistorySnapshot(t, integrationDB, table, false), "retry %d: %s", i, table)
						}
					}
				}
				assertPreserved()
			}
			primary := created.Emails[0]
			normalizedPrimary := primary
			normalizedPrimary.Value = " " + strings.ToUpper(primary.Value) + " "
			normalizedSecondary := secondary
			normalizedSecondary.Value = " ALTERNATE@EXAMPLE.COM "
			assertRetries(first, []string{
				addBody([]service.SCIMEmail{secondary}), addBody([]service.SCIMEmail{primary}),
				addBody([]service.SCIMEmail{normalizedSecondary}), addBody([]service.SCIMEmail{normalizedPrimary}),
				addBody([]service.SCIMEmail{secondary, normalizedSecondary, primary, primary}),
			})
			// A distinct supported complex value sharing a mailbox still changes the resource once.
			distinct := service.SCIMEmail{Value: secondary.Value, Type: "home", Display: "Home"}
			second := request(http.MethodPatch, addBody([]service.SCIMEmail{distinct, distinct}), first.Header().Get("ETag"))
			require.Equal(t, http.StatusOK, second.Code, second.Body.String())
			stored, err = r.GetUser(ctx, p, created.ID)
			require.NoError(t, err)
			require.Equal(t, created.Revision+2, stored.Revision)
			require.Equal(t, []service.SCIMEmail{primary, secondary, distinct}, stored.Emails)
			require.Equal(t, 2, patchCount())
			assertRetries(second, []string{addBody([]service.SCIMEmail{distinct})})
			additional := []service.SCIMEmail{}
			for i := 0; i < 17; i++ {
				additional = append(additional, service.SCIMEmail{Value: fmt.Sprintf("bounded%d@example.com", i)})
			}
			bounded := request(http.MethodPatch, addBody(additional), "")
			require.Equal(t, http.StatusOK, bounded.Code, bounded.Body.String())
			stored, err = r.GetUser(ctx, p, created.ID)
			require.NoError(t, err)
			require.Len(t, stored.Emails, 20)
			require.Equal(t, created.Revision+3, stored.Revision)
			require.Equal(t, 3, patchCount())
			assertRetries(bounded, []string{addBody(stored.Emails)})
			resourceBefore := enterpriseHistorySnapshot(t, integrationDB, "workspace_scim_users", false)
			var historyBoundary int64
			require.NoError(t, integrationDB.QueryRow(`SELECT max(id) FROM workspace_audit_logs`).Scan(&historyBoundary))
			var historyBefore string
			require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_agg(to_jsonb(a) ORDER BY id)::text FROM workspace_audit_logs a WHERE id<=$1`, historyBoundary).Scan(&historyBefore))
			for _, tc := range []struct {
				name, body, etag, scimType string
				status                     int
			}{
				{"stale version", addBody([]service.SCIMEmail{secondary}), initial.Header().Get("ETag"), "", http.StatusPreconditionFailed},
				{"distinct twenty-first", addBody([]service.SCIMEmail{{Value: "distinct21@example.com"}}), "", "invalidValue", http.StatusBadRequest},
				{"raw twenty-one duplicates", addBody(append(append([]service.SCIMEmail{}, stored.Emails...), primary)), "", "invalidValue", http.StatusBadRequest},
				{"malformed value", addBody([]service.SCIMEmail{{Value: "invalid"}}), "", "invalidValue", http.StatusBadRequest},
				{"conflicting primary", addBody([]service.SCIMEmail{{Value: "new@example.com", Primary: true}}), "", "invalidValue", http.StatusBadRequest},
				{"immutable inbox", patchBody(service.SCIMPatchOperation{Op: "replace", Path: `emails[primary eq true].value`, Value: json.RawMessage(`"new@example.com"`)}), "", "mutability", http.StatusBadRequest},
				{"malformed complex attribute", patchBody(service.SCIMPatchOperation{Op: "add", Path: "emails", Value: json.RawMessage(`[{"value":"alternate@example.com","type":true}]`)}), "", "invalidValue", http.StatusBadRequest},
			} {
				t.Run(tc.name, func(t *testing.T) {
					res := request(http.MethodPatch, tc.body, tc.etag)
					require.Equal(t, tc.status, res.Code, res.Body.String())
					if tc.scimType != "" {
						var failure struct {
							Type string `json:"scimType"`
						}
						require.NoError(t, json.Unmarshal(res.Body.Bytes(), &failure))
						require.Equal(t, tc.scimType, failure.Type)
					}
					require.JSONEq(t, resourceBefore, enterpriseHistorySnapshot(t, integrationDB, "workspace_scim_users", false))
					require.Equal(t, 3, patchCount(), "rejections emit no resource mutation audit")
					assertPreserved()
				})
			}
			var historyAfter string
			require.NoError(t, integrationDB.QueryRow(`SELECT jsonb_agg(to_jsonb(a) ORDER BY id)::text FROM workspace_audit_logs a WHERE id<=$1`, historyBoundary).Scan(&historyAfter))
			require.JSONEq(t, historyBefore, historyAfter, "existing audit history is immutable; safe failure telemetry may append")
			final := request(http.MethodGet, "", "")
			require.Equal(t, http.StatusOK, final.Code, final.Body.String())
			require.JSONEq(t, bounded.Body.String(), final.Body.String())
			require.Equal(t, bounded.Header().Get("ETag"), final.Header().Get("ETag"))
		})
	}
}
