package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWorkspaceSecurityEnrolledFactorDoesNotAuthenticateSession(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultWorkspaceSecurityPolicy(7)
	policy.RequireMFA = true
	input := WorkspaceSecurityContext{WorkspaceType: WorkspaceTypeOrganization, Principal: PrincipalHuman, Session: SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now}, MFAEnrolled: true}
	decision := EvaluateWorkspaceSecurity(policy, input, now)
	require.False(t, decision.Allowed)
	require.Equal(t, "MFA_REQUIRED", decision.Reason)
	input.Session.MFASatisfied = true
	require.True(t, EvaluateWorkspaceSecurity(policy, input, now).Allowed)
	input.Session.MFASatisfied = false
	input.MFAEnrolled = false
	require.Equal(t, "MFA_ENROLLMENT_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
}

func TestWorkspaceSecurityRequirementsComposeWithAND(t *testing.T) {
	now := time.Now().UTC()
	maxAge := 3600
	policy := DefaultWorkspaceSecurityPolicy(7)
	policy.RequireSSO = true
	policy.RequireMFA = true
	policy.SessionMaxAgeSeconds = &maxAge
	input := WorkspaceSecurityContext{WorkspaceType: WorkspaceTypeOrganization, Principal: PrincipalHuman, MFAEnrolled: true, Session: SessionAuthentication{AuthMethod: "oidc", AuthenticatedAt: now, MFASatisfied: true}, Assurance: WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, ProviderRevision: 3, AuthenticatedAt: now, AuthMethod: "oidc"}, ProviderActive: true, ProviderApproved: true, ProviderRevision: 3, ProviderType: "oidc"}
	require.True(t, EvaluateWorkspaceSecurity(policy, input, now).Allowed)
	input.Session.MFASatisfied = false
	require.Equal(t, "MFA_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
	input.Session.MFASatisfied = true
	input.Assurance.WorkspaceID = 8
	require.Equal(t, "SSO_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
	input.Assurance.WorkspaceID = 7
	input.ProviderApproved = false
	require.Equal(t, "IDENTITY_PROVIDER_NOT_APPROVED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
	input.ProviderApproved = true
	input.ProviderRevision = 4
	require.Equal(t, "SSO_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
	input.ProviderRevision = 3
	input.Session.AuthenticatedAt = now.Add(-time.Hour)
	require.Equal(t, "WORKSPACE_REAUTH_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
}

func TestWorkspaceSecurityAgeIsOriginalAndLegacyAssuranceCeilingRemains(t *testing.T) {
	now := time.Now().UTC()
	policy := DefaultWorkspaceSecurityPolicy(7)
	input := WorkspaceSecurityContext{WorkspaceType: WorkspaceTypeOrganization, Principal: PrincipalHuman, Session: SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-48 * time.Hour)}}
	require.True(t, EvaluateWorkspaceSecurity(policy, input, now).Allowed, "NULL adds no general limit")
	age := 3600
	policy.SessionMaxAgeSeconds = &age
	require.False(t, EvaluateWorkspaceSecurity(policy, input, now).Allowed)
	input.Session.AuthenticatedAt = now.Add(-time.Hour + time.Second)
	require.True(t, EvaluateWorkspaceSecurity(policy, input, now).Allowed)
	input.Session.AuthenticatedAt = time.Time{}
	require.Equal(t, "WORKSPACE_REAUTH_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
	policy.RequireSSO = true
	age = 24 * 3600
	input.Session.AuthenticatedAt = now
	input.Assurance = WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, ProviderRevision: 3, AuthenticatedAt: now.Add(-12 * time.Hour), AuthMethod: "saml"}
	input.ProviderActive = true
	input.ProviderApproved = true
	input.ProviderRevision = 3
	input.ProviderType = "saml"
	require.Equal(t, "SSO_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
	input.Assurance.AuthenticatedAt = now
	input.Assurance.ValidUntil = now
	require.Equal(t, "SSO_REQUIRED", EvaluateWorkspaceSecurity(policy, input, now).Reason)
}

func TestWorkspaceSecurityPersonalAndMachineAreIndependent(t *testing.T) {
	policy := DefaultWorkspaceSecurityPolicy(7)
	policy.RequireMFA = true
	policy.RequireSSO = true
	age := 900
	policy.SessionMaxAgeSeconds = &age
	for _, input := range []WorkspaceSecurityContext{{WorkspaceType: WorkspaceTypePersonal, Principal: PrincipalHuman}, {WorkspaceType: WorkspaceTypeOrganization, Principal: PrincipalAPIKey}, {WorkspaceType: WorkspaceTypeOrganization, Principal: PrincipalServiceAccount}} {
		require.True(t, EvaluateWorkspaceSecurity(policy, input, time.Now()).Allowed)
	}
}

func TestWorkspaceSecurityPatchPreservesOmissionsAndExplicitNull(t *testing.T) {
	policy := DefaultWorkspaceSecurityPolicy(7)
	age := 3600
	policy.SessionMaxAgeSeconds = &age
	until := time.Now().Add(time.Hour)
	policy.SSOGraceUntil = &until
	var patch WorkspaceSecurityPolicyPatch
	require.NoError(t, json.Unmarshal([]byte(`{"expected_revision":1,"require_mfa":true}`), &patch))
	merged, err := ApplyWorkspaceSecurityPolicyPatch(policy, patch)
	require.NoError(t, err)
	require.True(t, merged.RequireMFA)
	require.Equal(t, &age, merged.SessionMaxAgeSeconds)
	require.Equal(t, &until, merged.SSOGraceUntil)
	require.NoError(t, json.Unmarshal([]byte(`{"expected_revision":1,"session_max_age_seconds":null,"sso_grace_until":null}`), &patch))
	merged, err = ApplyWorkspaceSecurityPolicyPatch(policy, patch)
	require.NoError(t, err)
	require.Nil(t, merged.SessionMaxAgeSeconds)
	require.Nil(t, merged.SSOGraceUntil)
	require.True(t, merged.AllowExternalMembers)
	require.True(t, merged.WorkspaceJITEnabled)
	require.Equal(t, "any", merged.InvitationPolicy)
	require.Equal(t, "any_active", merged.ApprovedIdentityProviderMode)
}
