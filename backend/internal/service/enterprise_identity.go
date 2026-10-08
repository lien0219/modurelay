package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"golang.org/x/net/idna"
)

const (
	WorkspaceTypePersonal     = "personal"
	WorkspaceTypeOrganization = "organization"
	WorkspaceRoleOwner        = "owner"
	WorkspaceRoleAdmin        = "admin"
	WorkspaceRoleDeveloper    = "developer"
	WorkspaceRoleBilling      = "billing"
	WorkspaceRoleViewer       = "viewer"
	MembershipSourceManual    = "manual"
	MembershipSourceOIDC      = "oidc"
	MembershipSourceSAML      = "saml"
	MembershipSourceSCIM      = "scim"
	PrincipalHuman            = "human"
	PrincipalAPIKey           = "api_key"
	PrincipalServiceAccount   = "service_account"
)

var (
	ErrEnterpriseIdentityInvalid = infraerrors.BadRequest("ENTERPRISE_IDENTITY_INVALID", "invalid enterprise identity configuration")
	ErrDomainAlreadyClaimed      = infraerrors.Conflict("DOMAIN_ALREADY_CLAIMED", "domain is already claimed")
	ErrOIDCStateNotFound         = errors.New("oidc state not found")
	ErrOIDCStateConsumed         = errors.New("oidc state already consumed")
	ErrOIDCStateExpired          = errors.New("oidc state expired")
	ErrOIDCStateSessionMismatch  = infraerrors.Unauthorized("OIDC_STATE_SESSION_MISMATCH", "oidc state is not bound to this browser session")
	ErrSSORequired               = infraerrors.Forbidden("SSO_REQUIRED", "workspace requires single sign-on")
	ErrOIDCProviderDisabled      = infraerrors.Forbidden("OIDC_PROVIDER_DISABLED", "identity provider is disabled")
	ErrOIDCAccountLinkRequired   = infraerrors.Conflict("OIDC_ACCOUNT_LINK_REQUIRED", "an authenticated account link is required")
)

// NormalizeEnterpriseDomain returns the lower-case IDNA lookup form used for
// uniqueness and domain claims. Email addresses and URL forms are rejected.
func NormalizeEnterpriseDomain(raw string) (string, error) {
	domain := strings.TrimSuffix(strings.TrimSpace(raw), ".")
	if domain == "" || strings.ContainsAny(domain, "/\\@:?#") || net.ParseIP(domain) != nil {
		return "", ErrEnterpriseIdentityInvalid
	}
	domain = strings.ToLower(domain)
	ascii, err := idna.Lookup.ToASCII(domain)
	if err != nil || len(ascii) > 255 || !strings.Contains(ascii, ".") {
		return "", ErrEnterpriseIdentityInvalid
	}
	labels := strings.Split(ascii, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return "", ErrEnterpriseIdentityInvalid
		}
	}
	return ascii, nil
}

// ValidateEnterpriseReturnTo accepts only an application-relative path. It
// intentionally rejects scheme-relative URLs and backslash variants.
func ValidateEnterpriseReturnTo(raw string) error {
	if raw == "" || len(raw) > 1024 || strings.ContainsAny(raw, "\\\r\n") || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ErrEnterpriseIdentityInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil || strings.HasPrefix(u.Path, "//") || strings.ContainsAny(u.Path, "\\\r\n\x00") || path.Clean(u.Path) != strings.TrimSuffix(u.Path, "/") && u.Path != "/" {
		return ErrEnterpriseIdentityInvalid
	}
	for _, prefix := range []string{"/workspaces", "/dashboard", "/profile"} {
		if u.Path == prefix || strings.HasPrefix(u.Path, prefix+"/") {
			return nil
		}
	}
	return ErrEnterpriseIdentityInvalid
}

// ValidateEnterpriseEndpoint validates a server-side OIDC endpoint. The
// caller must still pin DNS results immediately before each request.
func ValidateEnterpriseEndpoint(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || strings.ToLower(u.Scheme) != "https" || u.Hostname() == "" || u.Fragment != "" {
		return "", ErrEnterpriseIdentityInvalid
	}
	validated, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{AllowPrivate: false})
	if err != nil || urlvalidator.IsBlockedHost(u.Hostname()) {
		return "", ErrEnterpriseIdentityInvalid
	}
	_ = validated
	return strings.TrimSpace(raw), nil
}

// NewSecureToken returns an opaque, high entropy token and its SHA-256 hash.
// Only the hash belongs in durable state tables.
func NewSecureToken(size int) (token string, hash []byte, err error) {
	if size < 32 || size > 256 {
		return "", nil, ErrEnterpriseIdentityInvalid
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate secure token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, sum[:], nil
}

func HashEnterpriseToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func EnterpriseTokenHashHex(token string) string {
	return hex.EncodeToString(HashEnterpriseToken(token))
}

// EnterpriseTokenMatchesHash compares an opaque credential in constant time.
func EnterpriseTokenMatchesHash(token string, expected []byte) bool {
	actual := HashEnterpriseToken(token)
	return len(expected) == len(actual) && subtle.ConstantTimeCompare(actual, expected) == 1
}

type OIDCState struct {
	Protocol           string
	RequestID          string
	Hash               string
	WorkspaceID        int64
	ProviderID         int64
	ProviderRevision   int64
	BrowserSessionHash []byte
	NonceHash          []byte
	Nonce              string `json:"-"`
	PKCEVerifier       string
	ReturnTo           string
	Intent             string
	LinkUserID         *int64
	CreatedAt          time.Time
	ExpiresAt          time.Time
	_opaqueState       string
	_opaqueBrowser     string
	_opaqueNonce       string
}

func NewOIDCState(now time.Time, ttl time.Duration, workspaceID, providerID int64, returnTo, intent string) (*OIDCState, error) {
	if workspaceID <= 0 || providerID <= 0 || ttl <= 0 || ttl > 15*time.Minute || ValidateEnterpriseReturnTo(returnTo) != nil || (intent != "login" && intent != "link") {
		return nil, ErrEnterpriseIdentityInvalid
	}
	state, stateHash, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	nonce, nonceHash, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	verifier, _, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	browser, browserHash, err := NewSecureToken(32)
	if err != nil {
		return nil, err
	}
	// State.Hash is the durable hash representation; the browser receives the
	// opaque state token through the caller's response, never the stored hash.
	return &OIDCState{
		Hash:               hex.EncodeToString(stateHash),
		Protocol:           "oidc",
		WorkspaceID:        workspaceID,
		ProviderID:         providerID,
		BrowserSessionHash: browserHash,
		NonceHash:          nonceHash,
		Nonce:              nonce,
		PKCEVerifier:       verifier,
		ReturnTo:           returnTo,
		Intent:             intent,
		CreatedAt:          now,
		ExpiresAt:          now.Add(ttl),
		_opaqueState:       state,
		_opaqueBrowser:     browser,
		_opaqueNonce:       nonce,
	}, nil
}

// Opaque values are available to the start handler but intentionally omitted
// from JSON and persistence representations.
func (s *OIDCState) OpaqueState() string {
	if s == nil {
		return ""
	}
	return s._opaqueState
}

func (s *OIDCState) OpaqueBrowserSession() string {
	if s == nil {
		return ""
	}
	return s._opaqueBrowser
}

func (s *OIDCState) OpaqueNonce() string {
	if s == nil {
		return ""
	}
	return s._opaqueNonce
}

type OIDCRoleMapping struct {
	ClaimValue string `json:"claim_value"`
	Role       string `json:"role"`
	Priority   int    `json:"priority"`
}

type OIDCTeamMembership struct {
	TeamID     int64
	Source     string
	ProviderID int64
}

func MapOIDCRole(claims []string, mappings []OIDCRoleMapping, currentRole string, currentRoleManual bool) (string, bool) {
	if currentRoleManual {
		return currentRole, false
	}
	claimSet := make(map[string]struct{}, len(claims))
	for _, claim := range claims {
		claimSet[strings.TrimSpace(claim)] = struct{}{}
	}
	best := OIDCRoleMapping{Role: currentRole, Priority: -1 << 30}
	for _, mapping := range mappings {
		if mapping.Role == WorkspaceRoleOwner || !validOIDCManagedRole(mapping.Role) {
			continue
		}
		if _, ok := claimSet[strings.TrimSpace(mapping.ClaimValue)]; !ok {
			continue
		}
		if mapping.Priority > best.Priority || (mapping.Priority == best.Priority && oidcRoleRank(mapping.Role) > oidcRoleRank(best.Role)) {
			best = mapping
		}
	}
	if best.Priority == -1<<30 {
		return currentRole, false
	}
	return best.Role, true
}

func validOIDCManagedRole(role string) bool {
	return role == WorkspaceRoleViewer || role == WorkspaceRoleDeveloper || role == WorkspaceRoleAdmin || role == WorkspaceRoleBilling
}

func oidcRoleRank(role string) int {
	switch role {
	case WorkspaceRoleAdmin:
		return 4
	case WorkspaceRoleBilling:
		return 3
	case WorkspaceRoleDeveloper:
		return 2
	case WorkspaceRoleViewer:
		return 1
	default:
		return 0
	}
}

// ReconcileOIDCTeams changes only memberships sourced from OIDC. A missing or
// overage group claim is represented by groupsPresent=false and preserves the
// previous OIDC set to avoid accidental mass removal.
func ReconcileOIDCTeams(existing []OIDCTeamMembership, desired []int64, groupsPresent bool) []OIDCTeamMembership {
	if !groupsPresent {
		return append([]OIDCTeamMembership(nil), existing...)
	}
	desiredSet := make(map[int64]struct{}, len(desired))
	for _, id := range desired {
		if id > 0 {
			desiredSet[id] = struct{}{}
		}
	}
	result := make([]OIDCTeamMembership, 0, len(existing)+len(desiredSet))
	for _, member := range existing {
		if member.Source != MembershipSourceOIDC {
			result = append(result, member)
		}
	}
	for id := range desiredSet {
		result = append(result, OIDCTeamMembership{TeamID: id, Source: MembershipSourceOIDC})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TeamID < result[j].TeamID })
	return result
}

type WorkspaceSecurityPolicy struct {
	WorkspaceID                  int64                      `json:"workspace_id"`
	RequireSSO                   bool                       `json:"require_sso"`
	SSOGraceUntil                *time.Time                 `json:"sso_grace_until"`
	RequireMFA                   bool                       `json:"require_mfa"`
	SessionMaxAgeSeconds         *int                       `json:"session_max_age_seconds"`
	InvitationPolicy             string                     `json:"invitation_policy"`
	AllowExternalMembers         bool                       `json:"allow_external_members"`
	WorkspaceJITEnabled          bool                       `json:"workspace_jit_enabled"`
	ApprovedIdentityProviderMode string                     `json:"approved_identity_provider_mode"`
	ApprovedIdentityProviderIDs  []int64                    `json:"approved_identity_provider_ids"`
	Revision                     int64                      `json:"revision"`
	UpdatedBy                    *int64                     `json:"updated_by_user_id,omitempty"`
	UpdatedAt                    time.Time                  `json:"updated_at"`
	VerifiedDomains              []string                   `json:"verified_domains,omitempty"`
	ExternalMemberCount          int64                      `json:"external_member_count"`
	Decision                     *WorkspaceSecurityDecision `json:"decision,omitempty"`
	PrerequisiteReason           string                     `json:"prerequisite_reason,omitempty"`
	SessionMaxAgeMinSeconds      int                        `json:"session_max_age_min_seconds"`
	SessionMaxAgeMaxSeconds      int                        `json:"session_max_age_max_seconds"`
}

type WorkspaceAssurance struct {
	WorkspaceID      int64
	ProviderID       int64
	ProviderRevision int64
	AuthenticatedAt  time.Time
	ValidUntil       time.Time
	AuthMethod       string
}

type authenticationAssuranceContextKey struct{}

// WithAuthenticationAssurance carries control-plane authentication metadata
// through token issuance without putting it in request parameters.
func WithAuthenticationAssurance(ctx context.Context, assurance WorkspaceAssurance) context.Context {
	if _, ok := SessionAuthenticationFromContext(ctx); !ok {
		ctx = WithSessionAuthentication(ctx, SessionAuthentication{AuthMethod: assurance.AuthMethod, AuthenticatedAt: assurance.AuthenticatedAt})
	}
	return context.WithValue(ctx, authenticationAssuranceContextKey{}, assurance)
}

func AuthenticationAssuranceFromContext(ctx context.Context) (WorkspaceAssurance, bool) {
	if ctx == nil {
		return WorkspaceAssurance{}, false
	}
	assurance, ok := ctx.Value(authenticationAssuranceContextKey{}).(WorkspaceAssurance)
	return assurance, ok
}

func (a WorkspaceAssurance) Valid(now time.Time) bool {
	return a.WorkspaceID > 0 && a.ProviderID > 0 && !a.AuthenticatedAt.IsZero() && !a.AuthenticatedAt.After(now.Add(time.Minute)) && now.Sub(a.AuthenticatedAt) < LegacyEnterpriseAssuranceMaxAge && (a.ValidUntil.IsZero() || now.Before(a.ValidUntil))
}

func RequireWorkspaceAssurance(policy WorkspaceSecurityPolicy, workspaceType, principal string, assurance WorkspaceAssurance) error {
	if !policy.RequireSSO || workspaceType == WorkspaceTypePersonal || principal == PrincipalAPIKey || principal == PrincipalServiceAccount {
		return nil
	}
	if principal != PrincipalHuman || !assurance.Valid(time.Now()) || (policy.WorkspaceID > 0 && assurance.WorkspaceID != policy.WorkspaceID) {
		return ErrSSORequired
	}
	return nil
}

type JITConfig struct {
	Enabled              bool     `json:"enabled"`
	DefaultRole          string   `json:"default_role"`
	AllowedDomains       []string `json:"allowed_domains"`
	RequireVerifiedEmail bool     `json:"require_verified_email"`
}

func (c JITConfig) Allows(email string, emailVerified bool, domains []string, workspaceActive bool) bool {
	if !c.Enabled || !workspaceActive || !emailVerified {
		return false
	}
	part := strings.LastIndex(strings.ToLower(strings.TrimSpace(email)), "@")
	if part < 1 {
		return false
	}
	domain := strings.ToLower(strings.TrimSuffix(email[part+1:], "."))
	allowed := c.AllowedDomains
	if len(allowed) == 0 {
		allowed = domains
	}
	for _, candidate := range allowed {
		canonical, err := NormalizeEnterpriseDomain(candidate)
		if err == nil && canonical == domain {
			return true
		}
	}
	return false
}
