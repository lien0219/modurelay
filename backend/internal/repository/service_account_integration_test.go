//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type serviceAccountLimitSettings struct {
	service.SettingRepository
	values map[string]string
}

func (s serviceAccountLimitSettings) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}
func serviceAccountTestService(t *testing.T) (*service.ServiceAccountService, *service.APIKeyService, *service.WorkspaceService) {
	t.Helper()
	client := testEntClient(t)
	wr := NewWorkspaceRepository(integrationDB)
	keys := service.NewAPIKeyService(NewAPIKeyRepository(client, integrationDB), NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
	keys.ConfigureWorkspaces(wr)
	svc := service.NewServiceAccountService(NewServiceAccountRepository(integrationDB), service.NewWorkspaceAccessService(wr), keys, serviceAccountLimitSettings{values: map[string]string{service.SettingKeyServiceAccountsPerProject: "3", service.SettingKeyServiceAccountCredentials: "2"}})
	return svc, keys, service.NewWorkspaceService(wr)
}

func TestServiceAccountMutationsWithSingleConnectionPool(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	client := testEntClient(t)
	svc, keys, workspaces := serviceAccountTestService(t)
	owner := mustCreateUser(t, client, &service.User{Balance: 100})
	w, err := workspaces.CreateOrganization(context.Background(), owner.ID, "Single Pool", fmt.Sprintf("single-pool-%d", owner.ID))
	require.NoError(t, err)
	p, err := workspaces.CreateProject(context.Background(), owner.ID, w.ID, service.ProjectInput{Name: "CI", Slug: "ci"})
	require.NoError(t, err)
	settings := NewSettingRepository(client)
	svc = service.NewServiceAccountService(NewServiceAccountRepository(integrationDB), service.NewWorkspaceAccessService(NewWorkspaceRepository(integrationDB)), keys, settings)
	previous := integrationDB.Stats().MaxOpenConnections
	integrationDB.SetMaxOpenConns(1)
	t.Cleanup(func() { integrationDB.SetMaxOpenConns(previous) })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	sa, err := svc.Create(ctx, owner.ID, w.ID, p.ID, service.ServiceAccountInput{Name: "Backend", Slug: "backend"})
	require.NoError(t, err, "settings must not acquire a second connection inside a mutation")
	first, err := svc.CreateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, service.CreateAPIKeyRequest{Name: "first"})
	require.NoError(t, err, "payer/group validation must use the existing transaction")
	_, err = svc.RotateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, first.Credential.ID)
	require.NoError(t, err)
	name := "renamed"
	_, err = svc.UpdateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, first.Credential.ID, service.UpdateAPIKeyRequest{Name: &name})
	require.NoError(t, err)
}

func TestServiceAccountCredentialRepairsQuotaAndExpiration(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	svc, _, _ := serviceAccountTestService(t)
	reset, quota := true, float64(50)
	future := time.Now().Add(time.Hour)
	for _, tc := range []struct {
		name, status string
		update       service.UpdateAPIKeyRequest
	}{
		{"reset", service.StatusAPIKeyQuotaExhausted, service.UpdateAPIKeyRequest{ResetQuota: &reset}},
		{"increase", service.StatusAPIKeyQuotaExhausted, service.UpdateAPIKeyRequest{Quota: &quota}},
		{"clear_expiration", service.StatusAPIKeyExpired, service.UpdateAPIKeyRequest{ClearExpiration: true}},
		{"extend_expiration", service.StatusAPIKeyExpired, service.UpdateAPIKeyRequest{ExpiresAt: &future}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := integrationDB.Exec(`UPDATE api_keys SET status=$2,quota=20,quota_used=20,expires_at=NULL WHERE id=$1`, f.key, tc.status)
			require.NoError(t, err)
			updated, err := svc.UpdateCredential(context.Background(), f.payer, f.workspace, f.project, f.sa, f.key, tc.update)
			require.NoError(t, err)
			require.Equal(t, service.StatusActive, updated.Status)
		})
	}
}
func TestServiceAccountControlPlaneLifecycle(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	svc, keys, workspaces := serviceAccountTestService(t)
	owner := mustCreateUser(t, client, &service.User{Balance: 100})
	developer := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("sa-developer-%d@example.com", time.Now().UnixNano())})
	w, e := workspaces.CreateOrganization(ctx, owner.ID, "Machine Payer", fmt.Sprintf("machine-payer-%d", owner.ID))
	require.NoError(t, e)
	_, token, e := workspaces.CreateInvitation(ctx, owner.ID, w.ID, developer.Email, "developer", time.Hour)
	require.NoError(t, e)
	_, e = workspaces.AcceptInvitation(ctx, developer.ID, token)
	require.NoError(t, e)
	p, e := workspaces.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Machine API", Slug: "machine-api"})
	require.NoError(t, e)
	sa, e := svc.Create(ctx, developer.ID, w.ID, p.ID, service.ServiceAccountInput{Name: "CI", Slug: "ci"})
	require.NoError(t, e)
	first, e := svc.CreateCredential(ctx, developer.ID, w.ID, p.ID, sa.ID, service.CreateAPIKeyRequest{Name: "ci-key", IPWhitelist: []string{"127.0.0.1"}, Quota: 20, RateLimit5h: 5})
	require.NoError(t, e)
	require.NotEmpty(t, first.Secret)
	var raw string
	var actor *int64
	require.NoError(t, integrationDB.QueryRow(`SELECT key,user_id FROM api_keys WHERE id=$1`, first.Credential.ID).Scan(&raw, &actor))
	require.Nil(t, actor)
	require.Equal(t, service.HashServiceAccountCredential(first.Secret), raw)
	require.NotEqual(t, first.Secret, raw)
	metadata, e := svc.ListCredentials(ctx, owner.ID, w.ID, p.ID, sa.ID)
	require.NoError(t, e)
	bytes, e := json.Marshal(metadata)
	require.NoError(t, e)
	require.NotContains(t, string(bytes), first.Secret)
	require.NotContains(t, string(bytes), raw)
	rotated, e := svc.RotateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, first.Credential.ID)
	require.NoError(t, e)
	require.NotEqual(t, first.Secret, rotated.Secret)
	require.Equal(t, first.Credential.ID, *rotated.PreviousCredentialID)
	require.Equal(t, first.Credential.Quota, rotated.Credential.Quota)
	require.Equal(t, first.Credential.IPWhitelist, rotated.Credential.IPWhitelist)
	var oldStatus string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM api_keys WHERE id=$1`, first.Credential.ID).Scan(&oldStatus))
	require.Equal(t, "active", oldStatus)
	_, e = svc.CreateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, service.CreateAPIKeyRequest{Name: "over-limit"})
	require.ErrorIs(t, e, service.ErrServiceAccountLimit)
	_, e = svc.RevokeCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, first.Credential.ID, false)
	require.NoError(t, e)
	status := "active"
	_, e = svc.UpdateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, first.Credential.ID, service.UpdateAPIKeyRequest{Status: &status})
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	_, e = svc.RotateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, first.Credential.ID)
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	// The creator is provenance only, never the machine's runtime member.
	require.NoError(t, workspaces.RemoveMember(ctx, owner.ID, w.ID, developer.ID))
	live, e := keys.GetByKey(ctx, rotated.Secret)
	require.NoError(t, e)
	require.Equal(t, sa.ID, *live.ServiceAccountID)
	require.Nil(t, live.User)
	require.Equal(t, owner.ID, live.BillingUser().ID)
	_, e = svc.SetStatus(ctx, owner.ID, w.ID, p.ID, sa.ID, false, false)
	require.NoError(t, e)
	_, e = keys.GetByKey(ctx, rotated.Secret)
	require.Error(t, e)
	_, e = svc.SetStatus(ctx, owner.ID, w.ID, p.ID, sa.ID, true, false)
	require.NoError(t, e)
	_, e = keys.GetByKey(ctx, rotated.Secret)
	require.NoError(t, e)
	require.NoError(t, workspaces.ArchiveProject(ctx, owner.ID, w.ID, p.ID))
	_, e = svc.CreateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, service.CreateAPIKeyRequest{Name: "archived"})
	require.ErrorIs(t, e, service.ErrWorkspaceConflict)
	_, _, e = svc.List(ctx, owner.ID, w.ID, p.ID, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, e)
	var events, audits int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND event_type LIKE 'service_account.%'`, w.ID).Scan(&events))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_audit_logs WHERE workspace_id=$1 AND action LIKE 'service_account.%'`, w.ID).Scan(&audits))
	require.Equal(t, events, audits)
	require.GreaterOrEqual(t, events, 6)
}

func TestServiceAccountConcurrentLimitsAndOutboxRollback(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	owner := mustCreateUser(t, testEntClient(t), &service.User{Balance: 100})
	svc, _, workspaces := serviceAccountTestService(t)
	w, e := workspaces.CreateOrganization(ctx, owner.ID, "Concurrent Machines", fmt.Sprintf("concurrent-machines-%d", owner.ID))
	require.NoError(t, e)
	p, e := workspaces.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "CI", Slug: "ci"})
	require.NoError(t, e)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := svc.Create(ctx, owner.ID, w.ID, p.ID, service.ServiceAccountInput{Name: fmt.Sprintf("bot-%d", i), Slug: fmt.Sprintf("bot-%d", i)})
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	successes := 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, service.ErrServiceAccountLimit)
		}
	}
	require.Equal(t, 3, successes)
	accounts, _, e := svc.List(ctx, owner.ID, w.ID, p.ID, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, e)
	sa := accounts[0]
	results = make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := svc.CreateCredential(ctx, owner.ID, w.ID, p.ID, sa.ID, service.CreateAPIKeyRequest{Name: fmt.Sprintf("key-%d", i)})
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	successes = 0
	for e := range results {
		if e == nil {
			successes++
		} else {
			require.ErrorIs(t, e, service.ErrServiceAccountLimit)
		}
	}
	require.Equal(t, 2, successes)
	// Fault the actual durable outbox insertion. Account, audit and event must
	// all roll back in the same transaction.
	p2, e := workspaces.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Rollback", Slug: "rollback"})
	require.NoError(t, e)
	_, e = integrationDB.Exec(`CREATE FUNCTION service_account_test_reject_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF EXISTS(SELECT 1 FROM domain_events WHERE id=NEW.event_id AND event_type='service_account.created' AND payload->'data'->>'name'='rollback-bot') THEN RAISE EXCEPTION 'test outbox failure'; END IF; RETURN NEW; END $$`)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`CREATE TRIGGER service_account_test_reject_outbox BEFORE INSERT ON domain_event_outbox FOR EACH ROW EXECUTE FUNCTION service_account_test_reject_outbox()`)
	require.NoError(t, e)
	t.Cleanup(func() {
		_, e := integrationDB.Exec(`DROP TRIGGER IF EXISTS service_account_test_reject_outbox ON domain_event_outbox; DROP FUNCTION IF EXISTS service_account_test_reject_outbox()`)
		require.NoError(t, e)
	})
	_, e = svc.Create(ctx, owner.ID, w.ID, p2.ID, service.ServiceAccountInput{Name: "rollback-bot", Slug: "rollback-bot"})
	require.Error(t, e)
	for _, table := range []string{"service_accounts", "workspace_audit_logs", "domain_events"} {
		var n int
		require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM `+table+` WHERE project_id=$1`, p2.ID).Scan(&n))
		if table == "service_accounts" {
			require.Zero(t, n)
		} else {
			var leaked int
			column := "metadata"
			if table == "domain_events" {
				column = "payload"
			}
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM `+table+` WHERE project_id=$1 AND `+column+`::text LIKE '%rollback-bot%'`, p2.ID).Scan(&leaked))
			require.Zero(t, leaked)
		}
	}
}

func TestServiceAccountHTTPRoleMatrixAndIDOR(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	client := testEntClient(t)
	svc, _, workspaces := serviceAccountTestService(t)
	owner := mustCreateUser(t, client, &service.User{Balance: 100})
	w, e := workspaces.CreateOrganization(ctx, owner.ID, "Role Machines", fmt.Sprintf("role-machines-%d", owner.ID))
	require.NoError(t, e)
	p, e := workspaces.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Roles", Slug: "roles"})
	require.NoError(t, e)
	sa, e := svc.Create(ctx, owner.ID, w.ID, p.ID, service.ServiceAccountInput{Name: "bot", Slug: "bot"})
	require.NoError(t, e)
	p2, e := workspaces.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Other", Slug: "other"})
	require.NoError(t, e)
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		t.Run(role, func(t *testing.T) {
			actor := owner.ID
			if role != "owner" {
				u := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("sa-%s-%d@example.com", role, time.Now().UnixNano())})
				actor = u.ID
				_, token, e := workspaces.CreateInvitation(ctx, owner.ID, w.ID, u.Email, role, time.Hour)
				require.NoError(t, e)
				_, e = workspaces.AcceptInvitation(ctx, u.ID, token)
				require.NoError(t, e)
			}
			router := gin.New()
			router.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: actor}) })
			handler.NewServiceAccountHandler(svc).RegisterTenantRoutes(router.Group("/api/v1"))
			base := fmt.Sprintf("/api/v1/workspaces/%d/projects/%d/service-accounts/%d", w.ID, p.ID, sa.ID)
			for _, tc := range []struct {
				method, path, body string
				status             int
			}{{"GET", base, "", 200}, {"GET", base + "/credentials", "", map[bool]int{true: 200, false: 403}[role == "owner" || role == "admin" || role == "developer"]}, {"PATCH", base, `{"name":"bot"}`, map[bool]int{true: 200, false: 403}[role == "owner" || role == "admin" || role == "developer"]}, {"GET", fmt.Sprintf("/api/v1/workspaces/%d/projects/%d/service-accounts/%d", w.ID, p2.ID, sa.ID), "", 404}} {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(rec, req)
				require.Equal(t, tc.status, rec.Code, tc.path)
			}
		})
	}
}
