package service

import (
	"context"
	"errors"
	"time"
)

func (s *EnterpriseIdentityService) workspaceSecurityContext(ctx context.Context, policy WorkspaceSecurityPolicy, workspaceType, principal string, assurance WorkspaceAssurance) (WorkspaceSecurityContext, error) {
	auth, _ := SessionAuthenticationFromContext(ctx)
	input := WorkspaceSecurityContext{WorkspaceType: workspaceType, Principal: principal, Session: auth, Assurance: assurance, MFAEnrolled: auth.MFAEnrolled}
	if assurance.WorkspaceID == policy.WorkspaceID && assurance.ProviderID > 0 {
		provider, _, err := s.repo.GetProvider(ctx, policy.WorkspaceID, 0, assurance.ProviderID)
		if err != nil && !errors.Is(err, ErrWorkspaceNotFound) {
			return input, ErrSecurityPolicyUnavailable.WithCause(err)
		}
		if provider != nil && err == nil {
			input.ProviderActive = provider.Status == "active"
			input.ProviderApproved = WorkspaceProviderApproved(policy, provider.ID)
			input.ProviderRevision = provider.Revision
			input.ProviderType = provider.Type
		}
	}
	return input, nil
}

func (s *EnterpriseIdentityService) describeSecurityPolicy(ctx context.Context, policy *WorkspaceSecurityPolicy) (*WorkspaceSecurityPolicy, error) {
	if policy == nil {
		return nil, ErrSecurityPolicyUnavailable
	}
	assurance, _ := AuthenticationAssuranceFromContext(ctx)
	input, err := s.workspaceSecurityContext(ctx, *policy, WorkspaceTypeOrganization, PrincipalHuman, assurance)
	if err != nil {
		return nil, err
	}
	decision := EvaluateWorkspaceSecurity(*policy, input, s.now())
	policy.Decision = &decision
	policy.SessionMaxAgeMinSeconds = WorkspaceSessionAgeMinSeconds
	policy.SessionMaxAgeMaxSeconds = WorkspaceSessionAgeMaxSeconds
	return policy, nil
}

func (s *EnterpriseIdentityService) PatchSecurityPolicy(ctx context.Context, actorID, workspaceID int64, patch WorkspaceSecurityPolicyPatch) (*WorkspaceSecurityPolicy, error) {
	if err := s.require(ctx, actorID, workspaceID, "workspace_security.update"); err != nil {
		return nil, err
	}
	if err := RequireRecentAuthentication(ctx, s.now(), 10*time.Minute); err != nil {
		return nil, err
	}
	if patch.ExpectedRevision <= 0 {
		return nil, ErrWorkspaceInvalid
	}
	repo, ok := s.repo.(WorkspaceSecurityPolicyRepository)
	if !ok {
		return nil, ErrSecurityPolicyUnavailable
	}
	policy, err := repo.PatchSecurityPolicy(ctx, workspaceID, actorID, patch)
	if err != nil {
		return nil, err
	}
	return s.describeSecurityPolicy(ctx, policy)
}

func (s *EnterpriseIdentityService) PreviewSecurityPolicy(ctx context.Context, actorID, workspaceID int64, patch WorkspaceSecurityPolicyPatch) (*WorkspaceSecurityPolicy, error) {
	if err := s.require(ctx, actorID, workspaceID, "workspace_security.read"); err != nil {
		return nil, err
	}
	if patch.ExpectedRevision <= 0 {
		return nil, ErrWorkspaceInvalid
	}
	repo, ok := s.repo.(WorkspaceSecurityPolicyRepository)
	if !ok {
		return nil, ErrSecurityPolicyUnavailable
	}
	policy, err := repo.PreviewSecurityPolicy(ctx, workspaceID, actorID, patch)
	if err != nil {
		return nil, err
	}
	return s.describeSecurityPolicy(ctx, policy)
}
