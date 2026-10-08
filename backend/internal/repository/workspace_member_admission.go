package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Caller holds the Workspace row lock. Reads and the eventual membership/source
// write use the same transaction; an unavailable policy never becomes an allow.
func checkWorkspaceMemberAdmissionTx(ctx context.Context, tx *sql.Tx, workspaceID int64, request service.WorkspaceMemberAdmissionRequest) error {
	policy, err := loadWorkspaceSecurityPolicyTx(ctx, tx, workspaceID)
	if err != nil {
		return err
	}
	state := service.WorkspaceMemberAdmissionState{}
	if request.MemberID > 0 || request.UserID > 0 {
		var memberStatus, userStatus string
		var deleted sql.NullTime
		var email string
		err = tx.QueryRowContext(ctx, `SELECT m.status,m.administratively_suspended,m.administratively_removed,u.status,u.deleted_at,u.email FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND ($2::bigint=0 OR m.id=$2) AND ($3::bigint=0 OR m.user_id=$3)`, workspaceID, request.MemberID, request.UserID).Scan(&memberStatus, &state.AdministrativelySuspended, &state.AdministrativelyRemoved, &userStatus, &deleted, &email)
		if errors.Is(err, sql.ErrNoRows) && request.MemberID == 0 {
			// A user bound by token/identity can be new to this Workspace.
			err = tx.QueryRowContext(ctx, `SELECT status,deleted_at,email FROM users WHERE id=$1`, request.UserID).Scan(&userStatus, &deleted, &email)
		}
		if err != nil {
			return workspaceError(err)
		}
		if userStatus != "active" || deleted.Valid {
			return service.ErrUserNotActive
		}
		state.ExistingActive = memberStatus == "active" && !state.AdministrativelySuspended && !state.AdministrativelyRemoved
		request.Email = email
	}
	rows, err := tx.QueryContext(ctx, `SELECT normalized_domain FROM workspace_domains WHERE workspace_id=$1 AND status='verified' ORDER BY normalized_domain`, workspaceID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var domain string
		if err = rows.Scan(&domain); err != nil {
			_ = rows.Close()
			return err
		}
		state.VerifiedDomains = append(state.VerifiedDomains, domain)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if request.Source == service.AdmissionOIDCJIT || request.Source == service.AdmissionSAMLJIT {
		provider, _, err := getEnterpriseProvider(ctx, tx, workspaceID, request.ProviderID, false)
		if err != nil {
			return err
		}
		protocol := "oidc"
		if request.Source == service.AdmissionSAMLJIT {
			protocol = "saml"
		}
		if provider.Type != protocol || request.ProviderRevision > 0 && provider.Revision != request.ProviderRevision {
			return service.ErrOIDCStateSessionMismatch
		}
		state.ProviderActive = provider.Status == "active"
		state.ProviderApproved = service.WorkspaceProviderApproved(*policy, provider.ID)
		state.ProviderJITEnabled = provider.JITConfig.Enabled
	}
	return (service.WorkspaceMemberAdmissionPolicy{Policy: *policy}).Check(request, state)
}

func scimAdmissionError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, service.ErrSecurityPolicyUnavailable) {
		return service.NewSCIMError(503, "", "workspace admission policy is unavailable")
	}
	for _, denial := range []error{service.ErrInvitationsDisabled, service.ErrExternalMemberNotAllowed, service.ErrMemberDomainNotAllowed, service.ErrWorkspaceJITDisabled, service.ErrIdentityProviderNotApproved, service.ErrWorkspaceForbidden, service.ErrUserNotActive, service.ErrOIDCProviderDisabled, service.ErrOIDCAccountLinkRequired, service.ErrOIDCStateSessionMismatch} {
		if errors.Is(err, denial) {
			return service.NewSCIMError(409, "invalidValue", "workspace does not admit this identity")
		}
	}
	return err
}
