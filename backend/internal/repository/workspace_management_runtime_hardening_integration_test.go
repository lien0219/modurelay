//go:build integration

package repository

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceManagementGrantRevocationPreservesRuntimeDuringCacheOutage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, google := range []bool{false, true} {
		for _, machine := range []bool{false, true} {
			t.Run(fmt.Sprintf("google=%t/machine=%t", google, machine), func(t *testing.T) {
				isolateWorkspaceTestFixtures(t)
				ctx := context.Background()
				client := testEntClient(t)
				owner := mustCreateUser(t, client, &service.User{Balance: 100})
				wr := NewWorkspaceRepository(integrationDB)
				ws := service.NewWorkspaceService(wr)
				w, err := ws.CreateOrganization(ctx, owner.ID, "Management boundary", fmt.Sprintf("management-boundary-%d", owner.ID))
				require.NoError(t, err)
				actor := workspaceJoin(t, ctx, ws, owner.ID, w.ID, "developer")
				p, err := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Runtime", Slug: "runtime"})
				require.NoError(t, err)
				base := NewAPIKeyRepository(client, integrationDB)
				kr := &countTenantAuthRepo{APIKeyRepository: base, ProjectKeyRepository: base.(service.ProjectKeyRepository)}
				cache := &tenantAuthCacheOutage{entries: map[string]*service.APIKeyAuthCacheEntry{}}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				cfg.APIKeyAuth.L2TTLSeconds = 60
				keys := service.NewAPIKeyService(kr, NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, cache, cfg)
				keys.ConfigureWorkspaces(wr)
				identity := NewEnterpriseIdentityRepository(integrationDB, enterpriseTestEncryptor{})
				keys.SetEnterpriseIdentityService(service.NewEnterpriseIdentityService(identity, service.NewWorkspaceAccessService(wr), NewUserRepository(client, integrationDB), enterpriseTestEncryptor{}))
				ws.SetKeyInvalidator(keys)
				svc := service.NewServiceAccountService(NewServiceAccountRepository(integrationDB), service.NewWorkspaceAccessService(wr), keys, serviceAccountLimitSettings{values: map[string]string{service.SettingKeyServiceAccountsPerProject: "3", service.SettingKeyServiceAccountCredentials: "2"}})
				var secret string
				var keyID, accountID int64
				if machine {
					sa, err := svc.Create(ctx, actor.ID, w.ID, p.ID, service.ServiceAccountInput{Name: "Worker", Slug: "worker"})
					require.NoError(t, err)
					credential, err := svc.CreateCredential(ctx, actor.ID, w.ID, p.ID, sa.ID, service.CreateAPIKeyRequest{Name: "Worker"})
					require.NoError(t, err)
					secret, keyID, accountID = credential.Secret, credential.Credential.ID, sa.ID
				} else {
					key, err := keys.CreateForProject(ctx, actor.ID, w.ID, p.ID, service.CreateAPIKeyRequest{Name: "Human runtime"})
					require.NoError(t, err)
					secret, keyID = key.Key, key.ID
				}
				personal, err := keys.Create(ctx, actor.ID, service.CreateAPIKeyRequest{Name: "Personal"})
				require.NoError(t, err)
				_, err = ws.SetProjectAccessMode(ctx, owner.ID, w.ID, service.ProjectAccessModeAssigned)
				require.NoError(t, err)
				grant, err := ws.CreateProjectAccessGrant(ctx, owner.ID, w.ID, p.ID, service.ProjectAccessGrantInput{SubjectType: "member", SubjectID: otherMemberWorkspaceMemberID(t, ctx, w.ID, actor.ID), Role: "developer"})
				require.NoError(t, err)
				if machine {
					require.NoError(t, svc.RequirePolicy(ctx, actor.ID, w.ID, p.ID, accountID, "service_account_policy.update"))
				} else {
					_, err = keys.GetForUser(ctx, actor.ID, keyID)
					require.NoError(t, err)
					_, err = keys.GetForProject(ctx, actor.ID, w.ID, p.ID, keyID)
					require.NoError(t, err)
				}
				router := gin.New()
				if google {
					router.Use(middleware.APIKeyAuthGoogle(keys, cfg))
				} else {
					router.Use(gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(keys, nil, cfg)))
				}
				router.GET("/v1/usage", func(c *gin.Context) {
					key, ok := middleware.GetAPIKeyFromContext(c)
					require.True(t, ok)
					require.Equal(t, w.ID, key.Tenant.WorkspaceID)
					require.Equal(t, p.ID, key.Tenant.ProjectID)
					require.Equal(t, owner.ID, key.BillingUserID())
					if machine {
						require.Nil(t, key.User)
						require.Equal(t, accountID, *key.ServiceAccountID)
					} else {
						require.Equal(t, actor.ID, key.User.ID)
					}
					c.Status(200)
				})
				request := func() int {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest("GET", "/v1/usage?workspace_id=999&project_id=999", nil)
					req.Header.Set("Authorization", "Bearer "+secret)
					req.Header.Set("X-Project-ID", "999")
					router.ServeHTTP(rec, req)
					return rec.Code
				}
				require.Equal(t, 200, request())
				require.Equal(t, 200, request())
				lookups := kr.lookups.Load()
				require.Equal(t, int64(1), lookups)
				cache.outage = true
				require.NoError(t, ws.DeleteProjectAccessGrant(ctx, owner.ID, w.ID, p.ID, grant.ID))
				if machine {
					require.ErrorIs(t, svc.RequirePolicy(ctx, actor.ID, w.ID, p.ID, accountID, "service_account_policy.update"), service.ErrWorkspaceForbidden)
				} else {
					_, err = keys.GetForUser(ctx, actor.ID, keyID)
					require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
					_, err = keys.GetForProject(ctx, actor.ID, w.ID, p.ID, keyID)
					require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
					legacy, page, err := keys.List(ctx, actor.ID, pagination.DefaultPagination(), service.APIKeyListFilters{})
					require.NoError(t, err)
					require.EqualValues(t, 1, page.Total)
					require.Len(t, legacy, 1)
					require.Equal(t, personal.ID, legacy[0].ID)
					_, _, err = keys.ListForProject(ctx, actor.ID, w.ID, p.ID, pagination.DefaultPagination())
					require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
				}
				require.Equal(t, 200, request(), "management-grant revocation cannot change existing runtime authorization")
				_, err = integrationDB.Exec(`INSERT INTO workspace_security_policies(workspace_id,require_mfa,require_sso,session_max_age_seconds) VALUES($1,true,true,900) ON CONFLICT(workspace_id) DO UPDATE SET require_mfa=true,require_sso=true,session_max_age_seconds=900`, w.ID)
				require.NoError(t, err)
				require.Equal(t, 200, request(), "human Session requirements cannot become key or machine runtime requirements")
				require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, actor.ID))
				if machine {
					require.Equal(t, 200, request(), "creator removal does not remove the Project-owned machine identity")
				} else {
					require.NotEqual(t, 200, request(), "a human runtime key still requires its own live membership")
				}
				_, err = integrationDB.Exec(`UPDATE projects SET status='archived' WHERE id=$1`, p.ID)
				require.NoError(t, err)
				require.NotEqual(t, 200, request(), "both runtime principals require a live parent Project")
				require.Equal(t, lookups, kr.lookups.Load(), "cached credentials were exercised throughout the invalidation outage")
			})
		}
	}
}
