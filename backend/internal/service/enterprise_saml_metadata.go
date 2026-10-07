package service

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	samlMetadataMaxBytes  = 256 << 10
	samlResponseMaxBytes  = 512 << 10
	samlPersistentNameID  = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	samlUnspecifiedNameID = "urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
	samlRedirectBinding   = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"
	samlPostBinding       = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
)

var (
	ErrSAMLProviderInvalid  = infraerrors.BadRequest("SAML_PROVIDER_INVALID", "invalid SAML provider configuration")
	ErrSAMLMetadataInvalid  = infraerrors.BadRequest("SAML_METADATA_INVALID", "invalid or unsupported SAML metadata")
	ErrSAMLResponseInvalid  = infraerrors.Unauthorized("SAML_RESPONSE_INVALID", "invalid SAML response")
	ErrSAMLSignatureInvalid = infraerrors.Unauthorized("SAML_SIGNATURE_INVALID", "SAML signature validation failed")
	ErrSAMLReplayDetected   = infraerrors.Unauthorized("SAML_REPLAY_DETECTED", "SAML proof has already been used")
	ErrSAMLSubjectUnstable  = infraerrors.Unauthorized("SAML_SUBJECT_UNSTABLE", "a persistent SAML subject is required")
	ErrSAMLAssertionExpired = infraerrors.Unauthorized("SAML_ASSERTION_EXPIRED", "SAML assertion is outside its valid time window")
)

type samlMetadataEntity struct {
	XMLName    xml.Name          `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
	EntityID   string            `xml:"entityID,attr"`
	ValidUntil time.Time         `xml:"validUntil,attr"`
	IDPs       []samlMetadataIDP `xml:"urn:oasis:names:tc:SAML:2.0:metadata IDPSSODescriptor"`
}
type samlMetadataIDP struct {
	Protocol string `xml:"protocolSupportEnumeration,attr"`
	Signed   bool   `xml:"WantAuthnRequestsSigned,attr"`
	Keys     []struct {
		Use     string `xml:"use,attr"`
		KeyInfo struct {
			X509Data []struct {
				Certificates []string `xml:"http://www.w3.org/2000/09/xmldsig# X509Certificate"`
			} `xml:"http://www.w3.org/2000/09/xmldsig# X509Data"`
		} `xml:"http://www.w3.org/2000/09/xmldsig# KeyInfo"`
	} `xml:"urn:oasis:names:tc:SAML:2.0:metadata KeyDescriptor"`
	Endpoints []struct {
		Binding  string `xml:"Binding,attr"`
		Location string `xml:"Location,attr"`
	} `xml:"urn:oasis:names:tc:SAML:2.0:metadata SingleSignOnService"`
}

// ParseSAMLMetadata accepts one explicitly selected IdP. Metadata aggregates
// containing multiple IdPs are rejected rather than choosing an arbitrary one.
func ParseSAMLMetadata(raw []byte, now time.Time) (*SAMLProviderConfig, error) {
	if validateSAMLXML(raw, samlMetadataMaxBytes) != nil {
		return nil, ErrSAMLMetadataInvalid
	}
	var entity samlMetadataEntity
	if xml.Unmarshal(raw, &entity) != nil || len(entity.IDPs) != 1 || (!entity.ValidUntil.IsZero() && !now.Before(entity.ValidUntil)) {
		return nil, ErrSAMLMetadataInvalid
	}
	idp := entity.IDPs[0]
	if !strings.Contains(" "+idp.Protocol+" ", " urn:oasis:names:tc:SAML:2.0:protocol ") || len(idp.Endpoints) > 16 || len(idp.Keys) > 16 {
		return nil, ErrSAMLMetadataInvalid
	}
	config := &SAMLProviderConfig{IDPEntityID: entity.EntityID, AuthnRequestsSigned: true, MetadataSource: "xml", NameAttribute: "name", GroupsAttribute: "groups"}
	for _, endpoint := range idp.Endpoints {
		// Validate even unused endpoints, so imported metadata cannot conceal a
		// private-network endpoint that a future edit might accidentally activate.
		if _, err := ValidateEnterpriseEndpoint(endpoint.Location); err != nil {
			return nil, ErrSAMLMetadataInvalid
		}
		if endpoint.Binding == samlRedirectBinding {
			if config.SSOURL != "" && config.SSOURL != endpoint.Location {
				return nil, ErrSAMLMetadataInvalid
			}
			config.SSOURL = endpoint.Location
		}
	}
	for _, key := range idp.Keys {
		if key.Use != "" && key.Use != "signing" {
			continue
		}
		for _, data := range key.KeyInfo.X509Data {
			config.SigningCertificates = append(config.SigningCertificates, data.Certificates...)
		}
	}
	// Count before deduplication: repeated certs must not evade the import bound.
	if len(config.SigningCertificates) == 0 || len(config.SigningCertificates) > 8 {
		return nil, ErrSAMLMetadataInvalid
	}
	if err := normalizeSAMLConfig(config, now); err != nil {
		return nil, ErrSAMLMetadataInvalid
	}
	return config, nil
}

func parseSAMLSigningCertificate(raw string, now time.Time) (*x509.Certificate, string, error) {
	if len(raw) > 16384 {
		return nil, "", ErrSAMLProviderInvalid
	}
	raw = strings.TrimSpace(raw)
	var der []byte
	if block, rest := pem.Decode([]byte(raw)); block != nil {
		if block.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
			return nil, "", ErrSAMLProviderInvalid
		}
		der = block.Bytes
	} else {
		compact := strings.Join(strings.Fields(raw), "")
		var err error
		der, err = base64.StdEncoding.DecodeString(compact)
		if err != nil {
			return nil, "", ErrSAMLProviderInvalid
		}
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil || now.Before(cert.NotBefore) || !now.Before(cert.NotAfter) {
		return nil, "", ErrSAMLProviderInvalid
	}
	switch key := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		if key.N.BitLen() < 2048 || key.N.BitLen() > 8192 {
			return nil, "", ErrSAMLProviderInvalid
		}
	case *ecdsa.PublicKey:
		if key.Curve.Params().BitSize < 256 || key.Curve.Params().BitSize > 521 {
			return nil, "", ErrSAMLProviderInvalid
		}
	default:
		return nil, "", ErrSAMLProviderInvalid
	}
	if cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, "", ErrSAMLProviderInvalid
	}
	return cert, string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), nil
}

func normalizeSAMLConfig(config *SAMLProviderConfig, now time.Time) error {
	if config == nil {
		return ErrSAMLProviderInvalid
	}
	config.IDPEntityID = strings.TrimSpace(config.IDPEntityID)
	if config.IDPEntityID == "" || len(config.IDPEntityID) > 2048 || strings.ContainsAny(config.IDPEntityID, "\x00\r\n") {
		return ErrSAMLProviderInvalid
	}
	endpoint, err := ValidateEnterpriseEndpoint(config.SSOURL)
	if err != nil {
		return ErrSAMLProviderInvalid
	}
	config.SSOURL = endpoint
	if len(config.SigningCertificates) < 1 || len(config.SigningCertificates) > 8 {
		return ErrSAMLProviderInvalid
	}
	certs := make([]string, 0, len(config.SigningCertificates))
	seen := map[string]bool{}
	for _, raw := range config.SigningCertificates {
		_, normalized, err := parseSAMLSigningCertificate(raw, now)
		if err != nil {
			return err
		}
		if !seen[normalized] {
			certs = append(certs, normalized)
			seen[normalized] = true
		}
	}
	config.SigningCertificates = certs
	config.AuthnRequestsSigned = true
	if config.EmailAttribute == "" {
		config.EmailAttribute = "email"
	}
	for _, value := range []string{config.SubjectAttribute, config.EmailAttribute, config.NameAttribute, config.GroupsAttribute} {
		if len(value) > 512 || strings.ContainsAny(value, "\x00\r\n\t ") {
			return ErrSAMLProviderInvalid
		}
	}
	if config.SubjectAttribute != "" && (config.SubjectAttribute == config.EmailAttribute || strings.EqualFold(config.SubjectAttribute, "email") || strings.EqualFold(config.SubjectAttribute, "mail")) {
		return ErrSAMLProviderInvalid
	}
	if config.MetadataSource == "" {
		config.MetadataSource = "manual"
	}
	if config.MetadataSource != "manual" && config.MetadataSource != "xml" && config.MetadataSource != "url" {
		return ErrSAMLProviderInvalid
	}
	if config.MetadataURL != "" {
		if _, err := ValidateEnterpriseEndpoint(config.MetadataURL); err != nil {
			return ErrSAMLProviderInvalid
		}
	}
	return nil
}

func validateSAMLInput(input EnterpriseIdentityProviderInput) (EnterpriseIdentityProviderInput, error) {
	input.Type = "saml"
	input.ProviderKey = strings.ToLower(strings.TrimSpace(input.ProviderKey))
	input.Name = strings.TrimSpace(input.Name)
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,79}$`).MatchString(input.ProviderKey) || input.Name == "" || len(input.Name) > 120 || input.SAML == nil {
		return input, ErrSAMLProviderInvalid
	}
	if input.IssuerURL != "" || input.ClientID != "" || input.ClientSecret != nil || input.TokenAuthMethod != "" || input.AuthorizationEndpoint != "" || input.TokenEndpoint != "" || input.JWKSURI != "" || input.UserinfoEndpoint != "" || len(input.Scopes) > 0 || input.SecretAction != "" && input.SecretAction != "preserve" || len(input.ClaimMapping) > 0 {
		return input, ErrSAMLProviderInvalid
	}
	// Copy mutable input so normalization never edits a caller's cached config.
	config := *input.SAML
	config.SigningCertificates = append([]string(nil), config.SigningCertificates...)
	input.SAML = &config
	if config.MetadataXML != "" {
		if len(config.MetadataXML) > samlMetadataMaxBytes {
			return input, ErrSAMLMetadataInvalid
		}
	} else if config.MetadataURL != "" {
		if _, err := ValidateEnterpriseEndpoint(config.MetadataURL); err != nil {
			return input, ErrSAMLMetadataInvalid
		}
	} else if err := normalizeSAMLConfig(&config, time.Now()); err != nil {
		return input, err
	}
	if len(input.JITConfig.AllowedDomains) > 100 {
		return input, ErrSAMLProviderInvalid
	}
	if input.JITConfig.DefaultRole == "" {
		input.JITConfig.DefaultRole = WorkspaceRoleViewer
	}
	if !validOIDCManagedRole(input.JITConfig.DefaultRole) {
		return input, ErrSAMLProviderInvalid
	}
	input.JITConfig.RequireVerifiedEmail = true
	for index, domain := range input.JITConfig.AllowedDomains {
		canonical, err := NormalizeEnterpriseDomain(domain)
		if err != nil {
			return input, ErrSAMLProviderInvalid
		}
		input.JITConfig.AllowedDomains[index] = canonical
	}
	discovery := false
	input.DiscoveryEnabled = &discovery
	return input, nil
}

func fetchSAMLMetadata(ctx context.Context, endpoint string) ([]byte, error) {
	validated, err := ValidateEnterpriseEndpoint(endpoint)
	if err != nil {
		return nil, ErrSAMLMetadataInvalid
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, validated, nil)
	if err != nil {
		return nil, ErrSAMLMetadataInvalid
	}
	request.Header.Set("Accept", "application/samlmetadata+xml, application/xml, text/xml")
	response, err := newEnterpriseOIDCHTTPClient(ctx).client.Do(request)
	if err != nil {
		return nil, ErrSAMLMetadataInvalid
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || response.ContentLength > samlMetadataMaxBytes {
		return nil, ErrSAMLMetadataInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, samlMetadataMaxBytes+1))
	if err != nil || len(raw) > samlMetadataMaxBytes {
		return nil, ErrSAMLMetadataInvalid
	}
	return raw, nil
}

func (s *EnterpriseIdentityService) prepareSAMLProvider(ctx context.Context, input EnterpriseIdentityProviderInput, creating bool) (EnterpriseIdentityProviderInput, string, error) {
	input, err := validateSAMLInput(input)
	if err != nil {
		return input, "", err
	}
	config := input.SAML
	var metadata []byte
	source := ""
	if config.MetadataXML != "" {
		metadata = []byte(config.MetadataXML)
		source = "xml"
	} else if config.MetadataURL != "" {
		metadata, err = fetchSAMLMetadata(ctx, config.MetadataURL)
		if err != nil {
			return input, "", err
		}
		source = "url"
	}
	if metadata != nil {
		parsed, err := ParseSAMLMetadata(metadata, s.now())
		if err != nil {
			return input, "", err
		}
		config.IDPEntityID, config.SSOURL, config.SigningCertificates = parsed.IDPEntityID, parsed.SSOURL, parsed.SigningCertificates
		config.MetadataSource = source
	}
	config.MetadataXML = ""
	if err = normalizeSAMLConfig(config, s.now()); err != nil {
		return input, "", err
	}
	if !creating {
		return input, "", nil
	}
	if s.encryptor == nil {
		return input, "", ErrSAMLProviderInvalid
	}
	key, cert, err := newSAMLSPKey(s.now())
	if err != nil {
		return input, "", err
	}
	config.SPCertificate = cert
	config.NextSPCertificate = ""
	ciphertext, err := s.encryptSAMLKeyRing(samlSPKeyRing{CurrentPrivateKey: key})
	if err != nil {
		return input, "", err
	}
	return input, ciphertext, nil
}

func validateSAMLConfiguration(provider *EnterpriseIdentityProvider) error {
	if provider == nil || provider.Type != "saml" || provider.SAML == nil {
		return ErrSAMLProviderInvalid
	}
	config := *provider.SAML
	return normalizeSAMLConfig(&config, time.Now())
}
