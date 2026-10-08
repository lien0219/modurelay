package service

import (
	"encoding/base64"
	"encoding/xml"
	"net/mail"
	"strings"
	"time"

	"github.com/russellhaering/gosaml2/types"
)

type samlValidatedResponse struct {
	Claims          *EnterpriseIdentityClaims
	ResponseID      string
	AssertionID     string
	ExpiresAt       time.Time
	AuthenticatedAt time.Time
	ValidUntil      time.Time
}

// Library output contains the cryptographically validated/transformed assertion.
// The application adds strict request, subject and bounded condition checks.
func (s *EnterpriseIdentityService) validateSAMLResponse(provider *EnterpriseIdentityProvider, ciphertext, encoded, requestID, redirectURI string) (*samlValidatedResponse, error) {
	if requestID == "" || len(requestID) > 1024 || len(encoded) > base64.StdEncoding.EncodedLen(samlResponseMaxBytes) {
		return nil, ErrSAMLResponseInvalid
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || validateSAMLXML(raw, samlResponseMaxBytes) != nil {
		return nil, ErrSAMLResponseInvalid
	}
	var unverified types.Response
	if xml.Unmarshal(raw, &unverified) != nil || len(unverified.Assertions)+len(unverified.EncryptedAssertions) != 1 {
		return nil, ErrSAMLResponseInvalid
	}
	// No custom signature verification: the maintained library verifies the
	// signed response or every assertion and rejects unsigned/wrapped content.
	var response *types.Response
	var info *SAMLSPInfo
	attempts := 1
	if len(unverified.EncryptedAssertions) > 0 {
		attempts = 3
	}
	for index := 0; index < attempts; index++ {
		sp, endpoints, buildErr := s.samlServiceProvider(provider, ciphertext, redirectURI, index)
		if buildErr != nil {
			if index == 0 {
				return nil, buildErr
			}
			continue
		}
		response, err = sp.ValidateEncodedResponse(encoded)
		if err == nil {
			info = endpoints
			break
		}
	}
	if err != nil || response == nil {
		return nil, ErrSAMLSignatureInvalid
	}
	if len(response.Assertions) != 1 || response.ID == "" || len(response.ID) > 1024 || response.InResponseTo != requestID || response.Destination != info.ACSURL || response.Issuer == nil || response.Issuer.Value != provider.SAML.IDPEntityID || response.Status == nil || response.Status.StatusCode == nil || response.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		return nil, ErrSAMLResponseInvalid
	}
	assertion := &response.Assertions[0]
	if !response.SignatureValidated && !assertion.SignatureValidated {
		return nil, ErrSAMLSignatureInvalid
	}
	if assertion.ID == "" || len(assertion.ID) > 1024 || assertion.Version != "2.0" || assertion.Issuer == nil || assertion.Issuer.Value != provider.SAML.IDPEntityID || assertion.Subject == nil || assertion.Subject.SubjectConfirmation == nil || assertion.Conditions == nil {
		return nil, ErrSAMLResponseInvalid
	}
	now := s.now()
	for _, issued := range []time.Time{response.IssueInstant, assertion.IssueInstant} {
		if issued.IsZero() || issued.After(now.Add(2*time.Minute)) || issued.Before(now.Add(-15*time.Minute)) {
			return nil, ErrSAMLAssertionExpired
		}
	}
	confirmation := assertion.Subject.SubjectConfirmation
	data := confirmation.SubjectConfirmationData
	if confirmation.Method != "urn:oasis:names:tc:SAML:2.0:cm:bearer" || data == nil || data.InResponseTo != requestID || data.Recipient != info.ACSURL {
		return nil, ErrSAMLResponseInvalid
	}
	notBefore, err := time.Parse(time.RFC3339, assertion.Conditions.NotBefore)
	if err != nil {
		return nil, ErrSAMLResponseInvalid
	}
	expires, err := time.Parse(time.RFC3339, assertion.Conditions.NotOnOrAfter)
	if err != nil {
		return nil, ErrSAMLResponseInvalid
	}
	confirmationExpires, err := time.Parse(time.RFC3339, data.NotOnOrAfter)
	if err != nil {
		return nil, ErrSAMLResponseInvalid
	}
	if !expires.After(notBefore) || expires.Sub(notBefore) > time.Hour || notBefore.After(now.Add(2*time.Minute)) || !now.Before(expires) || !now.Before(confirmationExpires) || confirmationExpires.After(now.Add(time.Hour)) {
		return nil, ErrSAMLAssertionExpired
	}
	if confirmationExpires.Before(expires) {
		expires = confirmationExpires
	}
	restrictions := assertion.Conditions.AudienceRestrictions
	if len(restrictions) == 0 || len(restrictions) > 8 {
		return nil, ErrSAMLResponseInvalid
	}
	for _, restriction := range restrictions {
		if len(restriction.Audiences) == 0 || len(restriction.Audiences) > 16 {
			return nil, ErrSAMLResponseInvalid
		}
		matched := false
		for _, audience := range restriction.Audiences {
			if audience.Value == info.EntityID {
				matched = true
			}
		}
		if !matched {
			return nil, ErrSAMLResponseInvalid
		}
	}
	if assertion.AuthnStatement == nil || assertion.AuthnStatement.AuthnInstant == nil {
		return nil, ErrSAMLResponseInvalid
	}
	authenticated := *assertion.AuthnStatement.AuthnInstant
	if authenticated.IsZero() || authenticated.After(now.Add(2*time.Minute)) || now.Sub(authenticated) >= 12*time.Hour {
		return nil, ErrSAMLAssertionExpired
	}
	if assertion.AuthnStatement.SessionNotOnOrAfter != nil && !now.Before(*assertion.AuthnStatement.SessionNotOnOrAfter) {
		return nil, ErrSAMLAssertionExpired
	}
	claims, err := samlClaimsFromAssertion(assertion, provider.SAML)
	if err != nil {
		return nil, err
	}
	validUntil := expires
	if deadline := assertion.AuthnStatement.SessionNotOnOrAfter; deadline != nil && deadline.Before(validUntil) {
		validUntil = *deadline
	}
	return &samlValidatedResponse{Claims: claims, ResponseID: response.ID, AssertionID: assertion.ID, ExpiresAt: expires.Add(2 * time.Minute), AuthenticatedAt: authenticated, ValidUntil: validUntil}, nil
}

func samlClaimsFromAssertion(assertion *types.Assertion, config *SAMLProviderConfig) (*EnterpriseIdentityClaims, error) {
	attributes := map[string][]string{}
	if assertion.AttributeStatement != nil {
		if len(assertion.AttributeStatement.Attributes) > 100 {
			return nil, ErrSAMLResponseInvalid
		}
		for _, attribute := range assertion.AttributeStatement.Attributes {
			if attribute.Name == "" || len(attribute.Name) > 512 || len(attribute.Values) > 200 {
				return nil, ErrSAMLResponseInvalid
			}
			if _, duplicate := attributes[attribute.Name]; duplicate {
				return nil, ErrSAMLResponseInvalid
			}
			values := make([]string, 0, len(attribute.Values))
			for _, value := range attribute.Values {
				if value.NameID != nil || len(value.Value) > 4096 || strings.ContainsAny(value.Value, "\x00\r\n") {
					return nil, ErrSAMLResponseInvalid
				}
				values = append(values, strings.TrimSpace(value.Value))
			}
			attributes[attribute.Name] = values
		}
	}
	scalar := func(name string) (string, error) {
		values := attributes[name]
		if len(values) > 1 {
			return "", ErrSAMLResponseInvalid
		}
		if len(values) == 0 {
			return "", nil
		}
		return values[0], nil
	}
	subject := ""
	if config.SubjectAttribute != "" {
		var err error
		subject, err = scalar(config.SubjectAttribute)
		if err != nil {
			return nil, err
		}
	} else {
		nameID := assertion.Subject.NameID
		if nameID == nil {
			return nil, ErrSAMLSubjectUnstable
		}
		allowedFormat := nameID.Format == samlPersistentNameID || (config.AllowUnspecifiedNameID && (nameID.Format == samlUnspecifiedNameID || nameID.Format == ""))
		if !allowedFormat {
			return nil, ErrSAMLSubjectUnstable
		}
		if nameID.NameQualifier != "" && nameID.NameQualifier != config.IDPEntityID {
			return nil, ErrSAMLSubjectUnstable
		}
		subject = strings.TrimSpace(nameID.Value)
	}
	if subject == "" || len(subject) > 1024 || strings.ContainsAny(subject, "\x00\r\n") {
		return nil, ErrSAMLSubjectUnstable
	}
	claims := &EnterpriseIdentityClaims{Subject: subject, RawProtocol: "saml", EmailTrusted: true}
	email, err := scalar(config.EmailAttribute)
	if err != nil {
		return nil, err
	}
	if email != "" {
		email = strings.ToLower(strings.TrimSpace(email))
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || len(email) > 320 {
			return nil, ErrSAMLResponseInvalid
		}
		at := strings.LastIndex(email, "@")
		domain, err := NormalizeEnterpriseDomain(email[at+1:])
		if err != nil {
			return nil, ErrSAMLResponseInvalid
		}
		claims.Email, claims.EmailVerified = email[:at+1]+domain, true
	}
	claims.Name, err = scalar(config.NameAttribute)
	if err != nil || len(claims.Name) > 512 {
		return nil, ErrSAMLResponseInvalid
	}
	groups, present := attributes[config.GroupsAttribute]
	claims.GroupsPresent, claims.GroupsComplete = present, present
	seen := map[string]bool{}
	for _, group := range groups {
		if len(group) > 512 {
			return nil, ErrSAMLResponseInvalid
		}
		if group != "" && !seen[group] {
			claims.Groups = append(claims.Groups, group)
			seen[group] = true
		}
	}
	return claims, nil
}
