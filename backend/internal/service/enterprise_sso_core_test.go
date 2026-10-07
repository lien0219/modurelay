package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseSSOAssuranceMatchesProviderProtocol(t *testing.T) {
	now := time.Now().UTC()
	repo := &enterpriseEnforcementRepo{policy: WorkspaceIdentityPolicy{WorkspaceID: 7, RequireSSO: true}, provider: EnterpriseIdentityProvider{ID: 9, WorkspaceID: 7, Revision: 3, Type: "saml", Status: "active"}}
	svc := NewEnterpriseIdentityService(repo, nil, nil, nil)
	svc.SetClock(func() time.Time { return now })
	a := WorkspaceAssurance{WorkspaceID: 7, ProviderID: 9, ProviderRevision: 3, AuthMethod: "saml", AuthenticatedAt: now.Add(-time.Hour)}
	require.NoError(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, a))
	a.AuthMethod = "oidc"
	require.ErrorIs(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, a), ErrSSORequired)
	repo.provider.Type = "oidc"
	require.NoError(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, a))
	a.AuthMethod = "saml"
	require.ErrorIs(t, svc.CheckWorkspaceAccess(context.Background(), 7, WorkspaceTypeOrganization, PrincipalHuman, a), ErrSSORequired)
}

func TestEnterpriseSAMLMetadataXMLIsInputOnly(t *testing.T) {
	var input EnterpriseIdentityProviderInput
	require.NoError(t, json.Unmarshal([]byte(`{"type":"saml","saml":{"metadata_xml":"imported XML"}}`), &input))
	require.Equal(t, "imported XML", input.SAML.MetadataXML)
	encoded, err := json.Marshal(EnterpriseIdentityProvider{Type: "saml", SAML: input.SAML})
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "metadata_xml")
	require.NotContains(t, string(encoded), "imported XML")
}

func TestEnterpriseSAMLJSONPreservesDisabledOptionalAttributes(t *testing.T) {
	config := &SAMLProviderConfig{EmailAttribute: "email", NameAttribute: "", GroupsAttribute: ""}
	for _, tc := range []struct {
		name  string
		value any
	}{
		{"persisted config", config},
		{"public provider", EnterpriseIdentityProvider{Type: "saml", SAML: config}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.value)
			require.NoError(t, err)
			if tc.name == "public provider" {
				var envelope map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(encoded, &envelope))
				encoded = envelope["saml"]
			}
			var fields map[string]any
			require.NoError(t, json.Unmarshal(encoded, &fields))
			for _, field := range []string{"name_attribute", "groups_attribute"} {
				require.Contains(t, fields, field, "disabled attributes must remain explicit for editors that default omitted fields")
				require.Equal(t, "", fields[field])
			}
			// An editor starts with these defaults. Reading explicit empty values
			// must disable both mappings instead of silently restoring defaults.
			loaded := SAMLProviderConfig{NameAttribute: "name", GroupsAttribute: "groups"}
			require.NoError(t, json.Unmarshal(encoded, &loaded))
			require.Empty(t, loaded.NameAttribute)
			require.Empty(t, loaded.GroupsAttribute)
			require.Equal(t, "email", loaded.EmailAttribute)
		})
	}
}
