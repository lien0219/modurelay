package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMachineLiveUsageFreezesMachineAndPayer(t *testing.T) {
	record := &LiveCallRecord{CallID: "machine-live", CallHash: hashLiveCallID("machine-live"), APIKeyID: 22, AccountID: 11, ServiceAccountID: 9, WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, LeaseID: "lease", Model: "gpt-live", CreatedAt: time.Now().Add(-time.Second), ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, json.Unmarshal([]byte(`{"BudgetReservationID":"bca013b4-8dd7-4d67-9fbc-ab14c7e5d62b"}`), record))
	store := &liveTestStore{}
	require.NoError(t, store.SaveLiveCall(context.Background(), record, time.Hour))
	repo := &liveTestUsageRepo{}
	svc := &OpenAIGatewayService{cache: store, concurrencyService: NewConcurrencyService(&liveTestConcurrencyCache{}), usageLogRepo: repo}
	svc.finalizeLiveCall(record)
	svc.finalizeLiveCall(record)
	require.Len(t, repo.logs, 1)
	usage := repo.logs[0]
	require.Zero(t, usage.UserID)
	require.Equal(t, int64(9), *usage.ServiceAccountID)
	require.Equal(t, int64(3), *usage.BillingPrincipalUserID)
	require.NotNil(t, usage.BudgetReservationID, "machine Live usage must retain its admission receipt")
	require.Equal(t, "bca013b4-8dd7-4d67-9fbc-ab14c7e5d62b", *usage.BudgetReservationID)
	require.Equal(t, float64(0), usage.ActualCost)
}

func TestHumanLiveUsagePreservesLegacyAttribution(t *testing.T) {
	record := &LiveCallRecord{UserID: 2, APIKeyID: 22, AccountID: 11, WorkspaceID: 1, ProjectID: 3, BillingPrincipalUserID: 4, CreatedAt: time.Now()}
	usage := record.UsageLog()
	require.Equal(t, int64(2), usage.UserID)
	require.Nil(t, usage.ServiceAccountID)
	require.Nil(t, usage.WorkspaceID, "a legacy Live call has no tenant reservation")
	require.Nil(t, usage.ProjectID)
	require.Nil(t, usage.BillingPrincipalUserID)
}
