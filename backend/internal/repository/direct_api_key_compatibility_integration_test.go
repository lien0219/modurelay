//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestDirectAPIKeyCompatibilityGate protects the old /keys workflow after the
// policy and machine-identity migrations. It deliberately creates no
// organization workspace, custom project, or service account before the
// ordinary key is created.
func TestDirectAPIKeyCompatibilityGate(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Balance: 100})
	group := mustCreateGroup(t, client, &service.Group{
		Name:     fmt.Sprintf("direct-key-group-%d", user.ID),
		Platform: service.PlatformOpenAI,
	})

	workspaceRepo := NewWorkspaceRepository(integrationDB)
	keys := service.NewAPIKeyService(
		NewAPIKeyRepository(client, integrationDB),
		NewUserRepository(client, integrationDB),
		NewGroupRepository(client, integrationDB),
		NewUserSubscriptionRepository(client),
		nil,
		nil,
		&config.Config{},
	)
	keys.ConfigureWorkspaces(workspaceRepo)

	const secret = "direct-compatibility-key-2026"
	key, err := keys.Create(ctx, user.ID, service.CreateAPIKeyRequest{
		Name:        "Legacy direct key",
		GroupID:     &group.ID,
		CustomKey:   compatPtrString(secret),
		IPWhitelist: []string{"127.0.0.1"},
		Quota:       40,
		RateLimit5h: 3,
		RateLimit1d: 7,
		RateLimit7d: 11,
	})
	require.NoError(t, err)
	require.Equal(t, secret, key.Key, "direct creation must return the same one-time secret contract")
	require.Nil(t, key.ServiceAccountID, "ordinary keys must remain human credentials")
	require.Equal(t, service.PrincipalTypeUser, key.ExecutionPrincipal().Type)
	require.Equal(t, user.ID, key.ExecutionPrincipal().UserID)
	require.NotNil(t, key.ProjectID)

	var workspaceID, projectID, userID, serviceAccountID *int64
	var workspaceType string
	var defaultProject bool
	var storedSecret string
	var storedGroupID *int64
	var quota, rate5h, rate1d, rate7d float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT w.id,p.id,k.user_id,k.service_account_id,w.type,p.is_default,k.key,k.group_id,k.quota,k.rate_limit_5h,k.rate_limit_1d,k.rate_limit_7d
		FROM api_keys k JOIN projects p ON p.id=k.project_id JOIN workspaces w ON w.id=p.workspace_id
		WHERE k.id=$1`, key.ID).Scan(&workspaceID, &projectID, &userID, &serviceAccountID, &workspaceType, &defaultProject, &storedSecret, &storedGroupID, &quota, &rate5h, &rate1d, &rate7d))
	require.NotNil(t, workspaceID)
	require.NotNil(t, projectID)
	require.Equal(t, user.ID, *userID)
	require.Nil(t, serviceAccountID)
	require.Equal(t, "personal", workspaceType)
	require.True(t, defaultProject)
	require.Equal(t, secret, storedSecret)
	require.NotNil(t, storedGroupID)
	require.Equal(t, group.ID, *storedGroupID, "legacy group selection must remain attached to direct keys")
	require.Equal(t, float64(40), quota)
	require.Equal(t, float64(3), rate5h)
	require.Equal(t, float64(7), rate1d)
	require.Equal(t, float64(11), rate7d)

	live, err := keys.GetByKey(ctx, secret)
	require.NoError(t, err)
	require.Equal(t, secret, live.Key, "existing direct key secret must not be rewritten")
	require.Nil(t, live.ServiceAccountID)
	require.NotNil(t, live.Tenant)
	require.Equal(t, *workspaceID, live.Tenant.WorkspaceID)
	require.Equal(t, *projectID, live.Tenant.ProjectID)
	require.Equal(t, user.ID, live.Tenant.BillingPrincipalUserID)
	require.Equal(t, user.ID, live.BillingUserID())
	require.Equal(t, service.PrincipalTypeUser, live.ExecutionPrincipal().Type)

	policyRepo := NewPolicyRepository(integrationDB)
	resolver := domain.NewEffectivePolicyResolver(policyRepo)

	// No rows on a new personal workspace/project are unrestricted inheritance.
	defaultPolicy, err := middleware.ResolvePolicyForAPIKey(ctx, resolver, live)
	require.NoError(t, err)
	require.Nil(t, defaultPolicy.Layers.Workspace)
	require.Nil(t, defaultPolicy.Layers.Project)
	require.Nil(t, defaultPolicy.Layers.ServiceAccount)
	require.Nil(t, defaultPolicy.RPMLimit)
	require.Nil(t, defaultPolicy.DailyRequestLimit)
	require.Nil(t, defaultPolicy.MonthlyRequestLimit)
	require.Nil(t, defaultPolicy.DailyTokenLimit)
	require.Nil(t, defaultPolicy.MonthlyTokenLimit)
	require.True(t, defaultPolicy.AllowsModel("gpt-6"))
	require.True(t, defaultPolicy.AllowsPlatform(service.PlatformOpenAI))

	actorCtx := service.WithPolicyActor(ctx, user.ID)
	workspaceRPM, projectRPM := int64(25), int64(10)
	_, err = policyRepo.UpdatePolicy(actorCtx, domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: *workspaceID}, 0, domain.Policy{
		AllowedModels: []string{"gpt-6"},
		RPMLimit:      &workspaceRPM,
	})
	require.NoError(t, err)
	_, err = policyRepo.UpdatePolicy(actorCtx, domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: *projectID}, 0, domain.Policy{
		AllowedModels:    []string{"gpt-6", "claude-4"},
		AllowedPlatforms: []string{"openai"},
		RPMLimit:         &projectRPM,
	})
	require.NoError(t, err)

	// Create a machine identity only after the direct key exists. Its deny-all
	// policy is intentionally irrelevant to the ordinary key.
	serviceAccounts := service.NewServiceAccountService(
		NewServiceAccountRepository(integrationDB),
		service.NewWorkspaceAccessService(workspaceRepo),
		keys,
		serviceAccountLimitSettings{values: map[string]string{
			service.SettingKeyServiceAccountsPerProject: "3",
			service.SettingKeyServiceAccountCredentials: "2",
		}},
	)
	serviceAccount, err := serviceAccounts.Create(ctx, user.ID, *workspaceID, *projectID, service.ServiceAccountInput{
		Name: "Compatibility machine",
		Slug: "compatibility-machine",
	})
	require.NoError(t, err)
	_, err = policyRepo.UpdatePolicy(actorCtx, domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: serviceAccount.ID}, 0, domain.Policy{
		AllowedModels: []string{},
	})
	require.NoError(t, err)

	effective, err := middleware.ResolvePolicyForAPIKey(ctx, resolver, live)
	require.NoError(t, err)
	require.Nil(t, effective.Layers.ServiceAccount)
	require.Equal(t, int64(10), effective.RPMLimitValue())
	require.True(t, effective.AllowsModel("gpt-6"))
	require.False(t, effective.AllowsModel("claude-4"), "workspace policy must remain an AND restriction")
	require.True(t, effective.AllowsPlatform(service.PlatformOpenAI))
	require.False(t, effective.AllowsPlatform(service.PlatformAnthropic))

	// Disabling/removing the machine identity cannot invalidate the direct key.
	_, err = serviceAccounts.SetStatus(ctx, user.ID, *workspaceID, *projectID, serviceAccount.ID, false, false)
	require.NoError(t, err)
	live, err = keys.GetByKey(ctx, secret)
	require.NoError(t, err)
	require.Nil(t, live.ServiceAccountID)
	require.Equal(t, secret, live.Key)

	// A machine identity with no credentials or pending work may be removed
	// independently; the ordinary user credential must remain valid afterward.
	_, err = integrationDB.ExecContext(ctx, `DELETE FROM service_accounts WHERE id=$1`, serviceAccount.ID)
	require.NoError(t, err)
	live, err = keys.GetByKey(ctx, secret)
	require.NoError(t, err)
	require.Nil(t, live.ServiceAccountID)
	require.Equal(t, secret, live.Key)

	// The real budget and tenant-usage repositories must preserve the user as
	// execution principal while recording the personal tenant and payer.
	budget := service.NewBudgetService(NewBudgetRepository(integrationDB))
	reservation, err := budget.Reserve(ctx, service.BudgetAttribution{
		ActorUserID:            user.ID,
		APIKeyID:               key.ID,
		WorkspaceID:            *workspaceID,
		ProjectID:              *projectID,
		BillingPrincipalUserID: user.ID,
	}, "direct-compat-budget-"+uuid.NewString(), 1)
	require.NoError(t, err)
	account := mustCreateAccount(t, client, &service.Account{
		Name:     fmt.Sprintf("direct-compat-account-%d", user.ID),
		Type:     service.AccountTypeAPIKey,
		Platform: service.PlatformOpenAI,
	})
	platform := service.PlatformOpenAI
	log := &service.UsageLog{
		RequestID:              "direct-compat-usage-" + uuid.NewString(),
		UserID:                 user.ID,
		APIKeyID:               key.ID,
		AccountID:              account.ID,
		WorkspaceID:            compatPtrInt64(*workspaceID),
		ProjectID:              compatPtrInt64(*projectID),
		BillingPrincipalUserID: compatPtrInt64(user.ID),
		BudgetReservationID:    compatPtrString(reservation.ID),
		ResolvedPlatform:       &platform,
		Model:                  "gpt-6",
		ActualCost:             1,
		TotalCost:              1,
		RateMultiplier:         1,
		CreatedAt:              time.Now().UTC(),
	}
	cmd := &service.UsageBillingCommand{
		RequestID:              log.RequestID,
		UserID:                 user.ID,
		APIKeyID:               key.ID,
		AccountID:              account.ID,
		AccountType:            account.Type,
		WorkspaceID:            *workspaceID,
		ProjectID:              *projectID,
		BillingPrincipalUserID: user.ID,
		BudgetReservationID:    reservation.ID,
		BudgetActualCost:       1,
		BalanceCost:            1,
		ResolvedPlatform:       platform,
		Model:                  log.Model,
	}
	cmd.Normalize()
	billing, ok := NewUsageBillingRepository(client, integrationDB).(service.TenantUsageBillingRepository)
	require.True(t, ok)
	result, err := billing.ApplyTenantUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.True(t, result.Applied)

	var usageUserID, usageWorkspaceID, usageProjectID, usagePayerID int64
	var usageServiceAccountID *int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT user_id,service_account_id,workspace_id,project_id,billing_principal_user_id
		FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, log.RequestID, key.ID).
		Scan(&usageUserID, &usageServiceAccountID, &usageWorkspaceID, &usageProjectID, &usagePayerID))
	require.Equal(t, user.ID, usageUserID)
	require.Nil(t, usageServiceAccountID)
	require.Equal(t, *workspaceID, usageWorkspaceID)
	require.Equal(t, *projectID, usageProjectID)
	require.Equal(t, user.ID, usagePayerID)

	var reservationStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM budget_reservations WHERE id=$1`, reservation.ID).Scan(&reservationStatus))
	require.Equal(t, "finalized", reservationStatus)
}

func compatPtrString(value string) *string { return &value }
func compatPtrInt64(value int64) *int64    { return &value }
