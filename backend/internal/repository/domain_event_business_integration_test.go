//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func businessEvents(t *testing.T, workspaceID int64, eventType string) []service.DomainEvent {
	t.Helper()
	rows, err := integrationDB.Query(`SELECT payload FROM domain_events WHERE workspace_id=$1 AND ($2='' OR event_type=$2) ORDER BY created_at,id`, workspaceID, eventType)
	require.NoError(t, err)
	defer rows.Close()
	var events []service.DomainEvent
	for rows.Next() {
		var payload []byte
		require.NoError(t, rows.Scan(&payload))
		var event service.DomainEvent
		require.NoError(t, json.Unmarshal(payload, &event))
		require.Equal(t, 1, event.Version)
		events = append(events, event)
	}
	require.NoError(t, rows.Err())
	return events
}

func TestDomainEventPersonalWorkspaceRegistrationAndConcurrentEnsure(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx := context.Background()
	owner := mustCreateUser(t, testEntClient(t), &service.User{})
	repo := NewWorkspaceRepository(integrationDB)
	w, err := repo.EnsurePersonalWorkspace(ctx, owner.ID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := repo.EnsurePersonalWorkspace(ctx, owner.ID); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	events := businessEvents(t, w.ID, service.EventWorkspaceCreated)
	require.Len(t, events, 1, "registration trigger and concurrent ensures produce one creation event")
	require.Equal(t, owner.ID, *events[0].ActorUserID)
	require.Equal(t, "Personal", events[0].Data["name"])

	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()
	var userID, workspaceID int64
	require.NoError(t, tx.QueryRow(`INSERT INTO users(email,password_hash) VALUES($1,'test-only') RETURNING id`, uuid.NewString()+"@rollback.test").Scan(&userID))
	require.NoError(t, tx.QueryRow(`SELECT id FROM workspaces WHERE owner_user_id=$1 AND type='personal'`, userID).Scan(&workspaceID))
	var count int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1`, workspaceID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, tx.Rollback())
	require.Empty(t, businessEvents(t, workspaceID, ""), "registration rollback also removes outbox event")
}

func TestDomainEventProjectKeyMutationsKeepSecretsOutAndRollback(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, ws, owner, w := workspaceFixture(t)
	p, err := ws.CreateProject(ctx, owner.ID, w.ID, service.ProjectInput{Name: "Keys", Slug: "keys"})
	require.NoError(t, err)
	client := testEntClient(t)
	r := NewAPIKeyRepository(client, integrationDB)
	keys := service.NewAPIKeyService(r, NewUserRepository(client, integrationDB), NewGroupRepository(client, integrationDB), NewUserSubscriptionRepository(client), nil, nil, &config.Config{})
	keys.ConfigureWorkspaces(NewWorkspaceRepository(integrationDB))
	k, err := keys.CreateForProject(ctx, owner.ID, w.ID, p.ID, service.CreateAPIKeyRequest{Name: "Original"})
	require.NoError(t, err)
	name := "Renamed"
	_, err = keys.UpdateForProject(ctx, owner.ID, w.ID, p.ID, k.ID, service.UpdateAPIKeyRequest{Name: &name})
	require.NoError(t, err)
	rollback := errors.New("rollback key mutation")
	raw := r.(*apiKeyRepository)
	require.ErrorIs(t, raw.WithProjectKeyMutation(ctx, owner.ID, w.ID, p.ID, "key.update", func(txctx context.Context) error {
		require.NoError(t, clientFromContext(txctx, client).APIKey.UpdateOneID(k.ID).SetName("Rolled back").Exec(txctx))
		return rollback
	}), rollback)
	require.NoError(t, keys.DeleteForProject(ctx, owner.ID, w.ID, p.ID, k.ID))
	for _, typ := range []string{service.EventAPIKeyCreated, service.EventAPIKeyUpdated, service.EventAPIKeyRevoked} {
		events := businessEvents(t, w.ID, typ)
		require.Len(t, events, 1)
		require.Equal(t, p.ID, *events[0].ProjectID)
		require.Equal(t, float64(k.ID), events[0].Data["key_id"])
		require.Len(t, events[0].Data, 3)
		payload, err := json.Marshal(events[0])
		require.NoError(t, err)
		require.NotContains(t, string(payload), k.Key)
	}
	require.Equal(t, "Original", businessEvents(t, w.ID, service.EventAPIKeyCreated)[0].Data["key_name"])
	require.Equal(t, "Renamed", businessEvents(t, w.ID, service.EventAPIKeyUpdated)[0].Data["key_name"])
}

func TestDomainEventHardBudgetFirstRejectHasNoGhostAndNoStorm(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, amount := range []float64{0, 1} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			ctx, repo, a := budgetFixture(t)
			budgetPolicy(t, a, 100, amount, true)
			for i := 0; i < 20; i++ {
				res, err := repo.Reserve(ctx, a, uuid.NewString(), 2)
				require.ErrorIs(t, err, service.ErrProjectBudgetExceeded)
				require.Nil(t, res)
			}
			events := businessEvents(t, a.WorkspaceID, service.EventBudgetHardLimit)
			require.Len(t, events, 1)
			require.Equal(t, "admission_rejected", events[0].Data["reason_code"])
			var counters int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_counters WHERE workspace_scope_id=$1 OR project_scope_id=$2`, a.WorkspaceID, a.ProjectID).Scan(&counters))
			require.Zero(t, counters)
		})
	}
}

func TestDomainEventBudgetSpendCrossingsRevisionAndRelease(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 1000, 100, true)
	for i, step := range []struct {
		cost   float64
		events int
	}{{49, 0}, {2, 1}, {19, 1}, {11, 2}, {19, 3}} {
		res, err := repo.Reserve(ctx, a, uuid.NewString(), step.cost)
		require.NoError(t, err)
		require.NoError(t, repo.Finalize(ctx, res.ID, step.cost))
		require.NoError(t, repo.Finalize(ctx, res.ID, step.cost))
		var count int
		require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND project_id=$2 AND event_type IN ('budget.threshold_reached','budget.hard_limit_reached')`, a.WorkspaceID, a.ProjectID).Scan(&count))
		require.Equal(t, step.events, count, "step %d", i)
	}
	for i := 0; i < 20; i++ {
		_, err := repo.Reserve(ctx, a, uuid.NewString(), 1)
		require.ErrorIs(t, err, service.ErrProjectBudgetExceeded)
	}
	require.Len(t, businessEvents(t, a.WorkspaceID, service.EventBudgetHardLimit), 1)
	wr := NewWorkspaceRepository(integrationDB).(service.FinOpsRepository)
	_, err := wr.SetBudget(ctx, a.BillingPrincipalUserID, service.FinOpsScope{WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID}, service.BudgetPolicyInput{Amount: 200, HardLimit: true, Enabled: true, Timezone: "UTC"})
	require.NoError(t, err)
	var seeded int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM budget_alert_transitions WHERE scope_type='project' AND scope_id=$1 AND policy_revision=2 AND threshold=50`, a.ProjectID).Scan(&seeded))
	require.Equal(t, 1, seeded, "policy revision seeds already crossed finalized spend without emitting historical alerts")
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 60)
	require.NoError(t, err)
	require.NoError(t, repo.Finalize(ctx, res.ID, 60))
	var revisionCount int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND project_id=$2 AND event_type='budget.threshold_reached' AND payload->'data'->>'policy_revision'='2'`, a.WorkspaceID, a.ProjectID).Scan(&revisionCount))
	require.Equal(t, 1, revisionCount, "only new 80 percent crossing under revised policy")
	res, err = repo.Reserve(ctx, a, uuid.NewString(), 30)
	require.NoError(t, err)
	require.NoError(t, repo.Release(ctx, res.ID))
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM domain_events WHERE workspace_id=$1 AND project_id=$2 AND event_type='budget.threshold_reached' AND payload->'data'->>'policy_revision'='2'`, a.WorkspaceID, a.ProjectID).Scan(&revisionCount))
	require.Equal(t, 1, revisionCount, "released reservation does not emit spending alert")
}

func TestDomainEventSoftBudgetDoesNotBlockAndEmitsLimit(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, repo, a := budgetFixture(t)
	budgetPolicy(t, a, 1000, 100, false)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 101)
	require.NoError(t, err)
	require.NoError(t, repo.Finalize(ctx, res.ID, 101))
	require.Len(t, businessEvents(t, a.WorkspaceID, service.EventBudgetSoftLimit), 1)
}

func TestDomainEventBudgetConcurrentCrossingAndNewMonth(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a := budgetFixture(t)
	budgetPolicy(t, a, 1000, 100, true)
	now := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	repo := &budgetRepository{db: integrationDB, now: func() time.Time { return now }}
	first, err := repo.Reserve(ctx, a, uuid.NewString(), 49)
	require.NoError(t, err)
	require.NoError(t, repo.Finalize(ctx, first.ID, 49))
	left, err := repo.Reserve(ctx, a, uuid.NewString(), 1)
	require.NoError(t, err)
	right, err := repo.Reserve(ctx, a, uuid.NewString(), 1)
	require.NoError(t, err)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, res := range []*service.BudgetReservation{left, right} {
		wg.Add(1)
		go func(res *service.BudgetReservation) { defer wg.Done(); errs <- repo.Finalize(ctx, res.ID, 1) }(res)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	events := businessEvents(t, a.WorkspaceID, service.EventBudgetThreshold)
	require.Len(t, events, 1)
	require.Equal(t, "2026-01-01", events[0].Data["period_start"])
	require.Equal(t, float64(1), events[0].Data["reserved"], "snapshot retains the other live hold")
	now = now.AddDate(0, 1, 0)
	res, err := repo.Reserve(ctx, a, uuid.NewString(), 51)
	require.NoError(t, err)
	require.NoError(t, repo.Finalize(ctx, res.ID, 51))
	events = businessEvents(t, a.WorkspaceID, service.EventBudgetThreshold)
	require.Len(t, events, 2, "new month has a separate threshold transition")
}

func TestDomainEventMemberSuspendedAndRemovedRecipient(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, ws, owner, w := workspaceFixture(t)
	member := workspaceJoin(t, ctx, ws, owner.ID, w.ID, "viewer")
	resolver := NewNotificationRecipientResolver(integrationDB)
	require.NoError(t, ws.UpdateMember(ctx, owner.ID, w.ID, member.ID, "viewer", "suspended"))
	events := businessEvents(t, w.ID, service.EventMemberSuspended)
	require.Len(t, events, 1)
	ids, err := resolver.Resolve(ctx, &events[0])
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{owner.ID, member.ID}, ids)
	require.NoError(t, ws.RemoveMember(ctx, owner.ID, w.ID, member.ID))
	events = businessEvents(t, w.ID, service.EventMemberRemoved)
	require.Len(t, events, 1)
	ids, err = resolver.Resolve(ctx, &events[0])
	require.NoError(t, err)
	require.ElementsMatch(t, []int64{owner.ID, member.ID}, ids)
}

func TestDomainEventTenantVideoSettlementPendingRecoveredExactlyOnce(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	for _, platform := range []string{service.PlatformSeedance, service.PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			ctx, budget, a := budgetFixture(t)
			budgetPolicy(t, a, 10, 1, true)
			res, err := budget.Reserve(ctx, a, uuid.NewString(), 1)
			require.NoError(t, err)
			account := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "settlement-" + uuid.NewString(), Platform: platform, Type: service.AccountTypeAPIKey})
			cmd := budgetBillingCommand(a, res, 2)
			cmd.RequestID = "grok-video:" + platform + ":" + uuid.NewString()
			cmd.Model, cmd.ResolvedPlatform, cmd.AccountID = "public-model", platform, account.ID
			cmd.APIKeyQuotaCost, cmd.APIKeyRateLimitCost = 2, 2
			log := &service.UsageLog{RequestID: cmd.RequestID, UserID: a.ActorUserID, APIKeyID: a.APIKeyID, AccountID: account.ID, Model: cmd.Model, VideoCount: 1, ActualCost: 2, TotalCost: 2, RateMultiplier: 1, CreatedAt: time.Now().UTC(), WorkspaceID: &a.WorkspaceID, ProjectID: &a.ProjectID, BillingPrincipalUserID: &a.BillingPrincipalUserID, BudgetReservationID: &res.ID, ResolvedPlatform: &platform}
			billing := NewUsageBillingRepository(testEntClient(t), integrationDB).(service.VideoUsageBillingRepository)
			for i := 0; i < 20; i++ {
				_, err = billing.ApplyVideoUsage(ctx, cmd, log)
				require.ErrorIs(t, err, service.ErrProjectBudgetExceeded)
			}
			pending := businessEvents(t, a.WorkspaceID, service.EventBillingPending)
			require.Len(t, pending, 1)
			require.Equal(t, "project_budget_exceeded", pending[0].Data["reason_code"])
			payload, _ := json.Marshal(pending[0])
			require.NotContains(t, string(payload), "account_id")
			var balance float64
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, a.BillingPrincipalUserID).Scan(&balance))
			require.Equal(t, float64(100), balance)
			budgetCounter(t, "project", a.ProjectID, res.ProjectPeriodStart, 0, 1)
			_, err = NewWorkspaceRepository(integrationDB).(service.FinOpsRepository).SetBudget(ctx, a.BillingPrincipalUserID, service.FinOpsScope{WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID}, service.BudgetPolicyInput{Amount: 3, Enabled: true, HardLimit: true, Timezone: "UTC"})
			require.NoError(t, err)
			// Rejecting the recovered event must roll back wallet, usage, budget
			// and recovered state on the same transaction.
			_, err = integrationDB.Exec(fmt.Sprintf(`CREATE FUNCTION reject_test_recovered_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test event insert rejected'; END $$;
			 CREATE TRIGGER reject_test_recovered_event BEFORE INSERT ON domain_events FOR EACH ROW WHEN (NEW.event_type='billing.settlement_recovered' AND NEW.workspace_id=%d) EXECUTE FUNCTION reject_test_recovered_event()`, a.WorkspaceID))
			require.NoError(t, err)
			_, err = billing.ApplyVideoUsage(ctx, cmd, log)
			require.Error(t, err)
			require.Empty(t, businessEvents(t, a.WorkspaceID, service.EventBillingRecovered))
			budgetCounter(t, "project", a.ProjectID, res.ProjectPeriodStart, 0, 1)
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, a.BillingPrincipalUserID).Scan(&balance))
			require.Equal(t, float64(100), balance)
			_, err = integrationDB.Exec(`DROP TRIGGER reject_test_recovered_event ON domain_events; DROP FUNCTION reject_test_recovered_event()`)
			require.NoError(t, err)
			var wg sync.WaitGroup
			errs := make(chan error, 20)
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					copyCmd, copyLog := *cmd, *log
					_, err := billing.ApplyVideoUsage(ctx, &copyCmd, &copyLog)
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				require.NoError(t, err)
			}
			require.Len(t, businessEvents(t, a.WorkspaceID, service.EventBillingRecovered), 1)
			require.NoError(t, integrationDB.QueryRow(`SELECT balance FROM users WHERE id=$1`, a.BillingPrincipalUserID).Scan(&balance))
			require.Equal(t, float64(98), balance)
			budgetCounter(t, "project", a.ProjectID, res.ProjectPeriodStart, 2, 0)
			var usage int
			require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, cmd.RequestID, a.APIKeyID).Scan(&usage))
			require.Equal(t, 1, usage)
		})
	}
}
