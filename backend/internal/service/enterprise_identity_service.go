package service

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type EnterpriseDomain struct {
	ID                 int64      `json:"id"`
	WorkspaceID        int64      `json:"workspace_id"`
	Domain             string     `json:"domain"`
	NormalizedDomain   string     `json:"normalized_domain"`
	Status             string     `json:"status"`
	VerificationMethod string     `json:"verification_method"`
	DNSHost            string     `json:"dns_host,omitempty"`
	VerifiedAt         *time.Time `json:"verified_at,omitempty"`
	LastCheckedAt      *time.Time `json:"last_checked_at,omitempty"`
	LastErrorCode      string     `json:"last_error_code,omitempty"`
	CreatedByUserID    int64      `json:"created_by_user_id"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type EnterpriseDomainCreateResult struct {
	Domain            *EnterpriseDomain `json:"domain"`
	VerificationToken string            `json:"verification_token"`
	VerificationTXT   string            `json:"verification_txt"`
}

type EnterpriseIdentityProvider struct {
	SAML                  *SAMLProviderConfig `json:"saml,omitempty"`
	PublicID              string              `json:"saml_public_id,omitempty"`
	ID                    int64               `json:"id"`
	Revision              int64               `json:"revision"`
	TokenAuthMethod       string              `json:"token_auth_method"`
	WorkspaceID           int64               `json:"workspace_id"`
	Type                  string              `json:"type"`
	ProviderKey           string              `json:"provider_key"`
	Name                  string              `json:"name"`
	Status                string              `json:"status"`
	IsDefault             bool                `json:"is_default"`
	IssuerURL             string              `json:"issuer_url"`
	ClientID              string              `json:"client_id"`
	HasClientSecret       bool                `json:"has_client_secret"`
	Scopes                []string            `json:"scopes"`
	AuthorizationEndpoint string              `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string              `json:"token_endpoint,omitempty"`
	JWKSURI               string              `json:"jwks_uri,omitempty"`
	UserinfoEndpoint      string              `json:"userinfo_endpoint,omitempty"`
	DiscoveryEnabled      bool                `json:"discovery_enabled"`
	ClaimMapping          map[string]any      `json:"claim_mapping"`
	JITConfig             JITConfig           `json:"jit_config"`
	CreatedByUserID       int64               `json:"created_by_user_id"`
	CreatedAt             time.Time           `json:"created_at"`
	UpdatedAt             time.Time           `json:"updated_at"`
	DisabledAt            *time.Time          `json:"disabled_at,omitempty"`
	LastValidatedAt       *time.Time          `json:"last_validated_at,omitempty"`
	LastValidationCode    string              `json:"last_validation_code,omitempty"`
}

type EnterpriseIdentityProviderInput struct {
	Type                  string              `json:"type,omitempty"`
	SAML                  *SAMLProviderConfig `json:"saml,omitempty"`
	ProviderKey           string              `json:"provider_key"`
	Name                  string              `json:"name"`
	IssuerURL             string              `json:"issuer_url"`
	ClientID              string              `json:"client_id"`
	ClientSecret          *string             `json:"client_secret,omitempty"`
	SecretAction          string              `json:"secret_action,omitempty"`
	TokenAuthMethod       string              `json:"token_auth_method,omitempty"`
	AuthorizationEndpoint string              `json:"authorization_endpoint,omitempty"`
	TokenEndpoint         string              `json:"token_endpoint,omitempty"`
	JWKSURI               string              `json:"jwks_uri,omitempty"`
	UserinfoEndpoint      string              `json:"userinfo_endpoint,omitempty"`
	Scopes                []string            `json:"scopes"`
	IsDefault             bool                `json:"is_default"`
	DiscoveryEnabled      *bool               `json:"discovery_enabled,omitempty"`
	ClaimMapping          map[string]any      `json:"claim_mapping"`
	JITConfig             JITConfig           `json:"jit_config"`
	Revision              int64               `json:"revision,omitempty"`
}

type WorkspaceIdentityPolicy struct {
	WorkspaceID   int64      `json:"workspace_id"`
	RequireSSO    bool       `json:"require_sso"`
	SSOGraceUntil *time.Time `json:"sso_grace_until,omitempty"`
	Revision      int64      `json:"revision"`
	UpdatedBy     *int64     `json:"updated_by_user_id,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type EnterpriseIdentityRepository interface {
	ListDomains(context.Context, int64, pagination.PaginationParams) ([]EnterpriseDomain, int64, error)
	CreateDomain(context.Context, int64, int64, string, string, []byte) (*EnterpriseDomain, error)
	GetDomain(context.Context, int64, int64, int64) (*EnterpriseDomain, []byte, error)
	RegenerateDomain(context.Context, int64, int64, int64, []byte) (*EnterpriseDomain, error)
	MarkDomainChecked(context.Context, int64, int64, int64, []byte, string, bool) (*EnterpriseDomain, error)
	RevokeDomain(context.Context, int64, int64, int64) (*EnterpriseDomain, error)

	ListProviders(context.Context, int64, int64, pagination.PaginationParams) ([]EnterpriseIdentityProvider, int64, error)
	CreateProvider(context.Context, int64, int64, EnterpriseIdentityProviderInput, string, []string) (*EnterpriseIdentityProvider, error)
	GetProvider(context.Context, int64, int64, int64) (*EnterpriseIdentityProvider, string, error)
	UpdateProvider(context.Context, int64, int64, int64, EnterpriseIdentityProviderInput, *string, []string) (*EnterpriseIdentityProvider, error)
	DisableProvider(context.Context, int64, int64, int64) error
	RecordProviderValidation(context.Context, int64, int64, int64, int64, string) error
	ActiveProviderCount(context.Context, int64, int64) (int, error)
	HasVerifiedDomain(context.Context, int64, string) (bool, error)

	GetPolicy(context.Context, int64, int64) (*WorkspaceIdentityPolicy, error)
	UpdatePolicy(context.Context, int64, int64, bool, *time.Time) (*WorkspaceIdentityPolicy, error)

	CreateOIDCState(context.Context, *OIDCState) error
	ConsumeOIDCState(context.Context, string, []byte, time.Time) (*OIDCState, error)
	FindIdentity(context.Context, int64, int64, string) (*OIDCIdentity, error)
	BindIdentity(context.Context, OIDCIdentityBinding) error
	EnsureOIDCMembership(context.Context, int64, int64, int64, string) error
	CompleteIdentityLogin(context.Context, OIDCProvisionInput) (int64, error)
	GetMappings(context.Context, int64, int64) (*OIDCMappings, error)
	ReplaceMappings(context.Context, int64, int64, int64, OIDCMappings) error
	DiscoverSSO(context.Context, string) ([]SSODiscoveryProvider, error)
	HasUserIdentity(context.Context, int64, int64) (bool, error)
	BreakGlass(context.Context, int64, int64, string) (*WorkspaceIdentityPolicy, error)
	CreateLoginCompletion(context.Context, *OIDCLoginResult, []byte, []byte, time.Time) error
	ConsumeLoginCompletion(context.Context, []byte, []byte, time.Time) (*OIDCLoginResult, int64, error)
	PreviewLoginCompletion(context.Context, []byte, []byte, time.Time) (*OIDCLoginResult, int64, error)
}

type OIDCTeamMapping struct {
	ClaimValue string `json:"claim_value"`
	TeamID     int64  `json:"team_id"`
}

type OIDCMappings struct {
	Roles []OIDCRoleMapping `json:"roles"`
	Teams []OIDCTeamMapping `json:"teams"`
}

type SSODiscoveryProvider struct {
	Type         string `json:"type,omitempty"`
	SAMLPublicID string `json:"saml_public_id,omitempty"`
	WorkspaceID  int64  `json:"workspace_id"`
	ProviderID   int64  `json:"provider_id"`
	Name         string `json:"name"`
	IsDefault    bool   `json:"is_default"`
}

// OIDCProvisionInput contains only validated claims and server-bound link intent.
// The repository commits user creation, stable subject binding, membership and
// claim reconciliation together under the workspace lock.
type OIDCProvisionInput struct {
	Protocol                string
	WorkspaceID, ProviderID int64
	ProviderRevision        int64
	Claims                  *OIDCClaims
	LinkUserID              *int64
	PasswordHash            string
}

type OIDCIdentity struct {
	ID            int64
	WorkspaceID   int64
	ProviderID    int64
	UserID        int64
	Subject       string
	EmailAtLink   string
	EmailVerified bool
	LastSeenAt    time.Time
}

type OIDCIdentityBinding struct {
	WorkspaceID   int64
	ProviderID    int64
	UserID        int64
	Subject       string
	EmailAtLink   string
	EmailVerified bool
}

type EnterpriseIdentityService struct {
	repo      EnterpriseIdentityRepository
	access    *WorkspaceAccessService
	users     UserRepository
	encryptor SecretEncryptor
	resolver  enterpriseTXTResolver
	now       func() time.Time
}

type enterpriseTXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

type netTXTResolver struct{ resolver *net.Resolver }

func (r netTXTResolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, host)
}

func NewEnterpriseIdentityService(repo EnterpriseIdentityRepository, access *WorkspaceAccessService, users UserRepository, encryptor SecretEncryptor) *EnterpriseIdentityService {
	return &EnterpriseIdentityService{repo: repo, access: access, users: users, encryptor: encryptor, resolver: netTXTResolver{resolver: net.DefaultResolver}, now: time.Now}
}

func (s *EnterpriseIdentityService) SetTXTResolver(resolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}) {
	if resolver != nil {
		s.resolver = resolver
	}
}

func (s *EnterpriseIdentityService) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

func (s *EnterpriseIdentityService) require(ctx context.Context, actorID, workspaceID int64, permission string) error {
	if s == nil || s.repo == nil || s.access == nil {
		return ErrWorkspaceForbidden
	}
	_, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, permission)
	return err
}

func (s *EnterpriseIdentityService) ListDomains(ctx context.Context, actorID, workspaceID int64, p pagination.PaginationParams) ([]EnterpriseDomain, int64, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.read"); err != nil {
		return nil, 0, err
	}
	return s.repo.ListDomains(ctx, workspaceID, p)
}

func (s *EnterpriseIdentityService) CreateDomain(ctx context.Context, actorID, workspaceID int64, raw string) (*EnterpriseDomainCreateResult, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	canonical, err := NormalizeEnterpriseDomain(raw)
	if err != nil {
		return nil, err
	}
	token, hash, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	domain, err := s.repo.CreateDomain(ctx, workspaceID, actorID, canonical, token, hash)
	if err != nil {
		return nil, err
	}
	domain.DNSHost = "_modurelay-verification." + canonical
	return &EnterpriseDomainCreateResult{Domain: domain, VerificationToken: token, VerificationTXT: "modurelay-verification=" + token}, nil
}

func (s *EnterpriseIdentityService) RegenerateDomainToken(ctx context.Context, actorID, workspaceID, domainID int64) (*EnterpriseDomainCreateResult, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	token, hash, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	domain, err := s.repo.RegenerateDomain(ctx, workspaceID, actorID, domainID, hash)
	if err != nil {
		return nil, err
	}
	domain.DNSHost = "_modurelay-verification." + domain.NormalizedDomain
	return &EnterpriseDomainCreateResult{Domain: domain, VerificationToken: token, VerificationTXT: "modurelay-verification=" + token}, nil
}

func (s *EnterpriseIdentityService) VerifyDomain(ctx context.Context, actorID, workspaceID, domainID int64) (*EnterpriseDomain, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	domain, tokenHash, err := s.repo.GetDomain(ctx, workspaceID, actorID, domainID)
	if err != nil {
		return nil, err
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	records, lookupErr := s.resolver.LookupTXT(lookupCtx, "_modurelay-verification."+domain.NormalizedDomain)
	verified := false
	for _, record := range records {
		value := strings.TrimSpace(record)
		if strings.HasPrefix(value, "modurelay-verification=") {
			candidate := strings.TrimPrefix(value, "modurelay-verification=")
			if EnterpriseTokenMatchesHash(candidate, tokenHash) {
				verified = true
				break
			}
		}
	}
	errorCode := ""
	if lookupErr != nil {
		errorCode = "DNS_LOOKUP_FAILED"
	} else if !verified {
		errorCode = "TXT_MISMATCH"
	}
	return s.repo.MarkDomainChecked(ctx, workspaceID, actorID, domainID, tokenHash, errorCode, verified)
}

func (s *EnterpriseIdentityService) RevokeDomain(ctx context.Context, actorID, workspaceID, domainID int64) (*EnterpriseDomain, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	return s.repo.RevokeDomain(ctx, workspaceID, actorID, domainID)
}

func (s *EnterpriseIdentityService) ListProviders(ctx context.Context, actorID, workspaceID int64, p pagination.PaginationParams) ([]EnterpriseIdentityProvider, int64, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.read"); err != nil {
		return nil, 0, err
	}
	return s.repo.ListProviders(ctx, workspaceID, actorID, p)
}

func (s *EnterpriseIdentityService) GetProvider(ctx context.Context, actorID, workspaceID, providerID int64) (*EnterpriseIdentityProvider, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.read"); err != nil {
		return nil, err
	}
	provider, _, err := s.repo.GetProvider(ctx, workspaceID, actorID, providerID)
	return provider, err
}

func (s *EnterpriseIdentityService) CreateProvider(ctx context.Context, actorID, workspaceID int64, input EnterpriseIdentityProviderInput) (*EnterpriseIdentityProvider, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	input, err := validateIdentityProviderInput(input)
	if err != nil {
		return nil, err
	}
	if input.Type == "saml" {
		prepared, keys, err := s.prepareSAMLProvider(ctx, input, true)
		if err != nil {
			return nil, err
		}
		return s.repo.CreateProvider(ctx, workspaceID, actorID, prepared, keys, nil)
	}
	if _, err := validateEnterpriseProviderConfiguration(ctx, enterpriseProviderFromInput(input)); err != nil {
		return nil, err
	}
	ciphertext := ""
	if input.TokenAuthMethod != "none" {
		if input.ClientSecret == nil || strings.TrimSpace(*input.ClientSecret) == "" || input.SecretAction == "remove" || s.encryptor == nil {
			return nil, ErrEnterpriseIdentityInvalid
		}
		ciphertext, err = s.encryptor.Encrypt(*input.ClientSecret)
		if err != nil {
			return nil, fmt.Errorf("encrypt oidc client secret: %w", err)
		}
	} else if input.ClientSecret != nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	return s.repo.CreateProvider(ctx, workspaceID, actorID, input, ciphertext, defaultOIDCScopes(input.Scopes))
}

func (s *EnterpriseIdentityService) UpdateProvider(ctx context.Context, actorID, workspaceID, providerID int64, input EnterpriseIdentityProviderInput) (*EnterpriseIdentityProvider, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	input, err := validateIdentityProviderInput(input)
	if err != nil {
		return nil, err
	}
	if input.Revision <= 0 {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if input.Type == "saml" {
		prepared, _, err := s.prepareSAMLProvider(ctx, input, false)
		if err != nil {
			return nil, err
		}
		return s.repo.UpdateProvider(ctx, workspaceID, actorID, providerID, prepared, nil, nil)
	}
	if _, err := validateEnterpriseProviderConfiguration(ctx, enterpriseProviderFromInput(input)); err != nil {
		return nil, err
	}
	var ciphertext *string
	if input.SecretAction == "remove" {
		if input.TokenAuthMethod != "none" || input.ClientSecret != nil {
			return nil, ErrEnterpriseIdentityInvalid
		}
		empty := ""
		ciphertext = &empty
	} else if input.SecretAction == "replace" {
		if input.ClientSecret == nil || input.TokenAuthMethod == "none" {
			return nil, ErrEnterpriseIdentityInvalid
		}
		if strings.TrimSpace(*input.ClientSecret) == "" {
			return nil, ErrEnterpriseIdentityInvalid
		}
		if s.encryptor == nil {
			return nil, ErrEnterpriseIdentityInvalid
		}
		value, encryptErr := s.encryptor.Encrypt(*input.ClientSecret)
		if encryptErr != nil {
			return nil, fmt.Errorf("encrypt oidc client secret: %w", encryptErr)
		}
		ciphertext = &value
	} else if input.ClientSecret != nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	return s.repo.UpdateProvider(ctx, workspaceID, actorID, providerID, input, ciphertext, defaultOIDCScopes(input.Scopes))
}

func (s *EnterpriseIdentityService) DisableProvider(ctx context.Context, actorID, workspaceID, providerID int64) error {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return err
	}
	return s.repo.DisableProvider(ctx, workspaceID, actorID, providerID)
}

type OIDCStartResult struct {
	AuthorizationURL string
	BrowserCookie    string
	ExpiresAt        time.Time
}

type OIDCLoginResult struct {
	User       *User
	Workspace  int64
	ProviderID int64
	Claims     *OIDCClaims
	Assurance  WorkspaceAssurance
	ReturnTo   string
}

func (s *EnterpriseIdentityService) StartOIDC(ctx context.Context, workspaceID, providerID int64, returnTo, redirectURI string) (*OIDCStartResult, error) {
	return s.startOIDC(ctx, workspaceID, providerID, returnTo, redirectURI, nil)
}

func (s *EnterpriseIdentityService) StartOIDCLink(ctx context.Context, actorID, workspaceID, providerID int64, returnTo, redirectURI string) (*OIDCStartResult, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "workspace.read"); err != nil {
		return nil, err
	}
	return s.startOIDC(ctx, workspaceID, providerID, returnTo, redirectURI, &actorID)
}

func (s *EnterpriseIdentityService) startOIDC(ctx context.Context, workspaceID, providerID int64, returnTo, redirectURI string, linkUserID *int64) (*OIDCStartResult, error) {
	if ValidateEnterpriseReturnTo(returnTo) != nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	provider, _, err := s.repo.GetProvider(ctx, workspaceID, 0, providerID)
	if err != nil {
		return nil, err
	}
	if identityProtocol(provider.Type) != "oidc" {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if provider.Status != "active" {
		return nil, ErrOIDCProviderDisabled
	}
	document, err := enterpriseProviderDocument(ctx, provider)
	if err != nil {
		return nil, err
	}
	now := s.now()
	intent := "login"
	if linkUserID != nil {
		intent = "link"
	}
	state, err := NewOIDCState(now, 10*time.Minute, workspaceID, providerID, returnTo, intent)
	if err != nil {
		return nil, err
	}
	state.LinkUserID = linkUserID
	state.ProviderRevision = provider.Revision
	if err := s.repo.CreateOIDCState(ctx, state); err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("client_id", provider.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(defaultOIDCScopes(provider.Scopes), " "))
	params.Set("state", state.OpaqueState())
	params.Set("nonce", state.OpaqueNonce())
	params.Set("code_challenge", OIDCPKCEChallenge(state.PKCEVerifier))
	params.Set("code_challenge_method", "S256")
	authorize, err := url.Parse(document.AuthorizationEndpoint)
	if err != nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	query := authorize.Query()
	for key, values := range params {
		query[key] = values
	}
	if linkUserID != nil {
		query.Set("prompt", "login")
	}
	authorize.RawQuery = query.Encode()
	return &OIDCStartResult{AuthorizationURL: authorize.String(), BrowserCookie: state.OpaqueBrowserSession(), ExpiresAt: state.ExpiresAt}, nil
}

func (s *EnterpriseIdentityService) CompleteOIDC(ctx context.Context, stateOpaque, browserCookie, code, redirectURI string) (*OIDCLoginResult, error) {
	if strings.TrimSpace(stateOpaque) == "" || strings.TrimSpace(code) == "" || strings.TrimSpace(browserCookie) == "" {
		return nil, ErrEnterpriseIdentityInvalid
	}
	stateHash := EnterpriseTokenHashHex(stateOpaque)
	state, err := s.repo.ConsumeOIDCState(ctx, stateHash, HashEnterpriseToken(browserCookie), s.now())
	if err != nil {
		return nil, err
	}
	if identityProtocol(state.Protocol) != "oidc" {
		return nil, ErrOIDCStateSessionMismatch
	}
	if !OIDCNonceMatchesHash(browserCookie, state.BrowserSessionHash) {
		return nil, ErrOIDCStateSessionMismatch
	}
	provider, secret, err := s.repo.GetProvider(ctx, state.WorkspaceID, 0, state.ProviderID)
	if err != nil {
		return nil, err
	}
	if identityProtocol(provider.Type) != "oidc" {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if provider.Status != "active" {
		return nil, ErrOIDCProviderDisabled
	}
	if provider.Revision != state.ProviderRevision {
		return nil, ErrOIDCStateSessionMismatch
	}
	if secret != "" {
		if s.encryptor == nil {
			return nil, ErrEnterpriseIdentityInvalid
		}
		secret, err = s.encryptor.Decrypt(secret)
		if err != nil {
			return nil, ErrEnterpriseIdentityInvalid
		}
	}
	document, err := enterpriseProviderDocument(ctx, provider)
	if err != nil {
		return nil, err
	}
	tokens, err := ExchangeOIDCCode(ctx, document, provider.ClientID, secret, code, redirectURI, state.PKCEVerifier, provider.TokenAuthMethod)
	if err != nil {
		return nil, err
	}
	claims, err := ParseAndValidateOIDCIDToken(ctx, tokens.IDToken, document, provider.ClientID, state.Nonce)
	if err != nil {
		return nil, err
	}
	if state.Intent == "link" && (state.LinkUserID == nil || *state.LinkUserID <= 0) {
		return nil, ErrOIDCAccountLinkRequired
	}
	passwordToken, _, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	passwordHash, err := HashPasswordForEnterprise(passwordToken)
	if err != nil {
		return nil, err
	}
	if err := ApplyOIDCClaimMapping(claims, provider.ClaimMapping); err != nil {
		return nil, err
	}
	userID, err := s.repo.CompleteIdentityLogin(ctx, OIDCProvisionInput{WorkspaceID: state.WorkspaceID, ProviderID: state.ProviderID, ProviderRevision: state.ProviderRevision, Claims: claims, LinkUserID: state.LinkUserID, PasswordHash: passwordHash})
	if err != nil {
		return nil, err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil || user == nil || !user.IsActive() {
		return nil, ErrUserNotActive
	}
	return &OIDCLoginResult{User: user, Workspace: state.WorkspaceID, ProviderID: state.ProviderID, Claims: claims, ReturnTo: state.ReturnTo, Assurance: WorkspaceAssurance{WorkspaceID: state.WorkspaceID, ProviderID: state.ProviderID, ProviderRevision: state.ProviderRevision, AuthenticatedAt: s.now(), AuthMethod: "oidc"}}, nil
}

func (s *EnterpriseIdentityService) CreateLoginCompletion(ctx context.Context, result *OIDCLoginResult, browserCookie string) (string, error) {
	token, hash, err := NewSecureToken(32)
	if err != nil {
		return "", err
	}
	if err := s.repo.CreateLoginCompletion(ctx, result, hash, HashEnterpriseToken(browserCookie), s.now().Add(2*time.Minute)); err != nil {
		return "", err
	}
	return token, nil
}

func (s *EnterpriseIdentityService) ExchangeLoginCompletion(ctx context.Context, token, browserCookie string) (*OIDCLoginResult, error) {
	if token == "" || browserCookie == "" {
		return nil, ErrOIDCStateSessionMismatch
	}
	result, userID, err := s.repo.ConsumeLoginCompletion(ctx, HashEnterpriseToken(token), HashEnterpriseToken(browserCookie), s.now())
	if err != nil {
		return nil, err
	}
	result.User, err = s.users.GetByID(ctx, userID)
	if err != nil || result.User == nil || !result.User.IsActive() {
		return nil, ErrUserNotActive
	}
	return result, nil
}

// PreviewLoginCompletion lets the handler check local MFA before consuming
// browser-bound proof. Consumption repeats all live authorization checks.
func (s *EnterpriseIdentityService) PreviewLoginCompletion(ctx context.Context, token, browserCookie string) (*OIDCLoginResult, error) {
	if token == "" || browserCookie == "" {
		return nil, ErrOIDCStateSessionMismatch
	}
	result, userID, err := s.repo.PreviewLoginCompletion(ctx, HashEnterpriseToken(token), HashEnterpriseToken(browserCookie), s.now())
	if err != nil {
		return nil, err
	}
	result.User, err = s.users.GetByID(ctx, userID)
	if err != nil || result.User == nil || !result.User.IsActive() {
		return nil, ErrUserNotActive
	}
	return result, nil
}

func HashPasswordForEnterprise(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", ErrEnterpriseIdentityInvalid
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *EnterpriseIdentityService) GetPolicy(ctx context.Context, actorID, workspaceID int64) (*WorkspaceIdentityPolicy, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.read"); err != nil {
		return nil, err
	}
	return s.repo.GetPolicy(ctx, workspaceID, actorID)
}

// CheckWorkspaceAccess enforces SSO only for the requested organization
// workspace. Assurance from another workspace, a disabled provider, or an
// expired session is never accepted. Personal workspaces and machine
// principals remain outside the browser-session policy.
func (s *EnterpriseIdentityService) CheckWorkspaceAccess(ctx context.Context, workspaceID int64, workspaceType, principal string, assurance WorkspaceAssurance) error {
	if s == nil || s.repo == nil || workspaceID <= 0 {
		return ErrWorkspaceForbidden
	}
	policy, err := s.repo.GetPolicy(ctx, workspaceID, 0)
	if err != nil {
		return err
	}
	base := WorkspaceSecurityPolicy{WorkspaceID: policy.WorkspaceID, RequireSSO: policy.RequireSSO, SSOGraceUntil: policy.SSOGraceUntil, Revision: policy.Revision}
	if !base.RequireSSO || workspaceType == WorkspaceTypePersonal || principal == PrincipalAPIKey || principal == PrincipalServiceAccount {
		return nil
	}
	if policy.SSOGraceUntil != nil && s.now().Before(*policy.SSOGraceUntil) {
		return nil
	}
	if principal == "" {
		principal = PrincipalHuman
	}
	if principal != PrincipalHuman || !assurance.Valid(s.now()) || assurance.WorkspaceID != workspaceID || !validEnterpriseProtocol(assurance.AuthMethod) {
		return ErrSSORequired
	}
	provider, _, providerErr := s.repo.GetProvider(ctx, workspaceID, 0, assurance.ProviderID)
	if providerErr != nil || provider == nil || provider.Status != "active" || provider.Revision != assurance.ProviderRevision || identityProtocol(provider.Type) != assurance.AuthMethod {
		return ErrSSORequired
	}
	return nil
}

func (s *EnterpriseIdentityService) UpdatePolicy(ctx context.Context, actorID, workspaceID int64, requireSSO bool, graceUntil *time.Time) (*WorkspaceIdentityPolicy, error) {
	if err := s.require(ctx, actorID, workspaceID, "workspace_sso.update"); err != nil {
		return nil, err
	}
	if requireSSO {
		count, err := s.repo.ActiveProviderCount(ctx, workspaceID, actorID)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, ErrWorkspaceConflict
		}
		linked, err := s.repo.HasUserIdentity(ctx, workspaceID, actorID)
		if err != nil || !linked {
			return nil, ErrOIDCAccountLinkRequired
		}
		assurance, ok := AuthenticationAssuranceFromContext(ctx)
		if !ok || assurance.WorkspaceID != workspaceID || !validEnterpriseProtocol(assurance.AuthMethod) || !assurance.Valid(s.now()) {
			return nil, ErrSSORequired
		}
		provider, _, err := s.repo.GetProvider(ctx, workspaceID, actorID, assurance.ProviderID)
		if err != nil || provider == nil || provider.Status != "active" || provider.Revision != assurance.ProviderRevision || identityProtocol(provider.Type) != assurance.AuthMethod {
			return nil, ErrWorkspaceConflict
		}
		if _, err := validateEnterpriseProviderConfiguration(ctx, provider); err != nil {
			return nil, ErrWorkspaceConflict
		}
	}
	if graceUntil != nil && (graceUntil.Before(s.now()) || graceUntil.After(s.now().Add(7*24*time.Hour))) {
		return nil, ErrEnterpriseIdentityInvalid
	}
	return s.repo.UpdatePolicy(ctx, workspaceID, actorID, requireSSO, graceUntil)
}

func (s *EnterpriseIdentityService) GetMappings(ctx context.Context, actorID, workspaceID, providerID int64) (*OIDCMappings, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.read"); err != nil {
		return nil, err
	}
	return s.repo.GetMappings(ctx, workspaceID, providerID)
}

func (s *EnterpriseIdentityService) ReplaceMappings(ctx context.Context, actorID, workspaceID, providerID int64, input OIDCMappings) error {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return err
	}
	if len(input.Roles) > 200 || len(input.Teams) > 200 {
		return ErrEnterpriseIdentityInvalid
	}
	for _, m := range input.Roles {
		if !validOIDCManagedRole(m.Role) || strings.TrimSpace(m.ClaimValue) == "" || len(m.ClaimValue) > 512 || m.Priority < -100000 || m.Priority > 100000 {
			return ErrEnterpriseIdentityInvalid
		}
	}
	for _, m := range input.Teams {
		if m.TeamID <= 0 || strings.TrimSpace(m.ClaimValue) == "" || len(m.ClaimValue) > 512 {
			return ErrEnterpriseIdentityInvalid
		}
	}
	return s.repo.ReplaceMappings(ctx, workspaceID, actorID, providerID, input)
}

func (s *EnterpriseIdentityService) DiscoverSSO(ctx context.Context, emailOrDomain string) ([]SSODiscoveryProvider, error) {
	value := strings.TrimSpace(emailOrDomain)
	if strings.Contains(value, "@") {
		address, err := mail.ParseAddress(value)
		if err != nil || address.Address != value {
			return []SSODiscoveryProvider{}, nil
		}
		value = value[strings.LastIndex(value, "@")+1:]
	}
	domain, err := NormalizeEnterpriseDomain(value)
	if err != nil {
		return []SSODiscoveryProvider{}, nil
	}
	return s.repo.DiscoverSSO(ctx, domain)
}

func (s *EnterpriseIdentityService) BreakGlass(ctx context.Context, actorID, workspaceID int64, authenticatedAt time.Time, strong bool, reason string) (*WorkspaceIdentityPolicy, error) {
	access, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "owner.manage")
	if err != nil {
		return nil, err
	}
	if access.Member.Role != WorkspaceRoleOwner || access.Workspace.Type != WorkspaceTypeOrganization || !strong || authenticatedAt.IsZero() || authenticatedAt.After(s.now().Add(time.Minute)) || s.now().Sub(authenticatedAt) > 10*time.Minute || len(strings.TrimSpace(reason)) < 10 || len(reason) > 500 {
		return nil, ErrWorkspaceForbidden
	}
	return s.repo.BreakGlass(ctx, workspaceID, actorID, strings.TrimSpace(reason))
}

func validateIdentityProviderInput(input EnterpriseIdentityProviderInput) (EnterpriseIdentityProviderInput, error) {
	input.Type = identityProtocol(strings.ToLower(strings.TrimSpace(input.Type)))
	if !validEnterpriseProtocol(input.Type) {
		return input, ErrEnterpriseIdentityInvalid
	}
	if input.Type == "saml" {
		return validateSAMLInput(input)
	}
	if input.SAML != nil {
		return input, ErrEnterpriseIdentityInvalid
	}
	input.ProviderKey = strings.ToLower(strings.TrimSpace(input.ProviderKey))
	input.Name = strings.TrimSpace(input.Name)
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`).MatchString(input.ProviderKey) || input.Name == "" || len(input.Name) > 120 {
		return input, ErrEnterpriseIdentityInvalid
	}
	issuer, err := ValidateEnterpriseEndpoint(input.IssuerURL)
	if err != nil {
		return input, err
	}
	input.IssuerURL = issuer
	u, err := url.Parse(input.IssuerURL)
	if err != nil || u.RawQuery != "" {
		return input, ErrEnterpriseIdentityInvalid
	}
	if strings.TrimSpace(input.ClientID) == "" || len(input.ClientID) > 512 {
		return input, ErrEnterpriseIdentityInvalid
	}
	input.ClientID = strings.TrimSpace(input.ClientID)
	input.TokenAuthMethod = strings.ToLower(strings.TrimSpace(input.TokenAuthMethod))
	if input.TokenAuthMethod == "" {
		input.TokenAuthMethod = "client_secret_basic"
	}
	if input.TokenAuthMethod != "client_secret_basic" && input.TokenAuthMethod != "client_secret_post" && input.TokenAuthMethod != "none" {
		return input, ErrEnterpriseIdentityInvalid
	}
	if input.SecretAction == "" {
		input.SecretAction = "preserve"
		if input.ClientSecret != nil {
			input.SecretAction = "replace"
		}
	}
	if input.SecretAction != "preserve" && input.SecretAction != "replace" && input.SecretAction != "remove" {
		return input, ErrEnterpriseIdentityInvalid
	}
	if input.ClientSecret != nil && (len(*input.ClientSecret) > 8192 || strings.ContainsAny(*input.ClientSecret, "\x00\r\n")) {
		return input, ErrEnterpriseIdentityInvalid
	}
	if err := validateOIDCClaimMapping(input.ClaimMapping); err != nil {
		return input, err
	}
	if len(input.Scopes) > 32 || len(input.JITConfig.AllowedDomains) > 100 {
		return input, ErrEnterpriseIdentityInvalid
	}
	for _, scope := range input.Scopes {
		if len(scope) > 128 || strings.ContainsAny(scope, " \t\r\n\x00") {
			return input, ErrEnterpriseIdentityInvalid
		}
	}
	if input.DiscoveryEnabled == nil {
		enabled := true
		input.DiscoveryEnabled = &enabled
	}
	if !*input.DiscoveryEnabled {
		for _, endpoint := range []*string{&input.AuthorizationEndpoint, &input.TokenEndpoint, &input.JWKSURI} {
			validated, err := ValidateEnterpriseEndpoint(*endpoint)
			if err != nil {
				return input, err
			}
			*endpoint = validated
		}
	}
	if input.UserinfoEndpoint != "" {
		validated, err := ValidateEnterpriseEndpoint(input.UserinfoEndpoint)
		if err != nil {
			return input, err
		}
		input.UserinfoEndpoint = validated
	}
	if input.JITConfig.DefaultRole == "" {
		input.JITConfig.DefaultRole = WorkspaceRoleViewer
	}
	if !validOIDCManagedRole(input.JITConfig.DefaultRole) {
		return input, ErrEnterpriseIdentityInvalid
	}
	input.JITConfig.RequireVerifiedEmail = true
	for i, domain := range input.JITConfig.AllowedDomains {
		canonical, err := NormalizeEnterpriseDomain(domain)
		if err != nil {
			return input, err
		}
		input.JITConfig.AllowedDomains[i] = canonical
	}
	return input, nil
}

func enterpriseProviderDocument(ctx context.Context, provider *EnterpriseIdentityProvider) (*OIDCDiscoveryDocument, error) {
	if provider == nil || identityProtocol(provider.Type) != "oidc" {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if provider.DiscoveryEnabled {
		return DiscoverOIDC(ctx, provider.IssuerURL)
	}
	document := &OIDCDiscoveryDocument{Issuer: provider.IssuerURL, AuthorizationEndpoint: provider.AuthorizationEndpoint, TokenEndpoint: provider.TokenEndpoint, JWKSURI: provider.JWKSURI, UserinfoEndpoint: provider.UserinfoEndpoint}
	for _, endpoint := range []string{document.Issuer, document.AuthorizationEndpoint, document.TokenEndpoint, document.JWKSURI} {
		if _, err := ValidateEnterpriseEndpoint(endpoint); err != nil {
			return nil, err
		}
	}
	if document.UserinfoEndpoint != "" {
		if _, err := ValidateEnterpriseEndpoint(document.UserinfoEndpoint); err != nil {
			return nil, err
		}
	}
	return document, nil
}

func (s *EnterpriseIdentityService) TestProvider(ctx context.Context, actorID, workspaceID, providerID int64) error {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return err
	}
	provider, _, err := s.repo.GetProvider(ctx, workspaceID, actorID, providerID)
	if err != nil {
		return err
	}
	code, validationErr := "PROVIDER_DISABLED", error(ErrOIDCProviderDisabled)
	if provider.Status == "active" {
		code, validationErr = validateEnterpriseProviderConfiguration(ctx, provider)
	}
	if err := s.repo.RecordProviderValidation(ctx, workspaceID, actorID, providerID, provider.Revision, code); err != nil {
		return err
	}
	return validationErr
}

func enterpriseProviderFromInput(input EnterpriseIdentityProviderInput) *EnterpriseIdentityProvider {
	return &EnterpriseIdentityProvider{Type: input.Type, SAML: input.SAML, IssuerURL: input.IssuerURL, DiscoveryEnabled: input.DiscoveryEnabled == nil || *input.DiscoveryEnabled, AuthorizationEndpoint: input.AuthorizationEndpoint, TokenEndpoint: input.TokenEndpoint, JWKSURI: input.JWKSURI, UserinfoEndpoint: input.UserinfoEndpoint}
}

// Connection tests never contact the token endpoint or disclose upstream
// response bodies. Save/enable also require a reachable supported signing key.
func validateEnterpriseProviderConfiguration(ctx context.Context, provider *EnterpriseIdentityProvider) (string, error) {
	if provider != nil && provider.Type == "saml" {
		if err := validateSAMLConfiguration(provider); err != nil {
			return "CONFIGURATION_INVALID", err
		}
		return "SUCCESS", nil
	}
	document, err := enterpriseProviderDocument(ctx, provider)
	if err != nil {
		return "DISCOVERY_FAILED", ErrEnterpriseIdentityInvalid
	}
	algorithms := enterpriseOIDCSigningAlgorithms(document.IDTokenSigningAlgs)
	if len(algorithms) == 0 {
		return "CONFIGURATION_INVALID", ErrEnterpriseIdentityInvalid
	}
	keys, err := fetchEnterpriseJWKS(ctx, document.JWKSURI)
	if err != nil {
		return "VALIDATION_FAILED", ErrEnterpriseIdentityInvalid
	}
	for _, value := range keys {
		key, ok := value.(enterpriseOIDCSigningKey)
		if !ok {
			continue
		}
		for _, algorithm := range algorithms {
			if key.algorithm != "" && key.algorithm != algorithm {
				continue
			}
			if _, err := enterpriseOIDCKeyForToken(&jwt.Token{Method: jwt.GetSigningMethod(algorithm), Header: map[string]any{"kid": "configuration-probe"}}, map[string]any{"configuration-probe": key}); err == nil {
				return "SUCCESS", nil
			}
		}
	}
	return "CONFIGURATION_INVALID", ErrEnterpriseIdentityInvalid
}

func defaultOIDCScopes(scopes []string) []string {
	if len(scopes) == 0 {
		return []string{"openid", "profile", "email"}
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(scopes)+1)
	for _, scope := range append([]string{"openid"}, scopes...) {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			if _, ok := seen[scope]; !ok {
				seen[scope] = struct{}{}
				result = append(result, scope)
			}
		}
	}
	return result
}
