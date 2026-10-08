package service

import (
	"net/mail"
	"strings"
)

// WorkspaceMemberAdmissionSource identifies a server-controlled membership entry.
type WorkspaceMemberAdmissionSource string

const (
	AdmissionInvitationCreate WorkspaceMemberAdmissionSource = "invitation_create"
	AdmissionInvitationAccept WorkspaceMemberAdmissionSource = "invitation_accept"
	AdmissionOIDCJIT          WorkspaceMemberAdmissionSource = "oidc_jit"
	AdmissionSAMLJIT          WorkspaceMemberAdmissionSource = "saml_jit"
	AdmissionSCIMCreate       WorkspaceMemberAdmissionSource = "scim_create"
	AdmissionSCIM             WorkspaceMemberAdmissionSource = "scim"
	AdmissionAdminRestore     WorkspaceMemberAdmissionSource = "admin_restore"
)

type WorkspaceMemberAdmissionRequest struct {
	Source                       WorkspaceMemberAdmissionSource
	UserID, MemberID             int64
	Email                        string
	ProviderID, ProviderRevision int64
}

// WorkspaceMemberAdmissionState is loaded by the repository transaction. It is
// deliberately separate from the request: callers cannot declare retained access.
type WorkspaceMemberAdmissionState struct {
	ExistingActive                                       bool
	AdministrativelySuspended, AdministrativelyRemoved   bool
	VerifiedDomains                                      []string
	ProviderActive, ProviderApproved, ProviderJITEnabled bool
}

type WorkspaceMemberAdmissionPolicy struct{ Policy WorkspaceSecurityPolicy }

func (p WorkspaceMemberAdmissionPolicy) Check(request WorkspaceMemberAdmissionRequest, state WorkspaceMemberAdmissionState) error {
	invitation := request.Source == AdmissionInvitationCreate || request.Source == AdmissionInvitationAccept
	jit := request.Source == AdmissionOIDCJIT || request.Source == AdmissionSAMLJIT
	switch request.Source {
	case AdmissionInvitationCreate, AdmissionInvitationAccept, AdmissionOIDCJIT, AdmissionSAMLJIT, AdmissionSCIMCreate, AdmissionSCIM, AdmissionAdminRestore:
	default:
		return ErrWorkspaceInvalid
	}
	if invitation && p.Policy.InvitationPolicy == InvitationPolicyDisabled {
		return ErrInvitationsDisabled
	}
	administrativelyBlocked := state.AdministrativelySuspended || state.AdministrativelyRemoved
	if jit && administrativelyBlocked {
		return ErrWorkspaceForbidden
	}
	if request.Source == AdmissionSCIM && administrativelyBlocked {
		// Provisioning attribution may change while effective access stays blocked.
		return nil
	}
	if state.ExistingActive && !administrativelyBlocked && !invitation && request.Source != AdmissionSCIMCreate {
		return nil
	}
	if !p.Policy.AllowExternalMembers || invitation && p.Policy.InvitationPolicy == InvitationPolicyVerifiedDomains {
		internal := false
		email := strings.ToLower(strings.TrimSpace(request.Email))
		parsed, err := mail.ParseAddress(email)
		if err == nil && parsed.Address == email {
			at := strings.LastIndex(email, "@")
			if at > 0 {
				domain, err := NormalizeEnterpriseDomain(email[at+1:])
				if err == nil {
					for _, verified := range state.VerifiedDomains {
						canonical, err := NormalizeEnterpriseDomain(verified)
						if err == nil && canonical == domain {
							internal = true
							break
						}
					}
				}
			}
		}
		if !internal {
			if !p.Policy.AllowExternalMembers {
				return ErrExternalMemberNotAllowed
			}
			return ErrMemberDomainNotAllowed
		}
	}
	if jit {
		if !p.Policy.WorkspaceJITEnabled {
			return ErrWorkspaceJITDisabled
		}
		if !state.ProviderActive {
			return ErrOIDCProviderDisabled
		}
		if !state.ProviderApproved {
			return ErrIdentityProviderNotApproved
		}
		if !state.ProviderJITEnabled {
			return ErrOIDCAccountLinkRequired
		}
	}
	return nil
}
