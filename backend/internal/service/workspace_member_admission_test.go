package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkspaceMemberAdmissionPolicy(t *testing.T) {
	defaults := DefaultWorkspaceSecurityPolicy(7)
	for _, test := range []struct {
		name   string
		source WorkspaceMemberAdmissionSource
		email  string
		change func(*WorkspaceSecurityPolicy, *WorkspaceMemberAdmissionState)
		want   error
	}{
		{name: "default invitation accepts external", source: AdmissionInvitationCreate, email: "person@outside.example"},
		{name: "disabled invitation creation", source: AdmissionInvitationCreate, email: "person@company.example", change: func(p *WorkspaceSecurityPolicy, _ *WorkspaceMemberAdmissionState) {
			p.InvitationPolicy = InvitationPolicyDisabled
		}, want: ErrInvitationsDisabled},
		{name: "disabled invitation acceptance is never retained", source: AdmissionInvitationAccept, email: "person@company.example", change: func(p *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) {
			p.InvitationPolicy = InvitationPolicyDisabled
			s.ExistingActive = true
		}, want: ErrInvitationsDisabled},
		{name: "verified invitation rejects suffix domain", source: AdmissionInvitationCreate, email: "person@sub.company.example", change: func(p *WorkspaceSecurityPolicy, _ *WorkspaceMemberAdmissionState) {
			p.InvitationPolicy = InvitationPolicyVerifiedDomains
		}, want: ErrMemberDomainNotAllowed},
		{name: "verified invitation permits exact domain", source: AdmissionInvitationAccept, email: "person@COMPANY.EXAMPLE", change: func(p *WorkspaceSecurityPolicy, _ *WorkspaceMemberAdmissionState) {
			p.InvitationPolicy = InvitationPolicyVerifiedDomains
		}},
		{name: "external false constrains any invitation", source: AdmissionInvitationCreate, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, _ *WorkspaceMemberAdmissionState) { p.AllowExternalMembers = false }, want: ErrExternalMemberNotAllowed},
		{name: "workspace JIT and provider JIT compose", source: AdmissionOIDCJIT, email: "person@company.example", change: func(p *WorkspaceSecurityPolicy, _ *WorkspaceMemberAdmissionState) { p.WorkspaceJITEnabled = false }, want: ErrWorkspaceJITDisabled},
		{name: "provider JIT remains required", source: AdmissionSAMLJIT, email: "person@company.example", change: func(_ *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) { s.ProviderJITEnabled = false }, want: ErrOIDCAccountLinkRequired},
		{name: "approved provider constrains new JIT", source: AdmissionOIDCJIT, email: "person@company.example", change: func(_ *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) { s.ProviderApproved = false }, want: ErrIdentityProviderNotApproved},
		{name: "inactive member is not retained", source: AdmissionSAMLJIT, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, _ *WorkspaceMemberAdmissionState) { p.AllowExternalMembers = false }, want: ErrExternalMemberNotAllowed},
		{name: "existing active JIT source is retained", source: AdmissionOIDCJIT, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) {
			p.AllowExternalMembers = false
			p.WorkspaceJITEnabled = false
			s.ExistingActive = true
			s.ProviderJITEnabled = false
			s.ProviderApproved = false
		}},
		{name: "existing active SCIM metadata is retained", source: AdmissionSCIM, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) {
			p.AllowExternalMembers = false
			s.ExistingActive = true
		}},
		{name: "new SCIM resource is checked despite active member", source: AdmissionSCIMCreate, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) {
			p.AllowExternalMembers = false
			s.ExistingActive = true
		}, want: ErrExternalMemberNotAllowed},
		{name: "admin restoration requires current domain", source: AdmissionAdminRestore, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) {
			p.AllowExternalMembers = false
			s.AdministrativelySuspended = true
			s.AdministrativelyRemoved = true
		}, want: ErrExternalMemberNotAllowed},
		{name: "JIT cannot clear administrative block", source: AdmissionOIDCJIT, email: "person@company.example", change: func(_ *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) { s.AdministrativelySuspended = true }, want: ErrWorkspaceForbidden},
		{name: "SCIM source-only sync preserves administrative block", source: AdmissionSCIM, email: "person@outside.example", change: func(p *WorkspaceSecurityPolicy, s *WorkspaceMemberAdmissionState) {
			p.AllowExternalMembers = false
			s.AdministrativelySuspended = true
			s.AdministrativelyRemoved = true
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			policy := defaults
			state := WorkspaceMemberAdmissionState{VerifiedDomains: []string{"company.example"}, ProviderActive: true, ProviderApproved: true, ProviderJITEnabled: true}
			if test.change != nil {
				test.change(&policy, &state)
			}
			err := (WorkspaceMemberAdmissionPolicy{Policy: policy}).Check(WorkspaceMemberAdmissionRequest{Source: test.source, Email: test.email}, state)
			if test.want == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, test.want)
			}
		})
	}
}

func TestWorkspaceMemberAdmissionExternalPolicyIsUniform(t *testing.T) {
	policy := DefaultWorkspaceSecurityPolicy(7)
	policy.AllowExternalMembers = false
	for _, source := range []WorkspaceMemberAdmissionSource{AdmissionInvitationCreate, AdmissionInvitationAccept, AdmissionOIDCJIT, AdmissionSAMLJIT, AdmissionSCIMCreate, AdmissionSCIM, AdmissionAdminRestore} {
		t.Run(string(source), func(t *testing.T) {
			state := WorkspaceMemberAdmissionState{VerifiedDomains: []string{"company.example"}, ProviderActive: true, ProviderApproved: true, ProviderJITEnabled: true}
			require.ErrorIs(t, (WorkspaceMemberAdmissionPolicy{Policy: policy}).Check(WorkspaceMemberAdmissionRequest{Source: source, Email: "person@outside.example"}, state), ErrExternalMemberNotAllowed)
			require.NoError(t, (WorkspaceMemberAdmissionPolicy{Policy: policy}).Check(WorkspaceMemberAdmissionRequest{Source: source, Email: "person@company.example"}, state))
		})
	}
}

func TestWorkspaceMemberAdmissionCanonicalDomainsAndFailClosed(t *testing.T) {
	policy := DefaultWorkspaceSecurityPolicy(7)
	policy.AllowExternalMembers = false
	gate := WorkspaceMemberAdmissionPolicy{Policy: policy}
	request := WorkspaceMemberAdmissionRequest{Source: AdmissionInvitationCreate, Email: "person@BÜCHER.EXAMPLE"}
	require.NoError(t, gate.Check(request, WorkspaceMemberAdmissionState{VerifiedDomains: []string{"xn--bcher-kva.example"}}))
	require.ErrorIs(t, gate.Check(request, WorkspaceMemberAdmissionState{}), ErrExternalMemberNotAllowed)
	require.Error(t, gate.Check(WorkspaceMemberAdmissionRequest{Source: "unknown", Email: "person@company.example"}, WorkspaceMemberAdmissionState{VerifiedDomains: []string{"company.example"}}))
	require.Error(t, gate.Check(WorkspaceMemberAdmissionRequest{Source: AdmissionInvitationCreate, Email: "Name <person@company.example>"}, WorkspaceMemberAdmissionState{VerifiedDomains: []string{"company.example"}}))
}
