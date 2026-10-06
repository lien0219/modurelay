package migrations

import (
	"strings"
	"testing"
)

func TestWorkspaceGovernanceMigrationDefinesCompatibleTeamsAndProjectGrants(t *testing.T) {
	b, err := FS.ReadFile("293_workspace_governance_teams_project_access.sql")
	if err != nil {
		t.Fatalf("read governance migration: %v", err)
	}
	s := strings.ToLower(string(b))
	compact := strings.Join(strings.Fields(s), " ")
	for _, want := range []string{
		"alter table workspaces add column if not exists project_access_mode",
		"default 'all_projects'",
		"check (project_access_mode in ('all_projects', 'assigned_projects'))",
		"create table if not exists workspace_teams",
		"create table if not exists workspace_team_members",
		"create table if not exists project_access_grants",
		"subject_type",
		"check (subject_type in ('member', 'team'))",
		"check (role in ('viewer', 'developer', 'admin'))",
		"foreign key (workspace_id, project_id)",
		"foreign key (workspace_id, team_id)",
		"foreign key (workspace_id, workspace_member_id)",
		"create or replace function validate_project_access_grant_subject",
		"create trigger project_access_grants_subject_scope",
		"create index",
	} {
		if !strings.Contains(compact, want) {
			t.Errorf("migration missing %q", want)
		}
	}
	for _, forbidden := range []string{"update usage_logs set", "update api_keys set", "delete from usage_logs", "drop table workspaces"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("governance migration must not contain %q", forbidden)
		}
	}
}
