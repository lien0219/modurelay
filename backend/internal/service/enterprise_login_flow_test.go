package service

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type enterpriseFlowRepo struct {
	EnterpriseIdentityRepository
	provider    EnterpriseIdentityProvider
	state       *OIDCState
	consumed    bool
	provisioned int
}

func (r *enterpriseFlowRepo) GetProvider(context.Context, int64, int64, int64) (*EnterpriseIdentityProvider, string, error) {
	return &r.provider, "encrypted-secret", nil
}
func (r *enterpriseFlowRepo) CreateOIDCState(_ context.Context, state *OIDCState) error {
	r.state = state
	return nil
}
func (r *enterpriseFlowRepo) ConsumeOIDCState(_ context.Context, hash string, browser []byte, now time.Time) (*OIDCState, error) {
	if r.state == nil || hash != r.state.Hash || subtle.ConstantTimeCompare(browser, r.state.BrowserSessionHash) != 1 {
		return nil, ErrOIDCStateSessionMismatch
	}
	if r.consumed {
		return nil, ErrOIDCStateConsumed
	}
	if !now.Before(r.state.ExpiresAt) {
		return nil, ErrOIDCStateExpired
	}
	r.consumed = true
	return r.state, nil
}
func (r *enterpriseFlowRepo) CompleteIdentityLogin(_ context.Context, input OIDCProvisionInput) (int64, error) {
	if input.Claims.Subject != "stable-subject" || input.Claims.Email != "member@example.com" || !input.Claims.EmailVerified || input.ProviderRevision != r.provider.Revision {
		return 0, ErrEnterpriseIdentityInvalid
	}
	r.provisioned++
	return 42, nil
}

type enterpriseFlowUsers struct{ UserRepository }

func (enterpriseFlowUsers) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 42, Email: "member@example.com", Status: StatusActive}, nil
}

type enterpriseFlowEncryptor struct{}

func (enterpriseFlowEncryptor) Encrypt(string) (string, error) { return "encrypted-secret", nil }
func (enterpriseFlowEncryptor) Decrypt(value string) (string, error) {
	if value != "encrypted-secret" {
		return "", ErrEnterpriseIdentityInvalid
	}
	return "secret-after-decryption", nil
}

func TestEnterpriseServiceMockLoginBindsBrowserPKCEClaimsAndSingleUse(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	repo := &enterpriseFlowRepo{provider: EnterpriseIdentityProvider{ID: 9, WorkspaceID: 7, Revision: 3, Status: "active", IssuerURL: "https://identity.example.invalid/service-flow", ClientID: "enterprise-client", DiscoveryEnabled: true, TokenAuthMethod: "client_secret_basic"}}
	document := OIDCDiscoveryDocument{Issuer: repo.provider.IssuerURL, AuthorizationEndpoint: repo.provider.IssuerURL + "/authorize?tenant=1", TokenEndpoint: repo.provider.IssuerURL + "/token", JWKSURI: repo.provider.IssuerURL + "/keys", IDTokenSigningAlgs: []string{"RS256"}}
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/service-flow/.well-known/openid-configuration":
			return enterpriseOIDCTestResponse(request, 200, enterpriseOIDCTestJSON(t, document)), nil
		case "/service-flow/token":
			require.NoError(t, request.ParseForm())
			id, secret, ok := request.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "enterprise-client", id)
			require.Equal(t, "secret-after-decryption", secret)
			require.Equal(t, repo.state.PKCEVerifier, request.Form.Get("code_verifier"))
			require.Equal(t, "single-use-code", request.Form.Get("code"))
			claims := enterpriseOIDCTestClaims()
			claims["iss"] = document.Issuer
			claims["nonce"] = repo.state.Nonce
			return enterpriseOIDCTestResponse(request, 200, enterpriseOIDCTestJSON(t, OIDCTokenResponse{IDToken: enterpriseOIDCTestSignedToken(t, key, claims)})), nil
		case "/service-flow/keys":
			return enterpriseOIDCTestResponse(request, 200, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{enterpriseOIDCTestRSAJWK(&key.PublicKey)}})), nil
		default:
			return enterpriseOIDCTestResponse(request, 404, `{}`), nil
		}
	}))
	svc := NewEnterpriseIdentityService(repo, nil, enterpriseFlowUsers{}, enterpriseFlowEncryptor{})
	start, err := svc.StartOIDC(ctx, 7, 9, "/workspaces/7/identity", "https://app.example.invalid/api/v1/auth/sso/callback")
	require.NoError(t, err)
	authorize, err := url.Parse(start.AuthorizationURL)
	require.NoError(t, err)
	require.Equal(t, "1", authorize.Query().Get("tenant"))
	require.Equal(t, OIDCPKCEChallenge(repo.state.PKCEVerifier), authorize.Query().Get("code_challenge"))
	require.Equal(t, "S256", authorize.Query().Get("code_challenge_method"))
	state := authorize.Query().Get("state")
	_, err = svc.CompleteOIDC(ctx, state, "wrong-browser", "single-use-code", "https://app.example.invalid/api/v1/auth/sso/callback")
	require.ErrorIs(t, err, ErrOIDCStateSessionMismatch)
	require.False(t, repo.consumed)
	require.Zero(t, repo.provisioned)
	login, err := svc.CompleteOIDC(ctx, state, start.BrowserCookie, "single-use-code", "https://app.example.invalid/api/v1/auth/sso/callback")
	require.NoError(t, err)
	require.EqualValues(t, 42, login.User.ID)
	require.EqualValues(t, 3, login.Assurance.ProviderRevision)
	require.Equal(t, "/workspaces/7/identity", login.ReturnTo)
	require.Equal(t, 1, repo.provisioned)
	_, err = svc.CompleteOIDC(ctx, state, start.BrowserCookie, "single-use-code", "https://app.example.invalid/api/v1/auth/sso/callback")
	require.ErrorIs(t, err, ErrOIDCStateConsumed)
	require.Equal(t, 1, repo.provisioned)
}
