package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestMachineAuthSnapshotPreservesPrincipalWithoutUser(t *testing.T) {
	id, project := int64(9), int64(2)
	key := &APIKey{ID: 5, ServiceAccountID: &id, ServiceAccountStatus: StatusActive, ProjectID: &project, Status: StatusActive, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}}
	svc := &APIKeyService{}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	require.NotNil(t, snapshot)
	restored := svc.snapshotToAPIKey("sk-machine", snapshot)
	require.Nil(t, restored.User)
	require.Equal(t, key.ExecutionPrincipal(), restored.ExecutionPrincipal())
	require.Equal(t, HashServiceAccountCredential("sk-machine"), restored.Key)
	wire, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NotContains(t, string(wire), `"user":`)
	require.NotContains(t, string(wire), "sk-machine")
}

func TestMachineCredentialDigestCannotUseWarmAuthSnapshot(t *testing.T) {
	svc := &APIKeyService{}
	require.Equal(t, svc.authCacheKey("sk-machine"), svc.authCacheKey(HashServiceAccountCredential("sk-machine")))
	_, err := svc.getByKeyCached(context.Background(), HashServiceAccountCredential("sk-machine"))
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestMachineUsageSnapshotDoesNotAttributePayerAsActor(t *testing.T) {
	id := int64(9)
	key := &APIKey{ID: 5, ServiceAccountID: &id, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}}
	usage := &UsageLog{UserID: 3}
	applyTenantUsageSnapshot(context.Background(), usage, key, PlatformOpenAI, "reservation")
	require.Zero(t, usage.UserID)
	require.Equal(t, &id, usage.ServiceAccountID)
	require.Equal(t, int64(3), *usage.BillingPrincipalUserID)
}

func TestMachineBudgetAdmissionAcceptsExclusiveMachinePrincipal(t *testing.T) {
	id := int64(9)
	key := &APIKey{ID: 5, ServiceAccountID: &id, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}}
	repo := &machineBudgetRepo{}
	_, err := NewBudgetService(repo).Admit(context.Background(), key, "machine-request", 1, true)
	require.NoError(t, err)
	require.Zero(t, repo.attribution.ActorUserID)
	require.Equal(t, id, repo.attribution.ServiceAccountID)
}

func TestDirectKeyBudgetAdmissionUsesUserExecutionPrincipal(t *testing.T) {
	key := &APIKey{ID: 5, UserID: 7, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 7}}
	repo := &machineBudgetRepo{}
	_, err := NewBudgetService(repo).Admit(context.Background(), key, "direct-request", 1, true)
	require.NoError(t, err)
	require.Equal(t, int64(7), repo.attribution.ActorUserID)
	require.Zero(t, repo.attribution.ServiceAccountID)
	require.Equal(t, int64(7), repo.attribution.BillingPrincipalUserID)
	require.Equal(t, int64(5), repo.attribution.APIKeyID)
}

func TestDirectKeyUsageSnapshotKeepsUserActorAndNoServiceAccount(t *testing.T) {
	key := &APIKey{ID: 5, UserID: 7, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 7}}
	usage := &UsageLog{}
	applyTenantUsageSnapshot(context.Background(), usage, key, PlatformOpenAI, "direct-reservation")
	require.Equal(t, int64(7), usage.UserID)
	require.Nil(t, usage.ServiceAccountID)
	require.Equal(t, int64(1), *usage.WorkspaceID)
	require.Equal(t, int64(2), *usage.ProjectID)
	require.Equal(t, int64(7), *usage.BillingPrincipalUserID)
	require.Equal(t, "direct-reservation", *usage.BudgetReservationID)
}

func TestDirectKeyBillingCommandKeepsUserActorAndTenantAttribution(t *testing.T) {
	key := &APIKey{ID: 5, UserID: 7, User: &User{ID: 7}, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 7}}
	cmd := buildUsageBillingCommand("direct-billing", nil, &postUsageBillingParams{
		Cost:        &CostBreakdown{ActualCost: 1},
		User:        key.User,
		BillingUser: key.User,
		APIKey:      key,
		Account:     &Account{ID: 6},
	})
	require.NotNil(t, cmd)
	require.Equal(t, int64(7), cmd.UserID)
	require.Zero(t, cmd.ServiceAccountID)
	require.Equal(t, int64(1), cmd.WorkspaceID)
	require.Equal(t, int64(2), cmd.ProjectID)
	require.Equal(t, int64(7), cmd.BillingPrincipalUserID)
}

type machineAuthRPMRepo struct {
	UserGroupRateRepository
	queriedUser int64
	value       int
}

func (r *machineAuthRPMRepo) GetRPMOverrideByUserAndGroup(_ context.Context, userID, _ int64) (*int, error) {
	r.queriedUser = userID
	return &r.value, nil
}

func TestMachineAuthRPMOverridePreservesHumanOwner(t *testing.T) {
	for _, machine := range []bool{false, true} {
		t.Run(map[bool]string{false: "human", true: "machine"}[machine], func(t *testing.T) {
			group, sa := int64(4), int64(9)
			repo := &machineAuthRPMRepo{value: 17}
			key := &APIKey{ID: 5, UserID: 2, User: &User{ID: 2}, GroupID: &group, BillingPrincipal: &User{ID: 3}, Tenant: &TenantContext{BillingPrincipalUserID: 3}}
			want := int64(2)
			if machine {
				key.UserID, key.User, key.ServiceAccountID = 0, nil, &sa
				want = 3
			}
			svc := &APIKeyService{userGroupRateRepo: repo}
			snapshot := svc.snapshotFromAPIKey(context.Background(), key)
			require.Equal(t, want, repo.queriedUser)
			if machine {
				require.Equal(t, &repo.value, snapshot.BillingPrincipal.UserGroupRPMOverride)
			} else {
				require.Equal(t, &repo.value, snapshot.User.UserGroupRPMOverride)
			}
		})
	}
}

type machineResponseOwnerCacheKey struct {
	group int64
	key   string
}
type machineResponseOwnerCache struct {
	GatewayCache
	values map[machineResponseOwnerCacheKey]int64
}

func (r *machineResponseOwnerCache) GetSessionAccountID(_ context.Context, group int64, key string) (int64, error) {
	return r.values[machineResponseOwnerCacheKey{group, key}], nil
}
func (r *machineResponseOwnerCache) SetSessionAccountID(_ context.Context, group int64, key string, id int64, _ time.Duration) error {
	r.values[machineResponseOwnerCacheKey{group, key}] = id
	return nil
}

func TestMachineResponsesContinuationOwnerIsolation(t *testing.T) {
	cache := &machineResponseOwnerCache{values: map[machineResponseOwnerCacheKey]int64{}}
	svc := &OpenAIGatewayService{cache: cache}
	ctx := context.Background()
	require.NoError(t, svc.BindOpenAIHTTPResponseOwner(ctx, 4, "resp_machine", -9, 5))
	// A fresh service must load the owner from the shared store, including the
	// machine namespace. Rotation within one machine may continue the response.
	reader := &OpenAIGatewayService{cache: cache}
	for _, tc := range []struct {
		owner, key int64
		allowed    bool
	}{
		{-9, 5, true}, {-9, 6, true}, {-10, 5, false}, {9, 5, false}, {2, 5, false}, {0, 5, false},
	} {
		allowed, err := reader.ValidateOpenAIHTTPResponseOwner(ctx, 4, "resp_machine", tc.owner, tc.key)
		require.NoError(t, err)
		require.Equal(t, tc.allowed, allowed, "owner=%d key=%d", tc.owner, tc.key)
	}
	allowed, err := reader.ValidateOpenAIHTTPResponseOwner(ctx, 7, "resp_machine", -9, 5)
	require.NoError(t, err)
	require.False(t, allowed, "group boundary must be preserved")
}

func TestMachineFastPolicySeparatesExecutionFromFunding(t *testing.T) {
	for _, tc := range []struct {
		name      string
		principal ExecutionPrincipal
		want      int64
	}{
		{"human", ExecutionPrincipal{Type: PrincipalTypeUser, UserID: 2}, 2},
		{"machine", ExecutionPrincipal{Type: PrincipalTypeServiceAccount, ServiceAccountID: 9}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithExecutionPrincipal(context.WithValue(context.Background(), ctxkey.UserID, int64(3)), tc.principal)
			userID := openAIFastPolicyUserID(ctx)
			require.Equal(t, tc.want, userID)
			require.False(t, openAIFastPolicyUserMatches([]int64{3}, userID), "payer is not the execution user")
		})
	}
	legacy := context.WithValue(context.Background(), ctxkey.UserID, int64(2))
	require.Equal(t, int64(2), openAIFastPolicyUserID(legacy))
}

func TestMachineBillingCommandSeparatesExecutionFromFunding(t *testing.T) {
	id := int64(9)
	payer := &User{ID: 3}
	key := &APIKey{ID: 5, ServiceAccountID: &id, BillingPrincipal: payer, Tenant: &TenantContext{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}}
	cmd := buildUsageBillingCommand("request", nil, &postUsageBillingParams{Cost: &CostBreakdown{ActualCost: 1}, APIKey: key, Account: &Account{ID: 6}, BillingUser: payer, BudgetReservationID: "hold"})
	require.NotNil(t, cmd)
	require.Zero(t, cmd.UserID)
	require.Equal(t, id, cmd.ServiceAccountID)
	require.Equal(t, payer.ID, cmd.BillingPrincipalUserID)
	other := *cmd
	other.ServiceAccountID = 10
	other.RequestFingerprint = ""
	other.Normalize()
	require.NotEqual(t, cmd.RequestFingerprint, other.RequestFingerprint)
}

type machineBudgetRepo struct{ attribution BudgetAttribution }

func (r *machineBudgetRepo) Reserve(_ context.Context, a BudgetAttribution, request string, estimate float64) (*BudgetReservation, error) {
	r.attribution = a
	return &BudgetReservation{ID: "reservation", RequestID: request, ServiceAccountID: a.ServiceAccountID, ActorUserID: a.ActorUserID, APIKeyID: a.APIKeyID, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, Estimate: estimate, Status: "pending"}, nil
}
func (*machineBudgetRepo) Finalize(context.Context, string, float64) error { return nil }
func (*machineBudgetRepo) Release(context.Context, string) error           { return nil }
