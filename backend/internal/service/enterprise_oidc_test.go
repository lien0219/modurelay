package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type enterpriseOIDCRoundTripper func(*http.Request) (*http.Response, error)

func (f enterpriseOIDCRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func enterpriseOIDCTestResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{Request: request, StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func enterpriseOIDCTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func enterpriseOIDCTestClaims() jwt.MapClaims {
	now := time.Now().UTC()
	return jwt.MapClaims{
		"iss": "https://identity.example.com/tenant", "sub": "stable-subject", "aud": "enterprise-client",
		"exp": now.Add(time.Minute).Unix(), "iat": now.Add(-time.Minute).Unix(), "nonce": "browser-bound-nonce",
		"email": "member@example.com", "email_verified": true, "name": "Enterprise Member", "groups": []string{"engineering"},
	}
}

func enterpriseOIDCTestSignedToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "signing-key"
	raw, err := token.SignedString(key)
	require.NoError(t, err)
	return raw
}

func enterpriseOIDCTestCachedDocument(t *testing.T, key *rsa.PublicKey) *OIDCDiscoveryDocument {
	t.Helper()
	document := &OIDCDiscoveryDocument{Issuer: "https://identity.example.com/tenant", JWKSURI: "https://identity.example.com/claims-test-keys", IDTokenSigningAlgs: []string{"RS256"}}
	previous := defaultEnterpriseJWKSCache
	defaultEnterpriseJWKSCache = &enterpriseJWKSCache{items: map[string]enterpriseJWKSCacheEntry{document.JWKSURI: {keys: map[string]any{"signing-key": key}, expiresAt: time.Now().Add(time.Minute)}}}
	t.Cleanup(func() { defaultEnterpriseJWKSCache = previous })
	return document
}

func TestEnterpriseOIDCIDTokenRequiresSecurityClaims(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	cases := []struct {
		name   string
		change func(jwt.MapClaims)
	}{
		{"missing_expiration", func(c jwt.MapClaims) { delete(c, "exp") }},
		{"missing_issued_at", func(c jwt.MapClaims) { delete(c, "iat") }},
		{"future_issued_at", func(c jwt.MapClaims) { c["iat"] = time.Now().Add(time.Hour).Unix() }},
		{"future_not_before", func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(time.Hour).Unix() }},
		{"expired", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }},
		{"issuer_mismatch", func(c jwt.MapClaims) { c["iss"] = "https://identity.example.com/tenant/" }},
		{"audience_mismatch", func(c jwt.MapClaims) { c["aud"] = "different-client" }},
		{"different_authorized_party", func(c jwt.MapClaims) { c["azp"] = "different-client" }},
		{"invalid_authorized_party_type", func(c jwt.MapClaims) { c["azp"] = []string{"enterprise-client"} }},
		{"multiple_audiences_without_authorized_party", func(c jwt.MapClaims) { c["aud"] = []string{"enterprise-client", "another-client"} }},
		{"multiple_audiences_with_wrong_authorized_party", func(c jwt.MapClaims) {
			c["aud"] = []string{"enterprise-client", "another-client"}
			c["azp"] = "another-client"
		}},
		{"nonce_mismatch", func(c jwt.MapClaims) { c["nonce"] = "different-nonce" }},
		{"missing_nonce", func(c jwt.MapClaims) { delete(c, "nonce") }},
		{"blank_subject", func(c jwt.MapClaims) { c["sub"] = "   " }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			claims := enterpriseOIDCTestClaims()
			tc.change(claims)
			_, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, key, claims), document, "enterprise-client", "browser-bound-nonce")
			require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
		})
	}
}

func TestEnterpriseOIDCIDTokenAcceptsMultipleAudiencesWithAuthorizedParty(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	claims := enterpriseOIDCTestClaims()
	claims["aud"] = []string{"enterprise-client", "another-client"}
	claims["azp"] = "enterprise-client"
	result, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, key, claims), document, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err)
	require.Equal(t, "stable-subject", result.Subject)
	require.Equal(t, "enterprise-client", result.AuthorizedParty)
	require.Equal(t, []string{"enterprise-client", "another-client"}, result.Audience)
	require.False(t, result.ExpiresAt.IsZero())
	require.False(t, result.IssuedAt.IsZero())
}

func TestEnterpriseOIDCIDTokenAllowsIdentityWithoutEmailClaim(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	claims := enterpriseOIDCTestClaims()
	delete(claims, "email")
	result, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, key, claims), document, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err, "email may come from a subject-bound userinfo response or configured claim mapping")
	require.Equal(t, "stable-subject", result.Subject)
	require.Empty(t, result.Email)
}

func TestEnterpriseOIDCIDTokenRejectsOversizedToken(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	claims := enterpriseOIDCTestClaims()
	claims["padding"] = strings.Repeat("a", 128<<10)
	_, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, key, claims), document, "enterprise-client", "browser-bound-nonce")
	require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
}

func TestEnterpriseOIDCJSONRejectsOversizedOrTrailingResponses(t *testing.T) {
	for _, body := range []string{`{"issuer":"https://identity.example.com"}` + strings.Repeat(" ", 512<<10), `{"issuer":"https://identity.example.com"} {"issuer":"https://evil.example.com"}`} {
		client := &oidcHTTPClient{client: &http.Client{Transport: enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
			return enterpriseOIDCTestResponse(request, http.StatusOK, body), nil
		})}}
		var document OIDCDiscoveryDocument
		require.Error(t, client.getJSON(context.Background(), "https://identity.example.com/document", &document))
	}
}

func enterpriseOIDCTestRSAJWK(key *rsa.PublicKey) enterpriseJWK {
	return enterpriseJWK{KTY: "RSA", Use: "sig", Alg: "RS256", Kid: "signing-key", N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()), E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
}

func TestEnterpriseOIDCJWKRejectsUnsafeRSAKeys(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	base := enterpriseOIDCTestRSAJWK(&key.PublicKey)
	weakKey, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	for _, tc := range []struct {
		name string
		key  enterpriseJWK
	}{
		{"encryption_key", func() enterpriseJWK { k := base; k.Use = "enc"; return k }()},
		{"symmetric_algorithm", func() enterpriseJWK { k := base; k.Alg = "HS256"; return k }()},
		{"elliptic_algorithm", func() enterpriseJWK { k := base; k.Alg = "ES256"; return k }()},
		{"weak_modulus", enterpriseOIDCTestRSAJWK(&weakKey.PublicKey)},
		{"even_exponent", func() enterpriseJWK { k := base; k.E = "Ag"; return k }()},
		{"empty_modulus", func() enterpriseJWK { k := base; k.N = ""; return k }()},
	} {
		t.Run(tc.name, func(t *testing.T) { _, err := parseEnterpriseJWK(tc.key); require.Error(t, err) })
	}
	_, err = parseEnterpriseJWK(base)
	require.NoError(t, err)
}

func TestEnterpriseOIDCJWKRejectsInvalidCurveAndAlgorithm(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	base := enterpriseJWK{KTY: "EC", Use: "sig", Alg: "ES256", Kid: "ec-key", Crv: "P-256", X: base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))), Y: base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32)))}
	for _, tc := range []struct {
		name string
		key  enterpriseJWK
	}{
		{"point_off_curve", func() enterpriseJWK {
			k := base
			k.X = base64.RawURLEncoding.EncodeToString(make([]byte, 32))
			k.Y = k.X
			return k
		}()},
		{"mismatched_algorithm", func() enterpriseJWK { k := base; k.Alg = "ES384"; return k }()},
		{"truncated_coordinate", func() enterpriseJWK { k := base; k.X = "AQ"; return k }()},
	} {
		t.Run(tc.name, func(t *testing.T) { _, err := parseEnterpriseJWK(tc.key); require.Error(t, err) })
	}
	_, err = parseEnterpriseJWK(base)
	require.NoError(t, err)
}

func enterpriseOIDCTestJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

func enterpriseOIDCTestContext(transport http.RoundTripper) context.Context {
	return context.WithValue(context.Background(), enterpriseOIDCHTTPTransportContextKey{}, transport)
}

func TestEnterpriseOIDCMockAuthorizationCodeFlow(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	issuer := "https://identity.example.invalid/tenant/"
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	require.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", OIDCPKCEChallenge(verifier))
	claims := enterpriseOIDCTestClaims()
	claims["iss"] = issuer
	rawToken := enterpriseOIDCTestSignedToken(t, key, claims)
	document := OIDCDiscoveryDocument{Issuer: issuer, AuthorizationEndpoint: issuer + "authorize", TokenEndpoint: issuer + "token", JWKSURI: issuer + "keys", UserinfoEndpoint: issuer + "userinfo", ScopesSupported: []string{"openid", "profile", "email"}, IDTokenSigningAlgs: []string{"RS256"}}
	transport := enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme != "https" || request.URL.Host != "identity.example.invalid" {
			return nil, errors.New("unexpected OIDC destination")
		}
		switch request.URL.Path {
		case "/tenant/.well-known/openid-configuration":
			return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, document)), nil
		case "/tenant/token":
			if err := request.ParseForm(); err != nil {
				return nil, err
			}
			clientID, secret, ok := request.BasicAuth()
			if request.Method != http.MethodPost || !ok || clientID != "enterprise-client" || secret != "test-secret" || request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("code") != "single-use-code" || request.Form.Get("client_id") != "enterprise-client" || request.Form.Get("redirect_uri") != "https://app.example.invalid/api/v1/auth/sso/callback" || request.Form.Get("code_verifier") != verifier {
				return enterpriseOIDCTestResponse(request, http.StatusBadRequest, `{"error":"invalid_grant"}`), nil
			}
			return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, OIDCTokenResponse{AccessToken: "test-access-token", IDToken: rawToken, TokenType: "Bearer", ExpiresIn: 60})), nil
		case "/tenant/keys":
			return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{enterpriseOIDCTestRSAJWK(&key.PublicKey)}})), nil
		case "/tenant/userinfo":
			if request.Header.Get("Authorization") != "Bearer test-access-token" {
				return enterpriseOIDCTestResponse(request, http.StatusUnauthorized, `{"error":"invalid_token"}`), nil
			}
			return enterpriseOIDCTestResponse(request, http.StatusOK, `{"sub":"stable-subject","email":"member@example.com","email_verified":true}`), nil
		default:
			return enterpriseOIDCTestResponse(request, http.StatusNotFound, `{}`), nil
		}
	})
	ctx := enterpriseOIDCTestContext(transport)
	previous := defaultEnterpriseJWKSCache
	defaultEnterpriseJWKSCache = &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
	t.Cleanup(func() { defaultEnterpriseJWKSCache = previous })
	discovered, err := DiscoverOIDC(ctx, issuer)
	require.NoError(t, err)
	require.Equal(t, issuer, discovered.Issuer)
	tokens, err := ExchangeOIDCCode(ctx, discovered, "enterprise-client", "test-secret", "single-use-code", "https://app.example.invalid/api/v1/auth/sso/callback", verifier)
	require.NoError(t, err)
	validated, err := ParseAndValidateOIDCIDToken(ctx, tokens.IDToken, discovered, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err)
	require.Equal(t, "stable-subject", validated.Subject)
	require.Equal(t, "member@example.com", validated.Email)
	userinfo, err := FetchOIDCUserinfo(ctx, discovered.UserinfoEndpoint, tokens.AccessToken)
	require.NoError(t, err)
	require.Equal(t, "stable-subject", userinfo["sub"])
}

func TestEnterpriseOIDCDiscoveryRejectsIssuerMismatchAndPrivateMetadata(t *testing.T) {
	const issuer = "https://identity.example.invalid/tenant"
	for _, tc := range []struct {
		name   string
		change func(*OIDCDiscoveryDocument)
	}{
		{"trailing_slash_issuer_mismatch", func(d *OIDCDiscoveryDocument) { d.Issuer += "/" }},
		{"different_issuer", func(d *OIDCDiscoveryDocument) { d.Issuer = "https://other.example.invalid" }},
		{"private_authorization", func(d *OIDCDiscoveryDocument) { d.AuthorizationEndpoint = "https://127.0.0.1/authorize" }},
		{"private_token", func(d *OIDCDiscoveryDocument) { d.TokenEndpoint = "https://10.0.0.1/token" }},
		{"private_jwks", func(d *OIDCDiscoveryDocument) { d.JWKSURI = "https://169.254.169.254/keys" }},
		{"private_userinfo", func(d *OIDCDiscoveryDocument) { d.UserinfoEndpoint = "https://[::1]/userinfo" }},
		{"insecure_jwks", func(d *OIDCDiscoveryDocument) { d.JWKSURI = "http://keys.example.invalid/keys" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := OIDCDiscoveryDocument{Issuer: issuer, AuthorizationEndpoint: issuer + "/authorize", TokenEndpoint: issuer + "/token", JWKSURI: issuer + "/keys", UserinfoEndpoint: issuer + "/userinfo", IDTokenSigningAlgs: []string{"RS256"}}
			tc.change(&document)
			ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
				return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, document)), nil
			}))
			_, err := DiscoverOIDC(ctx, issuer)
			require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
		})
	}
}

func TestEnterpriseOIDCIDTokenRejectsUnadvertisedAlgorithm(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	document.IDTokenSigningAlgs = []string{"RS384"}
	_, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, key, enterpriseOIDCTestClaims()), document, "enterprise-client", "browser-bound-nonce")
	require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
}

func TestEnterpriseOIDCIDTokenRejectsInvalidIssuerConfiguration(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	for _, issuer := range []string{"http://identity.example.invalid", "https://127.0.0.1", "https://identity.example.invalid?tenant=1", "https://identity.example.invalid?", "https://identity.example.invalid#tenant"} {
		claims := enterpriseOIDCTestClaims()
		claims["iss"] = issuer
		document.Issuer = issuer
		_, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, key, claims), document, "enterprise-client", "browser-bound-nonce")
		require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid, issuer)
	}
}

func TestEnterpriseOIDCHTTPDoesNotFollowRedirects(t *testing.T) {
	calls := 0
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		response := enterpriseOIDCTestResponse(request, http.StatusFound, "")
		response.Header.Set("Location", "https://127.0.0.1/private")
		return response, nil
	}))
	_, err := DiscoverOIDC(ctx, "https://identity.example.invalid/redirect")
	require.Error(t, err)
	require.Equal(t, 1, calls, "redirect must never cause a second outbound request")
}

func TestEnterpriseOIDCJWKSUnknownKeyRotationAndRefreshLimits(t *testing.T) {
	firstKey, nextKey := enterpriseOIDCTestRSAKey(t), enterpriseOIDCTestRSAKey(t)
	firstJWK := enterpriseOIDCTestRSAJWK(&firstKey.PublicKey)
	nextJWK := enterpriseOIDCTestRSAJWK(&nextKey.PublicKey)
	nextJWK.Kid = "rotated-key"
	calls := 0
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		key := firstJWK
		if calls > 1 {
			key = nextJWK
		}
		return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{key}})), nil
	}))
	document := &OIDCDiscoveryDocument{Issuer: "https://identity.example.com/tenant", JWKSURI: "https://identity.example.invalid/rotation-keys", IDTokenSigningAlgs: []string{"RS256"}}
	previous := defaultEnterpriseJWKSCache
	defaultEnterpriseJWKSCache = &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
	t.Cleanup(func() { defaultEnterpriseJWKSCache = previous })
	_, err := ParseAndValidateOIDCIDToken(ctx, enterpriseOIDCTestSignedToken(t, firstKey, enterpriseOIDCTestClaims()), document, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, enterpriseOIDCTestClaims())
	token.Header["kid"] = "rotated-key"
	rotated, err := token.SignedString(nextKey)
	require.NoError(t, err)
	_, err = ParseAndValidateOIDCIDToken(ctx, rotated, document, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err, "an unknown kid must trigger one JWKS rotation refresh")
	for i := 0; i < 10; i++ {
		token.Header["kid"] = "unknown-key-" + strings.Repeat("x", i)
		raw, signErr := token.SignedString(nextKey)
		require.NoError(t, signErr)
		_, err = ParseAndValidateOIDCIDToken(ctx, raw, document, "enterprise-client", "browser-bound-nonce")
		require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
	}
	require.Equal(t, 2, calls, "untrusted unknown kids must not cause unlimited provider requests")
}

func TestEnterpriseOIDCJWKSFailureBackoffKeepsOnlyUnexpiredCachedKeys(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	calls := 0
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			return enterpriseOIDCTestResponse(request, http.StatusServiceUnavailable, `{}`), nil
		}
		return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{enterpriseOIDCTestRSAJWK(&key.PublicKey)}})), nil
	}))
	document := &OIDCDiscoveryDocument{Issuer: "https://identity.example.com/tenant", JWKSURI: "https://identity.example.invalid/backoff-keys", IDTokenSigningAlgs: []string{"RS256"}}
	previous := defaultEnterpriseJWKSCache
	defaultEnterpriseJWKSCache = &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
	t.Cleanup(func() { defaultEnterpriseJWKSCache = previous })
	knownToken := enterpriseOIDCTestSignedToken(t, key, enterpriseOIDCTestClaims())
	_, err := ParseAndValidateOIDCIDToken(ctx, knownToken, document, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err)
	unknown := jwt.NewWithClaims(jwt.SigningMethodRS256, enterpriseOIDCTestClaims())
	unknown.Header["kid"] = "unknown-key"
	unknownRaw, err := unknown.SignedString(key)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		_, err = ParseAndValidateOIDCIDToken(ctx, unknownRaw, document, "enterprise-client", "browser-bound-nonce")
		require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
	}
	_, err = ParseAndValidateOIDCIDToken(ctx, knownToken, document, "enterprise-client", "browser-bound-nonce")
	require.NoError(t, err, "a known unexpired cached key is safe during a provider outage")
	defaultEnterpriseJWKSCache.mu.Lock()
	entry := defaultEnterpriseJWKSCache.items[document.JWKSURI]
	entry.expiresAt = time.Now().Add(-time.Minute)
	defaultEnterpriseJWKSCache.items[document.JWKSURI] = entry
	defaultEnterpriseJWKSCache.mu.Unlock()
	_, err = ParseAndValidateOIDCIDToken(ctx, knownToken, document, "enterprise-client", "browser-bound-nonce")
	require.Error(t, err, "expired keys must never survive a provider outage")
	require.Equal(t, 2, calls, "provider failures must be cached for the refresh backoff")
}

func TestEnterpriseOIDCJWKSCoalescesConcurrentFetches(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	started, release := make(chan struct{}), make(chan struct{})
	var mutex sync.Mutex
	calls := 0
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		mutex.Lock()
		calls++
		if calls == 1 {
			close(started)
		}
		mutex.Unlock()
		<-release
		return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{enterpriseOIDCTestRSAJWK(&key.PublicKey)}})), nil
	}))
	cache := &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
	errors := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func() {
			_, err := cache.get(ctx, "https://identity.example.invalid/concurrent-keys", false)
			errors <- err
		}()
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("JWKS request did not start")
	}
	close(release)
	for i := 0; i < 16; i++ {
		require.NoError(t, <-errors)
	}
	require.Equal(t, 1, calls)
}

func TestEnterpriseOIDCJWKSCacheIsBounded(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{enterpriseOIDCTestRSAJWK(&key.PublicKey)}})), nil
	}))
	cache := &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
	for i := 0; i < 160; i++ {
		_, err := cache.get(ctx, "https://identity.example.invalid/keys/"+big.NewInt(int64(i)).String(), false)
		require.NoError(t, err)
	}
	require.LessOrEqual(t, len(cache.items), 128)
}

func TestEnterpriseOIDCJWKSCacheRejectsPrivateEndpointBeforeLookup(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	endpoint := "https://127.0.0.1/keys"
	cache := &enterpriseJWKSCache{items: map[string]enterpriseJWKSCacheEntry{endpoint: {keys: map[string]any{"signing-key": &key.PublicKey}, expiresAt: time.Now().Add(time.Minute)}}}
	_, err := cache.get(context.Background(), endpoint, false)
	require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
}

func TestEnterpriseOIDCIDTokenRejectsInvalidSignaturesAndAlgorithms(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	document := enterpriseOIDCTestCachedDocument(t, &key.PublicKey)
	wrongKey := enterpriseOIDCTestRSAKey(t)
	_, err := ParseAndValidateOIDCIDToken(context.Background(), enterpriseOIDCTestSignedToken(t, wrongKey, enterpriseOIDCTestClaims()), document, "enterprise-client", "browser-bound-nonce")
	require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
	for _, method := range []jwt.SigningMethod{jwt.SigningMethodNone, jwt.SigningMethodHS256} {
		token := jwt.NewWithClaims(method, enterpriseOIDCTestClaims())
		token.Header["kid"] = "signing-key"
		var signingKey any = []byte("invalid-symmetric-test-key")
		if method == jwt.SigningMethodNone {
			signingKey = jwt.UnsafeAllowNoneSignatureType
		}
		raw, signErr := token.SignedString(signingKey)
		require.NoError(t, signErr)
		_, err = ParseAndValidateOIDCIDToken(context.Background(), raw, document, "enterprise-client", "browser-bound-nonce")
		require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
	}
}

func TestEnterpriseOIDCJWKSRejectsDuplicateAndIncompatibleKeys(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	base := enterpriseOIDCTestRSAJWK(&key.PublicKey)
	for _, tc := range []struct {
		name string
		keys []enterpriseJWK
	}{
		{"duplicate_ids", []enterpriseJWK{base, base}},
		{"encryption_operations", func() []enterpriseJWK { k := base; k.KeyOps = []string{"encrypt"}; return []enterpriseJWK{k} }()},
		{"advertised_algorithm_mismatch", func() []enterpriseJWK { k := base; k.Alg = "RS384"; return []enterpriseJWK{k} }()},
		{"too_many_keys", func() []enterpriseJWK { return make([]enterpriseJWK, 129) }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
				return enterpriseOIDCTestResponse(request, http.StatusOK, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: tc.keys})), nil
			}))
			document := &OIDCDiscoveryDocument{Issuer: "https://identity.example.com/tenant", JWKSURI: "https://identity.example.invalid/invalid-keys", IDTokenSigningAlgs: []string{"RS256"}}
			previous := defaultEnterpriseJWKSCache
			defaultEnterpriseJWKSCache = &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
			t.Cleanup(func() { defaultEnterpriseJWKSCache = previous })
			_, err := ParseAndValidateOIDCIDToken(ctx, enterpriseOIDCTestSignedToken(t, key, enterpriseOIDCTestClaims()), document, "enterprise-client", "browser-bound-nonce")
			require.Error(t, err)
		})
	}
}

func TestEnterpriseOIDCCodeExchangeEncodesBasicClientCredentials(t *testing.T) {
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		clientID, secret, ok := request.BasicAuth()
		if !ok || clientID != "client%3Aid" || secret != "secret%2B%3A+value" {
			return enterpriseOIDCTestResponse(request, http.StatusUnauthorized, `{"error":"invalid_client"}`), nil
		}
		return enterpriseOIDCTestResponse(request, http.StatusOK, `{"id_token":"test-id-token","access_token":"test-access-token","token_type":"Bearer"}`), nil
	}))
	_, err := ExchangeOIDCCode(ctx, &OIDCDiscoveryDocument{TokenEndpoint: "https://identity.example.invalid/token"}, "client:id", "secret+: value", "single-use-code", "https://app.example.invalid/callback", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	require.NoError(t, err)
}

func TestEnterpriseOIDCCodeExchangeSupportsExplicitPostAndPublicClientAuthentication(t *testing.T) {
	for _, tc := range []struct {
		method string
		secret string
	}{
		{"client_secret_post", "post-secret"},
		{"none", ""},
	} {
		t.Run(tc.method, func(t *testing.T) {
			ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
				if err := request.ParseForm(); err != nil {
					return nil, err
				}
				if request.Header.Get("Authorization") != "" || request.Form.Get("client_id") != "enterprise-client" || request.Form.Get("client_secret") != tc.secret || request.Form.Get("code_verifier") != "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk" {
					return enterpriseOIDCTestResponse(request, http.StatusUnauthorized, `{"error":"invalid_client"}`), nil
				}
				return enterpriseOIDCTestResponse(request, http.StatusOK, `{"id_token":"test-id-token","token_type":"Bearer"}`), nil
			}))
			_, err := ExchangeOIDCCode(ctx, &OIDCDiscoveryDocument{TokenEndpoint: "https://identity.example.invalid/token"}, "enterprise-client", tc.secret, "single-use-code", "https://app.example.invalid/callback", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", tc.method)
			require.NoError(t, err)
		})
	}
}

func TestEnterpriseOIDCCodeExchangeRejectsContradictoryClientAuthentication(t *testing.T) {
	for _, tc := range []struct {
		method string
		secret string
	}{
		{"client_secret_basic", ""}, {"client_secret_post", ""}, {"none", "configured-secret"}, {"private_key_jwt", "configured-secret"},
	} {
		_, err := ExchangeOIDCCode(context.Background(), &OIDCDiscoveryDocument{TokenEndpoint: "https://identity.example.invalid/token"}, "enterprise-client", tc.secret, "single-use-code", "https://app.example.invalid/callback", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", tc.method)
		require.ErrorIs(t, err, ErrEnterpriseIdentityInvalid)
	}
}

func TestEnterpriseOIDCUserinfoRejectsInvalidTokensAndNonObjectResponses(t *testing.T) {
	for _, token := range []string{"", "header\r\ninjection", strings.Repeat("a", 64<<10)} {
		ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
			return enterpriseOIDCTestResponse(request, http.StatusOK, `{"sub":"stable-subject"}`), nil
		}))
		_, err := FetchOIDCUserinfo(ctx, "https://identity.example.invalid/userinfo", token)
		require.Error(t, err)
	}
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		return enterpriseOIDCTestResponse(request, http.StatusOK, "null"), nil
	}))
	_, err := FetchOIDCUserinfo(ctx, "https://identity.example.invalid/userinfo", "test-access-token")
	require.Error(t, err)
}

func TestEnterpriseOIDCJWKSColdFailureUsesBackoff(t *testing.T) {
	calls := 0
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		return enterpriseOIDCTestResponse(request, http.StatusServiceUnavailable, `{}`), nil
	}))
	cache := &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}
	for i := 0; i < 5; i++ {
		_, err := cache.get(ctx, "https://identity.example.invalid/outage-keys", false)
		require.Error(t, err)
	}
	require.Equal(t, 1, calls)
}

func TestEnterpriseOIDCHTTPTransportRejectsPrivateDialTargets(t *testing.T) {
	transport := newEnterpriseOIDCHTTPTransport()
	defer transport.CloseIdleConnections()
	for _, address := range []string{"127.0.0.1:443", "169.254.169.254:443", "10.0.0.1:443", "[::1]:443"} {
		connection, err := transport.DialContext(context.Background(), "tcp", address)
		require.Nil(t, connection)
		require.ErrorContains(t, err, "not allowed")
	}
}
