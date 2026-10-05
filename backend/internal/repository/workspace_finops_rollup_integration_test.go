//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceFinOpsRollupHistoryAndTimezone(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	client := testEntClient(t)
	owner := mustCreateUser(t, client, &service.User{Balance: 100})
	repo := NewWorkspaceRepository(integrationDB)
	ws := service.NewWorkspaceService(repo)
	w, err := ws.EnsurePersonalWorkspace(ctx, owner.ID)
	require.NoError(t, err)
	var projectID int64
	require.NoError(t, integrationDB.QueryRow(`SELECT id FROM projects WHERE workspace_id=$1 AND is_default`, w.ID).Scan(&projectID))
	key, err := client.APIKey.Create().SetUserID(owner.ID).SetProjectID(projectID).SetName("FinOps").SetKey(fmt.Sprintf("finops-%d", owner.ID)).SetStatus(service.StatusActive).Save(ctx)
	require.NoError(t, err)
	account := mustCreateAccount(t, client, &service.Account{Name: fmt.Sprintf("finops-account-%d", owner.ID)})
	logs := NewUsageLogRepository(client, integrationDB)
	budgets := NewBudgetRepository(integrationDB)
	start := time.Date(2026, 10, 4, 23, 59, 0, 0, time.UTC)
	platform := service.PlatformOpenAI
	var first *service.UsageLog
	for i, event := range []struct {
		at    time.Time
		spend float64
	}{{start.Add(30 * time.Second), 1.25}, {start.Add(75 * time.Second), 2.5}} {
		r, err := budgets.Reserve(ctx, service.BudgetAttribution{WorkspaceID: w.ID, ProjectID: projectID, BillingPrincipalUserID: owner.ID, ActorUserID: owner.ID, APIKeyID: key.ID}, fmt.Sprintf("finops-request-%d-%d", owner.ID, i), event.spend)
		require.NoError(t, err)
		log := &service.UsageLog{UserID: owner.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: fmt.Sprintf("finops-log-%d-%d", owner.ID, i), Model: "gpt-5.2", ActualCost: event.spend, TotalCost: event.spend, CreatedAt: event.at, WorkspaceID: &w.ID, ProjectID: &projectID, BillingPrincipalUserID: &owner.ID, BudgetReservationID: &r.ID, ResolvedPlatform: &platform}
		inserted, err := logs.Create(ctx, log)
		require.NoError(t, err)
		require.True(t, inserted)
		if i == 0 {
			first = log
			inserted, err = logs.Create(ctx, log)
			require.NoError(t, err)
			require.False(t, inserted)
		}
	}
	var rolledRequests int64
	var rolledSpend float64
	require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(SUM(request_count),0),COALESCE(SUM(actual_cost),0) FROM usage_tenant_daily_rollups WHERE workspace_id=$1`, w.ID).Scan(&rolledRequests, &rolledSpend))
	require.Equal(t, int64(2), rolledRequests, "duplicate usage inserts must not increment persistent rollups")
	require.Equal(t, 3.75, rolledSpend)
	finops := repo.(service.FinOpsRepository)
	overview, err := finops.GetOverview(ctx, service.FinOpsScope{WorkspaceID: w.ID}, start, start.Add(2*time.Minute), "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, int64(2), overview.Summary.Requests)
	require.Equal(t, 3.75, overview.Summary.Spend)
	payload, err := json.Marshal(overview)
	require.NoError(t, err)
	var response struct {
		Daily []struct {
			Date     string
			Requests int64
			Spend    float64
		} `json:"daily_spend"`
	}
	require.NoError(t, json.Unmarshal(payload, &response))
	require.Len(t, response.Daily, 1)
	require.Equal(t, "2026-10-05", response.Daily[0].Date)
	require.Equal(t, int64(2), response.Daily[0].Requests)
	require.Equal(t, 3.75, response.Daily[0].Spend)
	partial, err := finops.GetUsageSummary(ctx, service.FinOpsScope{WorkspaceID: w.ID, ProjectID: projectID}, start.Add(40*time.Second), start.Add(100*time.Second), "UTC")
	require.NoError(t, err)
	require.Equal(t, int64(1), partial.Requests)
	require.Equal(t, 2.5, partial.Spend)
	_, err = integrationDB.Exec(`UPDATE usage_logs SET project_id=NULL WHERE request_id=$1 AND api_key_id=$2`, first.RequestID, key.ID)
	require.Error(t, err, "historical tenant attribution cannot be rewritten")
}

func TestWorkspaceFinOpsRollupSettlementRollbackAndNonHourlyTimezone(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, billing, a, _, cmd, log := tenantUsageFixture(t)
	// Kathmandu midnight splits a UTC hour at :15. Daily views must retain that
	// exact split rather than assigning the whole hour to one local day.
	log.CreatedAt = time.Date(2026, 10, 4, 18, 14, 59, 0, time.UTC)
	result, err := billing.ApplyTenantUsage(ctx, cmd, log)
	require.NoError(t, err)
	require.True(t, result.Applied)
	logs := NewUsageLogRepository(testEntClient(t), integrationDB)
	second := *log
	second.RequestID += "-second"
	second.CreatedAt = log.CreatedAt.Add(2 * time.Second)
	second.ActualCost = 2
	inserted, err := logs.Create(ctx, &second)
	require.NoError(t, err)
	require.True(t, inserted)
	finops := NewWorkspaceRepository(integrationDB).(service.FinOpsRepository)
	start := log.CreatedAt.Truncate(time.Hour)
	end := start.Add(2 * time.Hour)
	view, err := finops.GetOverview(ctx, service.FinOpsScope{WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID}, start, end, "Asia/Kathmandu")
	require.NoError(t, err)
	require.Equal(t, int64(2), view.Summary.Requests)
	require.Equal(t, float64(3), view.Summary.Spend)
	require.Len(t, view.DailySpend, 2)
	require.Equal(t, "2026-10-04", view.DailySpend[0].Date)
	require.Equal(t, float64(1), view.DailySpend[0].Spend)
	require.Equal(t, "2026-10-05", view.DailySpend[1].Date)
	require.Equal(t, float64(2), view.DailySpend[1].Spend)
	require.Len(t, view.APIKeys, 1)
	require.Equal(t, int64(2), view.APIKeys[0].Requests)
	require.Equal(t, float64(3), view.APIKeys[0].Spend)

	tx := testTx(t)
	_, err = tx.Exec(`UPDATE usage_logs SET actual_cost=4 WHERE request_id=$1 AND api_key_id=$2`, second.RequestID, a.APIKeyID)
	require.NoError(t, err)
	var requests int64
	var spend float64
	require.NoError(t, tx.QueryRow(`SELECT SUM(request_count),SUM(actual_cost) FROM usage_tenant_hourly_rollups WHERE workspace_id=$1`, a.WorkspaceID).Scan(&requests, &spend))
	require.Equal(t, int64(2), requests)
	require.Equal(t, float64(5), spend)
	require.NoError(t, tx.Rollback())

	view, err = finops.GetOverview(ctx, service.FinOpsScope{WorkspaceID: a.WorkspaceID}, start, end, "UTC")
	require.NoError(t, err)
	require.Equal(t, float64(3), view.Summary.Spend, "rolled-back corrections must not publish aggregate spend")
	_, err = integrationDB.Exec(`DELETE FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, second.RequestID, a.APIKeyID)
	require.NoError(t, err)
	view, err = finops.GetOverview(ctx, service.FinOpsScope{WorkspaceID: a.WorkspaceID}, start, end, "UTC")
	require.NoError(t, err)
	require.Equal(t, int64(1), view.Summary.Requests)
	require.Equal(t, float64(1), view.Summary.Spend)
}

func TestWorkspaceUsageDatabaseRejectsPartialAndForeignSnapshots(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	_, _, _, _, _, log := tenantUsageFixture(t)
	_, _, foreign := budgetFixture(t)
	for _, change := range []string{"partial", "workspace", "project", "principal", "actor", "reservation"} {
		t.Run(change, func(t *testing.T) {
			row := *log
			switch change {
			case "partial":
				row.ProjectID = nil
			case "workspace":
				row.WorkspaceID = &foreign.WorkspaceID
			case "project":
				row.ProjectID = &foreign.ProjectID
			case "principal":
				row.BillingPrincipalUserID = &foreign.BillingPrincipalUserID
			case "actor":
				row.UserID = foreign.ActorUserID
			case "reservation":
				row.BudgetReservationID = nil
			}
			query, args := buildUsageLogInsertQuery([]usageLogInsertPrepared{prepareUsageLogInsert(&row)}, "ON CONFLICT (request_id, api_key_id) DO NOTHING")
			_, err := integrationDB.Exec(query, args...)
			require.Error(t, err, "database must reject inconsistent tenant history")
		})
	}
}

func TestWorkspaceBudgetViewUsesLocalMonthAndDisabledPolicy(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, billing, a, _, cmd, log := tenantUsageFixture(t)
	_, err := billing.ApplyTenantUsage(ctx, cmd, log)
	require.NoError(t, err)
	finops := NewWorkspaceRepository(integrationDB).(service.FinOpsRepository)
	scope := service.FinOpsScope{WorkspaceID: a.WorkspaceID}
	view, err := finops.GetBudget(ctx, scope)
	require.NoError(t, err)
	require.False(t, view.OverBudget, "an absent policy is not an exhausted zero-dollar budget")
	view, err = finops.SetBudget(ctx, a.BillingPrincipalUserID, scope, service.BudgetPolicyInput{Amount: .5, Enabled: true, Timezone: "Asia/Shanghai"})
	require.NoError(t, err)
	local, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Now().In(local)
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, local)
	require.True(t, view.PeriodStart.Equal(start))
	require.True(t, view.PeriodEnd.Equal(start.AddDate(0, 1, 0)))
	require.Equal(t, view.Spent > view.Policy.Amount, view.OverBudget)
}

type workspaceBudgetInvalidationRecorder struct{ workspaces []int64 }

func (r *workspaceBudgetInvalidationRecorder) InvalidateWorkspaceAuth(_ context.Context, workspaceID int64) {
	r.workspaces = append(r.workspaces, workspaceID)
}

func TestWorkspaceBudgetMutationInvalidatesOnlyCommittedChanges(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a := budgetFixture(t)
	svc := service.NewWorkspaceService(NewWorkspaceRepository(integrationDB))
	recorder := &workspaceBudgetInvalidationRecorder{}
	svc.SetKeyInvalidator(recorder)
	policy := service.BudgetPolicyInput{Amount: 10, HardLimit: true, Enabled: true, Timezone: "UTC"}
	_, err := svc.SetBudget(ctx, a.ActorUserID, a.WorkspaceID, a.ProjectID, policy)
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	require.Empty(t, recorder.workspaces)
	view, err := svc.SetBudget(ctx, a.BillingPrincipalUserID, a.WorkspaceID, a.ProjectID, policy)
	require.NoError(t, err)
	require.Equal(t, policy.Amount, view.Policy.Amount)
	require.Equal(t, []int64{a.WorkspaceID}, recorder.workspaces, "committed policy changes must explicitly invalidate admission snapshots")
}

func TestWorkspaceFinOpsBudgetRepositoryEnforcesTenantAndPermission(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a := budgetFixture(t)
	_, _, foreign := budgetFixture(t)
	finops := NewWorkspaceRepository(integrationDB).(service.FinOpsRepository)
	_, err := finops.GetBudget(ctx, service.FinOpsScope{WorkspaceID: a.WorkspaceID, ProjectID: foreign.ProjectID})
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	_, err = finops.SetBudget(ctx, a.ActorUserID, service.FinOpsScope{WorkspaceID: a.WorkspaceID}, service.BudgetPolicyInput{Amount: 5, Enabled: true, Timezone: "UTC"})
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden, "transaction rechecks the actor's budget permission")
	_, err = integrationDB.Exec(`UPDATE workspaces SET status='suspended' WHERE id=$1`, a.WorkspaceID)
	require.NoError(t, err)
	_, err = finops.SetBudget(ctx, a.BillingPrincipalUserID, service.FinOpsScope{WorkspaceID: a.WorkspaceID}, service.BudgetPolicyInput{Amount: 5, Enabled: true, Timezone: "UTC"})
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
}

func TestWorkspaceFinOpsMixedScopeCannotCountForeignKeys(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, local := budgetFixture(t)
	_, _, foreign := budgetFixture(t)
	finops := NewWorkspaceRepository(integrationDB).(service.FinOpsRepository)
	now := time.Now().UTC()
	view, err := finops.GetUsageSummary(ctx, service.FinOpsScope{WorkspaceID: local.WorkspaceID, ProjectID: foreign.ProjectID}, now.Add(-time.Hour), now, "UTC")
	require.NoError(t, err)
	require.Zero(t, view.APIKeys, "project statistics must constrain both workspace and project IDs")
}
