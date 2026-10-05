//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkspaceDeveloperKeysUsePayerAndLegacyReadsStayScoped(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	owner := mustCreateUser(t, client, &service.User{Balance: 100})
	developer := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("key-dev-%d@example.com", time.Now().UnixNano()), Balance: 0})
	wr := NewWorkspaceRepository(integrationDB)
	ws := service.NewWorkspaceService(wr)
	w, e := ws.CreateOrganization(ctx, owner.ID, "Payer", fmt.Sprintf("payer-%d", owner.ID))
	require.NoError(t, e)
	_, token, e := ws.CreateInvitation(ctx, owner.ID, w.ID, developer.Email, "developer", time.Hour)
	require.NoError(t, e)
	_, e = ws.AcceptInvitation(ctx, developer.ID, token)
	require.NoError(t, e)
	p, e := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "API", Slug: "api"})
	require.NoError(t, e)
	group := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("payer-exclusive-%d", owner.ID), IsExclusive: true})
	_, e = integrationDB.Exec(`INSERT INTO user_allowed_groups(user_id,group_id) VALUES($1,$2)`, owner.ID, group.ID)
	require.NoError(t, e)
	kr := NewAPIKeyRepository(client, integrationDB)
	ks := service.NewAPIKeyService(kr, NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
	ks.ConfigureWorkspaces(wr)
	k, e := ks.CreateForProject(ctx, developer.ID, w.ID, p.ID, service.CreateAPIKeyRequest{Name: "Developer", GroupID: &group.ID})
	require.NoError(t, e)
	require.Equal(t, developer.ID, k.UserID)
	require.NotEmpty(t, k.Key)
	require.NotEqual(t, "****", k.Key)
	live, e := ks.GetByKey(ctx, k.Key)
	require.NoError(t, e)
	require.Equal(t, developer.ID, live.User.ID)
	require.Equal(t, owner.ID, live.BillingUserID())
	require.Equal(t, float64(100), live.BillingUser().Balance)
	keys, total, e := ks.ListForProject(ctx, owner.ID, w.ID, p.ID, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, e)
	require.Len(t, keys, 1)
	require.Equal(t, int64(1), total)
	require.NotEqual(t, k.Key, keys[0].Key)
	require.Contains(t, keys[0].Key, "****")
	projectDetail, e := ks.GetForProject(ctx, owner.ID, w.ID, p.ID, k.ID)
	require.NoError(t, e)
	require.NotEqual(t, k.Key, projectDetail.Key)
	require.Contains(t, projectDetail.Key, "****")
	legacy, page, e := ks.List(ctx, developer.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, service.APIKeyListFilters{})
	require.NoError(t, e)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, legacy, 1)
	require.NotEqual(t, k.Key, legacy[0].Key)
	personal, e := ks.Create(ctx, developer.ID, service.CreateAPIKeyRequest{Name: "Personal"})
	require.NoError(t, e)
	detail, e := ks.GetForUser(ctx, developer.ID, personal.ID)
	require.NoError(t, e)
	require.Equal(t, personal.Key, detail.Key)
	name := "Updated by admin"
	updated, e := ks.UpdateForProject(ctx, owner.ID, w.ID, p.ID, k.ID, service.UpdateAPIKeyRequest{Name: &name})
	require.NoError(t, e)
	require.Equal(t, name, updated.Name)
	require.Equal(t, developer.ID, updated.UserID)
	require.NotEqual(t, k.Key, updated.Key)
	require.Contains(t, updated.Key, "****")
	require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, developer.ID))
	_, e = ks.Update(ctx, k.ID, developer.ID, service.UpdateAPIKeyRequest{Name: &name})
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	e = ks.Delete(ctx, k.ID, developer.ID)
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	legacy, page, e = ks.List(ctx, developer.ID, pagination.PaginationParams{Page: 1, PageSize: 20}, service.APIKeyListFilters{})
	require.NoError(t, e)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, legacy, 1)
	require.Equal(t, personal.Key, legacy[0].Key)
	_, e = ks.GetByKey(ctx, k.Key)
	require.Error(t, e)
}

type countTenantAuthRepo struct {
	service.APIKeyRepository
	service.ProjectKeyRepository
	lookups atomic.Int64
}

func (r *countTenantAuthRepo) GetByKeyForAuth(ctx context.Context, key string) (*service.APIKey, error) {
	r.lookups.Add(1)
	return r.APIKeyRepository.GetByKeyForAuth(ctx, key)
}

type tenantAuthCacheOutage struct {
	service.APIKeyCache
	entries map[string]*service.APIKeyAuthCacheEntry
	outage  bool
}

func (c *tenantAuthCacheOutage) GetAuthCache(_ context.Context, key string) (*service.APIKeyAuthCacheEntry, error) {
	return c.entries[key], nil
}
func (c *tenantAuthCacheOutage) SetAuthCache(_ context.Context, key string, e *service.APIKeyAuthCacheEntry, _ time.Duration) error {
	c.entries[key] = e
	return nil
}
func (c *tenantAuthCacheOutage) DeleteAuthCache(_ context.Context, key string) error {
	if c.outage {
		return errors.New("cache outage")
	}
	delete(c.entries, key)
	return nil
}
func (c *tenantAuthCacheOutage) PublishAuthCacheInvalidation(context.Context, string) error {
	return errors.New("cache invalidation outage")
}
func TestWorkspaceBothGatewaysRejectLiveChangesDuringAuthCacheOutage(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	client := testEntClient(t)
	for _, google := range []bool{false, true} {
		for _, change := range []string{"project_archive", "workspace_suspend", "key_revoke", "member_remove", "entitlement_remove"} {
			t.Run(fmt.Sprintf("google=%t/%s", google, change), func(t *testing.T) {
				owner := mustCreateUser(t, client, &service.User{Balance: 100})
				actor := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("cached-dev-%d@example.com", time.Now().UnixNano())})
				wr := NewWorkspaceRepository(integrationDB)
				ws := service.NewWorkspaceService(wr)
				w, e := ws.CreateOrganization(ctx, owner.ID, "Cache", fmt.Sprintf("cache-%d", owner.ID))
				require.NoError(t, e)
				_, token, e := ws.CreateInvitation(ctx, owner.ID, w.ID, actor.Email, "developer", time.Hour)
				require.NoError(t, e)
				_, e = ws.AcceptInvitation(ctx, actor.ID, token)
				require.NoError(t, e)
				p, e := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Cached", Slug: "cached"})
				require.NoError(t, e)
				group := mustCreateGroup(t, client, &service.Group{Name: fmt.Sprintf("cache-group-%d", owner.ID), IsExclusive: true})
				_, e = integrationDB.Exec(`INSERT INTO user_allowed_groups(user_id,group_id) VALUES($1,$2)`, owner.ID, group.ID)
				require.NoError(t, e)
				base := NewAPIKeyRepository(client, integrationDB)
				kr := &countTenantAuthRepo{APIKeyRepository: base, ProjectKeyRepository: base.(service.ProjectKeyRepository)}
				cache := &tenantAuthCacheOutage{entries: map[string]*service.APIKeyAuthCacheEntry{}}
				cfg := &config.Config{RunMode: config.RunModeSimple}
				cfg.APIKeyAuth.L2TTLSeconds = 60
				ks := service.NewAPIKeyService(kr, NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, cache, cfg)
				ks.ConfigureWorkspaces(wr)
				ws.SetKeyInvalidator(ks)
				k, e := ks.CreateForProject(ctx, actor.ID, w.ID, p.ID, service.CreateAPIKeyRequest{Name: "Cached", GroupID: &group.ID})
				require.NoError(t, e)
				router := gin.New()
				if google {
					router.Use(middleware.APIKeyAuthGoogle(ks, cfg))
				} else {
					router.Use(gin.HandlerFunc(middleware.NewAPIKeyAuthMiddleware(ks, nil, cfg)))
				}
				router.GET("/v1/usage", func(c *gin.Context) {
					key, _ := middleware.GetAPIKeyFromContext(c)
					require.Equal(t, w.ID, key.Tenant.WorkspaceID)
					require.Equal(t, actor.ID, key.User.ID)
					require.Equal(t, owner.ID, key.BillingUserID())
					c.Status(200)
				})
				request := func() int {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest("GET", "/v1/usage?workspace_id=999&project_id=999", nil)
					req.Header.Set("Authorization", "Bearer "+k.Key)
					req.Header.Set("X-Project-ID", "999")
					router.ServeHTTP(rec, req)
					return rec.Code
				}
				require.Equal(t, 200, request())
				require.Equal(t, 200, request())
				require.Equal(t, int64(1), kr.lookups.Load())
				cache.outage = true
				switch change {
				case "project_archive":
					require.NoError(t, ws.ArchiveProject(ctx, owner.ID, w.ID, p.ID))
				case "workspace_suspend":
					_, e = integrationDB.Exec(`UPDATE workspaces SET status='suspended' WHERE id=$1`, w.ID)
					require.NoError(t, e)
				case "key_revoke":
					require.NoError(t, ks.DeleteForProject(ctx, owner.ID, w.ID, p.ID, k.ID))
				case "member_remove":
					require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, actor.ID))
				case "entitlement_remove":
					_, e = integrationDB.Exec(`DELETE FROM user_allowed_groups WHERE user_id=$1 AND group_id=$2`, owner.ID, group.ID)
					require.NoError(t, e)
				}
				require.NotEqual(t, 200, request())
				require.Equal(t, int64(1), kr.lookups.Load(), "a cached key was used; the live gate must independently reject it")
			})
		}
	}
}

func TestWorkspaceKeyScopeAndLiveAdmission(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	owner := mustCreateUser(t, client, &service.User{Balance: 100})
	outsider := mustCreateUser(t, client, &service.User{})
	wr := NewWorkspaceRepository(integrationDB)
	ws := service.NewWorkspaceService(wr)
	w, e := ws.CreateOrganization(ctx, owner.ID, "Keys", fmt.Sprintf("keys-%d", owner.ID))
	require.NoError(t, e)
	p, e := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "API", Slug: "api", AllowedModels: []string{"gpt-allowed"}})
	require.NoError(t, e)
	kr := NewAPIKeyRepository(client, integrationDB)
	ks := service.NewAPIKeyService(kr, NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
	ks.ConfigureWorkspaces(wr)
	k, e := ks.CreateForProject(ctx, owner.ID, w.ID, p.ID, service.CreateAPIKeyRequest{Name: "Scoped"})
	require.NoError(t, e)
	var auditCount int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND project_id=$2 AND action='key.create' AND target_id=$3`, w.ID, p.ID, k.ID).Scan(&auditCount))
	require.Equal(t, 1, auditCount)
	require.Equal(t, owner.ID, k.UserID)
	require.Equal(t, p.ID, *k.ProjectID)
	_, e = ks.GetForProject(ctx, outsider.ID, w.ID, p.ID, k.ID)
	require.ErrorIs(t, e, service.ErrWorkspaceNotFound)
	got, e := ks.GetForProject(ctx, owner.ID, w.ID, p.ID, k.ID)
	require.NoError(t, e)
	require.NotEqual(t, k.Key, got.Key)
	live, e := ks.GetByKey(ctx, k.Key)
	require.NoError(t, e)
	require.Equal(t, w.ID, live.Tenant.WorkspaceID)
	require.True(t, live.AllowsModel("gpt-allowed"))
	require.False(t, live.AllowsModel("gpt-other"))
	_, e = integrationDB.Exec(`UPDATE api_keys SET project_id=(SELECT id FROM projects WHERE workspace_id<>$1 LIMIT 1) WHERE id=$2`, w.ID, k.ID)
	require.Error(t, e)
	require.NoError(t, ws.ArchiveProject(ctx, owner.ID, w.ID, p.ID))
	_, e = ks.GetByKey(ctx, k.Key)
	require.Error(t, e)
}

func TestWorkspaceArchivedInvitationReadAndAdminPermissions(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	owner := mustCreateUser(t, client, &service.User{Role: service.RoleAdmin})
	developer := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("developer-%d@example.com", time.Now().UnixNano())})
	ws := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	w, e := ws.CreateOrganization(ctx, owner.ID, "Read", fmt.Sprintf("read-%d", owner.ID))
	require.NoError(t, e)
	_, token, e := ws.CreateInvitation(ctx, owner.ID, w.ID, developer.Email, "developer", time.Hour)
	require.NoError(t, e)
	_, e = ws.AcceptInvitation(ctx, developer.ID, token)
	require.NoError(t, e)
	require.NoError(t, ws.AdminSetStatus(ctx, owner.ID, w.ID, "suspended"))
	_, _, e = ws.ListInvitations(ctx, owner.ID, w.ID, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, e)
	_, _, e = ws.ListInvitations(ctx, developer.ID, w.ID, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.ErrorIs(t, e, service.ErrWorkspaceForbidden)
	detail, e := ws.AdminInspect(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	b, e := json.Marshal(detail)
	require.NoError(t, e)
	require.Contains(t, string(b), `"permissions":[]`)
}

func TestWorkspaceVideoPendingPrincipalSurvivesLifecycleAfterTransfer(t *testing.T) {
	for _, lifecycle := range []string{"disabled", "deleted"} {
		t.Run(lifecycle, func(t *testing.T) {
			isolateWorkspaceTestFixtures(t)
			ctx, billing, a, reservation, cmd, log := tenantUsageFixture(t)
			workspaces := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
			require.NoError(t, workspaces.UpdateMember(ctx, a.BillingPrincipalUserID, a.WorkspaceID, a.ActorUserID, "owner", "active"))
			require.NoError(t, workspaces.ChangeBillingOwner(ctx, a.BillingPrincipalUserID, a.WorkspaceID, a.ActorUserID))
			require.NoError(t, workspaces.UpdateMember(ctx, a.ActorUserID, a.WorkspaceID, a.BillingPrincipalUserID, "billing", "active"))
			query := `UPDATE users SET status='disabled' WHERE id=$1`
			if lifecycle == "deleted" {
				query = `UPDATE users SET deleted_at=now() WHERE id=$1`
			}
			_, err := integrationDB.ExecContext(ctx, query, a.BillingPrincipalUserID)
			if lifecycle == "deleted" {
				require.Error(t, err, "pending work blocks deletion after billing-owner transfer")
			} else {
				require.NoError(t, err)
			}
			client := testEntClient(t)
			users := NewUserRepository(client, integrationDB)
			require.ErrorIs(t, users.(service.WorkspaceAdminLifecycleGuard).GuardWorkspaceUserDeletion(ctx, a.BillingPrincipalUserID), service.ErrWorkspaceConflict)
			keyRepo := NewAPIKeyRepository(client, integrationDB)
			keys := service.NewAPIKeyService(keyRepo, NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
			keys.ConfigureWorkspaces(NewWorkspaceRepository(integrationDB))
			key, err := keyRepo.GetByID(ctx, a.APIKeyID)
			require.NoError(t, err)
			live, err := keys.TenantAdmissionSnapshot(ctx, key)
			require.NoError(t, err)
			require.Equal(t, a.ActorUserID, live.BillingUserID(), "new requests resolve the replacement principal")
			pending := &service.GrokVideoPendingBilling{UserID: a.ActorUserID, APIKeyID: a.APIKeyID, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, BudgetReservationID: reservation.ID}
			frozen, err := keys.VideoPendingTenantSnapshot(ctx, pending, live)
			require.NoError(t, err, "already admitted work must retain its original funding principal")
			require.Equal(t, a.BillingPrincipalUserID, frozen.BillingUserID())
			require.Equal(t, a.ActorUserID, live.BillingUserID(), "restoring pending work must not mutate new admission")
			result, err := billing.ApplyTenantUsage(ctx, cmd, log)
			require.NoError(t, err)
			require.True(t, result.Applied)
			assertTenantUsageTotals(t, a, reservation, cmd, 99, 1, 0, 1, 1)
			require.NoError(t, users.Delete(ctx, a.BillingPrincipalUserID), "settled work no longer blocks the transferred user's deletion")
		})
	}
}
