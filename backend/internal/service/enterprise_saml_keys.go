package service

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"encoding/xml"
	"math/big"
	"net/url"
	"regexp"
	"strings"
	"time"

	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"
	dsigtypes "github.com/russellhaering/goxmldsig/types"
)

type samlSPKeyRing struct {
	CurrentPrivateKey   string    `json:"current_private_key"`
	NextPrivateKey      string    `json:"next_private_key,omitempty"`
	PreviousPrivateKey  string    `json:"previous_private_key,omitempty"`
	PreviousCertificate string    `json:"previous_certificate,omitempty"`
	PreviousValidUntil  time.Time `json:"previous_valid_until,omitempty"`
}

type SAMLSPInfo struct {
	EntityID               string `json:"entity_id"`
	ACSURL                 string `json:"acs_url"`
	MetadataURL            string `json:"metadata_url"`
	SigningCertificate     string `json:"signing_certificate,omitempty"`
	NextSigningCertificate string `json:"next_signing_certificate,omitempty"`
	IdPInitiatedSupported  bool   `json:"idp_initiated_supported"`
	SLOSupported           bool   `json:"slo_supported"`
}

func SAMLPublicEndpoints(publicID, redirectURI string) (*SAMLSPInfo, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(publicID) {
		return nil, ErrSAMLProviderInvalid
	}
	parsed, err := url.Parse(redirectURI)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, ErrSAMLProviderInvalid
	}
	origin := parsed.Scheme + "://" + parsed.Host
	metadata := origin + "/api/v1/auth/sso/saml/metadata/" + publicID
	return &SAMLSPInfo{EntityID: metadata, MetadataURL: metadata, ACSURL: origin + "/api/v1/auth/sso/saml/acs"}, nil
}

func newSAMLSPKey(now time.Time) (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return "", "", ErrSAMLProviderInvalid
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return "", "", ErrSAMLProviderInvalid
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "ModuRelay SAML Service Provider"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.AddDate(2, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, BasicConstraintsValid: true}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", ErrSAMLProviderInvalid
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", "", ErrSAMLProviderInvalid
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})), string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate})), nil
}

func (s *EnterpriseIdentityService) encryptSAMLKeyRing(ring samlSPKeyRing) (string, error) {
	if s.encryptor == nil || ring.CurrentPrivateKey == "" {
		return "", ErrSAMLProviderInvalid
	}
	raw, err := json.Marshal(ring)
	if err != nil {
		return "", ErrSAMLProviderInvalid
	}
	encrypted, err := s.encryptor.Encrypt(string(raw))
	if err != nil || encrypted == "" {
		return "", ErrSAMLProviderInvalid
	}
	return encrypted, nil
}

func (s *EnterpriseIdentityService) decryptSAMLKeyRing(ciphertext string) (*samlSPKeyRing, error) {
	if s.encryptor == nil || ciphertext == "" {
		return nil, ErrSAMLProviderInvalid
	}
	plain, err := s.encryptor.Decrypt(ciphertext)
	if err != nil || len(plain) > 65536 {
		return nil, ErrSAMLProviderInvalid
	}
	var ring samlSPKeyRing
	if json.Unmarshal([]byte(plain), &ring) != nil || ring.CurrentPrivateKey == "" {
		return nil, ErrSAMLProviderInvalid
	}
	return &ring, nil
}

func samlKeyStore(privateKey, certificate string) (*saml2.KeyStore, error) {
	block, rest := pem.Decode([]byte(privateKey))
	if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
		return nil, ErrSAMLProviderInvalid
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrSAMLProviderInvalid
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < 2048 {
		return nil, ErrSAMLProviderInvalid
	}
	cert, _, err := parseSAMLSigningCertificate(certificate, time.Now())
	if err != nil {
		return nil, err
	}
	public, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok || public.N.Cmp(key.N) != 0 || public.E != key.E {
		return nil, ErrSAMLProviderInvalid
	}
	return &saml2.KeyStore{Signer: key, Cert: cert.Raw}, nil
}

func (s *EnterpriseIdentityService) samlServiceProvider(provider *EnterpriseIdentityProvider, ciphertext, redirectURI string, keyIndex int) (*saml2.SAMLServiceProvider, *SAMLSPInfo, error) {
	if validateSAMLConfiguration(provider) != nil {
		return nil, nil, ErrSAMLProviderInvalid
	}
	info, err := SAMLPublicEndpoints(provider.PublicID, redirectURI)
	if err != nil {
		return nil, nil, err
	}
	ring, err := s.decryptSAMLKeyRing(ciphertext)
	if err != nil {
		return nil, nil, err
	}
	private, certificate := ring.CurrentPrivateKey, provider.SAML.SPCertificate
	switch keyIndex {
	case 1:
		private, certificate = ring.NextPrivateKey, provider.SAML.NextSPCertificate
	case 2:
		if !s.now().Before(ring.PreviousValidUntil) {
			return nil, nil, ErrSAMLProviderInvalid
		}
		private, certificate = ring.PreviousPrivateKey, ring.PreviousCertificate
	}
	key, err := samlKeyStore(private, certificate)
	if err != nil {
		return nil, nil, err
	}
	certs := make([]*x509.Certificate, 0, len(provider.SAML.SigningCertificates))
	for _, raw := range provider.SAML.SigningCertificates {
		cert, _, err := parseSAMLSigningCertificate(raw, s.now())
		if err != nil {
			return nil, nil, err
		}
		certs = append(certs, cert)
	}
	sp := &saml2.SAMLServiceProvider{
		IdentityProviderSSOURL: provider.SAML.SSOURL, IdentityProviderSSOBinding: samlRedirectBinding, IdentityProviderIssuer: provider.SAML.IDPEntityID,
		AssertionConsumerServiceURL: info.ACSURL, ServiceProviderIssuer: info.EntityID, AudienceURI: info.EntityID,
		SignAuthnRequests: true, SignAuthnRequestsAlgorithm: dsig.RSASHA256SignatureMethod, IDPCertificateStore: &dsig.MemoryX509CertificateStore{Roots: certs}, NameIdFormat: samlPersistentNameID,
		ValidateEncryptionCert: true, MaximumDecompressedBodySize: samlResponseMaxBytes, MaximumXMLTokens: 16000, Clock: dsig.NewRealClock(),
	}
	if provider.SAML.SubjectAttribute != "" {
		sp.NameIdFormat = ""
	} else if provider.SAML.AllowUnspecifiedNameID {
		sp.NameIdFormat = samlUnspecifiedNameID
	}
	if err = sp.SetSPKeyStore(key); err != nil {
		return nil, nil, ErrSAMLProviderInvalid
	}
	// gosaml2 v0.12.0 getDecryptCert still reads the legacy field rather
	// than the SetSPKeyStore override. Supply the same retained key through
	// both interfaces; signature, metadata and decryption stay in the library.
	sp.SPKeyStore = dsig.TLSCertKeyStore(tls.Certificate{Certificate: [][]byte{key.Cert}, PrivateKey: key.Signer}) //nolint:staticcheck // Required by gosaml2's decryption path.
	info.SigningCertificate, info.NextSigningCertificate = provider.SAML.SPCertificate, provider.SAML.NextSPCertificate
	return sp, info, nil
}

func (s *EnterpriseIdentityService) SAMLSPInformation(ctx context.Context, actorID, workspaceID, providerID int64, redirectURI string) (*SAMLSPInfo, error) {
	provider, err := s.GetProvider(ctx, actorID, workspaceID, providerID)
	if err != nil {
		return nil, err
	}
	if provider.Type != "saml" || provider.SAML == nil {
		return nil, ErrSAMLProviderInvalid
	}
	info, err := SAMLPublicEndpoints(provider.PublicID, redirectURI)
	if err != nil {
		return nil, err
	}
	info.SigningCertificate, info.NextSigningCertificate = provider.SAML.SPCertificate, provider.SAML.NextSPCertificate
	return info, nil
}

func (s *EnterpriseIdentityService) SAMLMetadata(ctx context.Context, publicID, redirectURI string) ([]byte, error) {
	repo, ok := s.repo.(SAMLReplayRepository)
	if !ok {
		return nil, ErrSAMLProviderInvalid
	}
	provider, secret, err := repo.GetSAMLProviderByPublicID(ctx, publicID)
	if err != nil {
		return nil, err
	}
	sp, _, err := s.samlServiceProvider(provider, secret, redirectURI, 0)
	if err != nil {
		return nil, err
	}
	metadata, err := sp.Metadata()
	if err != nil {
		return nil, ErrSAMLProviderInvalid
	}
	// gosaml2 leaves NameIDFormats unset. Describe the same formats our subject
	// validation accepts; stable attribute mode deliberately makes no NameID
	// format commitment because identity comes from the configured attribute.
	if provider.SAML.SubjectAttribute == "" {
		metadata.SPSSODescriptor.NameIDFormats = []string{samlPersistentNameID}
		if provider.SAML.AllowUnspecifiedNameID {
			metadata.SPSSODescriptor.NameIDFormats = append(metadata.SPSSODescriptor.NameIDFormats, samlUnspecifiedNameID)
		}
	}
	// During staging publish the next certificate for both usages. The private
	// counterpart stays in the encrypted key ring and is never serialized here.
	if provider.SAML.NextSPCertificate != "" {
		cert, _, err := parseSAMLSigningCertificate(provider.SAML.NextSPCertificate, s.now())
		if err != nil {
			return nil, err
		}
		for _, descriptor := range append([]types.KeyDescriptor(nil), metadata.SPSSODescriptor.KeyDescriptors...) {
			descriptor.KeyInfo.X509Data.X509Certificates = append([]dsigtypes.X509Certificate(nil), descriptor.KeyInfo.X509Data.X509Certificates...)
			descriptor.KeyInfo.X509Data.X509Certificates[0].Data = base64.StdEncoding.EncodeToString(cert.Raw)
			metadata.SPSSODescriptor.KeyDescriptors = append(metadata.SPSSODescriptor.KeyDescriptors, descriptor)
		}
	}
	raw, err := xml.Marshal(metadata)
	if err != nil {
		return nil, ErrSAMLProviderInvalid
	}
	return append([]byte(xml.Header), raw...), nil
}

// RotateSAMLSPKey stages a publishable certificate, then promotes it after IdP
// configuration. Previous decryption material remains for the state TTL only.
func (s *EnterpriseIdentityService) RotateSAMLSPKey(ctx context.Context, actorID, workspaceID, providerID, revision int64, action string) (*EnterpriseIdentityProvider, error) {
	if err := s.require(ctx, actorID, workspaceID, "identity.manage"); err != nil {
		return nil, err
	}
	repo, ok := s.repo.(SAMLKeyRepository)
	if !ok {
		return nil, ErrSAMLProviderInvalid
	}
	provider, secret, err := s.repo.GetProvider(ctx, workspaceID, actorID, providerID)
	if err != nil {
		return nil, err
	}
	if provider.Type != "saml" || provider.SAML == nil || provider.Revision != revision || provider.Status != "active" {
		return nil, ErrWorkspaceConflict
	}
	ring, err := s.decryptSAMLKeyRing(secret)
	if err != nil {
		return nil, err
	}
	currentCert, nextCert := provider.SAML.SPCertificate, provider.SAML.NextSPCertificate
	switch action {
	case "stage":
		if ring.NextPrivateKey != "" || nextCert != "" {
			return nil, ErrWorkspaceConflict
		}
		ring.NextPrivateKey, nextCert, err = newSAMLSPKey(s.now())
		if err != nil {
			return nil, err
		}
	case "promote":
		if ring.NextPrivateKey == "" || nextCert == "" {
			return nil, ErrWorkspaceConflict
		}
		ring.PreviousPrivateKey, ring.PreviousCertificate, ring.PreviousValidUntil = ring.CurrentPrivateKey, currentCert, s.now().Add(15*time.Minute)
		ring.CurrentPrivateKey, ring.NextPrivateKey = ring.NextPrivateKey, ""
		currentCert, nextCert = nextCert, ""
	default:
		return nil, ErrSAMLProviderInvalid
	}
	ciphertext, err := s.encryptSAMLKeyRing(*ring)
	if err != nil {
		return nil, err
	}
	return repo.UpdateSAMLKeys(ctx, workspaceID, actorID, providerID, revision, ciphertext, currentCert, nextCert)
}
