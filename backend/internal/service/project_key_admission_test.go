package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
)

type projectBillingCache struct{ billingCacheWorkerStub }

func (c *projectBillingCache) GetUserBalance(_ context.Context, id int64) (float64, error) {
	if id == 20 {
		return 100, nil
	}
	return 0, nil
}
func TestProjectBillingAdmissionUsesPayerBalance(t *testing.T) {
	actor := &User{ID: 10, Status: StatusActive}
	payer := &User{ID: 20, Status: StatusActive, Balance: 100}
	svc := &BillingCacheService{cache: &projectBillingCache{}, cfg: &config.Config{RunMode: config.RunModeStandard}}
	key := &APIKey{UserID: 10, User: actor, BillingPrincipal: payer}
	require.NoError(t, svc.CheckBillingEligibility(context.Background(), actor, key, nil, nil, ""))
}
func TestProjectModelsIntersectGroupAndEmptyDenies(t *testing.T) {
	k := &APIKey{Group: &Group{ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: []string{"group-*"}}}, Tenant: &TenantContext{AllowedModels: []string{"group-one", "outside"}}}
	require.True(t, k.AllowsModel("group-one"))
	require.False(t, k.AllowsModel("outside"))
	require.False(t, k.AllowsModel("group-two"))
	k.Tenant.AllowedModels = []string{}
	require.False(t, k.AllowsModel("group-one"))
	k.Tenant.AllowedModels = nil
	require.True(t, k.AllowsModel("group-two"))
	require.False(t, k.AllowsModel("outside"))
}
func TestWorkspaceEffectivePermissionsRetainHistoryWithoutWrites(t *testing.T) {
	for _, status := range []string{"suspended", "archived"} {
		ac := &WorkspaceAccess{Workspace: &Workspace{Status: status}, Member: &WorkspaceMember{Role: "owner", Status: "active"}}
		permissions := WorkspaceEffectivePermissions(ac)
		require.Contains(t, permissions, "invitation.read")
		require.Contains(t, permissions, "key.read")
		require.NotContains(t, permissions, "key.create")
		require.NotContains(t, permissions, "workspace.update")
	}
}
