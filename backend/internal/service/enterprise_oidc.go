package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/golang-jwt/jwt/v5"
)

const (
	enterpriseOIDCHTTPTimeout         = 10 * time.Second
	enterpriseOIDCMaxJSONBytes        = 512 << 10
	enterpriseOIDCMaxIDTokenBytes     = 64 << 10
	enterpriseOIDCMaxAccessTokenBytes = 32 << 10
	enterpriseOIDCMaxJWKSKeys         = 128
	enterpriseOIDCMaxJWKSCache        = 128
	enterpriseOIDCJWKSCacheTTL        = 10 * time.Minute
	enterpriseOIDCJWKSBackoff         = 30 * time.Second
)

type OIDCDiscoveryDocument struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint,omitempty"`
	ScopesSupported       []string `json:"scopes_supported,omitempty"`
	IDTokenSigningAlgs    []string `json:"id_token_signing_alg_values_supported,omitempty"`
}

type OIDCTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type OIDCClaims struct {
	Issuer          string
	Subject         string
	Audience        []string
	AuthorizedParty string
	Nonce           string
	Email           string
	EmailVerified   bool
	Name            string
	Groups          []string
	GroupsPresent   bool
	GroupsComplete  bool
	ExpiresAt       time.Time
	IssuedAt        time.Time
	Raw             map[string]any
}

type oidcHTTPClient struct {
	client *http.Client
}

// Only package-internal protocol tests can supply a transport. Endpoint
// validation and the redirect policy still run for every request.
type enterpriseOIDCHTTPTransportContextKey struct{}

func newEnterpriseOIDCHTTPTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:                  nil,
		ForceAttemptHTTP2:      true,
		TLSHandshakeTimeout:    5 * time.Second,
		ResponseHeaderTimeout:  5 * time.Second,
		MaxResponseHeaderBytes: 16 << 10,
		MaxIdleConns:           16,
		IdleConnTimeout:        30 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			pinnedCtx, err := urlvalidator.ResolveAndPinHost(ctx, host)
			if err != nil {
				return nil, err
			}
			return urlvalidator.DialContextWithPinnedIPs(pinnedCtx, network, address, dialer.DialContext)
		},
	}
}

var enterpriseOIDCHTTPTransport = newEnterpriseOIDCHTTPTransport()

func newEnterpriseOIDCHTTPClient(contexts ...context.Context) *oidcHTTPClient {
	var transport http.RoundTripper = enterpriseOIDCHTTPTransport
	if len(contexts) > 0 && contexts[0] != nil {
		if injected, ok := contexts[0].Value(enterpriseOIDCHTTPTransportContextKey{}).(http.RoundTripper); ok && injected != nil {
			transport = injected
		}
	}
	return &oidcHTTPClient{client: &http.Client{Transport: transport, Timeout: enterpriseOIDCHTTPTimeout, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func validateOIDCJSONEndpoint(raw string) (string, error) {
	validated, err := ValidateEnterpriseEndpoint(raw)
	if err != nil {
		return "", err
	}
	return validated, nil
}

func (c *oidcHTTPClient) getJSON(ctx context.Context, endpoint string, target any) error {
	validated, err := validateOIDCJSONEndpoint(endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validated, nil)
	if err != nil {
		return ErrEnterpriseIdentityInvalid
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("oidc endpoint returned status %d", resp.StatusCode)
	}
	if err := decodeEnterpriseOIDCJSON(resp.Body, target); err != nil {
		return fmt.Errorf("decode oidc response: %w", err)
	}
	return nil
}

func decodeEnterpriseOIDCJSON(reader io.Reader, target any) error {
	raw, err := io.ReadAll(io.LimitReader(reader, enterpriseOIDCMaxJSONBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > enterpriseOIDCMaxJSONBytes {
		return errors.New("oidc response exceeds size limit")
	}
	return json.Unmarshal(raw, target)
}

func DiscoverOIDC(ctx context.Context, issuer string) (*OIDCDiscoveryDocument, error) {
	issuerURL, err := validateEnterpriseOIDCIssuer(issuer)
	if err != nil {
		return nil, err
	}
	wellKnown := strings.TrimSuffix(issuerURL, "/") + "/.well-known/openid-configuration"
	var document OIDCDiscoveryDocument
	if err := newEnterpriseOIDCHTTPClient(ctx).getJSON(ctx, wellKnown, &document); err != nil {
		return nil, err
	}
	if document.Issuer != issuerURL {
		return nil, ErrEnterpriseIdentityInvalid
	}
	document.AuthorizationEndpoint, err = validateOIDCJSONEndpoint(document.AuthorizationEndpoint)
	if err != nil {
		return nil, err
	}
	document.TokenEndpoint, err = validateOIDCJSONEndpoint(document.TokenEndpoint)
	if err != nil {
		return nil, err
	}
	document.JWKSURI, err = validateOIDCJSONEndpoint(document.JWKSURI)
	if err != nil {
		return nil, err
	}
	if document.UserinfoEndpoint != "" {
		document.UserinfoEndpoint, err = validateOIDCJSONEndpoint(document.UserinfoEndpoint)
		if err != nil {
			return nil, err
		}
	}
	return &document, nil
}

func validateEnterpriseOIDCIssuer(raw string) (string, error) {
	issuer := strings.TrimSpace(raw)
	if _, err := validateOIDCJSONEndpoint(issuer); err != nil {
		return "", err
	}
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return "", ErrEnterpriseIdentityInvalid
	}
	return issuer, nil
}

func ExchangeOIDCCode(ctx context.Context, document *OIDCDiscoveryDocument, clientID, clientSecret, code, redirectURI, verifier string, authMethods ...string) (*OIDCTokenResponse, error) {
	if document == nil || strings.TrimSpace(clientID) == "" || strings.TrimSpace(code) == "" || strings.TrimSpace(verifier) == "" {
		return nil, ErrEnterpriseIdentityInvalid
	}
	validated, err := validateOIDCJSONEndpoint(document.TokenEndpoint)
	if err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", verifier)
	authMethod := "none"
	if clientSecret != "" {
		authMethod = "client_secret_basic"
	}
	if len(authMethods) > 1 {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if len(authMethods) == 1 && authMethods[0] != "" {
		authMethod = authMethods[0]
	}
	switch authMethod {
	case "client_secret_basic":
		if clientSecret == "" {
			return nil, ErrEnterpriseIdentityInvalid
		}
	case "client_secret_post":
		if clientSecret == "" {
			return nil, ErrEnterpriseIdentityInvalid
		}
		form.Set("client_secret", clientSecret)
	case "none":
		if clientSecret != "" {
			return nil, ErrEnterpriseIdentityInvalid
		}
	default:
		return nil, ErrEnterpriseIdentityInvalid
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, validated, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authMethod == "client_secret_basic" {
		// OAuth client authentication encodes each credential before joining
		// them with the Basic separator (RFC 6749 section 2.3.1).
		req.SetBasicAuth(url.QueryEscape(clientID), url.QueryEscape(clientSecret))
	}
	resp, err := newEnterpriseOIDCHTTPClient(ctx).client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc token endpoint returned status %d", resp.StatusCode)
	}
	var token OIDCTokenResponse
	if err := decodeEnterpriseOIDCJSON(resp.Body, &token); err != nil {
		return nil, err
	}
	if token.IDToken == "" || len(token.IDToken) > enterpriseOIDCMaxIDTokenBytes {
		return nil, ErrEnterpriseIdentityInvalid
	}
	return &token, nil
}

func FetchOIDCUserinfo(ctx context.Context, endpoint, accessToken string) (map[string]any, error) {
	if strings.TrimSpace(accessToken) == "" || len(accessToken) > enterpriseOIDCMaxAccessTokenBytes || strings.ContainsAny(accessToken, "\r\n") {
		return nil, ErrEnterpriseIdentityInvalid
	}
	validated, err := validateOIDCJSONEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, validated, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := newEnterpriseOIDCHTTPClient(ctx).client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oidc userinfo endpoint returned status %d", resp.StatusCode)
	}
	var claims map[string]any
	if err := decodeEnterpriseOIDCJSON(resp.Body, &claims); err != nil {
		return nil, err
	}
	if claims == nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	return claims, nil
}

type enterpriseJWKS struct {
	Keys []enterpriseJWK `json:"keys"`
}

type enterpriseJWK struct {
	KTY    string   `json:"kty"`
	Use    string   `json:"use"`
	Alg    string   `json:"alg"`
	Kid    string   `json:"kid"`
	KeyOps []string `json:"key_ops,omitempty"`
	N      string   `json:"n"`
	E      string   `json:"e"`
	Crv    string   `json:"crv"`
	X      string   `json:"x"`
	Y      string   `json:"y"`
}

type enterpriseJWKSCache struct {
	mu       sync.Mutex
	items    map[string]enterpriseJWKSCacheEntry
	inFlight map[string]*enterpriseJWKSFetch
}

type enterpriseJWKSCacheEntry struct {
	keys         map[string]any
	expiresAt    time.Time
	retryAfter   time.Time
	refreshAfter time.Time
	lastErr      error
}

type enterpriseJWKSFetch struct {
	done chan struct{}
	keys map[string]any
	err  error
}

type enterpriseOIDCSigningKey struct {
	publicKey any
	algorithm string
}

var defaultEnterpriseJWKSCache = &enterpriseJWKSCache{items: make(map[string]enterpriseJWKSCacheEntry)}

func (c *enterpriseJWKSCache) get(ctx context.Context, endpoint string, force bool) (map[string]any, error) {
	validated, err := validateOIDCJSONEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	endpoint = validated
	now := time.Now()
	c.mu.Lock()
	entry, ok := c.items[endpoint]
	validCachedKeys := ok && len(entry.keys) > 0 && now.Before(entry.expiresAt)
	if validCachedKeys && (!force || now.Before(entry.refreshAfter) || now.Before(entry.retryAfter)) {
		keys := entry.keys
		c.mu.Unlock()
		return keys, nil
	}
	if ok && now.Before(entry.retryAfter) {
		c.mu.Unlock()
		return nil, entry.lastErr
	}
	if pending, ok := c.inFlight[endpoint]; ok {
		c.mu.Unlock()
		select {
		case <-pending.done:
			return pending.keys, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if c.items == nil {
		c.items = make(map[string]enterpriseJWKSCacheEntry)
	}
	if c.inFlight == nil {
		c.inFlight = make(map[string]*enterpriseJWKSFetch)
	}
	if !ok && len(c.items) >= enterpriseOIDCMaxJWKSCache {
		oldestEndpoint := ""
		var oldestDeadline time.Time
		for candidate, cached := range c.items {
			if _, fetching := c.inFlight[candidate]; fetching {
				continue
			}
			if oldestEndpoint == "" || cached.expiresAt.Before(oldestDeadline) {
				oldestEndpoint, oldestDeadline = candidate, cached.expiresAt
			}
		}
		if oldestEndpoint == "" {
			c.mu.Unlock()
			return nil, errors.New("oidc jwks cache is busy")
		}
		delete(c.items, oldestEndpoint)
	}
	if force {
		entry.refreshAfter = now.Add(enterpriseOIDCJWKSBackoff)
	}
	c.items[endpoint] = entry
	pending := &enterpriseJWKSFetch{done: make(chan struct{})}
	c.inFlight[endpoint] = pending
	c.mu.Unlock()

	keys, fetchErr := fetchEnterpriseJWKS(ctx, validated)
	finishedAt := time.Now()
	c.mu.Lock()
	if fetchErr == nil {
		entry.keys, entry.expiresAt = keys, finishedAt.Add(enterpriseOIDCJWKSCacheTTL)
		entry.lastErr, entry.retryAfter = nil, time.Time{}
	} else {
		entry.lastErr, entry.retryAfter = fetchErr, finishedAt.Add(enterpriseOIDCJWKSBackoff)
		if len(entry.keys) > 0 && finishedAt.Before(entry.expiresAt) {
			keys, fetchErr = entry.keys, nil
		} else {
			keys = nil
		}
	}
	c.items[endpoint] = entry
	pending.keys, pending.err = keys, fetchErr
	delete(c.inFlight, endpoint)
	close(pending.done)
	c.mu.Unlock()
	return keys, fetchErr
}

func fetchEnterpriseJWKS(ctx context.Context, endpoint string) (map[string]any, error) {
	var document enterpriseJWKS
	if err := newEnterpriseOIDCHTTPClient(ctx).getJSON(ctx, endpoint, &document); err != nil {
		return nil, err
	}
	if len(document.Keys) > enterpriseOIDCMaxJWKSKeys {
		return nil, errors.New("oidc jwks exceeds key limit")
	}
	keys := make(map[string]any, len(document.Keys))
	for _, key := range document.Keys {
		if key.Kid == "" || len(key.Kid) > 512 {
			continue
		}
		publicKey, err := parseEnterpriseJWK(key)
		if err != nil {
			continue
		}
		if _, duplicate := keys[key.Kid]; duplicate {
			return nil, errors.New("oidc jwks contains duplicate signing key IDs")
		}
		keys[key.Kid] = enterpriseOIDCSigningKey{publicKey: publicKey, algorithm: key.Alg}
	}
	if len(keys) == 0 {
		return nil, errors.New("oidc jwks contains no supported keys")
	}
	return keys, nil
}

func parseEnterpriseJWK(key enterpriseJWK) (any, error) {
	if key.Use != "" && key.Use != "sig" {
		return nil, errors.New("oidc jwk is not a signature key")
	}
	if len(key.KeyOps) > 0 {
		canVerify := false
		for _, operation := range key.KeyOps {
			if operation != "verify" {
				return nil, errors.New("oidc jwk operations do not permit signature verification")
			}
			canVerify = true
		}
		if !canVerify {
			return nil, errors.New("oidc jwk does not permit signature verification")
		}
	}
	switch key.KTY {
	case "RSA":
		if key.Alg != "" && key.Alg != "RS256" && key.Alg != "RS384" && key.Alg != "RS512" {
			return nil, errors.New("oidc rsa key algorithm is unsupported")
		}
		n, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil || len(n) > 1024 {
			return nil, errors.New("invalid rsa modulus")
		}
		modulus := new(big.Int).SetBytes(n)
		if modulus.BitLen() < 2048 || modulus.BitLen() > 8192 || modulus.Bit(0) == 0 {
			return nil, errors.New("invalid rsa modulus")
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil || len(eBytes) == 0 || len(eBytes) > 4 {
			return nil, errors.New("invalid rsa exponent")
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		if e < 3 || e > 1<<31-1 || e%2 == 0 {
			return nil, errors.New("invalid rsa exponent")
		}
		return &rsa.PublicKey{N: modulus, E: e}, nil
	case "EC":
		var curve elliptic.Curve
		var algorithm string
		switch key.Crv {
		case "P-256":
			curve = elliptic.P256()
			algorithm = "ES256"
		case "P-384":
			curve = elliptic.P384()
			algorithm = "ES384"
		case "P-521":
			curve = elliptic.P521()
			algorithm = "ES512"
		default:
			return nil, errors.New("unsupported ec curve")
		}
		if key.Alg != "" && key.Alg != algorithm {
			return nil, errors.New("oidc ec key algorithm does not match its curve")
		}
		coordinateSize := (curve.Params().BitSize + 7) / 8
		x, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil || len(x) != coordinateSize {
			return nil, errors.New("invalid ec coordinate")
		}
		y, err := base64.RawURLEncoding.DecodeString(key.Y)
		if err != nil || len(y) != coordinateSize {
			return nil, errors.New("invalid ec coordinate")
		}
		point := make([]byte, 1+2*coordinateSize)
		point[0] = 4 // SEC 1 uncompressed point encoding.
		copy(point[1:], x)
		copy(point[1+coordinateSize:], y)
		publicKey, err := ecdsa.ParseUncompressedPublicKey(curve, point)
		if err != nil {
			return nil, errors.New("oidc ec key is not on its curve")
		}
		return publicKey, nil
	default:
		return nil, errors.New("unsupported jwk type")
	}
}

var errEnterpriseOIDCKeyNotFound = errors.New("oidc token key not found")

func enterpriseOIDCSigningAlgorithms(advertised []string) []string {
	supported := []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}
	if len(advertised) == 0 {
		return supported
	}
	allowed := make([]string, 0, len(supported))
	for _, algorithm := range supported {
		for _, candidate := range advertised {
			if candidate == algorithm {
				allowed = append(allowed, algorithm)
				break
			}
		}
	}
	return allowed
}

func enterpriseOIDCKeyForToken(token *jwt.Token, keys map[string]any) (any, error) {
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" || len(kid) > 512 {
		return nil, errors.New("oidc token kid is required")
	}
	key, ok := keys[kid]
	if !ok {
		return nil, errEnterpriseOIDCKeyNotFound
	}
	if signingKey, ok := key.(enterpriseOIDCSigningKey); ok {
		if signingKey.algorithm != "" && signingKey.algorithm != token.Method.Alg() {
			return nil, errors.New("oidc token algorithm does not match its key")
		}
		key = signingKey.publicKey
	}
	switch publicKey := key.(type) {
	case *rsa.PublicKey:
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("oidc token algorithm does not match its key type")
		}
	case *ecdsa.PublicKey:
		method, ok := token.Method.(*jwt.SigningMethodECDSA)
		if !ok || method.CurveBits != publicKey.Curve.Params().BitSize {
			return nil, errors.New("oidc token algorithm does not match its key curve")
		}
	default:
		return nil, errors.New("oidc token signing key is unsupported")
	}
	return key, nil
}

func ParseAndValidateOIDCIDToken(ctx context.Context, rawToken string, document *OIDCDiscoveryDocument, clientID, nonce string) (*OIDCClaims, error) {
	if document == nil || strings.TrimSpace(rawToken) == "" || len(rawToken) > enterpriseOIDCMaxIDTokenBytes || strings.TrimSpace(clientID) == "" || strings.TrimSpace(nonce) == "" {
		return nil, ErrEnterpriseIdentityInvalid
	}
	algorithms := enterpriseOIDCSigningAlgorithms(document.IDTokenSigningAlgs)
	issuer, issuerErr := validateEnterpriseOIDCIssuer(document.Issuer)
	if len(algorithms) == 0 || issuerErr != nil || issuer != document.Issuer {
		return nil, ErrEnterpriseIdentityInvalid
	}
	parser := jwt.NewParser(jwt.WithValidMethods(algorithms), jwt.WithIssuer(document.Issuer), jwt.WithAudience(clientID), jwt.WithExpirationRequired(), jwt.WithIssuedAt(), jwt.WithJSONNumber(), jwt.WithLeeway(30*time.Second))
	var unverifiedClaims jwt.MapClaims
	unverified, _, err := parser.ParseUnverified(rawToken, &unverifiedClaims)
	if err != nil || unverified == nil {
		return nil, ErrEnterpriseIdentityInvalid
	}
	allowedAlgorithm := false
	for _, algorithm := range algorithms {
		if unverified.Method.Alg() == algorithm {
			allowedAlgorithm = true
			break
		}
	}
	kid, _ := unverified.Header["kid"].(string)
	if !allowedAlgorithm || kid == "" || len(kid) > 512 {
		return nil, ErrEnterpriseIdentityInvalid
	}
	keys, err := defaultEnterpriseJWKSCache.get(ctx, document.JWKSURI, false)
	if err != nil {
		return nil, err
	}
	var claims jwt.MapClaims
	token, err := parser.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
		return enterpriseOIDCKeyForToken(token, keys)
	})
	if errors.Is(err, errEnterpriseOIDCKeyNotFound) {
		keys, refreshErr := defaultEnterpriseJWKSCache.get(ctx, document.JWKSURI, true)
		if refreshErr == nil {
			token, err = parser.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (any, error) {
				return enterpriseOIDCKeyForToken(token, keys)
			})
		}
	}
	if err != nil || token == nil || !token.Valid {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if claimsNonce, _ := claims["nonce"].(string); claimsNonce == "" || !OIDCNonceMatchesHash(claimsNonce, HashEnterpriseToken(nonce)) {
		return nil, ErrEnterpriseIdentityInvalid
	}
	result, err := oidcClaimsFromMap(claims)
	if err != nil || result.IssuedAt.IsZero() || !result.ExpiresAt.After(result.IssuedAt) {
		return nil, ErrEnterpriseIdentityInvalid
	}
	if (len(result.Audience) > 1 && result.AuthorizedParty == "") || (result.AuthorizedParty != "" && result.AuthorizedParty != clientID) {
		return nil, ErrEnterpriseIdentityInvalid
	}
	return result, nil
}

func oidcClaimsFromMap(claims jwt.MapClaims) (*OIDCClaims, error) {
	issuer, _ := claims["iss"].(string)
	subject, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	if issuer == "" || strings.TrimSpace(subject) == "" || len(subject) > 1024 {
		return nil, ErrEnterpriseIdentityInvalid
	}
	verified := false
	if value, ok := claims["email_verified"].(bool); ok {
		verified = value
	}
	result := &OIDCClaims{Issuer: issuer, Subject: subject, Email: email, EmailVerified: verified, Name: name, Raw: map[string]any(claims)}
	if value, ok := claims["nonce"].(string); ok {
		result.Nonce = value
	}
	if value, ok := claims["aud"].(string); ok {
		result.Audience = []string{value}
	}
	if value, ok := claims["aud"].([]any); ok {
		for _, v := range value {
			if s, ok := v.(string); ok {
				result.Audience = append(result.Audience, s)
			}
		}
	}
	if value, ok := claims["azp"].(string); ok {
		result.AuthorizedParty = value
	}
	if value, present := claims["azp"]; present {
		if authorizedParty, ok := value.(string); !ok || authorizedParty == "" {
			return nil, ErrEnterpriseIdentityInvalid
		}
	}
	if value, err := claims.GetExpirationTime(); err == nil && value != nil {
		result.ExpiresAt = value.Time
	}
	if value, err := claims.GetIssuedAt(); err == nil && value != nil {
		result.IssuedAt = value.Time
	}
	result.Groups = oidcStringClaim(claims["groups"])
	return result, nil
}

func oidcStringClaim(value any) []string {
	if single, ok := value.(string); ok && single != "" {
		return []string{single}
	}
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if item, ok := value.(string); ok && item != "" {
			result = append(result, item)
		}
	}
	return result
}

func OIDCPKCEChallenge(verifier string) string {
	return base64.RawURLEncoding.EncodeToString(sha256Bytes([]byte(verifier)))
}

func sha256Bytes(value []byte) []byte {
	sum := sha256.Sum256(value)
	return sum[:]
}

func OIDCNonceMatchesHash(nonce string, expected []byte) bool {
	actual := sha256Bytes([]byte(nonce))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
