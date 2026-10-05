package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestExecutionPrincipalExclusiveIdentity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		principal ExecutionPrincipal
		valid     bool
	}{
		{"human", ExecutionPrincipal{Type: PrincipalTypeUser, UserID: 19}, true},
		{"machine", ExecutionPrincipal{Type: PrincipalTypeServiceAccount, ServiceAccountID: 31}, true},
		{"creator mixed into machine", ExecutionPrincipal{Type: PrincipalTypeServiceAccount, UserID: 19, ServiceAccountID: 31}, false},
		{"unknown", ExecutionPrincipal{Type: "admin", UserID: 19}, false},
		{"missing machine", ExecutionPrincipal{Type: PrincipalTypeServiceAccount}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.valid {
				require.NoError(t, tc.principal.Validate())
			} else {
				require.Error(t, tc.principal.Validate())
			}
		})
	}
}

func TestServiceAccountKeyDoesNotAttributeCreator(t *testing.T) {
	id := int64(31)
	key := &APIKey{UserID: 99, ServiceAccountID: &id, BillingPrincipal: &User{ID: 7}}
	require.Equal(t, ExecutionPrincipal{Type: PrincipalTypeServiceAccount, ServiceAccountID: 31}, key.ExecutionPrincipal())
	require.Equal(t, int64(7), key.BillingUserID())
	require.Nil(t, key.User)
	ctx := WithExecutionPrincipal(context.Background(), key.ExecutionPrincipal())
	require.Equal(t, key.ExecutionPrincipal(), ExecutionPrincipalFromContext(ctx))
}

func TestServiceAccountCredentialDigestIsLookupOnly(t *testing.T) {
	// Known SHA-256 of "abc", independent of the production helper.
	require.Equal(t, "sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", HashServiceAccountCredential("abc"))
	require.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", CredentialAuthCacheDigest(HashServiceAccountCredential("abc")))
	require.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", CredentialAuthCacheDigest("abc"))
}
