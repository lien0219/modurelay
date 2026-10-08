//go:build integration

package repository

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestWorkspaceSecurityMigrationPreservesExistingSSOAndSources(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, selectDockerImage(ctx, postgresImageTag), tcpostgres.WithDatabase("security_history"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := openSQLWithRetry(ctx, dsn, 30*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	historical := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name >= "297" {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		historical[name] = &fstest.MapFile{Data: body}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, historical))
	var owner int64
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('security-history-owner@example.com','historical-hash') RETURNING id`).Scan(&owner))
	workspace, err := service.NewWorkspaceService(NewWorkspaceRepository(db)).CreateOrganization(ctx, owner, "Security history", "security-history")
	require.NoError(t, err)
	identity := NewEnterpriseIdentityRepository(db, enterpriseTestEncryptor{}).(*enterpriseIdentityRepository)
	enabled := true
	provider, err := identity.CreateProvider(ctx, workspace.ID, owner, service.EnterpriseIdentityProviderInput{ProviderKey: "history", Name: "Historical OIDC", IssuerURL: "https://idp.example.com", ClientID: "client", TokenAuthMethod: "client_secret_basic", DiscoveryEnabled: &enabled, ClaimMapping: map[string]any{}, JITConfig: service.JITConfig{Enabled: true, DefaultRole: "viewer"}}, "cipher:historical", []string{"openid", "email"})
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO workspace_user_identities(workspace_id,provider_id,user_id,subject,email_at_link,email_verified) VALUES($1,$2,$3,'history-owner','security-history-owner@example.com',true)`, workspace.ID, provider.ID, owner)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_sso,revision,updated_by_user_id) VALUES($1,true,7,$2) ON CONFLICT(workspace_id) DO UPDATE SET require_sso=true,revision=7,updated_by_user_id=$2`, workspace.ID, owner)
	require.NoError(t, err)
	var oldPolicy string
	require.NoError(t, db.QueryRow(`SELECT to_jsonb(p)::text FROM workspace_security_policies p WHERE workspace_id=$1`, workspace.ID).Scan(&oldPolicy))
	before := map[string]string{}
	for _, table := range []string{"users", "workspaces", "workspace_members", "workspace_membership_sources", "workspace_team_membership_sources", "workspace_user_identities", "api_keys", "service_accounts", "usage_logs", "budget_reservations"} {
		before[table] = enterpriseHistorySnapshot(t, db, table, false)
	}
	require.NoError(t, ApplyMigrations(ctx, db))
	var preserved string
	require.NoError(t, db.QueryRow(`SELECT (to_jsonb(p)-'require_mfa'-'session_max_age_seconds'-'invitation_policy'-'allow_external_members'-'workspace_jit_enabled'-'approved_identity_provider_mode')::text FROM workspace_security_policies p WHERE workspace_id=$1`, workspace.ID).Scan(&preserved))
	require.JSONEq(t, oldPolicy, preserved)
	for table, snapshot := range before {
		require.JSONEq(t, snapshot, enterpriseHistorySnapshot(t, db, table, false), table)
	}
	body, err := migrations.FS.ReadFile("297_workspace_security_policy.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = db.Exec(string(body))
		require.NoError(t, err, "raw latest migration rerun must remain idempotent")
	}
	policy, err := identity.GetPolicy(ctx, workspace.ID, owner)
	require.NoError(t, err)
	require.True(t, policy.RequireSSO)
	require.EqualValues(t, 7, policy.Revision)
	require.False(t, policy.RequireMFA)
	require.Nil(t, policy.SessionMaxAgeSeconds)
	require.Equal(t, "any", policy.InvitationPolicy)
	require.True(t, policy.AllowExternalMembers)
	require.True(t, policy.WorkspaceJITEnabled)
	require.Equal(t, "any_active", policy.ApprovedIdentityProviderMode)
	identityService := service.NewEnterpriseIdentityService(identity, nil, nil, nil)
	assurance := service.WorkspaceAssurance{WorkspaceID: workspace.ID, ProviderID: provider.ID, ProviderRevision: provider.Revision, AuthMethod: "oidc", AuthenticatedAt: time.Now()}
	require.NoError(t, identityService.CheckWorkspaceAccess(ctx, workspace.ID, service.WorkspaceTypeOrganization, service.PrincipalHuman, assurance))
	assurance.AuthenticatedAt = time.Now().Add(-12 * time.Hour)
	require.ErrorIs(t, identityService.CheckWorkspaceAccess(ctx, workspace.ID, service.WorkspaceTypeOrganization, service.PrincipalHuman, assurance), service.ErrSSORequired, "migration cannot extend legacy SSO assurance lifetime")
}
