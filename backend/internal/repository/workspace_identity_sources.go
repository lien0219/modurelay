package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Caller holds the Workspace lock; provenance remains independent of effective
// access, so administrative suspension never erases provisioning attribution.
func reconcileWorkspaceMemberSources(ctx context.Context, tx *sql.Tx, w, m int64) error {
	var role, status, source string
	var provider sql.NullInt64
	var suspended bool
	var effective sql.NullInt64
	if e := tx.QueryRowContext(ctx, `SELECT role,status,membership_source,membership_provider_id,administratively_suspended,effective_membership_source_id FROM workspace_members WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, w, m).Scan(&role, &status, &source, &provider, &suspended, &effective); e != nil {
		return e
	}
	var chosenRole, chosenSource string
	var chosenProvider sql.NullInt64
	var chosenID int64
	e := tx.QueryRowContext(ctx, `SELECT role,source_type,provider_id,id FROM workspace_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND active ORDER BY (source_type='manual') DESC,(id=$5) DESC,(source_type IN('oidc','saml') AND source_type=$3 AND provider_id IS NOT DISTINCT FROM $4) DESC,id LIMIT 1`, w, m, source, provider, effective).Scan(&chosenRole, &chosenSource, &chosenProvider, &chosenID)
	if e == sql.ErrNoRows {
		_, e = tx.ExecContext(ctx, `UPDATE workspace_members SET status='suspended',effective_membership_source_id=NULL,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND (status<>'suspended' OR effective_membership_source_id IS NOT NULL)`, w, m)
		return e
	}
	if e != nil {
		return e
	}
	nextStatus := "active"
	if suspended {
		nextStatus = "suspended"
	}
	_, e = tx.ExecContext(ctx, `UPDATE workspace_members SET role=$3,status=$4,membership_source=$5,membership_provider_id=$6,effective_membership_source_id=$7,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND (role,status,membership_source,membership_provider_id,effective_membership_source_id) IS DISTINCT FROM ($3::varchar,$4::varchar,$5::varchar,$6::bigint,$7::bigint)`, w, m, chosenRole, nextStatus, chosenSource, chosenProvider, chosenID)
	return e
}
func reconcileWorkspaceTeamSources(ctx context.Context, tx *sql.Tx, w, m int64) error {
	// Selected attribution is stable and uses the matching typed source. Manual
	// wins; no provider disable causes implicit removal of attributed grants.
	_, e := tx.ExecContext(ctx, `DELETE FROM workspace_team_members tm WHERE tm.workspace_id=$1 AND tm.workspace_member_id=$2 AND NOT EXISTS(SELECT 1 FROM workspace_team_membership_sources s WHERE s.workspace_id=tm.workspace_id AND s.member_id=tm.workspace_member_id AND s.team_id=tm.team_id AND s.active)`, w, m)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspace_team_members(workspace_id,team_id,workspace_member_id,membership_source,membership_provider_id) SELECT DISTINCT ON(team_id) workspace_id,team_id,member_id,source_type,provider_id FROM workspace_team_membership_sources WHERE workspace_id=$1 AND member_id=$2 AND active ORDER BY team_id,(source_type='manual') DESC,id ON CONFLICT(workspace_id,team_id,workspace_member_id) DO UPDATE SET membership_source=EXCLUDED.membership_source,membership_provider_id=EXCLUDED.membership_provider_id`, w, m)
	return e
}
func upsertManualMemberSource(ctx context.Context, tx *sql.Tx, w, m int64, role string, active bool) error {
	_, e := tx.ExecContext(ctx, `INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,role,active) VALUES($1,$2,'manual',$3,$4) ON CONFLICT(workspace_id,member_id) WHERE source_type='manual' DO UPDATE SET role=EXCLUDED.role,active=EXCLUDED.active`, w, m, role, active)
	return e
}
func upsertProviderMemberSource(ctx context.Context, tx *sql.Tx, w, m, p int64, protocol, role string) error {
	if role == service.WorkspaceRoleOwner {
		role = service.WorkspaceRoleViewer
	}
	_, e := tx.ExecContext(ctx, `INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,provider_id,role,active) VALUES($1,$2,$3,$4,$5,true) ON CONFLICT(workspace_id,member_id,source_type,provider_id) DO UPDATE SET role=EXCLUDED.role,active=true`, w, m, protocol, p, role)
	return e
}
