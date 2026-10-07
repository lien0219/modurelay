package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseClaimMappingPreservesStableIdentityAndHandlesMissingGroups(t *testing.T) {
	claims := &OIDCClaims{Subject: "stable-subject", Issuer: "https://idp.example", Raw: map[string]any{
		"profile":        map[string]any{"mail": "Owner@EXAMPLE.COM", "display": "Owner"},
		"email_verified": true,
		"membership":     map[string]any{"groups": []any{"engineering", "engineering", "finance"}},
	}}
	err := ApplyOIDCClaimMapping(claims, map[string]any{"email": "profile.mail", "name": "profile.display", "groups": "membership.groups"})
	require.NoError(t, err)
	require.Equal(t, "stable-subject", claims.Subject)
	require.Equal(t, "https://idp.example", claims.Issuer)
	require.Equal(t, "owner@example.com", claims.Email)
	require.True(t, claims.EmailVerified)
	require.Equal(t, "Owner", claims.Name)
	require.Equal(t, []string{"engineering", "finance"}, claims.Groups)
	require.True(t, claims.GroupsPresent)
	require.True(t, claims.GroupsComplete)

	delete(claims.Raw, "membership")
	require.NoError(t, ApplyOIDCClaimMapping(claims, map[string]any{"groups": "membership.groups"}))
	require.False(t, claims.GroupsPresent)
	require.False(t, claims.GroupsComplete)
}

func TestEnterpriseClaimMappingRejectsUnverifiedEmailAndClaimExpressions(t *testing.T) {
	claims := &OIDCClaims{Raw: map[string]any{"email": "person@example.com", "email_verified": "true", "groups": []any{}}}
	require.NoError(t, ApplyOIDCClaimMapping(claims, nil))
	require.False(t, claims.EmailVerified, "only a JSON boolean is proof of verification")
	require.True(t, claims.GroupsPresent)
	require.True(t, claims.GroupsComplete, "an explicit empty array can remove managed memberships")
	for _, mapping := range []map[string]any{{"subject": "email"}, {"groups": "groups.*"}, {"email": "x[0]"}, {"groups": 42}} {
		require.Error(t, ApplyOIDCClaimMapping(claims, mapping))
	}
	claims.Raw["_claim_names"] = map[string]any{"groups": "src1"}
	require.NoError(t, ApplyOIDCClaimMapping(claims, nil))
	require.False(t, claims.GroupsComplete, "distributed/overage groups cannot revoke managed access")
}

func TestEnterpriseReturnToRejectsEncodedAndUnapprovedRedirects(t *testing.T) {
	for _, path := range []string{"/%2fevil.example", "/%5cevil.example", "/workspaces/1/../../auth/sso/callback", "/auth/sso/callback", "/api/v1/auth/sso/callback", "/other-page"} {
		require.Error(t, ValidateEnterpriseReturnTo(path), path)
	}
	require.NoError(t, ValidateEnterpriseReturnTo("/workspaces/12/identity?linked=1"))
}
