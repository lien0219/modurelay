//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func budgetFixture(t *testing.T) (context.Context, service.BudgetRepository, service.BudgetAttribution) {
	t.Helper()
	ctx, ws, owner, w := workspaceFixture(t)
	actor := workspaceJoin(t, ctx, ws, owner.ID, w.ID, "developer")
	p, err := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Budget", Slug: "budget"})
	require.NoError(t, err)
	var keyID int64
	err = integrationDB.QueryRowContext(ctx, `INSERT INTO api_keys(user_id,project_id,key,name,status) VALUES($1,$2,$3,'Budget','active') RETURNING id`, actor.ID, p.ID, "sk-budget-"+uuid.NewString()).Scan(&keyID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `UPDATE users SET balance=100 WHERE id=$1`, owner.ID)
	require.NoError(t, err)
	return ctx, NewBudgetRepository(integrationDB), service.BudgetAttribution{ActorUserID: actor.ID, APIKeyID: keyID, WorkspaceID: w.ID, ProjectID: p.ID, BillingPrincipalUserID: owner.ID}
}

func budgetPolicy(t *testing.T, a service.BudgetAttribution, workspace, project float64, hard bool) {
	t.Helper()
	_, err := integrationDB.Exec(`INSERT INTO workspace_budget_policies(workspace_id,amount,hard_limit) VALUES($1,$2,$3)`, a.WorkspaceID, workspace, hard)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`INSERT INTO project_budget_policies(project_id,amount,hard_limit) VALUES($1,$2,$3)`, a.ProjectID, project, hard)
	require.NoError(t, err)
}

func budgetCounter(t *testing.T, typ string, id int64, period time.Time, spent, reserved float64) {
	t.Helper()
	var gotSpent, gotReserved float64
	err := integrationDB.QueryRow(`SELECT spent,reserved FROM budget_counters WHERE scope_type=$1 AND scope_id=$2 AND period_start=$3`, typ, id, period).Scan(&gotSpent, &gotReserved)
	require.NoError(t, err)
	require.InDelta(t, spent, gotSpent, 1e-9)
	require.InDelta(t, reserved, gotReserved, 1e-9)
}

func budgetBillingCommand(a service.BudgetAttribution, res *service.BudgetReservation, actual float64) *service.UsageBillingCommand {
	return &service.UsageBillingCommand{RequestID: "billing-" + res.RequestID, UserID: a.ActorUserID, APIKeyID: a.APIKeyID, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, BudgetReservationID: res.ID, BudgetActualCost: actual, BalanceCost: actual}
}

func TestBudgetDuplicateRequiresFullAttributionAndPendingState(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 100, 100, true)
	request := uuid.NewString()
	res, err := repo.Reserve(ctx, a, "  "+request+"  ", 2)
	require.NoError(t, err)
	duplicate, err := repo.Reserve(ctx, a, request, 2)
	require.NoError(t, err)
	require.Equal(t, res.ID, duplicate.ID)
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 0, 2)
	budgetCounter(t, "project", a.ProjectID, res.PeriodStart, 0, 2)
	duplicate, err = repo.Reserve(ctx, a, request, 3)
	require.Error(t, err, "duplicate amount must not change the admission snapshot")
	require.Nil(t, duplicate)
	changed := a
	changed.ActorUserID = a.BillingPrincipalUserID
	duplicate, err = repo.Reserve(ctx, changed, request, 2)
	require.Error(t, err, "duplicate actor must not inherit the original reservation")
	require.Nil(t, duplicate)
	require.NoError(t, repo.Release(ctx, res.ID))
	duplicate, err = repo.Reserve(ctx, a, request, 2)
	require.Error(t, err, "released reservations must not admit a new upstream request")
	require.Nil(t, duplicate)
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 0, 0)
	budgetCounter(t, "project", a.ProjectID, res.PeriodStart, 0, 0)
}

func TestBudgetReserveRevalidatesLiveIdentity(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, mutation := range []string{"key_revoke", "actor_disable", "member_suspend", "viewer_role", "workspace_suspend", "project_archive", "billing_owner", "key_creator"} {
		t.Run(mutation, func(t *testing.T) {
			ctx, repo, a := budgetFixture(t)
			budgetPolicy(t, a, 100, 100, true)
			first, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
			require.NoError(t, err)
			var query string
			var id int64
			switch mutation {
			case "key_revoke":
				query, id = `UPDATE api_keys SET deleted_at=now() WHERE id=$1`, a.APIKeyID
			case "actor_disable":
				query, id = `UPDATE users SET status='disabled' WHERE id=$1`, a.ActorUserID
			case "member_suspend":
				query, id = `UPDATE workspace_members SET status='suspended' WHERE user_id=$1`, a.ActorUserID
			case "viewer_role":
				query, id = `UPDATE workspace_members SET role='viewer' WHERE user_id=$1`, a.ActorUserID
			case "workspace_suspend":
				query, id = `UPDATE workspaces SET status='suspended' WHERE id=$1`, a.WorkspaceID
			case "project_archive":
				query, id = `UPDATE projects SET status='archived' WHERE id=$1`, a.ProjectID
			case "billing_owner":
				query, id = fmt.Sprintf(`UPDATE workspaces SET billing_owner_user_id=%d WHERE id=$1`, a.ActorUserID), a.WorkspaceID
			case "key_creator":
				query, id = fmt.Sprintf(`UPDATE api_keys SET user_id=%d WHERE id=$1`, a.BillingPrincipalUserID), a.APIKeyID
			}
			_, err = integrationDB.Exec(query, id)
			require.NoError(t, err)
			duplicate, err := repo.Reserve(ctx, a, first.RequestID, 2)
			require.Error(t, err)
			require.Nil(t, duplicate)
			res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
			require.Error(t, err)
			require.Nil(t, res)
			budgetCounter(t, "workspace", a.WorkspaceID, first.PeriodStart, 0, 2)
			budgetCounter(t, "project", a.ProjectID, first.PeriodStart, 0, 2)
			require.NoError(t, repo.Release(ctx, first.ID), "settlement uses immutable attribution even after revocation")
		})
	}
}

func TestBudgetHardTopUpRollsBackWalletAndBillingClaim(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 10, 3, true)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	cmd := budgetBillingCommand(a, res, 4)
	billing := NewUsageBillingRepository(testEntClient(t), integrationDB)
	_, err = billing.Apply(ctx, cmd)
	require.ErrorIs(t, err, service.ErrProjectBudgetExceeded)
	var balance float64
	require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, a.BillingPrincipalUserID).Scan(&balance))
	require.Equal(t, float64(100), balance)
	var claims int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&claims))
	require.Zero(t, claims)
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 0, 2)
	budgetCounter(t, "project", a.ProjectID, res.PeriodStart, 0, 2)
	cmd.BudgetActualCost, cmd.BalanceCost, cmd.RequestFingerprint = 3, 3, ""
	result, err := billing.Apply(ctx, cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	result, err = billing.Apply(ctx, cmd)
	require.NoError(t, err)
	require.False(t, result.Applied)
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 3, 0)
	budgetCounter(t, "project", a.ProjectID, res.PeriodStart, 3, 0)
	require.NoError(t, repo.Release(ctx, res.ID))
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 3, 0)
}

func TestBudgetSoftTopUpAndFinalizeIdempotency(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 1, 1, false)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	require.NoError(t, repo.Finalize(ctx, res.ID, 4))
	require.NoError(t, repo.Finalize(ctx, res.ID, 4))
	require.Error(t, repo.Finalize(ctx, res.ID, 5), "retry actual must match the committed amount")
	require.NoError(t, repo.Release(ctx, res.ID))
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 4, 0)
	budgetCounter(t, "project", a.ProjectID, res.PeriodStart, 4, 0)
	duplicate, err := repo.Reserve(ctx, a, res.RequestID, 2)
	require.Error(t, err)
	require.Nil(t, duplicate)
}

func TestBudgetReleaseRejectsMissingOrUnderflowCounter(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, mutation := range []string{"missing", "underflow"} {
		t.Run(mutation, func(t *testing.T) {
			ctx, repo, a := budgetFixture(t)
			res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
			require.NoError(t, err)
			query := `DELETE FROM budget_counters WHERE scope_type='project' AND scope_id=$1`
			if mutation == "underflow" {
				query = `UPDATE budget_counters SET reserved=1 WHERE scope_type='project' AND scope_id=$1`
			}
			_, err = integrationDB.Exec(query, a.ProjectID)
			require.NoError(t, err)
			require.Error(t, repo.Release(ctx, res.ID))
			budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 0, 2)
			var status string
			require.NoError(t, integrationDB.QueryRow(`SELECT status FROM budget_reservations WHERE id=$1`, res.ID).Scan(&status))
			require.Equal(t, "pending", status)
		})
	}
}

func TestBudgetHardLimitConcurrentReservationsAreAtomic(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 10, 10, true)
	const attempts = 20
	results := make(chan error, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := repo.Reserve(ctx, a, uuid.NewString(), 1)
			results <- err
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrWorkspaceBudgetExceeded)
		}
	}
	require.Equal(t, 10, successes)
	var reservations int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_reservations WHERE workspace_id=$1`, a.WorkspaceID).Scan(&reservations))
	require.Equal(t, 10, reservations)
}

func TestBudgetReleaseAndFinalizeRaceHasOneEffect(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); <-start; results <- repo.Release(ctx, res.ID) }()
	go func() { defer wg.Done(); <-start; results <- repo.Finalize(ctx, res.ID, 1) }()
	close(start)
	wg.Wait()
	close(results)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM budget_reservations WHERE id=$1`, res.ID).Scan(&status))
	spent := float64(0)
	if status == "finalized" {
		spent = 1
	} else {
		require.Equal(t, "released", status)
	}
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, spent, 0)
	budgetCounter(t, "project", a.ProjectID, res.PeriodStart, spent, 0)
	for err := range results {
		if status == "finalized" {
			require.NoError(t, err)
		}
	}
}

func TestBudgetSchemaDefendsCountersAndPolicies(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, corrupt := range []string{"negative_spend", "negative_reserved", "nan_spend", "orphan_scope", "nan_policy"} {
		t.Run(corrupt, func(t *testing.T) {
			_, _, a := budgetFixture(t)
			query := `INSERT INTO budget_counters(scope_type,scope_id,period_start,spent) VALUES('workspace',$1,'2026-03-01',-1)`
			id := a.WorkspaceID
			switch corrupt {
			case "negative_reserved":
				query = `INSERT INTO budget_counters(scope_type,scope_id,period_start,reserved) VALUES('workspace',$1,'2026-03-01',-1)`
			case "nan_spend":
				query = `INSERT INTO budget_counters(scope_type,scope_id,period_start,spent) VALUES('workspace',$1,'2026-03-01','NaN')`
			case "orphan_scope":
				id = 1 << 50
			case "nan_policy":
				query = `INSERT INTO workspace_budget_policies(workspace_id,amount) VALUES($1,'NaN')`
			}
			_, err := integrationDB.Exec(query, id)
			require.Error(t, err, "database constraints must defend ledger invariants")
		})
	}
}

func TestBudgetQuotaSettlementDoesNotWaitForAdmissionScopeLock(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	billingTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer billingTx.Rollback()
	// Subscription and quota-only settlement can lock the key without locking
	// the wallet user. A concurrent admission then owns its scope before waiting
	// for this same key; settlement must not acquire that scope lock in reverse.
	_, err = billingTx.Exec(`UPDATE api_keys SET quota_used=quota_used+1 WHERE id=$1`, a.APIKeyID)
	require.NoError(t, err)
	admissionTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer admissionTx.Rollback()
	require.NoError(t, lockBudgetAdmissionUsers(ctx, admissionTx, a))
	require.NoError(t, lockBudgetScopes(ctx, admissionTx, a.WorkspaceID, a.ProjectID, &a))
	settleCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	err = finalizeTenantBudgetTx(settleCtx, billingTx, budgetBillingCommand(a, res, 1))
	require.NoError(t, err, "key-to-workspace lock inversion must not deadlock quota settlement")
	require.NoError(t, billingTx.Commit())
	require.NoError(t, admissionTx.Rollback())
	budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, 1, 0)
}

func TestBudgetMonthBoundaryUsesIndependentScopePeriods(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a := budgetFixture(t)
	budgetPolicy(t, a, 10, 10, true)
	_, err := integrationDB.Exec(`UPDATE project_budget_policies SET timezone='America/Los_Angeles' WHERE project_id=$1`, a.ProjectID)
	require.NoError(t, err)
	now := time.Date(2026, 3, 1, 0, 30, 0, 0, time.UTC)
	repo := &budgetRepository{db: integrationDB, now: func() time.Time { return now }}
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
	require.NoError(t, err)
	workspacePeriod := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	projectPeriod := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	require.Equal(t, workspacePeriod, res.PeriodStart)
	require.Equal(t, projectPeriod, res.ProjectPeriodStart)
	require.Equal(t, workspacePeriod.AddDate(0, 1, 0), res.PeriodEnd)
	require.Equal(t, projectPeriod.AddDate(0, 1, 0), res.ProjectPeriodEnd)
	budgetCounter(t, "workspace", a.WorkspaceID, workspacePeriod, 0, 2)
	budgetCounter(t, "project", a.ProjectID, projectPeriod, 0, 2)
	// Delayed completion stays in each admission month even after policy edits.
	now = now.AddDate(0, 2, 0)
	_, err = integrationDB.Exec(`UPDATE project_budget_policies SET timezone='Asia/Shanghai' WHERE project_id=$1`, a.ProjectID)
	require.NoError(t, err)
	require.NoError(t, repo.Finalize(ctx, res.ID, 3))
	budgetCounter(t, "workspace", a.WorkspaceID, workspacePeriod, 3, 0)
	budgetCounter(t, "project", a.ProjectID, projectPeriod, 3, 0)
}

func TestBudgetConcurrentDuplicateCreatesOneReservation(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	requestID := uuid.NewString()
	const attempts = 12
	results := make(chan *service.BudgetReservation, attempts)
	errors := make(chan error, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, err := repo.Reserve(ctx, a, requestID, 1)
			results <- res
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
	var first *service.BudgetReservation
	for res := range results {
		if first == nil {
			first = res
		}
		require.Equal(t, first.ID, res.ID)
	}
	budgetCounter(t, "workspace", a.WorkspaceID, first.PeriodStart, 0, 1)
	budgetCounter(t, "project", a.ProjectID, first.ProjectPeriodStart, 0, 1)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_reservations WHERE workspace_id=$1`, a.WorkspaceID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestBudgetConcurrentTopUpsDoNotOverrunHardCapacity(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 3, 3, true)
	first, err := repo.Reserve(ctx, a, uuid.NewString(), 1)
	require.NoError(t, err)
	second, err := repo.Reserve(ctx, a, uuid.NewString(), 1)
	require.NoError(t, err)
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, res := range []*service.BudgetReservation{first, second} {
		wg.Add(1)
		go func(res *service.BudgetReservation) {
			defer wg.Done()
			<-start
			results <- repo.Finalize(ctx, res.ID, 2)
		}(res)
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			require.ErrorIs(t, err, service.ErrWorkspaceBudgetExceeded)
		}
	}
	require.Equal(t, 1, success)
	budgetCounter(t, "workspace", a.WorkspaceID, first.PeriodStart, 2, 1)
	budgetCounter(t, "project", a.ProjectID, first.ProjectPeriodStart, 2, 1)
	var pending int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_reservations WHERE workspace_id=$1 AND status='pending'`, a.WorkspaceID).Scan(&pending))
	require.Equal(t, 1, pending)
}

func TestBudgetNewBillingClaimCannotReuseTerminalReservation(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, terminal := range []string{"released", "finalized"} {
		t.Run(terminal, func(t *testing.T) {
			ctx, repo, a := budgetFixture(t)
			res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
			require.NoError(t, err)
			billing := NewUsageBillingRepository(testEntClient(t), integrationDB)
			spent := float64(0)
			if terminal == "finalized" {
				_, err = billing.Apply(ctx, budgetBillingCommand(a, res, 1))
				require.NoError(t, err)
				spent = 1
			} else {
				require.NoError(t, repo.Release(ctx, res.ID))
			}
			cmd := budgetBillingCommand(a, res, 1)
			cmd.RequestID += "-new"
			_, err = billing.Apply(ctx, cmd)
			require.ErrorIs(t, err, service.ErrBudgetReservationClosed)
			var balance float64
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, a.BillingPrincipalUserID).Scan(&balance))
			require.Equal(t, 100-spent, balance)
			var claims int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_billing_dedup WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&claims))
			require.Zero(t, claims)
			budgetCounter(t, "workspace", a.WorkspaceID, res.PeriodStart, spent, 0)
			budgetCounter(t, "project", a.ProjectID, res.ProjectPeriodStart, spent, 0)
		})
	}
}

func TestBudgetProjectRejectionLeavesNoWorkspaceGhost(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 10, 1, true)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
	require.ErrorIs(t, err, service.ErrProjectBudgetExceeded)
	require.Nil(t, res)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_counters WHERE (scope_type='workspace' AND scope_id=$1) OR (scope_type='project' AND scope_id=$2)`, a.WorkspaceID, a.ProjectID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_reservations WHERE workspace_id=$1`, a.WorkspaceID).Scan(&count))
	require.Zero(t, count)
}
