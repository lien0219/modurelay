package service

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/require"
)

func samlProtocolFixture(t *testing.T) (*EnterpriseIdentityService, *EnterpriseIdentityProvider, string, string, time.Time, *rsa.PrivateKey) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	certificate, _, idpKey := samlTestCertificate(t, now)
	private, spCertificate, err := newSAMLSPKey(now)
	require.NoError(t, err)
	identity := &EnterpriseIdentityService{encryptor: samlFixtureEncryptor{}, now: time.Now}
	secret, err := identity.encryptSAMLKeyRing(samlSPKeyRing{CurrentPrivateKey: private})
	require.NoError(t, err)
	provider := &EnterpriseIdentityProvider{ID: 9, WorkspaceID: 7, Type: "saml", Status: "active", Revision: 3, PublicID: strings.Repeat("a", 43), SAML: &SAMLProviderConfig{IDPEntityID: "https://idp.example.com/entity", SSOURL: "https://idp.example.com/sso", SigningCertificates: []string{certificate}, EmailAttribute: "email", NameAttribute: "name", GroupsAttribute: "groups", SPCertificate: spCertificate}}
	return identity, provider, secret, "https://relay.example.com/api/v1/auth/sso/callback", now, idpKey
}

func samlResponseFixture(t *testing.T, provider *EnterpriseIdentityProvider, redirectURI, requestID string, now time.Time, idpKey *rsa.PrivateKey, mutate func(*etree.Element), signature string) string {
	t.Helper()
	info, err := SAMLPublicEndpoints(provider.PublicID, redirectURI)
	require.NoError(t, err)
	response := etree.NewElement("samlp:Response")
	response.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
	response.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
	response.CreateAttr("ID", "response-generated-fixture")
	response.CreateAttr("Version", "2.0")
	response.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	response.CreateAttr("Destination", info.ACSURL)
	response.CreateAttr("InResponseTo", requestID)
	response.CreateElement("saml:Issuer").SetText(provider.SAML.IDPEntityID)
	response.CreateElement("samlp:Status").CreateElement("samlp:StatusCode").CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:Success")
	assertion := response.CreateElement("saml:Assertion")
	assertion.CreateAttr("ID", "assertion-generated-fixture")
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	assertion.CreateElement("saml:Issuer").SetText(provider.SAML.IDPEntityID)
	subject := assertion.CreateElement("saml:Subject")
	nameID := subject.CreateElement("saml:NameID")
	nameID.CreateAttr("Format", samlPersistentNameID)
	nameID.SetText("stable-subject")
	confirmation := subject.CreateElement("saml:SubjectConfirmation")
	confirmation.CreateAttr("Method", "urn:oasis:names:tc:SAML:2.0:cm:bearer")
	data := confirmation.CreateElement("saml:SubjectConfirmationData")
	data.CreateAttr("InResponseTo", requestID)
	data.CreateAttr("Recipient", info.ACSURL)
	data.CreateAttr("NotOnOrAfter", now.Add(5*time.Minute).Format(time.RFC3339))
	conditions := assertion.CreateElement("saml:Conditions")
	conditions.CreateAttr("NotBefore", now.Add(-time.Minute).Format(time.RFC3339))
	conditions.CreateAttr("NotOnOrAfter", now.Add(5*time.Minute).Format(time.RFC3339))
	conditions.CreateElement("saml:AudienceRestriction").CreateElement("saml:Audience").SetText(info.EntityID)
	assertion.CreateElement("saml:AuthnStatement").CreateAttr("AuthnInstant", now.Format(time.RFC3339))
	attributes := assertion.CreateElement("saml:AttributeStatement")
	for name, values := range map[string][]string{"email": {"member@example.com"}, "name": {"Enterprise Member"}, "groups": {"engineering", "finance"}} {
		attribute := attributes.CreateElement("saml:Attribute")
		attribute.CreateAttr("Name", name)
		for _, value := range values {
			attribute.CreateElement("saml:AttributeValue").SetText(value)
		}
	}
	if mutate != nil {
		mutate(response)
	}
	if signature != "none" {
		block, _ := pem.Decode([]byte(provider.SAML.SigningCertificates[0]))
		require.NotNil(t, block)
		signer, err := dsig.NewSigningContext(idpKey, [][]byte{block.Bytes})
		require.NoError(t, err)
		require.NoError(t, signer.SetSignatureMethod(dsig.RSASHA256SignatureMethod))
		signer.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
		if signature == "assertion" {
			// Detach with the inherited namespace declarations before signing.
			assertion.CreateAttr("xmlns:saml", "urn:oasis:names:tc:SAML:2.0:assertion")
			assertion.CreateAttr("xmlns:samlp", "urn:oasis:names:tc:SAML:2.0:protocol")
			signed, err := signer.SignEnveloped(assertion)
			require.NoError(t, err)
			response.RemoveChild(assertion)
			response.AddChild(signed)
		} else {
			signed, err := signer.SignEnveloped(response)
			require.NoError(t, err)
			response = signed
		}
	}
	doc := etree.NewDocument()
	doc.SetRoot(response)
	raw, err := doc.WriteToBytes()
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(raw)
}

func TestSAMLValidatesSignedProtocolAndStableClaims(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	for _, signature := range []string{"response", "assertion"} {
		t.Run(signature, func(t *testing.T) {
			encoded := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, nil, signature)
			verified, err := identity.validateSAMLResponse(provider, secret, encoded, "_request", redirect)
			require.NoError(t, err)
			require.Equal(t, "stable-subject", verified.Claims.Subject)
			require.Equal(t, "member@example.com", verified.Claims.Email)
			require.True(t, verified.Claims.EmailVerified)
			require.Equal(t, []string{"engineering", "finance"}, verified.Claims.Groups)
			require.Equal(t, "saml", verified.Claims.RawProtocol)
			require.NotEmpty(t, verified.ResponseID)
			require.NotEmpty(t, verified.AssertionID)
		})
	}
}

func TestSAMLRejectsForgedMismatchedUnsignedAndWrappedAssertions(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	for name, mutate := range map[string]func(*etree.Element){
		"request": func(root *etree.Element) { root.CreateAttr("InResponseTo", "_other") },
		"confirmation request": func(root *etree.Element) {
			root.FindElement("./Assertion/Subject/SubjectConfirmation/SubjectConfirmationData").CreateAttr("InResponseTo", "_other")
		},
		"audience": func(root *etree.Element) {
			root.FindElement("./Assertion/Conditions/AudienceRestriction/Audience").SetText("https://wrong.example")
		},
		"no audience": func(root *etree.Element) {
			conditions := root.FindElement("./Assertion/Conditions")
			conditions.RemoveChild(conditions.FindElement("./AudienceRestriction"))
		},
		"destination":         func(root *etree.Element) { root.CreateAttr("Destination", "https://wrong.example/acs") },
		"missing destination": func(root *etree.Element) { root.RemoveAttr("Destination") },
		"recipient": func(root *etree.Element) {
			root.FindElement("./Assertion/Subject/SubjectConfirmation/SubjectConfirmationData").CreateAttr("Recipient", "https://wrong.example/acs")
		},
		"issuer": func(root *etree.Element) {
			root.FindElement("./Assertion/Issuer").SetText("https://wrong.example/entity")
		},
		"response issuer": func(root *etree.Element) { root.FindElement("./Issuer").SetText("https://wrong.example/entity") },
		"expired": func(root *etree.Element) {
			root.FindElement("./Assertion/Conditions").CreateAttr("NotOnOrAfter", now.Add(-time.Minute).Format(time.RFC3339))
		},
		"future": func(root *etree.Element) {
			root.FindElement("./Assertion/Conditions").CreateAttr("NotBefore", now.Add(10*time.Minute).Format(time.RFC3339))
		},
		"status": func(root *etree.Element) {
			root.FindElement("./Status/StatusCode").CreateAttr("Value", "urn:oasis:names:tc:SAML:2.0:status:AuthnFailed")
		},
		"transient": func(root *etree.Element) {
			root.FindElement("./Assertion/Subject/NameID").CreateAttr("Format", "urn:oasis:names:tc:SAML:2.0:nameid-format:transient")
		},
		"email subject": func(root *etree.Element) {
			root.FindElement("./Assertion/Subject/NameID").CreateAttr("Format", "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress")
		},
		"multiple assertions": func(root *etree.Element) {
			other := root.FindElement("./Assertion").Copy()
			other.CreateAttr("ID", "second-assertion")
			root.AddChild(other)
		},
		"duplicate attributes": func(root *etree.Element) {
			attributes := root.FindElement("./Assertion/AttributeStatement")
			attribute := attributes.CreateElement("saml:Attribute")
			attribute.CreateAttr("Name", "email")
			attribute.CreateElement("saml:AttributeValue").SetText("attacker@example.com")
		},
	} {
		t.Run(name, func(t *testing.T) {
			encoded := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, mutate, "response")
			_, err := identity.validateSAMLResponse(provider, secret, encoded, "_request", redirect)
			require.Error(t, err)
		})
	}
	unsigned := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, nil, "none")
	_, err := identity.validateSAMLResponse(provider, secret, unsigned, "_request", redirect)
	require.Error(t, err)
	encoded := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, nil, "response")
	raw, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	tampered := strings.ReplaceAll(string(raw), "member@example.com", "attacker@example.com")
	_, err = identity.validateSAMLResponse(provider, secret, base64.StdEncoding.EncodeToString([]byte(tampered)), "_request", redirect)
	require.Error(t, err)
	// Put a signed Assertion under an unexpected wrapper and introduce an
	// unsigned sibling. Libraries must reject the wrapping attempt.
	wrapped, err := base64.StdEncoding.DecodeString(samlResponseFixture(t, provider, redirect, "_request", now, idpKey, nil, "assertion"))
	require.NoError(t, err)
	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromBytes(wrapped))
	root := doc.Root()
	assertion := root.FindElement("./Assertion")
	root.RemoveChild(assertion)
	root.CreateElement("saml:Advice").AddChild(assertion)
	bytes, err := doc.WriteToBytes()
	require.NoError(t, err)
	_, err = identity.validateSAMLResponse(provider, secret, base64.StdEncoding.EncodeToString(bytes), "_request", redirect)
	require.Error(t, err)
}

func TestSAMLExplicitStableSubjectAndCompleteEmptyGroups(t *testing.T) {
	identity, provider, secret, redirect, now, idpKey := samlProtocolFixture(t)
	provider.SAML.SubjectAttribute = "employeeId"
	encoded := samlResponseFixture(t, provider, redirect, "_request", now, idpKey, func(root *etree.Element) {
		root.FindElement("./Assertion/Subject/NameID").CreateAttr("Format", "urn:oasis:names:tc:SAML:2.0:nameid-format:transient")
		attrs := root.FindElement("./Assertion/AttributeStatement")
		attr := attrs.CreateElement("saml:Attribute")
		attr.CreateAttr("Name", "employeeId")
		attr.CreateElement("saml:AttributeValue").SetText("immutable-employee-123")
		for _, group := range attrs.FindElements("./Attribute") {
			if group.SelectAttrValue("Name", "") == "groups" {
				group.Child = nil
			}
		}
	}, "response")
	verified, err := identity.validateSAMLResponse(provider, secret, encoded, "_request", redirect)
	require.NoError(t, err)
	require.Equal(t, "immutable-employee-123", verified.Claims.Subject)
	require.Empty(t, verified.Claims.Groups)
	require.True(t, verified.Claims.GroupsPresent)
	require.True(t, verified.Claims.GroupsComplete)
}
