package service

import (
	"context"
	"time"
)

// RequireGlobalAdminOperationAuthentication narrows the lifecycle proof to a
// human session. A recent proof on its own cannot turn an API-key/machine
// subject into an operator authorization.
func RequireGlobalAdminOperationAuthentication(ctx context.Context, now time.Time) error {
	auth, ok := SessionAuthenticationFromContext(ctx)
	if !ok || auth.AuthenticatedAt.IsZero() {
		return ErrRecentAuthenticationRequired
	}
	switch auth.AuthMethod {
	case "password", "oidc", "saml", "passkey":
	default:
		return ErrRecentAuthenticationRequired
	}
	return RequireLifecycleStrongAuthentication(ctx, now)
}

// AdminOperateWorkspace performs the only supported global status transitions.
// Authentication is checked before entering the repository transaction; the
// repository reloads the actor's live role and MFA enrollment under FOR SHARE.
func (s *WorkspaceService) AdminOperateWorkspace(ctx context.Context, actorID, workspaceID int64, in AdminOperationInput) (*AdminOperationReceipt, error) {
	action := in.CanonicalAction()
	outcome := "error"
	defer func() { observeAdminOperation(action, outcome) }()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if in.CanonicalAction() != "suspend" && in.CanonicalAction() != "resume" {
		return nil, ErrWorkspaceInvalid
	}
	if err := RequireGlobalAdminOperationAuthentication(ctx, time.Now().UTC()); err != nil {
		return nil, err
	}
	r, ok := s.repo.(EnterpriseAdminRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	receipt, err := r.AdminOperateWorkspace(ctx, actorID, workspaceID, in)
	if err == nil {
		outcome = "success"
	}
	if err == nil || receipt != nil {
		s.invalidate(ctx, workspaceID)
	}
	return receipt, err
}

// AdminRetryWebhook retries one dead delivery after the repository has
// revalidated its endpoint, delivery state, attempt fence and per-delivery cap.
func (s *WorkspaceService) AdminRetryWebhook(ctx context.Context, actorID, workspaceID, webhookID, deliveryID int64, in AdminOperationInput) (*AdminOperationReceipt, error) {
	action := in.CanonicalAction()
	outcome := "error"
	defer func() { observeAdminOperation(action, outcome) }()
	if err := in.Validate(); err != nil {
		return nil, err
	}
	if in.CanonicalAction() != "retry_webhook" {
		return nil, ErrWorkspaceInvalid
	}
	if err := RequireGlobalAdminOperationAuthentication(ctx, time.Now().UTC()); err != nil {
		return nil, err
	}
	r, ok := s.repo.(EnterpriseAdminRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	receipt, err := r.AdminRetryWebhook(ctx, actorID, workspaceID, webhookID, deliveryID, in)
	if err == nil {
		outcome = "success"
	}
	if err == nil || receipt != nil {
		s.invalidate(ctx, workspaceID)
	}
	return receipt, err
}
