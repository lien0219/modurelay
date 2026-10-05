//go:build integration

package repository

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func tenantUsageFixture(t *testing.T) (context.Context, service.TenantUsageBillingRepository, service.BudgetAttribution, *service.BudgetReservation, *service.UsageBillingCommand, *service.UsageLog) {
	t.Helper()
	ctx, budget, a := budgetFixture(t)
	res, err := budget.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "tenant-usage-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformOpenAI})
	cmd := budgetBillingCommand(a, res, 1)
	cmd.AccountID, cmd.AccountType, cmd.Model, cmd.ResolvedPlatform = account.ID, service.AccountTypeAPIKey, "gpt-5", service.PlatformOpenAI
	log := &service.UsageLog{
		RequestID: cmd.RequestID, UserID: cmd.UserID, APIKeyID: cmd.APIKeyID, AccountID: cmd.AccountID,
		WorkspaceID: &a.WorkspaceID, ProjectID: &a.ProjectID, BillingPrincipalUserID: &a.BillingPrincipalUserID,
		BudgetReservationID: &res.ID, ResolvedPlatform: &cmd.ResolvedPlatform, Model: cmd.Model,
		ActualCost: 1, TotalCost: 1, RateMultiplier: 1, CreatedAt: time.Now().UTC(),
	}
	repo, ok := NewUsageBillingRepository(testEntClient(t), integrationDB).(service.TenantUsageBillingRepository)
	require.True(t, ok, "tenant money and FinOps usage must expose atomic persistence")
	return ctx, repo, a, res, cmd, log
}

func assertTenantUsageTotals(t *testing.T, a service.BudgetAttribution, res *service.BudgetReservation, cmd *service.UsageBillingCommand, balance, spent, reserved float64, logs, claims int) {
	t.Helper()
	var gotBalance float64
	var gotLogs, gotClaims int
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, a.BillingPrincipalUserID).Scan(&gotBalance))
	require.Equal(t, balance, gotBalance)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&gotLogs))
	require.Equal(t, logs, gotLogs)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&gotClaims))
	require.Equal(t, claims, gotClaims)
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, spent, reserved)
	budgetCounter(t, "project", a.ProjectID, res.ProjectPeriodStart, spent, reserved)
}

func TestBudgetTenantUsageInsertFailureRollsBackMoneyAndBudget(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a, res, cmd, log := tenantUsageFixture(t)
	invalid := strings.Repeat("x", 513)
	log.UserAgent = &invalid
	_, err := repo.ApplyTenantUsage(ctx, cmd, log)
	require.Error(t, err)
	assertTenantUsageTotals(t, a, res, cmd, 100, 0, 2, 0, 0)
	log.UserAgent = nil
	result, err := repo.ApplyTenantUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.True(t, result.TenantUsageLogPersisted)
	result, err = repo.ApplyTenantUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.True(t, result.TenantUsageLogPersisted)
	assertTenantUsageTotals(t, a, res, cmd, 99, 1, 0, 1, 1)
}

func TestBudgetTenantUsageRejectsForeignOrConflictingPaidRow(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, conflict := range []string{"amount", "actor", "principal", "reservation", "platform"} {
		t.Run(conflict, func(t *testing.T) {
			ctx, repo, a, res, cmd, log := tenantUsageFixture(t)
			foreign := *log
			switch conflict {
			case "amount":
				foreign.ActualCost = .5
			case "actor":
				foreign.UserID = a.BillingPrincipalUserID
			case "principal":
				foreign.BillingPrincipalUserID = &a.ActorUserID
			case "reservation":
				id := uuid.NewString()
				foreign.BudgetReservationID = &id
			case "platform":
				platform := service.PlatformAnthropic
				foreign.ResolvedPlatform = &platform
			}
			query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(&foreign)}, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
			_, err := integrationDB.Exec(query, args...)
			if conflict == "actor" || conflict == "principal" || conflict == "reservation" {
				require.Error(t, err, "database rejects a usage snapshot that does not match its admission reservation")
				assertTenantUsageTotals(t, a, res, cmd, 100, 0, 2, 0, 0)
				return
			}
			require.NoError(t, err)
			var before, after []byte
			require.NoError(t, integrationDB.QueryRow(`SELECT to_jsonb(u) FROM usage_logs u WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&before))
			_, err = repo.ApplyTenantUsage(ctx, cmd, log)
			require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
			assertTenantUsageTotals(t, a, res, cmd, 100, 0, 2, 1, 0)
			require.NoError(t, integrationDB.QueryRow(`SELECT to_jsonb(u) FROM usage_logs u WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&after))
			require.JSONEq(t, string(before), string(after), "existing paid rows are immutable")
		})
	}
}

func TestBudgetTenantUsageConcurrentRetriesPersistOnePaidRow(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a, res, cmd, log := tenantUsageFixture(t)
	const attempts = 8
	results := make(chan *service.UsageBillingApplyResult, attempts)
	errors := make(chan error, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			command, row := *cmd, *log
			<-start
			result, err := repo.ApplyTenantUsage(ctx, &command, &row)
			results <- result
			errors <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	applied := 0
	for result := range results {
		require.True(t, result.TenantUsageLogPersisted)
		if result.Applied {
			applied++
		}
	}
	require.Equal(t, 1, applied)
	assertTenantUsageTotals(t, a, res, cmd, 99, 1, 0, 1, 1)
}

func tenantVideoBudgetFixture(t *testing.T) (context.Context, service.VideoUsageBillingRepository, service.BudgetAttribution, *service.BudgetReservation, *service.UsageBillingCommand, *service.UsageLog) {
	t.Helper()
	ctx, budget, a := budgetFixture(t)
	res, err := budget.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "tenant-video-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformSeedance})
	cmd := budgetBillingCommand(a, res, 1)
	cmd.RequestID = "grok-video:seedance:" + res.RequestID
	cmd.AccountID, cmd.AccountType, cmd.Model, cmd.ResolvedPlatform = account.ID, service.AccountTypeAPIKey, "vendor-video", service.PlatformSeedance
	resolution, duration, mode := "720p", 5, "video"
	log := &service.UsageLog{RequestID: cmd.RequestID, UserID: cmd.UserID, APIKeyID: cmd.APIKeyID, AccountID: cmd.AccountID,
		WorkspaceID: &a.WorkspaceID, ProjectID: &a.ProjectID, BillingPrincipalUserID: &a.BillingPrincipalUserID,
		BudgetReservationID: &res.ID, ResolvedPlatform: &cmd.ResolvedPlatform, Model: cmd.Model,
		ActualCost: 1, TotalCost: 1, RateMultiplier: 1, CreatedAt: time.Now().UTC(), VideoCount: 1,
		VideoResolution: &resolution, VideoDurationSeconds: &duration, BillingMode: &mode}
	repo, ok := NewUsageBillingRepository(testEntClient(t), integrationDB).(service.VideoUsageBillingRepository)
	require.True(t, ok)
	return ctx, repo, a, res, cmd, log
}

func TestBudgetVideoLegacyPlaceholderCorrectionAddsFrozenTenantSnapshot(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a, res, cmd, log := tenantVideoBudgetFixture(t)
	legacy := *log
	legacy.ActualCost = 0
	legacy.WorkspaceID, legacy.ProjectID, legacy.BillingPrincipalUserID, legacy.BudgetReservationID, legacy.ResolvedPlatform = nil, nil, nil, nil, nil
	query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(&legacy)}, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	_, err := integrationDB.Exec(query, args...)
	require.NoError(t, err)
	execResult, execErr := integrationDB.Exec(`UPDATE usage_logs SET workspace_id=$3,project_id=$4,billing_principal_user_id=$5,budget_reservation_id=$6::uuid,resolved_platform=$7 WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID, a.WorkspaceID, a.ProjectID, a.BillingPrincipalUserID, res.ID, cmd.ResolvedPlatform)
	if execErr == nil {
		rows, rowsErr := execResult.RowsAffected()
		require.NoError(t, rowsErr)
		require.Zero(t, rows, "ordinary SQL must not silently update a legacy usage row")
	}
	err = execErr
	require.Error(t, err, "ordinary SQL must not invent tenant attribution for a legacy usage row")
	result, err := repo.ApplyVideoUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.True(t, result.VideoUsageLogPersisted)
	var frozen bool
	require.NoError(t, integrationDB.QueryRow(`SELECT workspace_id=$3 AND project_id=$4 AND billing_principal_user_id=$5 AND budget_reservation_id=$6::uuid AND resolved_platform=$7 FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID, a.WorkspaceID, a.ProjectID, a.BillingPrincipalUserID, res.ID, cmd.ResolvedPlatform).Scan(&frozen))
	require.True(t, frozen)
	assertTenantUsageTotals(t, a, res, cmd, 99, 1, 0, 1, 1)
}

func TestBudgetVideoAttributedZeroCostRowIsNeverRewritten(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a, res, cmd, log := tenantVideoBudgetFixture(t)
	existing := *log
	existing.ActualCost = 0
	query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(&existing)}, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
	_, err := integrationDB.Exec(query, args...)
	require.NoError(t, err)
	_, err = repo.ApplyVideoUsage(ctx, cmd, log)
	require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
	assertTenantUsageTotals(t, a, res, cmd, 100, 0, 2, 1, 0)
	var actual float64
	require.NoError(t, integrationDB.QueryRow(`SELECT actual_cost FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&actual))
	require.Zero(t, actual)
}

func TestBudgetVideoRejectsTenantLogSnapshotMismatch(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a, res, cmd, log := tenantVideoBudgetFixture(t)
	log.BillingPrincipalUserID = &a.ActorUserID
	_, err := repo.ApplyVideoUsage(ctx, cmd, log)
	require.Error(t, err)
	assertTenantUsageTotals(t, a, res, cmd, 100, 0, 2, 0, 0)
}
