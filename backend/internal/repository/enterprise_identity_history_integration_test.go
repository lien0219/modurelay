//go:build integration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestEnterpriseIdentityMigrationsPreservePhaseAAndGatewayRows(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, selectDockerImage(ctx, postgresImageTag), tcpostgres.WithDatabase("identity_history"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
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
		if name >= "294" {
			continue
		}
		body, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		historical[name] = &fstest.MapFile{Data: body}
	}
	require.Contains(t, historical, "293_workspace_governance_teams_project_access.sql")
	require.NoError(t, applyMigrationsFS(ctx, db, historical))
	var owner, member int64
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('history-owner@example.com','historical-hash') RETURNING id`).Scan(&owner))
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('history-member@example.com','historical-hash') RETURNING id`).Scan(&member))
	workspaces := service.NewWorkspaceService(NewWorkspaceRepository(db))
	workspace, err := workspaces.CreateOrganization(ctx, owner, "History", "identity-history")
	require.NoError(t, err)
	project, err := workspaces.CreateProject(ctx, owner, workspace.ID, service.ProjectInput{Name: "Production", Slug: "production"})
	require.NoError(t, err)
	_, token, err := workspaces.CreateInvitation(ctx, owner, workspace.ID, "history-member@example.com", "developer", time.Hour)
	require.NoError(t, err)
	_, err = workspaces.AcceptInvitation(ctx, member, token)
	require.NoError(t, err)
	team, err := workspaces.CreateTeam(ctx, owner, workspace.ID, service.WorkspaceTeamInput{Name: "History", Slug: "history"})
	require.NoError(t, err)
	var memberID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, workspace.ID, member).Scan(&memberID))
	require.NoError(t, workspaces.AddTeamMember(ctx, owner, workspace.ID, team.ID, memberID))
	_, err = workspaces.CreateProjectAccessGrant(ctx, owner, workspace.ID, project.ID, service.ProjectAccessGrantInput{SubjectType: service.ProjectAccessSubjectTeam, SubjectID: team.ID, Role: service.ProjectAccessRoleDeveloper})
	require.NoError(t, err)
	var directKey, machine, account int64
	require.NoError(t, db.QueryRow(`INSERT INTO api_keys(user_id,project_id,key,name) VALUES($1,$2,'sk-phase-a-history','Direct') RETURNING id`, owner, project.ID).Scan(&directKey))
	require.NoError(t, db.QueryRow(`INSERT INTO service_accounts(workspace_id,project_id,name,slug,created_by_user_id) VALUES($1,$2,'History machine','history-machine',$3) RETURNING id`, workspace.ID, project.ID, owner).Scan(&machine))
	_, err = db.Exec(`INSERT INTO api_keys(project_id,service_account_id,key,key_suffix,name) VALUES($1,$2,'sha256:'||repeat('a',64),'abcd','Machine')`, project.ID, machine)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`INSERT INTO accounts(name,platform,type) VALUES('History upstream','openai','apikey') RETURNING id`).Scan(&account))
	_, err = db.Exec(`INSERT INTO budget_reservations(id,request_id,actor_user_id,api_key_id,workspace_id,project_id,billing_principal_user_id,period_start,period_end,project_period_start,project_period_end,estimate,actual,status,finalized_at) VALUES('b46210d9-af1b-4ba9-80ab-000000000001','identity-history',$1,$2,$3,$4,$1,'2026-10-01','2026-11-01','2026-10-01','2026-11-01',3,2.5,'finalized',now())`, owner, directKey, workspace.ID, project.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost,workspace_id,project_id,billing_principal_user_id,budget_reservation_id,resolved_platform) VALUES($1,$2,$3,'identity-history','historical-model',2.5,$4,$5,$1,'b46210d9-af1b-4ba9-80ab-000000000001','openai')`, owner, directKey, account, workspace.ID, project.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost) VALUES($1,$2,$3,'identity-legacy-history','legacy-model',1.5)`, owner, directKey, account)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO workspace_policies(workspace_id,allowed_models,rpm_limit,created_by_user_id,updated_by_user_id) VALUES($1,ARRAY['historical-model'],10,$2,$2)`, workspace.ID, owner)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO workspace_budget_policies(workspace_id,amount) VALUES($1,1000)`, workspace.ID)
	require.NoError(t, err)
	tables := []string{"users", "workspaces", "workspace_members", "projects", "workspace_teams", "workspace_team_members", "project_access_grants", "api_keys", "service_accounts", "accounts", "workspace_policies", "workspace_budget_policies", "budget_reservations", "usage_logs", "usage_tenant_hourly_rollups", "workspace_audit_logs", "domain_events", "domain_event_outbox"}
	snapshots := map[string]string{}
	for _, table := range tables {
		snapshots[table] = enterpriseHistorySnapshot(t, db, table, false)
	}
	require.NoError(t, ApplyMigrations(ctx, db), "full history through 293 upgrades to Phase B")
	require.NoError(t, ApplyMigrations(ctx, db), "checksum-aware rerun is idempotent")
	for _, table := range tables {
		require.JSONEq(t, snapshots[table], enterpriseHistorySnapshot(t, db, table, true), "historical %s rows must remain unchanged", table)
	}
	var unexpected int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_members WHERE membership_source<>'manual' OR membership_provider_id IS NOT NULL`).Scan(&unexpected))
	require.Zero(t, unexpected)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_team_members WHERE membership_source<>'manual' OR membership_provider_id IS NOT NULL`).Scan(&unexpected))
	require.Zero(t, unexpected)
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_security_policies WHERE require_sso`).Scan(&unexpected))
	require.Zero(t, unexpected)
	var policies, workspaceCount int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspace_security_policies`).Scan(&policies))
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM workspaces`).Scan(&workspaceCount))
	require.Equal(t, workspaceCount, policies)
}

func enterpriseHistorySnapshot(t *testing.T, db *sql.DB, table string, upgraded bool) string {
	t.Helper()
	expression := "to_jsonb(row)"
	if upgraded && (table == "workspace_members" || table == "workspace_team_members") {
		expression += "-'membership_source'-'membership_provider_id'"
	}
	// The table names come exclusively from the fixed test allowlist above.
	query := `SELECT COALESCE(jsonb_agg(value ORDER BY value::text),'[]'::jsonb)::text FROM (SELECT ` + expression + ` AS value FROM ` + table + ` AS row) snapshot`
	var result string
	require.NoError(t, db.QueryRow(strings.TrimSpace(query)).Scan(&result))
	return result
}
