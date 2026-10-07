package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

type enterpriseValidationAccess struct{ WorkspaceRepository }

func (enterpriseValidationAccess) GetAccess(_ context.Context, actor, workspace, project int64) (*WorkspaceAccess, error) {
	return &WorkspaceAccess{Workspace: &Workspace{ID: workspace, Type: WorkspaceTypeOrganization, Status: "active"}, Member: &WorkspaceMember{UserID: actor, Role: WorkspaceRoleOwner, Status: "active"}}, nil
}

type enterpriseValidationRepo struct {
	EnterpriseIdentityRepository
	provider EnterpriseIdentityProvider
	codes    []string
}

func (r *enterpriseValidationRepo) GetProvider(context.Context, int64, int64, int64) (*EnterpriseIdentityProvider, string, error) {
	return &r.provider, "", nil
}
func (r *enterpriseValidationRepo) RecordProviderValidation(_ context.Context, w, a, p, v int64, code string) error {
	r.codes = append(r.codes, code)
	return nil
}

func TestEnterpriseProviderValidationProbesJWKSAndRecordsOnlySafeResults(t *testing.T) {
	key := enterpriseOIDCTestRSAKey(t)
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "invalid_jwks"}[failed], func(t *testing.T) {
			repo := &enterpriseValidationRepo{provider: EnterpriseIdentityProvider{ID: 9, WorkspaceID: 7, Revision: 2, Status: "active", DiscoveryEnabled: true, IssuerURL: "https://identity.example.invalid/validation"}}
			jwksCalls := 0
			ctx := enterpriseOIDCTestContext(enterpriseOIDCRoundTripper(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path == "/validation/.well-known/openid-configuration" {
					document := OIDCDiscoveryDocument{Issuer: repo.provider.IssuerURL, AuthorizationEndpoint: repo.provider.IssuerURL + "/authorize", TokenEndpoint: repo.provider.IssuerURL + "/token", JWKSURI: repo.provider.IssuerURL + "/keys", IDTokenSigningAlgs: []string{"RS256"}}
					return enterpriseOIDCTestResponse(request, 200, enterpriseOIDCTestJSON(t, document)), nil
				}
				jwksCalls++
				if failed {
					return enterpriseOIDCTestResponse(request, 503, `{"secret":"never record this"}`), nil
				}
				return enterpriseOIDCTestResponse(request, 200, enterpriseOIDCTestJSON(t, enterpriseJWKS{Keys: []enterpriseJWK{enterpriseOIDCTestRSAJWK(&key.PublicKey)}})), nil
			}))
			svc := NewEnterpriseIdentityService(repo, NewWorkspaceAccessService(enterpriseValidationAccess{}), nil, nil)
			err := svc.TestProvider(ctx, 1, 7, 9)
			if failed {
				require.Error(t, err)
				require.Equal(t, []string{"VALIDATION_FAILED"}, repo.codes)
			} else {
				require.NoError(t, err)
				require.Equal(t, []string{"SUCCESS"}, repo.codes)
			}
			require.Equal(t, 1, jwksCalls, "test connection must test the signing key endpoint")
		})
	}
}
