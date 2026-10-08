package repository

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

const workspaceSecurityPolicyQuery = `SELECT w.id,COALESCE(p.require_sso,false),p.sso_grace_until,COALESCE(p.revision,1),p.updated_by_user_id,COALESCE(p.updated_at,w.updated_at),COALESCE(p.require_mfa,false),p.session_max_age_seconds,COALESCE(p.invitation_policy,'any'),COALESCE(p.allow_external_members,true),COALESCE(p.workspace_jit_enabled,true),COALESCE(p.approved_identity_provider_mode,'any_active'),ARRAY(SELECT provider_id FROM workspace_security_approved_providers ap WHERE ap.workspace_id=w.id ORDER BY provider_id) FROM workspaces w LEFT JOIN workspace_security_policies p ON p.workspace_id=w.id WHERE w.id=$1`

func loadWorkspaceSecurityPolicy(ctx context.Context, q workspaceSQL, workspaceID int64) (*service.WorkspaceSecurityPolicy, error) {
	return scanIdentityPolicy(q.QueryRowContext(ctx, workspaceSecurityPolicyQuery, workspaceID))
}

// Caller owns the Workspace row lock; all admission reads use its transaction.
func loadWorkspaceSecurityPolicyTx(ctx context.Context, tx *sql.Tx, workspaceID int64) (*service.WorkspaceSecurityPolicy, error) {
	policy, err := loadWorkspaceSecurityPolicy(ctx, tx, workspaceID)
	if err != nil && !errors.Is(err, service.ErrWorkspaceNotFound) {
		return nil, service.ErrSecurityPolicyUnavailable.WithCause(err)
	}
	return policy, err
}

func describeWorkspaceSecurityPolicyTx(ctx context.Context, tx *sql.Tx, policy *service.WorkspaceSecurityPolicy) error {
	policy.VerifiedDomains = []string{}
	rows, err := tx.QueryContext(ctx, `SELECT normalized_domain FROM workspace_domains WHERE workspace_id=$1 AND status='verified' ORDER BY normalized_domain`, policy.WorkspaceID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var domain string
		if err = rows.Scan(&domain); err != nil {
			_ = rows.Close()
			return err
		}
		policy.VerifiedDomains = append(policy.VerifiedDomains, domain)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	return tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_members m JOIN users u ON u.id=m.user_id AND u.status='active' AND u.deleted_at IS NULL WHERE m.workspace_id=$1 AND m.status='active' AND NOT EXISTS(SELECT 1 FROM workspace_domains d WHERE d.workspace_id=m.workspace_id AND d.status='verified' AND d.normalized_domain=lower(rtrim(split_part(u.email,'@',2),'.')))`, policy.WorkspaceID).Scan(&policy.ExternalMemberCount)
}

// Match the global user-lifecycle and invitation lock order: User -> Workspace -> Policy.
// TOTP disable locks only User and reads the requirements without Workspace locks.
// NO KEY UPDATE still excludes factor/lifecycle changes while allowing audit
// foreign-key KEY SHARE locks held by Workspace-first transactions.
func (r *enterpriseIdentityRepository) beginSecurityMutation(ctx context.Context, workspaceID, actorID int64) (*sql.Tx, *service.WorkspaceAccess, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, false, err
	}
	fail := func(err error) (*sql.Tx, *service.WorkspaceAccess, bool, error) {
		_ = tx.Rollback()
		return nil, nil, false, err
	}
	var enrolled bool
	if err = tx.QueryRowContext(ctx, `SELECT totp_enabled AND totp_secret_encrypted IS NOT NULL AND totp_secret_encrypted<>'' FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR NO KEY UPDATE`, actorID).Scan(&enrolled); err != nil {
		return fail(enterpriseIdentityError(err))
	}
	if err = lockWorkspace(ctx, tx, workspaceID, true); err != nil {
		return fail(err)
	}
	access, err := workspaceAccess(ctx, tx, actorID, workspaceID, 0)
	if err != nil {
		return fail(err)
	}
	if err = service.CheckWorkspacePermission(access, "workspace_security.update"); err != nil {
		return fail(err)
	}
	if access.Workspace.Type != service.WorkspaceTypeOrganization || access.Member.Role != service.WorkspaceRoleOwner {
		return fail(service.ErrWorkspaceForbidden)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_security_policies(workspace_id) VALUES($1) ON CONFLICT DO NOTHING`, workspaceID); err != nil {
		return fail(err)
	}
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM workspace_security_policies WHERE workspace_id=$1 FOR UPDATE`, workspaceID).Scan(&revision); err != nil {
		return fail(err)
	}
	return tx, access, enrolled, nil
}

func validateSecurityProviderIDsTx(ctx context.Context, tx *sql.Tx, policy service.WorkspaceSecurityPolicy) error {
	if len(policy.ApprovedIdentityProviderIDs) == 0 {
		return nil
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_identity_providers WHERE workspace_id=$1 AND id=ANY($2)`, policy.WorkspaceID, pq.Array(policy.ApprovedIdentityProviderIDs)).Scan(&count); err != nil {
		return err
	}
	if count != len(policy.ApprovedIdentityProviderIDs) {
		return service.ErrWorkspaceConflict
	}
	return nil
}

func validateWorkspaceSecurityOwnerTx(ctx context.Context, tx *sql.Tx, actorID int64, policy service.WorkspaceSecurityPolicy, enrolled bool, now time.Time) error {
	auth, _ := service.SessionAuthenticationFromContext(ctx)
	// Enrollment is authoritative under the actor lock, never a client hint.
	auth.MFAEnrolled = enrolled
	ctx = service.WithSessionAuthentication(ctx, auth)
	if policy.RequireMFA && (!enrolled || !auth.MFASatisfied) {
		return service.ErrMFARequired
	}
	if err := service.RequireRecentAuthentication(ctx, now, 10*time.Minute); err != nil {
		return err
	}
	if policy.SessionMaxAgeSeconds != nil && (auth.AuthenticatedAt.IsZero() || now.Sub(auth.AuthenticatedAt) >= time.Duration(*policy.SessionMaxAgeSeconds)*time.Second || auth.AuthenticatedAt.After(now.Add(time.Minute))) {
		return service.ErrWorkspaceReauthRequired
	}
	if policy.RequireSSO {
		assurance, ok := service.AuthenticationAssuranceFromContext(ctx)
		if !ok || assurance.WorkspaceID != policy.WorkspaceID || !assurance.Valid(now) || (assurance.AuthMethod != "oidc" && assurance.AuthMethod != "saml") {
			return service.ErrSSORequired
		}
		if !service.WorkspaceProviderApproved(policy, assurance.ProviderID) {
			return service.ErrIdentityProviderNotApproved
		}
		var ready bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_domains WHERE workspace_id=$1 AND status='verified') AND EXISTS(SELECT 1 FROM workspace_user_identities i JOIN workspace_identity_providers p ON p.workspace_id=i.workspace_id AND p.id=i.provider_id AND p.status='active' WHERE i.workspace_id=$1 AND i.user_id=$2 AND i.provider_id=$3 AND p.revision=$4 AND p.type=$5)`, policy.WorkspaceID, actorID, assurance.ProviderID, assurance.ProviderRevision, assurance.AuthMethod).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			return service.ErrWorkspaceConflict
		}
	}
	return nil
}

func validateWorkspaceSecurityGracePatch(current service.WorkspaceSecurityPolicy, patch service.WorkspaceSecurityPolicyPatch, now time.Time) error {
	until := patch.SSOGraceUntil.Value
	if !patch.SSOGraceUntil.Present || until == nil || current.SSOGraceUntil != nil && current.SSOGraceUntil.Equal(*until) {
		return nil
	}
	if !until.After(now) || until.After(now.Add(7*24*time.Hour)) {
		return service.ErrWorkspaceInvalid
	}
	return nil
}

func writeWorkspaceSecurityPolicyTx(ctx context.Context, tx *sql.Tx, actorID int64, policy service.WorkspaceSecurityPolicy) (*service.WorkspaceSecurityPolicy, error) {
	_, err := tx.ExecContext(ctx, `UPDATE workspace_security_policies SET require_sso=$2,sso_grace_until=$3,require_mfa=$4,session_max_age_seconds=$5,invitation_policy=$6,allow_external_members=$7,workspace_jit_enabled=$8,approved_identity_provider_mode=$9,revision=revision+1,updated_by_user_id=$10,updated_at=now() WHERE workspace_id=$1`, policy.WorkspaceID, policy.RequireSSO, policy.SSOGraceUntil, policy.RequireMFA, policy.SessionMaxAgeSeconds, policy.InvitationPolicy, policy.AllowExternalMembers, policy.WorkspaceJITEnabled, policy.ApprovedIdentityProviderMode, actorID)
	if err != nil {
		return nil, enterpriseIdentityError(err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM workspace_security_approved_providers WHERE workspace_id=$1`, policy.WorkspaceID); err != nil {
		return nil, err
	}
	for _, providerID := range policy.ApprovedIdentityProviderIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_security_approved_providers(workspace_id,provider_id) VALUES($1,$2)`, policy.WorkspaceID, providerID); err != nil {
			return nil, enterpriseIdentityError(err)
		}
	}
	return loadWorkspaceSecurityPolicyTx(ctx, tx, policy.WorkspaceID)
}

func workspaceSecurityChangedFields(old, next service.WorkspaceSecurityPolicy) string {
	changes := []string{}
	for _, field := range []struct {
		name              string
		previous, current any
	}{{"require_sso", old.RequireSSO, next.RequireSSO}, {"sso_grace_until", old.SSOGraceUntil, next.SSOGraceUntil}, {"require_mfa", old.RequireMFA, next.RequireMFA}, {"session_max_age_seconds", old.SessionMaxAgeSeconds, next.SessionMaxAgeSeconds}, {"invitation_policy", old.InvitationPolicy, next.InvitationPolicy}, {"allow_external_members", old.AllowExternalMembers, next.AllowExternalMembers}, {"workspace_jit_enabled", old.WorkspaceJITEnabled, next.WorkspaceJITEnabled}, {"approved_identity_provider_mode", old.ApprovedIdentityProviderMode, next.ApprovedIdentityProviderMode}, {"approved_identity_provider_ids", old.ApprovedIdentityProviderIDs, next.ApprovedIdentityProviderIDs}} {
		if !reflect.DeepEqual(field.previous, field.current) {
			changes = append(changes, field.name)
		}
	}
	return strings.Join(changes, ",")
}

func appendWorkspaceSecurityMutationTx(ctx context.Context, tx *sql.Tx, actorID int64, old, next service.WorkspaceSecurityPolicy) error {
	data := service.DomainEventData{"previous_revision": old.Revision, "policy_revision": next.Revision, "changed_fields": workspaceSecurityChangedFields(old, next), "require_sso": next.RequireSSO, "require_mfa": next.RequireMFA, "invitation_policy": next.InvitationPolicy, "allow_external_members": next.AllowExternalMembers, "workspace_jit_enabled": next.WorkspaceJITEnabled, "approved_identity_provider_mode": next.ApprovedIdentityProviderMode}
	if next.SessionMaxAgeSeconds != nil {
		data["session_max_age_seconds"] = *next.SessionMaxAgeSeconds
	} else {
		data["session_max_age_seconds"] = nil
	}
	metadata := map[string]any{}
	for k, v := range data {
		metadata[k] = v
	}
	metadata["previous_require_sso"] = old.RequireSSO
	metadata["previous_require_mfa"] = old.RequireMFA
	metadata["previous_invitation_policy"] = old.InvitationPolicy
	metadata["previous_allow_external_members"] = old.AllowExternalMembers
	metadata["previous_workspace_jit_enabled"] = old.WorkspaceJITEnabled
	metadata["previous_approved_identity_provider_mode"] = old.ApprovedIdentityProviderMode
	metadata["previous_session_max_age_seconds"] = old.SessionMaxAgeSeconds
	metadata["previous_approved_provider_count"] = len(old.ApprovedIdentityProviderIDs)
	metadata["approved_provider_count"] = len(next.ApprovedIdentityProviderIDs)
	if err := appendWorkspaceAudit(ctx, tx, next.WorkspaceID, actorID, nil, "workspace.security_policy.updated", "workspace", next.WorkspaceID, metadata); err != nil {
		return err
	}
	event, err := service.NewDomainEvent(service.EventWorkspaceSecurityPolicyUpdated, next.WorkspaceID, 0, actorID, "workspace", strconv.FormatInt(next.WorkspaceID, 10), data)
	if err != nil {
		return err
	}
	if err = insertDomainEventTx(ctx, tx, event, ""); err != nil {
		return err
	}
	if old.RequireSSO != next.RequireSSO {
		kind := service.EventSSOEnforcementDisabled
		if next.RequireSSO {
			kind = service.EventSSOEnforcementEnabled
		}
		event, err = service.NewDomainEvent(kind, next.WorkspaceID, 0, actorID, "workspace", strconv.FormatInt(next.WorkspaceID, 10), service.DomainEventData{"policy_revision": next.Revision, "require_sso": next.RequireSSO})
		if err != nil {
			return err
		}
		return insertDomainEventTx(ctx, tx, event, "")
	}
	return nil
}

func (r *enterpriseIdentityRepository) PatchSecurityPolicy(ctx context.Context, workspaceID, actorID int64, patch service.WorkspaceSecurityPolicyPatch) (*service.WorkspaceSecurityPolicy, error) {
	if patch.ExpectedRevision <= 0 {
		return nil, service.ErrWorkspaceInvalid
	}
	tx, _, enrolled, err := r.beginSecurityMutation(ctx, workspaceID, actorID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	return r.patchSecurityPolicyTx(ctx, tx, workspaceID, actorID, patch, enrolled)
}

func (r *enterpriseIdentityRepository) patchSecurityPolicyTx(ctx context.Context, tx *sql.Tx, workspaceID, actorID int64, patch service.WorkspaceSecurityPolicyPatch, enrolled bool) (*service.WorkspaceSecurityPolicy, error) {
	current, err := loadWorkspaceSecurityPolicyTx(ctx, tx, workspaceID)
	if err != nil {
		return nil, err
	}
	if patch.ExpectedRevision != current.Revision {
		return nil, service.ErrWorkspaceSecurityPolicyConflict
	}
	if err = evaluateWorkspaceSecurityTx(ctx, tx, workspaceID, actorID); err != nil {
		return nil, err
	}
	next, err := service.ApplyWorkspaceSecurityPolicyPatch(*current, patch)
	if err != nil {
		return nil, err
	}
	if err = validateWorkspaceSecurityGracePatch(*current, patch, time.Now()); err != nil {
		return nil, err
	}
	if err = validateSecurityProviderIDsTx(ctx, tx, next); err != nil {
		return nil, err
	}
	if err = validateWorkspaceSecurityOwnerTx(ctx, tx, actorID, next, enrolled, time.Now()); err != nil {
		return nil, err
	}
	item, err := writeWorkspaceSecurityPolicyTx(ctx, tx, actorID, next)
	if err != nil {
		return nil, err
	}
	if err = appendWorkspaceSecurityMutationTx(ctx, tx, actorID, *current, *item); err != nil {
		return nil, err
	}
	if err = describeWorkspaceSecurityPolicyTx(ctx, tx, item); err != nil {
		return nil, err
	}
	return item, enterpriseIdentityError(tx.Commit())
}

func (r *enterpriseIdentityRepository) PreviewSecurityPolicy(ctx context.Context, workspaceID, actorID int64, patch service.WorkspaceSecurityPolicyPatch) (*service.WorkspaceSecurityPolicy, error) {
	tx, _, err := r.beginScoped(ctx, workspaceID, actorID, "workspace_security.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	current, err := loadWorkspaceSecurityPolicyTx(ctx, tx, workspaceID)
	if err != nil {
		return nil, err
	}
	if patch.ExpectedRevision != current.Revision {
		return nil, service.ErrWorkspaceSecurityPolicyConflict
	}
	next, err := service.ApplyWorkspaceSecurityPolicyPatch(*current, patch)
	if err != nil {
		return nil, err
	}
	if err = validateWorkspaceSecurityGracePatch(*current, patch, time.Now()); err != nil {
		return nil, err
	}
	if err = validateSecurityProviderIDsTx(ctx, tx, next); err != nil {
		return nil, err
	}
	if err = describeWorkspaceSecurityPolicyTx(ctx, tx, &next); err != nil {
		return nil, err
	}
	var enrolled bool
	if err = tx.QueryRowContext(ctx, `SELECT totp_enabled AND totp_secret_encrypted IS NOT NULL AND totp_secret_encrypted<>'' FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, actorID).Scan(&enrolled); err != nil {
		return nil, err
	}
	if err = validateWorkspaceSecurityOwnerTx(ctx, tx, actorID, next, enrolled, time.Now()); err != nil {
		var application *infraerrors.ApplicationError
		if !errors.As(err, &application) {
			return nil, service.ErrSecurityPolicyUnavailable.WithCause(err)
		}
		next.PrerequisiteReason = application.Reason
	}
	return &next, enterpriseIdentityError(tx.Commit())
}

func evaluateWorkspaceSecurityTx(ctx context.Context, tx *sql.Tx, workspaceID, actorID int64) error {
	policy, err := loadWorkspaceSecurityPolicyTx(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	auth, _ := service.SessionAuthenticationFromContext(ctx)
	assurance, _ := service.AuthenticationAssuranceFromContext(ctx)
	input := service.WorkspaceSecurityContext{WorkspaceType: service.WorkspaceTypeOrganization, Principal: service.PrincipalHuman, Session: auth, Assurance: assurance}
	if err = tx.QueryRowContext(ctx, `SELECT totp_enabled FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, actorID).Scan(&input.MFAEnrolled); err != nil {
		return service.ErrWorkspaceForbidden
	}
	if assurance.WorkspaceID == workspaceID && assurance.ProviderID > 0 {
		p, _, providerErr := getEnterpriseProvider(ctx, tx, workspaceID, assurance.ProviderID, false)
		if providerErr != nil && !errors.Is(providerErr, service.ErrWorkspaceNotFound) {
			return service.ErrSecurityPolicyUnavailable.WithCause(providerErr)
		}
		if p != nil && providerErr == nil {
			input.ProviderActive = p.Status == "active"
			input.ProviderApproved = service.WorkspaceProviderApproved(*policy, p.ID)
			input.ProviderRevision = p.Revision
			input.ProviderType = p.Type
		}
	}
	decision := service.EvaluateWorkspaceSecurity(*policy, input, time.Now())
	return service.WorkspaceSecurityDecisionError(*policy, decision)
}
