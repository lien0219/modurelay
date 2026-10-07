package service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSAMLXMLRejectsDTDAndBoundedInputs(t *testing.T) {
	for name, raw := range map[string]string{
		"dtd":                 `<!DOCTYPE root [<!ENTITY x SYSTEM "file:///etc/passwd">]><root>&x;</root>`,
		"entity":              `<root>&unknown;</root>`,
		"oversize":            `<root>` + strings.Repeat("x", 256<<10) + `</root>`,
		"depth":               strings.Repeat(`<x>`, 65) + strings.Repeat(`</x>`, 65),
		"duplicate id":        `<root><a ID="duplicate"/><b ID="duplicate"/></root>`,
		"duplicate attribute": `<root ID="one" ID="two"/>`,
		"trailing document":   `<root/><other/>`,
	} {
		t.Run(name, func(t *testing.T) { require.Error(t, validateSAMLXML([]byte(raw), 256<<10)) })
	}
	require.NoError(t, validateSAMLXML([]byte(`<?xml version="1.0"?><root xmlns="urn:example"><a ID="unique">safe</a></root>`), 256<<10))
}

func samlTestCertificate(t *testing.T, now time.Time) (string, []byte, *rsa.PrivateKey) {
	t.Helper()
	key := enterpriseOIDCTestRSAKey(t)
	template := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: "generated-test-idp"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(24 * time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), der, key
}

func samlTestMetadata(certificates ...[]byte) []byte {
	keys := ""
	for _, der := range certificates {
		keys += fmt.Sprintf(`<md:KeyDescriptor use="signing"><ds:KeyInfo><ds:X509Data><ds:X509Certificate>%s</ds:X509Certificate></ds:X509Data></ds:KeyInfo></md:KeyDescriptor>`, base64.StdEncoding.EncodeToString(der))
	}
	return []byte(`<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" xmlns:ds="http://www.w3.org/2000/09/xmldsig#" entityID="https://idp.example.com/entity"><md:IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol" WantAuthnRequestsSigned="true">` + keys + `<md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://idp.example.com/sso"/></md:IDPSSODescriptor></md:EntityDescriptor>`)
}

func TestSAMLMetadataValidatesTrustedSigningRolloverAndEndpoints(t *testing.T) {
	now := time.Now()
	_, first, _ := samlTestCertificate(t, now)
	_, second, _ := samlTestCertificate(t, now.Add(time.Minute))
	config, err := ParseSAMLMetadata(samlTestMetadata(first, second), now)
	require.NoError(t, err)
	require.Equal(t, "https://idp.example.com/entity", config.IDPEntityID)
	require.Equal(t, "https://idp.example.com/sso", config.SSOURL)
	require.Len(t, config.SigningCertificates, 2)
	require.True(t, config.AuthnRequestsSigned)
	for name, raw := range map[string][]byte{
		"malformed":       []byte(`<broken`),
		"xxe":             []byte(`<!DOCTYPE metadata [<!ENTITY evil SYSTEM "file:///etc/passwd">]><metadata>&evil;</metadata>`),
		"localhost":       []byte(strings.ReplaceAll(string(samlTestMetadata(first)), "https://idp.example.com/sso", "https://localhost/sso")),
		"private":         []byte(strings.ReplaceAll(string(samlTestMetadata(first)), "https://idp.example.com/sso", "https://10.0.0.1/sso")),
		"http":            []byte(strings.ReplaceAll(string(samlTestMetadata(first)), "https://idp.example.com/sso", "http://idp.example.com/sso")),
		"no certificate":  samlTestMetadata(),
		"too many certs":  samlTestMetadata(first, first, first, first, first, first, first, first, first),
		"wrong namespace": []byte(strings.ReplaceAll(string(samlTestMetadata(first)), "urn:oasis:names:tc:SAML:2.0:metadata", "urn:attacker")),
	} {
		t.Run(name, func(t *testing.T) { _, err := ParseSAMLMetadata(raw, now); require.Error(t, err) })
	}
	_, expired, _ := samlTestCertificate(t, now.Add(-48*time.Hour))
	_, err = ParseSAMLMetadata(samlTestMetadata(expired), now)
	require.Error(t, err)
}

func TestSAMLConfigurationPreservesDisabledOptionalAttributes(t *testing.T) {
	now := time.Now()
	certificate, _, _ := samlTestCertificate(t, now)
	config := &SAMLProviderConfig{IDPEntityID: "https://idp.example/entity", SSOURL: "https://idp.example/sso", SigningCertificates: []string{certificate}, EmailAttribute: "email", NameAttribute: "", GroupsAttribute: ""}
	require.NoError(t, normalizeSAMLConfig(config, now))
	require.Empty(t, config.NameAttribute)
	require.Empty(t, config.GroupsAttribute)
}
