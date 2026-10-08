package service

import (
	"context"
	"strings"
	"time"
)

type EnterpriseSSOStartResult struct {
	AuthorizationURL string
	BrowserCookie    string
	ExpiresAt        time.Time
	Protocol         string
}

func (s *EnterpriseIdentityService) StartEnterpriseSSO(ctx context.Context, workspaceID, providerID int64, returnTo, redirectURI string, linkUserID *int64) (*EnterpriseSSOStartResult, error) {
	if ValidateEnterpriseReturnTo(returnTo) != nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if linkUserID != nil {
		if *linkUserID <= 0 {
			return nil, ErrOIDCAccountLinkRequired
		}
		if _, err := s.access.RequireWorkspace(ctx, *linkUserID, workspaceID, "workspace.read"); err != nil {
			return nil, err
		}
	}
	provider, secret, err := s.repo.GetProvider(ctx, workspaceID, 0, providerID)
	if err != nil {
		return nil, err
	}
	if provider.Status != "active" {
		return nil, ErrOIDCProviderDisabled
	}
	if identityProtocol(provider.Type) != "saml" {
		result, err := s.startOIDC(ctx, workspaceID, providerID, returnTo, redirectURI, linkUserID)
		if err != nil {
			return nil, err
		}
		return &EnterpriseSSOStartResult{AuthorizationURL: result.AuthorizationURL, BrowserCookie: result.BrowserCookie, ExpiresAt: result.ExpiresAt, Protocol: "oidc"}, nil
	}
	if !strings.HasPrefix(redirectURI, "https://") {
		return nil, ErrSAMLProviderInvalid
	}
	sp, _, err := s.samlServiceProvider(provider, secret, redirectURI, 0)
	if err != nil {
		return nil, err
	}
	sp.ForceAuthn = linkUserID != nil
	document, err := sp.BuildAuthRequestDocumentNoSig()
	if err != nil {
		return nil, ErrSAMLProviderInvalid
	}
	intent := "login"
	if linkUserID != nil {
		intent = "link"
	}
	state, err := NewOIDCState(s.now(), 10*time.Minute, workspaceID, providerID, returnTo, intent)
	if err != nil {
		return nil, err
	}
	state.Protocol, state.RequestID, state.ProviderRevision, state.LinkUserID = "saml", document.Root().SelectAttrValue("ID", ""), provider.Revision, linkUserID
	authorization, err := sp.BuildAuthURLRedirect(state.OpaqueState(), document)
	if err != nil {
		return nil, ErrSAMLProviderInvalid
	}
	if err = s.repo.CreateOIDCState(ctx, state); err != nil {
		return nil, err
	}
	return &EnterpriseSSOStartResult{AuthorizationURL: authorization, BrowserCookie: state.OpaqueBrowserSession(), ExpiresAt: state.ExpiresAt, Protocol: "saml"}, nil
}

func (s *EnterpriseIdentityService) CompleteSAML(ctx context.Context, relayState, browserCookie, encodedResponse, redirectURI string) (*EnterpriseSSOLoginResult, error) {
	if len(relayState) != 43 || len(browserCookie) != 43 || len(encodedResponse) == 0 {
		return nil, ErrSAMLResponseInvalid
	}
	state, err := s.repo.ConsumeOIDCState(ctx, EnterpriseTokenHashHex(relayState), HashEnterpriseToken(browserCookie), s.now())
	if err != nil {
		return nil, err
	}
	if state.Protocol != "saml" || state.RequestID == "" || !EnterpriseTokenMatchesHash(browserCookie, state.BrowserSessionHash) {
		return nil, ErrOIDCStateSessionMismatch
	}
	provider, secret, err := s.repo.GetProvider(ctx, state.WorkspaceID, 0, state.ProviderID)
	if err != nil {
		return nil, err
	}
	if provider.Type != "saml" || provider.Status != "active" || provider.Revision != state.ProviderRevision {
		return nil, ErrOIDCStateSessionMismatch
	}
	verified, err := s.validateSAMLResponse(provider, secret, encodedResponse, state.RequestID, redirectURI)
	if err != nil {
		return nil, err
	}
	repo, ok := s.repo.(SAMLReplayRepository)
	if !ok {
		return nil, ErrSAMLProviderInvalid
	}
	if err = repo.UseSAMLResponse(ctx, state.WorkspaceID, state.ProviderID, verified.ResponseID, verified.AssertionID, verified.ExpiresAt); err != nil {
		return nil, err
	}
	if state.Intent == "link" && (state.LinkUserID == nil || *state.LinkUserID <= 0) {
		return nil, ErrOIDCAccountLinkRequired
	}
	password, _, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	hash, err := HashPasswordForEnterprise(password)
	if err != nil {
		return nil, err
	}
	userID, err := s.repo.CompleteIdentityLogin(ctx, EnterpriseProvisionInput{WorkspaceID: state.WorkspaceID, ProviderID: state.ProviderID, ProviderRevision: state.ProviderRevision, Protocol: "saml", Claims: verified.Claims, LinkUserID: state.LinkUserID, PasswordHash: hash})
	if err != nil {
		return nil, err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil || user == nil || !user.IsActive() {
		return nil, ErrUserNotActive
	}
	return &EnterpriseSSOLoginResult{User: user, Workspace: state.WorkspaceID, ProviderID: state.ProviderID, Claims: verified.Claims, ReturnTo: state.ReturnTo, Assurance: WorkspaceAssurance{WorkspaceID: state.WorkspaceID, ProviderID: state.ProviderID, ProviderRevision: state.ProviderRevision, AuthenticatedAt: verified.AuthenticatedAt, ValidUntil: verified.ValidUntil, AuthMethod: "saml"}}, nil
}
