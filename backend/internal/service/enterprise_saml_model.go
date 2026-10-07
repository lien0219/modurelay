package service

import (
	"context"
	"encoding/json"
	"time"
)

// SAMLProviderConfig contains public configuration only. Private SP key material
// lives in a separate encrypted persistence column. MetadataXML is import input;
// it is cleared before persistence and never included in provider responses.
type SAMLProviderConfig struct {
	IDPEntityID            string   `json:"idp_entity_id"`
	SSOURL                 string   `json:"sso_url"`
	SigningCertificates    []string `json:"signing_certificates"`
	MetadataURL            string   `json:"metadata_url,omitempty"`
	MetadataXML            string   `json:"metadata_xml,omitempty"`
	MetadataSource         string   `json:"metadata_source,omitempty"`
	SubjectAttribute       string   `json:"subject_attribute,omitempty"`
	AllowUnspecifiedNameID bool     `json:"allow_unspecified_name_id"`
	EmailAttribute         string   `json:"email_attribute,omitempty"`
	NameAttribute          string   `json:"name_attribute"`
	GroupsAttribute        string   `json:"groups_attribute"`
	SPCertificate          string   `json:"sp_certificate,omitempty"`
	NextSPCertificate      string   `json:"next_sp_certificate,omitempty"`
	AuthnRequestsSigned    bool     `json:"authn_requests_signed"`
}

// MarshalJSON keeps import XML out of all API responses, including accidental
// serialization of an input config. Unmarshal retains the import-only field.
func (c SAMLProviderConfig) MarshalJSON() ([]byte, error) {
	type publicConfig SAMLProviderConfig
	copy := publicConfig(c)
	copy.MetadataXML = ""
	return json.Marshal(copy)
}

type EnterpriseIdentityClaims = OIDCClaims
type EnterpriseProvisionInput = OIDCProvisionInput
type EnterpriseSSOLoginResult = OIDCLoginResult
type EnterpriseMappings = OIDCMappings

type SAMLReplayRepository interface {
	GetSAMLProviderByPublicID(context.Context, string) (*EnterpriseIdentityProvider, string, error)
	UseSAMLResponse(context.Context, int64, int64, string, string, time.Time) error
}

type SAMLKeyRepository interface {
	UpdateSAMLKeys(context.Context, int64, int64, int64, int64, string, string, string) (*EnterpriseIdentityProvider, error)
}

func identityProtocol(protocol string) string {
	if protocol == "" {
		return "oidc"
	}
	return protocol
}

func validEnterpriseProtocol(protocol string) bool {
	return protocol == "oidc" || protocol == "saml"
}
