package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const scimUserColumns = `id,workspace_id,connector_id,user_id,member_id,COALESCE(external_id,''),username,active,attributes,revision,deleted,created_at,updated_at`

func scanSCIMUser(row workspaceScanner) (*service.SCIMUser, error) {
	u := &service.SCIMUser{}
	var raw []byte
	e := row.Scan(&u.ID, &u.WorkspaceID, &u.ConnectorID, &u.UserID, &u.MemberID, &u.ExternalID, &u.UserName, &u.Active, &raw, &u.Revision, &u.Deleted, &u.Meta.Created, &u.Meta.LastModified)
	if e != nil {
		return nil, scimError(e)
	}
	var in service.SCIMUserInput
	if e = json.Unmarshal(raw, &in); e != nil {
		return nil, e
	}
	u.Schemas = []string{service.SCIMUserSchema}
	u.Name = in.Name
	u.DisplayName = in.DisplayName
	u.Emails = in.Emails
	u.Meta.ResourceType = "User"
	u.Meta.Version = fmt.Sprintf(`W/"%d"`, u.Revision)
	return u, nil
}
func scimUser(ctx context.Context, q workspaceSQL, p *service.SCIMPrincipal, id string, deleted bool) (*service.SCIMUser, error) {
	if !service.ValidSCIMResourceID(id) {
		return nil, service.NewSCIMError(404, "", "resource not found")
	}
	return scanSCIMUser(q.QueryRowContext(ctx, `SELECT `+scimUserColumns+` FROM workspace_scim_users WHERE workspace_id=$1 AND connector_id=$2 AND id=$3 AND (NOT deleted OR $4)`, p.WorkspaceID, p.ConnectorID, id, deleted))
}
func (r *enterpriseSCIMRepository) GetUser(ctx context.Context, p *service.SCIMPrincipal, id string) (*service.SCIMUser, error) {
	tx, e := r.beginSync(ctx, p, false)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	u, e := scimUser(ctx, tx, p, id, false)
	if e != nil {
		return nil, e
	}
	return u, tx.Commit()
}
func scimListBounds(q service.SCIMListQuery) (int, int, error) {
	if q.StartIndex == 0 {
		q.StartIndex = 1
	}
	if q.StartIndex < 1 || q.Count < 0 || q.Count > 100 {
		return 0, 0, service.NewSCIMError(400, "invalidValue", "invalid pagination")
	}
	return q.Count, q.StartIndex - 1, nil
}
func (r *enterpriseSCIMRepository) ListUsers(ctx context.Context, p *service.SCIMPrincipal, q service.SCIMListQuery) ([]service.SCIMUser, int, error) {
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
	case "username":
		column = "username"
		q.Value = strings.ToLower(strings.TrimSpace(q.Value))
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
	if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_scim_users WHERE `+where, args...).Scan(&total); e != nil {
		return nil, 0, e
	}
	args = append(args, count, offset)
	rows, e := tx.QueryContext(ctx, `SELECT `+scimUserColumns+` FROM workspace_scim_users WHERE `+where+fmt.Sprintf(" ORDER BY created_at,id LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if e != nil {
		return nil, 0, e
	}
	out := []service.SCIMUser{}
	for rows.Next() {
		u, e := scanSCIMUser(rows)
		if e != nil {
			_ = rows.Close()
			return nil, 0, e
		}
		out = append(out, *u)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return nil, 0, e
	}
	return out, total, tx.Commit()
}
func resolveSCIMUser(ctx context.Context, tx *sql.Tx, w int64, email string) (int64, error) {
	domain, e := service.NormalizeEnterpriseDomain(email[strings.LastIndex(email, "@")+1:])
	if e != nil {
		return 0, service.NewSCIMError(409, "uniqueness", "identity cannot be provisioned")
	}
	var verified bool
	if e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_domains WHERE workspace_id=$1 AND normalized_domain=$2 AND status='verified')`, w, domain).Scan(&verified); e != nil {
		return 0, e
	}
	if !verified {
		return 0, service.NewSCIMError(409, "uniqueness", "identity cannot be provisioned")
	}
	if e = lockOIDCEmailIdentity(ctx, tx, email); e != nil {
		return 0, e
	}
	rows, e := tx.QueryContext(ctx, `SELECT id FROM users WHERE lower(trim(email))=$1 AND deleted_at IS NULL ORDER BY id LIMIT 2`, email)
	if e != nil {
		return 0, e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			_ = rows.Close()
			return 0, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	_ = rows.Close()
	if e != nil {
		return 0, e
	}
	if len(ids) > 1 {
		return 0, service.NewSCIMError(409, "uniqueness", "identity cannot be provisioned")
	}
	if len(ids) == 1 {
		var safe bool
		e = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users u WHERE u.id=$1 AND u.status='active' AND u.deleted_at IS NULL AND (EXISTS(SELECT 1 FROM auth_identities a WHERE a.user_id=u.id AND a.provider_type='email' AND a.provider_key='email' AND lower(trim(a.provider_subject))=$2 AND a.verified_at IS NOT NULL) OR EXISTS(SELECT 1 FROM workspace_user_identities i JOIN workspace_identity_providers p ON p.workspace_id=i.workspace_id AND p.id=i.provider_id AND p.status='active' WHERE i.workspace_id=$3 AND i.user_id=u.id AND i.email_verified AND lower(trim(i.email_at_link))=$2)))`, ids[0], email, w).Scan(&safe)
		if e != nil {
			return 0, e
		}
		if !safe {
			return 0, service.NewSCIMError(409, "uniqueness", "identity cannot be provisioned")
		}
		return ids[0], nil
	}
	exists, e := oidcEmailAlreadyExists(ctx, tx, email)
	if e != nil {
		return 0, e
	}
	if exists {
		return 0, service.NewSCIMError(409, "uniqueness", "identity cannot be provisioned")
	}
	secret, _, e := service.NewSecureToken(32)
	if e != nil {
		return 0, e
	}
	hash, e := service.HashPasswordForEnterprise(secret)
	if e != nil {
		return 0, e
	}
	var id int64
	if e = tx.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,signup_source) VALUES($1,$2,'user','active','scim') RETURNING id`, email, hash).Scan(&id); e != nil {
		return 0, scimError(e)
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO auth_identities(user_id,provider_type,provider_key,provider_subject,verified_at,metadata) VALUES($1,'email','email',$2,now(),'{"category":"scim"}')`, id, email)
	return id, e
}
func protectSCIMMember(ctx context.Context, tx *sql.Tx, w, m int64, deactivate bool) error {
	var role string
	var billing bool
	e := tx.QueryRowContext(ctx, `SELECT m.role,m.user_id=w.billing_owner_user_id FROM workspace_members m JOIN workspaces w ON w.id=m.workspace_id WHERE m.workspace_id=$1 AND m.id=$2`, w, m).Scan(&role, &billing)
	if e != nil {
		return e
	}
	if role == service.WorkspaceRoleOwner || billing && deactivate {
		return service.NewSCIMError(409, "mutability", "protected workspace identity")
	}
	return nil
}
func normalizedSCIMUser(in *service.SCIMUserInput, id string) (*service.SCIMUserInput, string, []byte, error) {
	email, e := service.ValidateSCIMUserInput(in)
	if e != nil {
		return nil, "", nil, e
	}
	if in.ID != "" && in.ID != id {
		return nil, "", nil, service.NewSCIMError(400, "mutability", "id is immutable")
	}
	copy := *in
	copy.ID = id
	copy.UserName = strings.ToLower(strings.TrimSpace(copy.UserName))
	copy.Emails = append([]service.SCIMEmail(nil), in.Emails...)
	for i := range copy.Emails {
		copy.Emails[i].Value = strings.ToLower(strings.TrimSpace(copy.Emails[i].Value))
		if len(copy.Emails) == 1 {
			copy.Emails[i].Primary = true
		}
	}
	if copy.Active == nil {
		a := true
		copy.Active = &a
	}
	raw, e := json.Marshal(copy)
	if len(raw) > 65536 {
		return nil, "", nil, service.NewSCIMError(413, "tooLarge", "resource too large")
	}
	return &copy, email, raw, e
}
func (r *enterpriseSCIMRepository) MutateUser(ctx context.Context, p *service.SCIMPrincipal, id string, m service.SCIMUserMutation) (*service.SCIMUser, error) {
	tx, e := r.beginSync(ctx, p, true)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback() }()
	var u *service.SCIMUser
	in := m.User
	action := m.Action
	if action == "create" {
		if id != "" {
			return nil, service.NewSCIMError(400, "invalidValue", "invalid create target")
		}
		id, _, e = service.NewSecureToken(32)
		if e != nil {
			return nil, e
		}
	} else {
		u, e = scimUser(ctx, tx, p, id, true)
		if e != nil {
			return nil, e
		}
		if u.Deleted {
			if action == "delete" {
				return nil, tx.Commit()
			}
			return nil, service.NewSCIMError(404, "", "resource not found")
		}
		if m.IfMatch != 0 && m.IfMatch != u.Revision {
			return nil, service.NewSCIMError(412, "", "resource version changed")
		}
		if action == "patch" {
			in, e = service.ApplySCIMUserPatch(u, m.Patch)
			if e != nil {
				return nil, e
			}
		}
		if action != "replace" && action != "patch" && action != "delete" {
			return nil, service.NewSCIMError(400, "invalidValue", "invalid mutation")
		}
	}
	var raw []byte
	var email string
	if action != "delete" {
		in, email, raw, e = normalizedSCIMUser(in, id)
		if e != nil {
			return nil, e
		}
	}
	if u == nil {
		if e = checkWorkspaceMemberAdmissionTx(ctx, tx, p.WorkspaceID, service.WorkspaceMemberAdmissionRequest{Source: service.AdmissionSCIMCreate, Email: email}); e != nil {
			return nil, scimAdmissionError(e)
		}
		uid, e := resolveSCIMUser(ctx, tx, p.WorkspaceID, email)
		if e != nil {
			return nil, e
		}
		var member int64
		e = tx.QueryRowContext(ctx, `SELECT id FROM workspace_members WHERE workspace_id=$1 AND user_id=$2`, p.WorkspaceID, uid).Scan(&member)
		if errors.Is(e, sql.ErrNoRows) {
			if _, e = tx.ExecContext(ctx, `SELECT set_config('modurelay.scim_insert','true',true)`); e != nil {
				return nil, e
			}
			e = tx.QueryRowContext(ctx, `INSERT INTO workspace_members(workspace_id,user_id,role,status,membership_source) SELECT $1,$2,default_role,'suspended','scim' FROM workspace_scim_connectors WHERE id=$3 RETURNING id`, p.WorkspaceID, uid, p.ConnectorID).Scan(&member)
		}
		if e != nil {
			return nil, scimError(e)
		}
		if e = protectSCIMMember(ctx, tx, p.WorkspaceID, member, !*in.Active); e != nil {
			return nil, e
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO workspace_scim_users(id,workspace_id,connector_id,user_id,member_id,external_id,username,primary_email,active,attributes) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10)`, id, p.WorkspaceID, p.ConnectorID, uid, member, in.ExternalID, in.UserName, email, *in.Active, raw)
		if e != nil {
			return nil, scimError(e)
		}
		_, e = tx.ExecContext(ctx, `INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,connector_id,scim_user_id,role,active) SELECT $1,$2,'scim',$3,$4,default_role,$5 FROM workspace_scim_connectors WHERE id=$3`, p.WorkspaceID, member, p.ConnectorID, id, *in.Active)
		if e != nil {
			return nil, e
		}
		u, e = scimUser(ctx, tx, p, id, true)
		if e != nil {
			return nil, e
		}
	} else {
		deactivate := action == "delete" || !*in.Active
		if e = protectSCIMMember(ctx, tx, p.WorkspaceID, u.MemberID, deactivate); e != nil {
			return nil, e
		}
		if !deactivate {
			if e = checkWorkspaceMemberAdmissionTx(ctx, tx, p.WorkspaceID, service.WorkspaceMemberAdmissionRequest{Source: service.AdmissionSCIM, MemberID: u.MemberID, UserID: u.UserID}); e != nil {
				return nil, scimAdmissionError(e)
			}
		}
		if action == "delete" {
			_, e = tx.ExecContext(ctx, `UPDATE workspace_scim_users SET active=false,deleted=true,revision=revision+1,updated_at=now() WHERE id=$1`, id)
		} else {
			var bound string
			var old []byte
			if e = tx.QueryRowContext(ctx, `SELECT primary_email,attributes FROM workspace_scim_users WHERE id=$1`, id).Scan(&bound, &old); e != nil {
				return nil, e
			}
			if email != bound {
				return nil, service.NewSCIMError(400, "mutability", "primary inbox is immutable")
			}
			var same bool
			if e = tx.QueryRowContext(ctx, `SELECT attributes=$2::jsonb AND active=$3 AND COALESCE(external_id,'')=$4 AND username=$5 FROM workspace_scim_users WHERE id=$1`, id, raw, *in.Active, in.ExternalID, in.UserName).Scan(&same); e != nil {
				return nil, e
			}
			if same {
				return u, tx.Commit()
			}
			_, e = tx.ExecContext(ctx, `UPDATE workspace_scim_users SET external_id=NULLIF($2,''),username=$3,active=$4,attributes=$5,revision=revision+1,updated_at=now() WHERE id=$1`, id, in.ExternalID, in.UserName, *in.Active, raw)
		}
		if e != nil {
			return nil, scimError(e)
		}
		u, e = scimUser(ctx, tx, p, id, true)
		if e != nil {
			return nil, e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE workspace_membership_sources SET active=$2 WHERE scim_user_id=$1`, id, u.Active && !u.Deleted); e != nil {
			return nil, e
		}
	}
	if e = reconcileSCIMMember(ctx, tx, p, u.MemberID); e != nil {
		return nil, e
	}
	if action == "delete" {
		e = removeDeletedSCIMUserGroups(ctx, tx, p, id, u.MemberID)
	} else {
		e = reconcileSCIMGroupsForUser(ctx, tx, p, id)
	}
	if e != nil {
		return nil, e
	}
	if e = appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.user_"+action, "", "scim_connector", p.ConnectorID, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "resource_id": id, "operation": action}); e != nil {
		return nil, e
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	if action == "delete" {
		return nil, nil
	}
	return u, nil
}
func reconcileSCIMMember(ctx context.Context, tx *sql.Tx, p *service.SCIMPrincipal, m int64) error {
	var oldRole, oldStatus string
	var uid int64
	if e := tx.QueryRowContext(ctx, `SELECT role,status,user_id FROM workspace_members WHERE workspace_id=$1 AND id=$2`, p.WorkspaceID, m).Scan(&oldRole, &oldStatus, &uid); e != nil {
		return e
	}
	if e := reconcileWorkspaceMemberSources(ctx, tx, p.WorkspaceID, m); e != nil {
		return scimAdmissionError(e)
	}
	var role, status string
	if e := tx.QueryRowContext(ctx, `SELECT role,status FROM workspace_members WHERE workspace_id=$1 AND id=$2`, p.WorkspaceID, m).Scan(&role, &status); e != nil {
		return e
	}
	if oldRole == role && oldStatus == status {
		return nil
	}
	event := service.EventMemberRoleChanged
	if status == "suspended" {
		event = service.EventMemberSuspended
	} else if oldStatus != "active" {
		event = service.EventMemberJoined
	}
	return appendSCIMMutation(ctx, tx, p.WorkspaceID, 0, "scim.member_reconciled", event, "member", m, service.DomainEventData{"category": "scim", "connector_id": p.ConnectorID, "member_id": m, "user_id": uid, "role": role, "status": status})
}
