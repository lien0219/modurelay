//go:build integration

package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestServiceAccountLiveZeroCostTelemetryPersistence(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	ctx := context.Background()
	key := &service.APIKey{ID: f.key, ServiceAccountID: &f.sa, Tenant: &service.TenantContext{WorkspaceID: f.workspace, ProjectID: f.project, BillingPrincipalUserID: f.payer}}
	budget := service.NewBudgetService(NewBudgetRepository(integrationDB))
	handle, err := budget.Admit(ctx, key, "machine-live-zero", 0, true)
	require.NoError(t, err)
	require.NotNil(t, handle)
	require.NoError(t, budget.Finalize(ctx, handle.ID(), 0))
	record := &service.LiveCallRecord{BudgetReservationID: handle.ID(), ServiceAccountID: f.sa, WorkspaceID: f.workspace, ProjectID: f.project, BillingPrincipalUserID: f.payer, APIKeyID: f.key, AccountID: f.account, CallID: "machine-live-zero", CallHash: HashLiveCallID("machine-live-zero"), Model: "gpt-live", CreatedAt: time.Now().Add(-time.Second).Truncate(time.Millisecond), ExpiresAt: time.Now().Add(time.Hour).Truncate(time.Millisecond), Controller: service.LiveControllerPending}
	store := NewGatewayCache(testRedis(t)).(service.LiveCallStore)
	require.NoError(t, store.SaveLiveCall(ctx, record, time.Hour))
	fromRedis, err := store.GetLiveCall(ctx, record.CallHash)
	require.NoError(t, err)
	require.Equal(t, record, fromRedis, "sideband and observer must reload all frozen dimensions")
	// Disable after admission: historical telemetry uses the accepted receipt.
	_, err = integrationDB.Exec(`UPDATE service_accounts SET status='disabled' WHERE id=$1`, f.sa)
	require.NoError(t, err)
	repo := NewUsageLogRepository(testEntClient(t), integrationDB)
	usage := fromRedis.UsageLog()
	inserted, err := repo.Create(ctx, usage)
	require.NoError(t, err)
	require.True(t, inserted)
	loaded, err := repo.GetByID(ctx, usage.ID)
	require.NoError(t, err)
	require.Zero(t, loaded.UserID)
	require.Equal(t, &f.sa, loaded.ServiceAccountID)
	require.Equal(t, &f.payer, loaded.BillingPrincipalUserID)
	require.Equal(t, handle.ID(), *loaded.BudgetReservationID)
	require.Equal(t, service.RequestTypeLive, loaded.EffectiveRequestType())
	require.Zero(t, loaded.ActualCost)
}

func TestServiceAccountUsagePersistenceAndFinOpsSnapshots(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	reservation := f.reservation(t, "machine-usage-persist")
	_, err := integrationDB.Exec(`UPDATE budget_reservations SET status='finalized',actual=2,finalized_at=now() WHERE id=$1`, reservation)
	require.NoError(t, err)
	reservationID, platform := reservation.String(), "openai"
	at := time.Date(2026, 10, 5, 10, 30, 0, 0, time.UTC)
	log := &service.UsageLog{ServiceAccountID: &f.sa, APIKeyID: f.key, AccountID: f.account, WorkspaceID: &f.workspace, ProjectID: &f.project, BillingPrincipalUserID: &f.payer, BudgetReservationID: &reservationID, ResolvedPlatform: &platform, RequestID: "machine-usage-persist", Model: "model-test", InputTokens: 100, OutputTokens: 20, ActualCost: 2, CreatedAt: at}
	repo := NewUsageLogRepository(testEntClient(t), integrationDB)
	inserted, err := repo.Create(context.Background(), log)
	require.NoError(t, err)
	require.True(t, inserted)
	loaded, err := repo.GetByID(context.Background(), log.ID)
	require.NoError(t, err)
	require.Zero(t, loaded.UserID)
	require.Equal(t, &f.sa, loaded.ServiceAccountID)
	_, err = integrationDB.Exec(`UPDATE service_accounts SET name='Renamed',status='disabled' WHERE id=$1`, f.sa)
	require.NoError(t, err)
	wr := NewWorkspaceRepository(integrationDB)
	ws := service.NewWorkspaceService(wr)
	start, end := at.Truncate(time.Hour), at.Truncate(time.Hour).Add(time.Hour)
	overview, err := ws.GetOverview(context.Background(), f.payer, f.workspace, f.project, start, end, "UTC", f.sa)
	require.NoError(t, err)
	require.Equal(t, int64(1), overview.Summary.Requests)
	require.Equal(t, float64(2), overview.Summary.Spend)
	require.Len(t, overview.ServiceAccounts, 1)
	require.Equal(t, int64(120), overview.ServiceAccounts[0].Tokens)
	require.Equal(t, "Renamed", overview.ServiceAccounts[0].Name)
	require.Equal(t, []string{"model-test"}, overview.ServiceAccounts[0].Models)
	edges, err := ws.GetOverview(context.Background(), f.payer, f.workspace, f.project, start.Add(10*time.Minute), end.Add(5*time.Minute), "UTC", f.sa)
	require.NoError(t, err)
	require.Equal(t, overview.Summary.Spend, edges.Summary.Spend)
	other := serviceAccountSchemaFixture(t)
	_, err = ws.GetOverview(context.Background(), f.payer, f.workspace, f.project, start, end, "UTC", other.sa)
	require.Error(t, err, "cross-tenant usage filter must not disclose another machine")
	loaded, err = repo.GetByID(context.Background(), log.ID)
	require.NoError(t, err)
	require.Equal(t, &f.sa, loaded.ServiceAccountID)
}
