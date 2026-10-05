package service

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServiceAccountVideoSnapshotFreezesMachineAndPayer(t *testing.T) {
	original := &GrokVideoPendingBilling{ServiceAccountID: 31, WorkspaceID: 4, ProjectID: 5, BillingPrincipalUserID: 7, BudgetReservationID: "accepted-hold"}
	currentID := int64(99)
	current := &APIKey{ID: 9, UserID: 22, ServiceAccountID: &currentID, ServiceAccountStatus: "disabled", Tenant: &TenantContext{WorkspaceID: 4, ProjectID: 5, BillingPrincipalUserID: 8}}
	frozen := original.ApplyTenantSnapshot(current)
	require.Equal(t, int64(31), *frozen.ServiceAccountID)
	require.Zero(t, frozen.UserID)
	require.Nil(t, frozen.User)
	require.Equal(t, int64(7), frozen.BillingUserID())
	require.Equal(t, int64(99), *current.ServiceAccountID)
	require.Equal(t, int64(-31), original.OwnershipID())
	require.Equal(t, "sa:31:9:task", grokVideoPendingBillingKey("task", frozen.VideoTaskOwnershipID(), 9))
}

func TestServiceAccountVideoSnapshotRejectsMissingTenantAndMixedActor(t *testing.T) {
	require.Error(t, validateGrokVideoPendingBillingTenant(&GrokVideoPendingBilling{ServiceAccountID: 31}))
	require.Error(t, validateGrokVideoPendingBillingTenant(&GrokVideoPendingBilling{UserID: 22, ServiceAccountID: 31, WorkspaceID: 4, ProjectID: 5, BillingPrincipalUserID: 7, BudgetReservationID: "hold"}))
	valid := &GrokVideoPendingBilling{ServiceAccountID: 31, WorkspaceID: 4, ProjectID: 5, BillingPrincipalUserID: 7, BudgetReservationID: "hold"}
	require.NoError(t, validateGrokVideoPendingBillingTenant(valid))
	data, err := json.Marshal(valid)
	require.NoError(t, err)
	var restored GrokVideoPendingBilling
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, int64(31), restored.ServiceAccountID)
}

func TestHumanVideoOwnershipKeyUnchanged(t *testing.T) {
	require.Equal(t, "22:9:task", grokVideoPendingBillingKey("task", 22, 9))
	require.Empty(t, grokVideoPendingBillingKey("task", 0, 9))
	require.NotEqual(t, GrokMediaVideoRequestSessionHash("task", 31, 9), GrokMediaVideoRequestSessionHash("task", -31, 9))
	require.NotEmpty(t, GrokMediaVideoRequestSessionHash("task", -31, 9))
}
