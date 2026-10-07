package service

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/russellhaering/gosaml2/types"
	"github.com/stretchr/testify/require"
)

// Only the test IdP constructs encrypted fixture bytes. Production decrypts
// and verifies them entirely through gosaml2 and goxmldsig.
func samlEncryptFixtureAssertion(t *testing.T, encoded, certificate string) string {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromBytes(raw))
	root := doc.Root()
	assertion := root.FindElement("./Assertion")
	require.NotNil(t, assertion)
	assertionDoc := etree.NewDocument()
	assertionDoc.SetRoot(assertion.Copy())
	plain, err := assertionDoc.WriteToBytes()
	require.NoError(t, err)
	block, _ := pem.Decode([]byte(certificate))
	require.NotNil(t, block)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	aesKey := make([]byte, 32)
	_, err = rand.Read(aesKey)
	require.NoError(t, err)
	aesBlock, err := aes.NewCipher(aesKey)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(aesBlock)
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	require.NoError(t, err)
	ciphertext := gcm.Seal(append([]byte(nil), nonce...), nonce, plain, nil)
	publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	require.True(t, ok)
	encryptedKey, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, publicKey, aesKey, nil)
	require.NoError(t, err)
	root.RemoveChild(assertion)
	encrypted := root.CreateElement("saml:EncryptedAssertion")
	encrypted.CreateAttr("xmlns:xenc", "http://www.w3.org/2001/04/xmlenc#")
	encrypted.CreateAttr("xmlns:ds", "http://www.w3.org/2000/09/xmldsig#")
	data := encrypted.CreateElement("xenc:EncryptedData")
	data.CreateAttr("Type", "http://www.w3.org/2001/04/xmlenc#Element")
	data.CreateElement("xenc:EncryptionMethod").CreateAttr("Algorithm", types.MethodAES256GCM)
	key := data.CreateElement("ds:KeyInfo").CreateElement("xenc:EncryptedKey")
	key.CreateElement("xenc:EncryptionMethod").CreateAttr("Algorithm", types.MethodRSAOAEP)
	key.CreateElement("xenc:CipherData").CreateElement("xenc:CipherValue").SetText(base64.StdEncoding.EncodeToString(encryptedKey))
	data.CreateElement("xenc:CipherData").CreateElement("xenc:CipherValue").SetText(base64.StdEncoding.EncodeToString(ciphertext))
	raw, err = doc.WriteToBytes()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestSAMLEncryptedAssertionsSupportCurrentNextAndPreviousDecryptionKeys(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	signed := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, nil, "assertion")
	encrypted := samlEncryptFixtureAssertion(t, signed, provider.SAML.SPCertificate)
	sp, _, err := identity.samlServiceProvider(provider, secret, redirect, 0)
	require.NoError(t, err)
	_, err = sp.ValidateEncodedResponse(encrypted)
	require.NoError(t, err, "test IdP encryption must produce a library-valid fixture")
	verified, err := identity.validateSAMLResponse(provider, secret, encrypted, "_request", redirect)
	require.NoError(t, err)
	require.Equal(t, "stable-subject", verified.Claims.Subject)
	ring, err := identity.decryptSAMLKeyRing(secret)
	require.NoError(t, err)
	ring.NextPrivateKey, provider.SAML.NextSPCertificate, err = newSAMLSPKey(now)
	require.NoError(t, err)
	secret, err = identity.encryptSAMLKeyRing(*ring)
	require.NoError(t, err)
	nextEncrypted := samlEncryptFixtureAssertion(t, signed, provider.SAML.NextSPCertificate)
	_, err = identity.validateSAMLResponse(provider, secret, nextEncrypted, "_request", redirect)
	require.NoError(t, err)
	oldEncrypted := samlEncryptFixtureAssertion(t, signed, provider.SAML.SPCertificate)
	ring.PreviousPrivateKey, ring.PreviousCertificate, ring.PreviousValidUntil = ring.CurrentPrivateKey, provider.SAML.SPCertificate, now.Add(15*time.Minute)
	ring.CurrentPrivateKey, ring.NextPrivateKey = ring.NextPrivateKey, ""
	provider.SAML.SPCertificate, provider.SAML.NextSPCertificate = provider.SAML.NextSPCertificate, ""
	secret, err = identity.encryptSAMLKeyRing(*ring)
	require.NoError(t, err)
	_, err = identity.validateSAMLResponse(provider, secret, oldEncrypted, "_request", redirect)
	require.NoError(t, err)
	identity.SetClock(func() time.Time { return now.Add(16 * time.Minute) })
	_, err = identity.validateSAMLResponse(provider, secret, oldEncrypted, "_request", redirect)
	require.Error(t, err)
	identity.SetClock(time.Now)
	wrongKey, wrongCert, err := newSAMLSPKey(now)
	require.NoError(t, err)
	require.NotEmpty(t, wrongKey)
	_, err = identity.validateSAMLResponse(provider, secret, samlEncryptFixtureAssertion(t, signed, wrongCert), "_request", redirect)
	require.Error(t, err)
	unsigned := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, nil, "none")
	_, err = identity.validateSAMLResponse(provider, secret, samlEncryptFixtureAssertion(t, unsigned, provider.SAML.SPCertificate), "_request", redirect)
	require.Error(t, err)
}

type samlFlowRepository struct {
	enterpriseFlowRepo
	secret        string
	replays       map[string]bool
	lastProvision EnterpriseProvisionInput
}

func (r *samlFlowRepository) GetProvider(context.Context, int64, int64, int64) (*EnterpriseIdentityProvider, string, error) {
	return &r.provider, r.secret, nil
}
func (r *samlFlowRepository) GetSAMLProviderByPublicID(context.Context, string) (*EnterpriseIdentityProvider, string, error) {
	return &r.provider, r.secret, nil
}
func (r *samlFlowRepository) UseSAMLResponse(_ context.Context, workspace, provider int64, response, assertion string, _ time.Time) error {
	if workspace != r.provider.WorkspaceID || provider != r.provider.ID {
		return ErrWorkspaceNotFound
	}
	if r.replays[response] || r.replays[assertion] {
		return ErrSAMLReplayDetected
	}
	r.replays[response], r.replays[assertion] = true, true
	return nil
}
func (r *samlFlowRepository) CompleteIdentityLogin(ctx context.Context, input OIDCProvisionInput) (int64, error) {
	if input.Protocol != "saml" || input.Claims.RawProtocol != "saml" || !input.Claims.EmailTrusted {
		return 0, ErrSAMLResponseInvalid
	}
	r.lastProvision = input
	return r.enterpriseFlowRepo.CompleteIdentityLogin(ctx, input)
}

type samlFlowAccessRepository struct{ WorkspaceRepository }

func (samlFlowAccessRepository) GetAccess(context.Context, int64, int64, int64) (*WorkspaceAccess, error) {
	return &WorkspaceAccess{Workspace: &Workspace{ID: 7, Type: WorkspaceTypeOrganization, Status: StatusActive}, Member: &WorkspaceMember{ID: 1, WorkspaceID: 7, UserID: 42, Role: WorkspaceRoleDeveloper, Status: StatusActive}}, nil
}

func TestSAMLMockIdPFlowBindsSignedRequestBrowserStateReplayAndProvisioning(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	repo := &samlFlowRepository{enterpriseFlowRepo: enterpriseFlowRepo{provider: *provider}, secret: secret, replays: map[string]bool{}}
	identity.repo, identity.users, identity.access = repo, enterpriseFlowUsers{}, NewWorkspaceAccessService(samlFlowAccessRepository{})
	start, err := identity.StartEnterpriseSSO(context.Background(), 7, 9, "/workspaces/7/identity", redirect, nil)
	require.NoError(t, err)
	authorize, err := url.Parse(start.AuthorizationURL)
	require.NoError(t, err)
	require.Equal(t, "saml", start.Protocol)
	require.Equal(t, repo.state.OpaqueState(), authorize.Query().Get("RelayState"))
	require.NotEmpty(t, repo.state.RequestID)
	require.EqualValues(t, 3, repo.state.ProviderRevision)
	requestBytes, err := base64.StdEncoding.DecodeString(authorize.Query().Get("SAMLRequest"))
	require.NoError(t, err)
	reader := flate.NewReader(bytes.NewReader(requestBytes))
	requestXML, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.NoError(t, reader.Close())
	require.Contains(t, string(requestXML), repo.state.RequestID)
	require.Contains(t, string(requestXML), "AssertionConsumerServiceURL=")
	ring, err := identity.decryptSAMLKeyRing(secret)
	require.NoError(t, err)
	store, err := samlKeyStore(ring.CurrentPrivateKey, provider.SAML.SPCertificate)
	require.NoError(t, err)
	requestSignature, err := base64.StdEncoding.DecodeString(authorize.Query().Get("Signature"))
	require.NoError(t, err)
	signedQuery := "SAMLRequest=" + url.QueryEscape(authorize.Query().Get("SAMLRequest")) + "&RelayState=" + url.QueryEscape(authorize.Query().Get("RelayState")) + "&SigAlg=" + url.QueryEscape(authorize.Query().Get("SigAlg"))
	digest := sha256.Sum256([]byte(signedQuery))
	privateKey, ok := store.Signer.(*rsa.PrivateKey)
	require.True(t, ok)
	require.NoError(t, rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, digest[:], requestSignature))
	response := samlResponseFixture(t, provider, redirect, repo.state.RequestID, now, idpKey, nil, "assertion")
	_, err = identity.CompleteSAML(context.Background(), repo.state.OpaqueState(), strings.Repeat("b", 43), response, redirect)
	require.ErrorIs(t, err, ErrOIDCStateSessionMismatch)
	require.False(t, repo.consumed)
	result, err := identity.CompleteSAML(context.Background(), repo.state.OpaqueState(), start.BrowserCookie, response, redirect)
	require.NoError(t, err)
	require.EqualValues(t, 42, result.User.ID)
	require.Equal(t, "saml", result.Assurance.AuthMethod)
	require.Equal(t, now, result.Assurance.AuthenticatedAt)
	require.Equal(t, "/workspaces/7/identity", result.ReturnTo)
	require.Equal(t, 1, repo.provisioned)
	require.NotEmpty(t, repo.lastProvision.PasswordHash)
	_, err = identity.CompleteSAML(context.Background(), repo.state.OpaqueState(), start.BrowserCookie, response, redirect)
	require.ErrorIs(t, err, ErrOIDCStateConsumed)
	next, err := identity.StartEnterpriseSSO(context.Background(), 7, 9, "/workspaces/7/identity", redirect, nil)
	require.NoError(t, err)
	repo.consumed = false
	repeated := samlResponseFixture(t, provider, redirect, repo.state.RequestID, now, idpKey, nil, "assertion")
	_, err = identity.CompleteSAML(context.Background(), repo.state.OpaqueState(), next.BrowserCookie, repeated, redirect)
	require.ErrorIs(t, err, ErrSAMLReplayDetected)
	require.Equal(t, 1, repo.provisioned)
	_, err = identity.StartEnterpriseSSO(context.Background(), 7, 9, "//evil.example", redirect, nil)
	require.Error(t, err)
	_, err = identity.StartEnterpriseSSO(context.Background(), 7, 9, "/workspaces/7", strings.Replace(redirect, "https:", "http:", 1), nil)
	require.Error(t, err)
}

func TestSAMLSignedRedirectNameIDPolicyMatchesConfiguredSubject(t *testing.T) {
	identity, fixture, secret, redirect, _, _ := samlProtocolFixture(t)
	for _, tc := range []struct {
		name             string
		allowUnspecified bool
		subjectAttribute string
		requestFormat    string
		metadataFormats  []string
	}{
		{"persistent default", false, "", samlPersistentNameID, []string{samlPersistentNameID}},
		{"approved unspecified", true, "", samlUnspecifiedNameID, []string{samlPersistentNameID, samlUnspecifiedNameID}},
		{"stable attribute", false, "employeeId", "", nil},
		{"stable attribute overrides unspecified", true, "employeeId", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			provider := *fixture
			config := *fixture.SAML
			config.AllowUnspecifiedNameID, config.SubjectAttribute = tc.allowUnspecified, tc.subjectAttribute
			provider.SAML = &config
			repo := &samlFlowRepository{enterpriseFlowRepo: enterpriseFlowRepo{provider: provider}, secret: secret, replays: map[string]bool{}}
			identity.repo = repo
			start, err := identity.StartEnterpriseSSO(context.Background(), 7, 9, "/workspaces/7", redirect, nil)
			require.NoError(t, err)
			authorize, err := url.Parse(start.AuthorizationURL)
			require.NoError(t, err)
			query := authorize.Query()
			require.Equal(t, "http://www.w3.org/2001/04/xmldsig-more#rsa-sha256", query.Get("SigAlg"))
			signature, err := base64.StdEncoding.DecodeString(query.Get("Signature"))
			require.NoError(t, err)
			signedQuery := "SAMLRequest=" + url.QueryEscape(query.Get("SAMLRequest")) + "&RelayState=" + url.QueryEscape(query.Get("RelayState")) + "&SigAlg=" + url.QueryEscape(query.Get("SigAlg"))
			block, _ := pem.Decode([]byte(provider.SAML.SPCertificate))
			require.NotNil(t, block)
			certificate, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)
			require.NoError(t, certificate.CheckSignature(x509.SHA256WithRSA, []byte(signedQuery), signature), "the actual Redirect request must remain signed with its advertised SP certificate")
			compressed, err := base64.StdEncoding.DecodeString(query.Get("SAMLRequest"))
			require.NoError(t, err)
			reader := flate.NewReader(bytes.NewReader(compressed))
			raw, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.NoError(t, reader.Close())
			document := etree.NewDocument()
			require.NoError(t, document.ReadFromBytes(raw))
			require.Equal(t, repo.state.RequestID, document.Root().SelectAttrValue("ID", ""))
			policy := document.Root().FindElement("./NameIDPolicy")
			require.NotNil(t, policy)
			require.Equal(t, tc.requestFormat, policy.SelectAttrValue("Format", ""))
			if tc.subjectAttribute != "" {
				require.Nil(t, policy.SelectAttr("Format"), "stable subject attributes must not constrain NameID format")
			}
			metadataXML, err := identity.SAMLMetadata(context.Background(), provider.PublicID, redirect)
			require.NoError(t, err)
			var metadata types.EntityDescriptor
			require.NoError(t, xml.Unmarshal(metadataXML, &metadata))
			require.NotNil(t, metadata.SPSSODescriptor)
			require.True(t, metadata.SPSSODescriptor.AuthnRequestsSigned)
			require.Equal(t, tc.metadataFormats, metadata.SPSSODescriptor.NameIDFormats)
		})
	}
}

func TestSAMLApprovedUnspecifiedNameIDRetainsStableSubjectRequirement(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	provider.SAML.AllowUnspecifiedNameID = true
	for _, tc := range []struct {
		format   string
		accepted bool
	}{
		{samlPersistentNameID, true},
		{samlUnspecifiedNameID, true},
		{"urn:oasis:names:tc:SAML:2.0:nameid-format:transient", false},
		{"urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress", false},
	} {
		t.Run(tc.format, func(t *testing.T) {
			encoded := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, func(root *etree.Element) {
				root.FindElement("./Assertion/Subject/NameID").CreateAttr("Format", tc.format)
			}, "assertion")
			verified, err := identity.validateSAMLResponse(provider, secret, encoded, "_request", redirect)
			if tc.accepted {
				require.NoError(t, err)
				require.Equal(t, "stable-subject", verified.Claims.Subject)
			} else {
				require.ErrorIs(t, err, ErrSAMLSubjectUnstable)
			}
		})
	}
}

func TestSAMLMockIdPRejectsExpiredRevisionDisabledAndProtocolChangedState(t *testing.T) {
	for _, change := range []string{"expired", "revision", "disabled", "protocol"} {
		t.Run(change, func(t *testing.T) {
			identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
			repo := &samlFlowRepository{enterpriseFlowRepo: enterpriseFlowRepo{provider: *provider}, secret: secret, replays: map[string]bool{}}
			identity.repo, identity.users = repo, enterpriseFlowUsers{}
			start, err := identity.StartEnterpriseSSO(context.Background(), 7, 9, "/workspaces/7", redirect, nil)
			require.NoError(t, err)
			encoded := samlResponseFixture(t, provider, redirect, repo.state.RequestID, now, idpKey, nil, "response")
			switch change {
			case "expired":
				repo.state.ExpiresAt = now.Add(-time.Second)
			case "revision":
				repo.provider.Revision++
			case "disabled":
				repo.provider.Status = "disabled"
			case "protocol":
				repo.state.Protocol = "oidc"
			}
			_, err = identity.CompleteSAML(context.Background(), repo.state.OpaqueState(), start.BrowserCookie, encoded, redirect)
			require.Error(t, err)
			require.Zero(t, repo.provisioned)
		})
	}
}

func TestSAMLMetadataFetchRejectsPrivateRedirectsAndBoundedRemoteBodies(t *testing.T) {
	for _, endpoint := range []string{"http://idp.example/metadata", "file:///metadata", "https://localhost/metadata", "https://10.0.0.1/metadata", "https://169.254.169.254/metadata", "https://[::1]/metadata"} {
		_, err := fetchSAMLMetadata(context.Background(), endpoint)
		require.ErrorIs(t, err, ErrSAMLMetadataInvalid)
	}
	calls := 0
	ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		calls++
		res := enterpriseOIDCTestResponse(request, http.StatusFound, "")
		res.Header.Set("Location", "https://127.0.0.1/private")
		return res, nil
	}))
	_, err := fetchSAMLMetadata(ctx, "https://idp.example.invalid/metadata")
	require.Error(t, err)
	require.Equal(t, 1, calls)
	ctx = enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
		return enterpriseOIDCTestResponse(request, http.StatusOK, strings.Repeat("x", samlMetadataMaxBytes+1)), nil
	}))
	_, err = fetchSAMLMetadata(ctx, "https://idp.example.invalid/metadata")
	require.Error(t, err)
	// DNS answers are checked and the resolved address is the only dial target;
	// the shared transport rejects private answers even for direct dial calls.
	transport := newEnterpriseOIDCHTTPTransport()
	defer transport.CloseIdleConnections()
	require.Nil(t, transport.Proxy)
	conn, err := transport.DialContext(context.Background(), "tcp", "127.0.0.1:443")
	require.Nil(t, conn)
	require.ErrorContains(t, err, "not allowed")
}
