package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const scimGroupColumns = `id,workspace_id,connector_id,COALESCE(external_id,''),display_name,team_id,revision,deleted,created_at,updated_at`

func scanSCIMGroup(row workspaceScanner) (*service.SCIMGroup, error) {
	g := &service.SCIMGroup{Schemas: []string{service.SCIMGroupSchema}, Members: []service.SCIMMember{}}
	e := row.Scan(&g.ID, &g.WorkspaceID, &g.ConnectorID, &g.ExternalID, &g.DisplayName, &g.TeamID, &g.Revision, &g.Deleted, &g.Meta.Created, &g.Meta.LastModified)
	if e != nil {
		return nil, scimError(e)
	}
	g.Meta.ResourceType = "Group"
	g.Meta.Version = fmt.Sprintf(`W/"%d"`, g.Revision)
	return g, nil
}
func scimGroup(ctx context.Context, q workspaceSQL, p *service.SCIMPrincipal, id string, deleted bool) (*service.SCIMGroup, error) {
	if !service.ValidSCIMResourceID(id) {
		return nil, service.NewSCIMError(404, "", "resource not found")
	}
	g, e := scanSCIMGroup(q.QueryRowContext(ctx, `SELECT `+scimGroupColumns+` FROM workspace_scim_groups WHERE workspace_id=$1 AND connector_id=$2 AND id=$3 AND (NOT deleted OR $4)`, p.WorkspaceID, p.ConnectorID, id, deleted))
	if e != nil {
		return nil, e
	}
	rows, e := q.QueryContext(ctx, `SELECT user_id FROM workspace_scim_group_members WHERE workspace_id=$1 AND connector_id=$2 AND group_id=$3 ORDER BY user_id`, p.WorkspaceID, p.ConnectorID, id)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var m service.SCIMMember
		if e = rows.Scan(&m.Value); e != nil {
			_ = rows.Close()
			return nil, e
		}
		g.Members = append(g.Members, m)
	}
	e = rows.Err()
	_ = rows.Close()
	return g, e
}
func (r *enterpriseSCIMRepository) GetGroup(ctx context.Context, p *service.SCIMPrincipal, id string) (*service.SCIMGroup, error) {
	tx, e := r.beginSync(ctx, p, false)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	g, e := scimGroup(ctx, tx, p, id, false)
	if e != nil {
		return nil, e
	}
	return g, tx.Commit()
}
func (r *enterpriseSCIMRepository) ListGroups(ctx context.Context, p *service.SCIMPrincipal, q service.SCIMListQuery) ([]service.SCIMGroup, int, error) {
	count, offset, e := scimListBounds(q)
	if e != nil {
		return nil, 0, e
	}
	column := ""
	switch strings.ToLower(q.Attribute) {
	case "":
	case "id":
		column = "id"
	case "externalid":
		column = "external_id"
	case "displayname":
		column = "display_name"
	default:
		return nil, 0, service.NewSCIMError(400, "invalidFilter", "unsupported filter")
	}
	tx, e := r.beginSync(ctx, p, false)
	if e != nil {
		return nil, 0, e
	}
	defer func() { _ = tx.Rollback() }()
	where := `workspace_id=$1 AND connector_id=$2 AND NOT deleted`
	args := []any{p.WorkspaceID, p.ConnectorID}
	if column != "" {
		where += " AND " + column + "=$3"
		args = append(args, q.Value)
	}
	var total int
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_scim_groups WHERE `+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	args = append(args, count, offset)
	rows, e := tx.QueryContext(ctx, `SELECT id FROM workspace_scim_groups WHERE `+where+fmt.Sprintf(" ORDER BY created_at,id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, 0, e
	}
	out := []service.SCIMGroup{}
	for _, id := range ids {
		g, e := scimGroup(ctx, tx, p, id, false)
		if e != nil {
			return nil, 0, e
		}
		out = append(out, *g)
	}
	return out, total, tx.Commit()
}
func groupMemberIDs(members []service.SCIMMember) []string {
	ids := make([]string, 0, len(members))
	for _, m := range members {
		ids = append(ids, m.Value)
	}
	sort.Strings(ids)
	return ids
}
func (r *enterpriseSCIMRepository) MutateGroup(ctx context.Context, p *service.SCIMPrincipal, id string, m service.SCIMGroupMutation) (*service.SCIMGroup, error) {
	tx, e := r.beginSync(ctx, p, true)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	in := m.Group
	var current *service.SCIMGroup
	if m.Action == "create" {
		if id != "" {
			return nil, service.NewSCIMError(400, "invalidValue", "invalid create target")
		}
		id, _, e = service.NewSecureToken(32)
		if e != nil {
			return nil, e
		}
	} else {
		current, e = scimGroup(ctx, tx, p, id, true)
		if e != nil {
			return nil, e
		}
		if current.Deleted {
			if m.Action == "delete" {
				return nil, tx.Commit()
			}
			return nil, service.NewSCIMError(404, "", "resource not found")
		}
		if m.IfMatch != 0 && m.IfMatch != current.Revision {
			return nil, service.NewSCIMError(412, "", "resource version changed")
		}
		if m.Action == "patch" {
			in, e = service.ApplySCIMGroupPatch(current, m.Patch)
			if e != nil {
				return nil, e
			}
		}
		if m.Action != "replace" && m.Action != "patch" && m.Action != "delete" {
			return nil, service.NewSCIMError(400, "invalidValue", "invalid mutation")
		}
	}
	if m.Action != "delete" {
		if e = service.ValidateSCIMGroupInput(in); e != nil {
			return nil, e
		}
		if in.ID != "" && in.ID != id {
			return nil, service.NewSCIMError(400, "mutability", "id is immutable")
		}
		if m.Action != "patch" && len(in.Members) > 200 {
			return nil, service.NewSCIMError(413, "tooLarge", "too many incoming members")
		}
		ids := groupMemberIDs(in.Members)
		var n int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_scim_users WHERE workspace_id=$1 AND connector_id=$2 AND id=ANY($3) AND NOT deleted`, p.WorkspaceID, p.ConnectorID, pq.Array(ids)).Scan(&n); e != nil {
			return nil, e
		}
		if n != len(ids) {
			return nil, service.NewSCIMError(400, "invalidValue", "invalid scoped group member")
		}
	}
	added, removed := 0, 0
	if current == nil {
		_, e = tx.ExecContext(ctx, `INSERT INTO workspace_scim_groups(id,workspace_id,connector_id,external_id,display_name) VALUES($1,$2,$3,NULLIF($4,''),$5)`, id, p.WorkspaceID, p.ConnectorID, in.ExternalID, in.DisplayName)
		added = len(in.Members)
	} else if m.Action == "delete" {
		_, e = tx.ExecContext(ctx, `UPDATE workspace_scim_groups SET deleted=true,revision=revision+1,updated_at=now() WHERE id=$1`, id)
		removed = len(current.Members)
	} else {
		oldIDs, newIDs := groupMemberIDs(current.Members), groupMemberIDs(in.Members)
		oldSet, newSet := map[string]bool{}, map[string]bool{}
		for _, v := range oldIDs {
			oldSet[v] = true
		}
		for _, v := range newIDs {
			newSet[v] = true
			if !oldSet[v] {
				added++
			}
		}
		for _, v := range oldIDs {
			if !newSet[v] {
				removed++
			}
		}
		if added == 0 && removed == 0 && current.ExternalID == in.ExternalID && current.DisplayName == in.DisplayName {
			return current, tx.Commit()
		}
		_, e = tx.ExecContext(ctx, `UPDATE workspace_scim_groups SET external_id=NULLIF($2,''),display_name=$3,revision=revision+1,updated_at=now() WHERE id=$1`, id, in.ExternalID, in.DisplayName)
	}
	if e != nil {
		return nil, scimError(e)
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM workspace_scim_group_members WHERE workspace_id=$1 AND connector_id=$2 AND group_id=$3`, p.WorkspaceID, p.ConnectorID, id); e != nil {
		return nil, e
	}
	if m.Action != "delete" {
		ids := groupMemberIDs(in.Members)
		if _, e = tx.ExecContext(ctx, `INSERT INTO workspace_scim_group_members(workspace_id,connector_id,group_id,user_id) SELECT $1,$2,$3,unnest($4::text[])`, p.WorkspaceID, p.ConnectorID, id, pq.Array(ids)); e != nil {
			return nil, scimError(e)
		}
	}
	if e = reconcileSCIMGroup(ctx, tx, p, id); e != nil {
		return nil, e
	}
	if e = appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.group_"+m.Action, "", "scim_connector", p.ConnectorID, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "resource_id": id, "operation": m.Action, "added_count": added, "removed_count": removed}); e != nil {
		return nil, e
	}
	g, e := scimGroup(ctx, tx, p, id, true)
	if e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	if m.Action == "delete" {
		return nil, nil
	}
	return g, nil
}
func (r *enterpriseSCIMRepository) ListGroupBindings(ctx context.Context, w, a, c int64) ([]service.SCIMGroupBinding, error) {
	tx, e := r.beginControl(ctx, w, a, "provisioning.read")
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	if _, e = scimConnector(ctx, tx, w, c); e != nil {
		return nil, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id,display_name,COALESCE(external_id,''),revision,team_id FROM workspace_scim_groups WHERE workspace_id=$1 AND connector_id=$2 AND NOT deleted ORDER BY id LIMIT 100`, w, c)
	if e != nil {
		return nil, e
	}
	out := []service.SCIMGroupBinding{}
	for rows.Next() {
		var b service.SCIMGroupBinding
		if e = rows.Scan(&b.ID, &b.DisplayName, &b.ExternalID, &b.Revision, &b.TeamID); e != nil {
			_ = rows.Close()
			return nil, e
		}
		out = append(out, b)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, e
	}
	return out, tx.Commit()
}
func (r *enterpriseSCIMRepository) BindGroup(ctx context.Context, w, a, c int64, id string, in service.SCIMGroupBindingInput) error {
	if !service.ValidSCIMResourceID(id) || in.Revision <= 0 {
		return service.ErrWorkspaceInvalid
	}
	tx, e := r.beginControl(ctx, w, a, "provisioning.manage")
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback() }()
	if _, e = scimConnector(ctx, tx, w, c); e != nil {
		return e
	}
	p := &service.SCIMPrincipal{WorkspaceID: w, ConnectorID: c}
	g, e := scimGroup(ctx, tx, p, id, false)
	if e != nil {
		return e
	}
	if g.Revision != in.Revision {
		return service.ErrWorkspaceConflict
	}
	if in.TeamID != nil {
		var live bool
		if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_teams WHERE workspace_id=$1 AND id=$2 AND status='active')`, w, *in.TeamID).Scan(&live); e != nil {
			return e
		}
		if !live {
			return service.ErrWorkspaceConflict
		}
	}
	if g.TeamID == nil && in.TeamID == nil || g.TeamID != nil && in.TeamID != nil && *g.TeamID == *in.TeamID {
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `UPDATE workspace_scim_groups SET team_id=$2,revision=revision+1,updated_at=now() WHERE id=$1`, id, in.TeamID); e != nil {
		return e
	}
	if e = reconcileSCIMGroup(ctx, tx, p, id); e != nil {
		return e
	}
	if e = appendSCIMMutation(ctx, tx, w, a, "scim.group_bound", "", "scim_connector", c, service.DomainEventData{"category": "scim", "connector_id": c, "resource_id": id, "operation": "bind"}); e != nil {
		return e
	}
	return tx.Commit()
}
func reconcileSCIMGroupsForUser(ctx context.Context, tx *sql.Tx, p *service.SCIMPrincipal, id string) error {
	var member int64
	var active bool
	if e := tx.QueryRowContext(ctx, `SELECT member_id,active AND NOT deleted FROM workspace_scim_users WHERE workspace_id=$1 AND connector_id=$2 AND id=$3`, p.WorkspaceID, p.ConnectorID, id).Scan(&member, &active); e != nil {
		return e
	}
	before, e := scimMaterializedTeams(ctx, tx, p.WorkspaceID, member)
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE workspace_team_membership_sources SET active=$4 WHERE workspace_id=$1 AND connector_id=$2 AND member_id=$3 AND source_type='scim'`, p.WorkspaceID, p.ConnectorID, member, active); e != nil {
		return e
	}
	if e = reconcileWorkspaceTeamSources(ctx, tx, p.WorkspaceID, member); e != nil {
		return e
	}
	after, e := scimMaterializedTeams(ctx, tx, p.WorkspaceID, member)
	if e != nil {
		return e
	}
	for team := range before {
		if !after[team] {
			if e = appendSCIMTeamEvent(ctx, tx, p, member, team, false); e != nil {
				return e
			}
		}
	}
	for team := range after {
		if !before[team] {
			if e = appendSCIMTeamEvent(ctx, tx, p, member, team, true); e != nil {
				return e
			}
		}
	}
	return nil
}
func reconcileSCIMGroup(ctx context.Context, tx *sql.Tx, p *service.SCIMPrincipal, id string) error {
	rows, e := tx.QueryContext(ctx, `SELECT DISTINCT member_id FROM workspace_team_membership_sources WHERE workspace_id=$1 AND connector_id=$2 AND scim_group_id=$3 UNION SELECT u.member_id FROM workspace_scim_group_members gm JOIN workspace_scim_users u ON u.workspace_id=gm.workspace_id AND u.connector_id=gm.connector_id AND u.id=gm.user_id WHERE gm.workspace_id=$1 AND gm.connector_id=$2 AND gm.group_id=$3`, p.WorkspaceID, p.ConnectorID, id)
	if e != nil {
		return e
	}
	ids := []int64{}
	for rows.Next() {
		var m int64
		if e = rows.Scan(&m); e != nil {
			_ = rows.Close()
			return e
		}
		ids = append(ids, m)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return e
	}
	before := map[int64]map[int64]bool{}
	for _, m := range ids {
		if e = protectSCIMMember(ctx, tx, p.WorkspaceID, m, false); e != nil {
			return e
		}
		before[m], e = scimMaterializedTeams(ctx, tx, p.WorkspaceID, m)
		if e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM workspace_team_membership_sources WHERE workspace_id=$1 AND connector_id=$2 AND scim_group_id=$3`, p.WorkspaceID, p.ConnectorID, id); e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,connector_id,scim_group_id,active) SELECT g.workspace_id,u.member_id,g.team_id,'scim',g.connector_id,g.id,u.active AND NOT u.deleted FROM workspace_scim_groups g JOIN workspace_scim_group_members gm ON gm.workspace_id=g.workspace_id AND gm.connector_id=g.connector_id AND gm.group_id=g.id JOIN workspace_scim_users u ON u.workspace_id=gm.workspace_id AND u.connector_id=gm.connector_id AND u.id=gm.user_id JOIN workspace_teams t ON t.workspace_id=g.workspace_id AND t.id=g.team_id AND t.status='active' WHERE g.workspace_id=$1 AND g.connector_id=$2 AND g.id=$3 AND NOT g.deleted`, p.WorkspaceID, p.ConnectorID, id)
	if e != nil {
		return e
	}
	for _, m := range ids {
		if e = reconcileWorkspaceTeamSources(ctx, tx, p.WorkspaceID, m); e != nil {
			return e
		}
		after, e := scimMaterializedTeams(ctx, tx, p.WorkspaceID, m)
		if e != nil {
			return e
		}
		for team := range before[m] {
			if !after[team] {
				if e = appendSCIMTeamEvent(ctx, tx, p, m, team, false); e != nil {
					return e
				}
			}
		}
		for team := range after {
			if !before[m][team] {
				if e = appendSCIMTeamEvent(ctx, tx, p, m, team, true); e != nil {
					return e
				}
			}
		}
	}
	return nil
}
func scimMaterializedTeams(ctx context.Context, tx *sql.Tx, w, m int64) (map[int64]bool, error) {
	rows, e := tx.QueryContext(ctx, `SELECT team_id FROM workspace_team_members WHERE workspace_id=$1 AND workspace_member_id=$2`, w, m)
	if e != nil {
		return nil, e
	}
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			_ = rows.Close()
			return nil, e
		}
		out[id] = true
	}
	e = rows.Err()
	_ = rows.Close()
	return out, e
}
func appendSCIMTeamEvent(ctx context.Context, tx *sql.Tx, p *service.SCIMPrincipal, m, team int64, add bool) error {
	event := service.EventWorkspaceTeamMemberRemoved
	if add {
		event = service.EventWorkspaceTeamMemberAdded
	}
	return appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.team_reconciled", event, "team", team, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "team_id": team, "member_id": m})
}

// Deactivation retains assignments. Deletion removes references and versions the
// affected Groups in the same transaction as the User tombstone.
func removeDeletedSCIMUserGroups(ctx context.Context, tx *sql.Tx, p *service.SCIMPrincipal, id string, member int64) error {
	rows, e := tx.QueryContext(ctx, `DELETE FROM workspace_scim_group_members WHERE workspace_id=$1 AND connector_id=$2 AND user_id=$3 RETURNING group_id`, p.WorkspaceID, p.ConnectorID, id)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var group string
		if e = rows.Scan(&group); e != nil {
			_ = rows.Close()
			return e
		}
		ids = append(ids, group)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return e
	}
	if len(ids) == 0 {
		return nil
	}
	before, e := scimMaterializedTeams(ctx, tx, p.WorkspaceID, member)
	if e != nil {
		return e
	}
	sort.Strings(ids)
	for _, group := range ids {
		result, e := tx.ExecContext(ctx, `UPDATE workspace_scim_groups SET revision=revision+1,updated_at=clock_timestamp() WHERE workspace_id=$1 AND connector_id=$2 AND id=$3 AND NOT deleted`, p.WorkspaceID, p.ConnectorID, group)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		// Other members can have been manually promoted to Owner. Remove only
		// the deletion target's attribution without reconciling those members.
		if _, e = tx.ExecContext(ctx, `DELETE FROM workspace_team_membership_sources WHERE workspace_id=$1 AND connector_id=$2 AND scim_group_id=$3 AND member_id=$4 AND source_type='scim'`, p.WorkspaceID, p.ConnectorID, group, member); e != nil {
			return e
		}
		if n > 0 {
			if e = appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.group_member_deleted", "", "scim_connector", p.ConnectorID, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "resource_id": group, "operation": "remove", "removed_count": 1}); e != nil {
				return e
			}
		}
	}
	if e = reconcileWorkspaceTeamSources(ctx, tx, p.WorkspaceID, member); e != nil {
		return e
	}
	after, e := scimMaterializedTeams(ctx, tx, p.WorkspaceID, member)
	if e != nil {
		return e
	}
	for team := range before {
		if !after[team] {
			if e = appendSCIMTeamEvent(ctx, tx, p, member, team, false); e != nil {
				return e
			}
		}
	}
	return nil
}
