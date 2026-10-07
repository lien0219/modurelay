package service

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type samlFixtureEncryptor struct{}

func (samlFixtureEncryptor) Encrypt(value string) (string, error) {
	return "test-encrypted:" + base64.StdEncoding.EncodeToString([]byte(value)), nil
}
func (samlFixtureEncryptor) Decrypt(value string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "test-encrypted:"))
	return string(raw), err
}

func TestSAMLSPKeysAreEncryptedAndPublicEndpointsAreServerBound(t *testing.T) {
	now := time.Now()
	key, cert, err := newSAMLSPKey(now)
	require.NoError(t, err)
	require.Contains(t, key, "PRIVATE KEY")
	_, _, err = parseSAMLSigningCertificate(cert, now)
	require.NoError(t, err)
	identity := &EnterpriseIdentityService{encryptor: samlFixtureEncryptor{}, now: time.Now}
	ciphertext, err := identity.encryptSAMLKeyRing(samlSPKeyRing{CurrentPrivateKey: key})
	require.NoError(t, err)
	require.NotContains(t, ciphertext, "PRIVATE KEY")
	ring, err := identity.decryptSAMLKeyRing(ciphertext)
	require.NoError(t, err)
	require.Equal(t, key, ring.CurrentPrivateKey)
	info, err := SAMLPublicEndpoints(strings.Repeat("a", 43), "https://relay.example.com/api/v1/auth/sso/callback")
	require.NoError(t, err)
	require.Equal(t, "https://relay.example.com/api/v1/auth/sso/saml/acs", info.ACSURL)
	require.Equal(t, "https://relay.example.com/api/v1/auth/sso/saml/metadata/"+strings.Repeat("a", 43), info.MetadataURL)
	require.Equal(t, info.MetadataURL, info.EntityID)
	for _, uri := range []string{"//evil.example/callback", "javascript:evil", "https://user:password@relay.example/callback", "https://relay.example/callback?host=evil"} {
		_, err := SAMLPublicEndpoints(strings.Repeat("a", 43), uri)
		require.Error(t, err)
	}
}
