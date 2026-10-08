package service

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	LegacyEnterpriseAssuranceMaxAge = 12 * time.Hour
	WorkspaceSessionAgeMinSeconds   = 900
	WorkspaceSessionAgeMaxSeconds   = 30 * 24 * 3600
	InvitationPolicyAny             = "any"
	InvitationPolicyVerifiedDomains = "verified_domains_only"
	InvitationPolicyDisabled        = "disabled"
	ApprovedProvidersAnyActive      = "any_active"
	ApprovedProvidersSelected       = "selected"
)

var (
	ErrWorkspaceMFARequired            = infraerrors.Forbidden("WORKSPACE_MFA_REQUIRED", "an active organization requires this MFA factor")
	ErrMFARequired                     = infraerrors.Forbidden("MFA_REQUIRED", "the current session must complete MFA")
	ErrMFAEnrollmentRequired           = infraerrors.Forbidden("MFA_ENROLLMENT_REQUIRED", "enroll and verify MFA before entering this workspace")
	ErrWorkspaceReauthRequired         = infraerrors.Forbidden("WORKSPACE_REAUTH_REQUIRED", "workspace session authentication is too old; authenticate again")
	ErrIdentityProviderNotApproved     = infraerrors.Forbidden("IDENTITY_PROVIDER_NOT_APPROVED", "identity provider is not approved for this workspace")
	ErrWorkspaceSecurityPolicyConflict = infraerrors.Conflict("WORKSPACE_SECURITY_POLICY_CONFLICT", "workspace security policy changed; reload before saving")
	ErrSecurityPolicyUnavailable       = infraerrors.New(503, "SECURITY_POLICY_UNAVAILABLE", "workspace security policy is temporarily unavailable")
	ErrInvitationsDisabled             = infraerrors.Forbidden("INVITATIONS_DISABLED", "workspace invitations are disabled")
	ErrExternalMemberNotAllowed        = infraerrors.Forbidden("EXTERNAL_MEMBER_NOT_ALLOWED", "workspace does not admit external members")
	ErrMemberDomainNotAllowed          = infraerrors.Forbidden("MEMBER_DOMAIN_NOT_ALLOWED", "member email must belong to a verified workspace domain")
	ErrWorkspaceJITDisabled            = infraerrors.Forbidden("WORKSPACE_JIT_DISABLED", "workspace does not permit new JIT membership")
)

func DefaultWorkspaceSecurityPolicy(workspaceID int64) WorkspaceSecurityPolicy {
	return WorkspaceSecurityPolicy{WorkspaceID: workspaceID, InvitationPolicy: InvitationPolicyAny, AllowExternalMembers: true, WorkspaceJITEnabled: true, ApprovedIdentityProviderMode: ApprovedProvidersAnyActive, ApprovedIdentityProviderIDs: []int64{}, Revision: 1, SessionMaxAgeMinSeconds: WorkspaceSessionAgeMinSeconds, SessionMaxAgeMaxSeconds: WorkspaceSessionAgeMaxSeconds}
}

// NullableSecurityValue distinguishes an omitted PATCH field from explicit null.
type NullableSecurityValue[T any] struct {
	Present bool
	Value   *T
}

func (v *NullableSecurityValue[T]) UnmarshalJSON(data []byte) error {
	v.Present = true
	v.Value = nil
	if string(data) == "null" {
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	v.Value = &value
	return nil
}

type WorkspaceSecurityPolicyPatch struct {
	ExpectedRevision             int64                            `json:"expected_revision" binding:"required,gt=0"`
	RequireSSO                   *bool                            `json:"require_sso"`
	RequireMFA                   *bool                            `json:"require_mfa"`
	SSOGraceUntil                NullableSecurityValue[time.Time] `json:"sso_grace_until"`
	SessionMaxAgeSeconds         NullableSecurityValue[int]       `json:"session_max_age_seconds"`
	InvitationPolicy             *string                          `json:"invitation_policy"`
	AllowExternalMembers         *bool                            `json:"allow_external_members"`
	WorkspaceJITEnabled          *bool                            `json:"workspace_jit_enabled"`
	ApprovedIdentityProviderMode *string                          `json:"approved_identity_provider_mode"`
	ApprovedIdentityProviderIDs  *[]int64                         `json:"approved_identity_provider_ids"`
}

func ApplyWorkspaceSecurityPolicyPatch(current WorkspaceSecurityPolicy, patch WorkspaceSecurityPolicyPatch) (WorkspaceSecurityPolicy, error) {
	next := current
	next.Decision = nil
	next.ApprovedIdentityProviderIDs = append([]int64{}, current.ApprovedIdentityProviderIDs...)
	if patch.RequireSSO != nil {
		next.RequireSSO = *patch.RequireSSO
	}
	if patch.RequireMFA != nil {
		next.RequireMFA = *patch.RequireMFA
	}
	if patch.SSOGraceUntil.Present {
		next.SSOGraceUntil = patch.SSOGraceUntil.Value
	}
	if patch.SessionMaxAgeSeconds.Present {
		next.SessionMaxAgeSeconds = patch.SessionMaxAgeSeconds.Value
	}
	if patch.InvitationPolicy != nil {
		next.InvitationPolicy = *patch.InvitationPolicy
	}
	if patch.AllowExternalMembers != nil {
		next.AllowExternalMembers = *patch.AllowExternalMembers
	}
	if patch.WorkspaceJITEnabled != nil {
		next.WorkspaceJITEnabled = *patch.WorkspaceJITEnabled
	}
	if patch.ApprovedIdentityProviderMode != nil {
		next.ApprovedIdentityProviderMode = *patch.ApprovedIdentityProviderMode
	}
	if patch.ApprovedIdentityProviderIDs != nil {
		next.ApprovedIdentityProviderIDs = append([]int64{}, (*patch.ApprovedIdentityProviderIDs)...)
	}
	if next.SessionMaxAgeSeconds != nil && (*next.SessionMaxAgeSeconds < WorkspaceSessionAgeMinSeconds || *next.SessionMaxAgeSeconds > WorkspaceSessionAgeMaxSeconds) {
		return next, ErrWorkspaceInvalid
	}
	if next.InvitationPolicy != InvitationPolicyAny && next.InvitationPolicy != InvitationPolicyVerifiedDomains && next.InvitationPolicy != InvitationPolicyDisabled {
		return next, ErrWorkspaceInvalid
	}
	if next.ApprovedIdentityProviderMode != ApprovedProvidersAnyActive && next.ApprovedIdentityProviderMode != ApprovedProvidersSelected {
		return next, ErrWorkspaceInvalid
	}
	if len(next.ApprovedIdentityProviderIDs) > 200 {
		return next, ErrWorkspaceInvalid
	}
	seen := map[int64]bool{}
	for _, id := range next.ApprovedIdentityProviderIDs {
		if id <= 0 || seen[id] {
			return next, ErrWorkspaceInvalid
		}
		seen[id] = true
	}
	slices.Sort(next.ApprovedIdentityProviderIDs)
	return next, nil
}

type WorkspaceSecurityContext struct {
	WorkspaceType    string
	Principal        string
	Session          SessionAuthentication
	Assurance        WorkspaceAssurance
	MFAEnrolled      bool
	ProviderActive   bool
	ProviderApproved bool
	ProviderRevision int64
	ProviderType     string
}

type WorkspaceSecurityDecision struct {
	Allowed                  bool   `json:"allowed"`
	Reason                   string `json:"reason"`
	RequiresSSO              bool   `json:"requires_sso"`
	RequiresMFA              bool   `json:"requires_mfa"`
	RequiresMFAEnrollment    bool   `json:"requires_mfa_enrollment"`
	RequiresReauthentication bool   `json:"requires_reauthentication"`
	ProviderAllowed          bool   `json:"provider_allowed"`
	PolicyRevision           int64  `json:"policy_revision"`
}

// EvaluateWorkspaceSecurity consumes trusted session proof, never enrollment as proof.
// Provider/age/MFA requirements compose monotonically with AND.
func EvaluateWorkspaceSecurity(policy WorkspaceSecurityPolicy, input WorkspaceSecurityContext, now time.Time) WorkspaceSecurityDecision {
	decision := WorkspaceSecurityDecision{Allowed: true, ProviderAllowed: input.ProviderActive && input.ProviderApproved, PolicyRevision: policy.Revision}
	if input.WorkspaceType == WorkspaceTypePersonal || input.Principal == PrincipalAPIKey || input.Principal == PrincipalServiceAccount {
		return decision
	}
	deny := func(reason string) WorkspaceSecurityDecision {
		decision.Allowed = false
		decision.Reason = reason
		return decision
	}
	if input.WorkspaceType != WorkspaceTypeOrganization || (input.Principal != "" && input.Principal != PrincipalHuman) {
		return deny("WORKSPACE_FORBIDDEN")
	}
	if age := policy.SessionMaxAgeSeconds; age != nil && (input.Session.AuthMethod == "" || input.Session.AuthenticatedAt.IsZero() || input.Session.AuthenticatedAt.After(now.Add(time.Minute)) || now.Sub(input.Session.AuthenticatedAt) >= time.Duration(*age)*time.Second) {
		decision.RequiresReauthentication = true
		return deny("WORKSPACE_REAUTH_REQUIRED")
	}
	if policy.RequireSSO && (policy.SSOGraceUntil == nil || !now.Before(*policy.SSOGraceUntil)) {
		decision.RequiresSSO = true
		a := input.Assurance
		if !a.Valid(now) || a.WorkspaceID != policy.WorkspaceID || !validEnterpriseProtocol(a.AuthMethod) || !input.ProviderActive || input.ProviderRevision != a.ProviderRevision || identityProtocol(input.ProviderType) != a.AuthMethod {
			return deny("SSO_REQUIRED")
		}
		if !input.ProviderApproved {
			return deny("IDENTITY_PROVIDER_NOT_APPROVED")
		}
	}
	if policy.RequireMFA {
		decision.RequiresMFA = true
		if !input.Session.MFASatisfied {
			if !input.MFAEnrolled {
				decision.RequiresMFAEnrollment = true
				return deny("MFA_ENROLLMENT_REQUIRED")
			}
			return deny("MFA_REQUIRED")
		}
	}
	return decision
}

func (d WorkspaceSecurityDecision) Error() error {
	if d.Allowed {
		return nil
	}
	switch d.Reason {
	case "SSO_REQUIRED":
		return ErrSSORequired
	case "MFA_REQUIRED":
		return ErrMFARequired
	case "MFA_ENROLLMENT_REQUIRED":
		return ErrMFAEnrollmentRequired
	case "WORKSPACE_REAUTH_REQUIRED":
		return ErrWorkspaceReauthRequired
	case "IDENTITY_PROVIDER_NOT_APPROVED":
		return ErrIdentityProviderNotApproved
	default:
		return ErrWorkspaceForbidden
	}
}

func WorkspaceProviderApproved(policy WorkspaceSecurityPolicy, providerID int64) bool {
	return policy.ApprovedIdentityProviderMode != ApprovedProvidersSelected || slices.Contains(policy.ApprovedIdentityProviderIDs, providerID)
}

// Invitation denials carry only the safe target tenant ID, never its token.
func WorkspaceSecurityErrorForWorkspace(err error, workspaceID int64) error {
	if err == nil {
		return nil
	}
	application := infraerrors.FromError(err)
	metadata := map[string]string{}
	for key, value := range application.Metadata {
		metadata[key] = value
	}
	metadata["workspace_id"] = strconv.FormatInt(workspaceID, 10)
	return application.WithMetadata(metadata)
}

// Safe denial metadata supports recovery without exempting any tenant route.
func WorkspaceSecurityDecisionError(policy WorkspaceSecurityPolicy, decision WorkspaceSecurityDecision) error {
	err := decision.Error()
	if err == nil {
		return nil
	}
	return infraerrors.FromError(err).WithMetadata(map[string]string{
		"workspace_id":    strconv.FormatInt(policy.WorkspaceID, 10),
		"requires_sso":    strconv.FormatBool(policy.RequireSSO),
		"requires_mfa":    strconv.FormatBool(policy.RequireMFA),
		"policy_revision": strconv.FormatInt(policy.Revision, 10),
	})
}

type WorkspaceSecurityPolicyRepository interface {
	PatchSecurityPolicy(context.Context, int64, int64, WorkspaceSecurityPolicyPatch) (*WorkspaceSecurityPolicy, error)
	PreviewSecurityPolicy(context.Context, int64, int64, WorkspaceSecurityPolicyPatch) (*WorkspaceSecurityPolicy, error)
}
