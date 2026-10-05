//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func workspaceWebhookFixture(t *testing.T) (context.Context, *service.WorkspaceService, *service.WorkspaceWebhookService, *workspaceWebhookRepository, *service.User, *service.Workspace) {
	t.Helper()
	isolateWorkspaceTestFixtures(t)
	ctx, workspaces, owner, workspace := workspaceFixture(t)
	repo := NewWorkspaceWebhookRepository(integrationDB).(*workspaceWebhookRepository)
	encryptor := svcEncryptorFixture(t)
	svc := service.NewWorkspaceWebhookService(repo, service.NewWorkspaceAccessService(NewWorkspaceRepository(integrationDB)), encryptor, true)
	return ctx, workspaces, svc, repo, owner, workspace
}

func createWorkspaceWebhookFixture(t *testing.T, ctx context.Context, svc *service.WorkspaceWebhookService, actorID, workspaceID int64) *service.WorkspaceWebhook {
	t.Helper()
	endpoint, secret, err := svc.Create(ctx, actorID, workspaceID, service.CreateWorkspaceWebhookInput{Name: "Test", URL: "https://8.8.8.8/events", EventTypes: []string{service.EventWorkspaceUpdated, service.EventWebhookTest}})
	require.NoError(t, err)
	require.NotEmpty(t, secret)
	return endpoint
}

func TestWorkspaceWebhookCRUDSQLNullsAndRotation(t *testing.T) {
	ctx, _, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoint, secret, err := svc.Create(ctx, owner.ID, workspace.ID, service.CreateWorkspaceWebhookInput{Name: "Initial", URL: "https://8.8.8.8/events", EventTypes: []string{service.EventWorkspaceUpdated, service.EventWebhookTest}})
	require.NoError(t, err)
	stored, err := repo.GetWebhook(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	require.NotEqual(t, secret, stored.SecretCurrentEncrypted)
	decoded, err := svcEncryptorFixture(t).Decrypt(stored.SecretCurrentEncrypted)
	require.NoError(t, err)
	require.Equal(t, secret, decoded)
	encoded, err := json.Marshal(stored)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), secret)
	require.NotContains(t, string(encoded), stored.SecretCurrentEncrypted)
	name := "Renamed"
	updated, err := svc.Update(ctx, owner.ID, workspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{Name: &name})
	require.NoError(t, err)
	require.Equal(t, name, updated.Name)
	require.ElementsMatch(t, endpoint.EventTypes, updated.EventTypes)
	newEvents := []string{service.EventWebhookTest}
	updated, err = svc.Update(ctx, owner.ID, workspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{EventTypes: &newEvents})
	require.NoError(t, err)
	require.Equal(t, newEvents, updated.EventTypes)
	rotated, newSecret, err := svc.Rotate(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	require.NotEqual(t, secret, newSecret)
	require.Equal(t, newEvents, rotated.EventTypes)
	require.NotNil(t, rotated.PreviousSecretUntil)
	require.WithinDuration(t, time.Now().Add(24*time.Hour), *rotated.PreviousSecretUntil, 2*time.Second)
	stored, err = repo.GetWebhook(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	oldPlain, err := svcEncryptorFixture(t).Decrypt(stored.SecretPreviousEncrypted)
	require.NoError(t, err)
	require.Equal(t, secret, oldPlain)
	currentPlain, err := svcEncryptorFixture(t).Decrypt(stored.SecretCurrentEncrypted)
	require.NoError(t, err)
	require.Equal(t, newSecret, currentPlain)
	delivery, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	require.Equal(t, "pending", delivery.Status)
	require.Nil(t, delivery.ResponseStatus)
	require.Empty(t, delivery.ResponsePreview)
	require.Empty(t, delivery.LastError)
	require.Nil(t, delivery.DeliveredAt)
	require.Nil(t, delivery.LastAttemptAt)
	require.Empty(t, delivery.ErrorCode)
	items, total, err := svc.Deliveries(ctx, owner.ID, workspace.ID, endpoint.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	claims, err := repo.ClaimDeliveries(ctx, "worker", 8, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, delivery.ID, claims[0].ID)
	require.NotNil(t, claims[0].LastAttemptAt)
	require.WithinDuration(t, time.Now(), *claims[0].LastAttemptAt, 2*time.Second)
	require.NoError(t, repo.MarkDeliveryDead(ctx, delivery.ID, claims[0].LockOwner, 500, "failed", "HTTP 500"))
	items, _, err = svc.Deliveries(ctx, owner.ID, workspace.ID, endpoint.ID, 1, 20)
	require.NoError(t, err)
	require.Equal(t, "delivery_terminal", items[0].ErrorCode)
	retried, err := svc.Retry(ctx, owner.ID, workspace.ID, endpoint.ID, delivery.ID)
	require.NoError(t, err)
	require.Equal(t, "retrying", retried.Status)
	require.Zero(t, retried.Attempts)
	require.Empty(t, retried.LastError)
	require.Empty(t, retried.ErrorCode)
	require.NotNil(t, retried.LastAttemptAt)
	require.Equal(t, *claims[0].LastAttemptAt, *retried.LastAttemptAt)
	claims, err = repo.ClaimDeliveries(ctx, "worker", 8, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, delivery.EventID, claims[0].EventID)
	require.NoError(t, repo.MarkDeliverySuccess(ctx, delivery.ID, claims[0].LockOwner, 204, "accepted"))
	items, _, err = svc.Deliveries(ctx, owner.ID, workspace.ID, endpoint.ID, 1, 20)
	require.NoError(t, err)
	require.Empty(t, items[0].ErrorCode)
	_, err = svc.Retry(ctx, owner.ID, workspace.ID, endpoint.ID, delivery.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound, "only dead deliveries can be retried")
	disabled := false
	_, err = svc.Update(ctx, owner.ID, workspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{Enabled: &disabled})
	require.NoError(t, err)
	_, err = svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	endpoints, err := svc.List(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	require.Len(t, endpoints, 1)
	require.False(t, endpoints[0].Enabled)
	require.NotNil(t, endpoints[0].DisabledAt)
	require.WithinDuration(t, time.Now(), *endpoints[0].DisabledAt, 2*time.Second)
	enabled := true
	resumed, err := svc.Update(ctx, owner.ID, workspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{Enabled: &enabled})
	require.NoError(t, err)
	require.Nil(t, resumed.DisabledAt)
	require.NoError(t, svc.Delete(ctx, owner.ID, workspace.ID, endpoint.ID))
	_, err = repo.GetWebhook(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
}

func svcEncryptorFixture(t *testing.T) service.SecretEncryptor {
	t.Helper()
	e, err := NewAESEncryptor(&config.Config{Totp: config.TotpConfig{EncryptionKey: strings.Repeat("42", 32)}})
	require.NoError(t, err)
	return e
}

func TestWorkspaceWebhookCrossTenantIDsAreNotFound(t *testing.T) {
	ctx, workspaces, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	delivery, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	claims, err := repo.ClaimDeliveries(ctx, "tenant-test", 1, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.NoError(t, repo.MarkDeliveryDead(ctx, delivery.ID, claims[0].LockOwner, 400, "", "HTTP 400"))
	otherEndpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	_, err = svc.Retry(ctx, owner.ID, workspace.ID, otherEndpoint.ID, delivery.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound, "delivery ID is bound to its own endpoint")
	other := mustCreateUser(t, testEntClient(t), &service.User{Email: fmt.Sprintf("webhook-other-%d@example.test", time.Now().UnixNano())})
	otherWorkspace, err := workspaces.CreateOrganization(ctx, other.ID, "Other", fmt.Sprintf("other-%d", other.ID))
	require.NoError(t, err)
	name := "Forbidden"
	operations := map[string]func() error{
		"read": func() error { _, e := repo.GetWebhook(ctx, other.ID, otherWorkspace.ID, endpoint.ID); return e },
		"update": func() error {
			_, e := svc.Update(ctx, other.ID, otherWorkspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{Name: &name})
			return e
		},
		"rotate": func() error { _, _, e := svc.Rotate(ctx, other.ID, otherWorkspace.ID, endpoint.ID); return e },
		"test":   func() error { _, e := svc.Test(ctx, other.ID, otherWorkspace.ID, endpoint.ID); return e },
		"deliveries": func() error {
			_, _, e := svc.Deliveries(ctx, other.ID, otherWorkspace.ID, endpoint.ID, 1, 20)
			return e
		},
		"retry":  func() error { _, e := svc.Retry(ctx, other.ID, otherWorkspace.ID, endpoint.ID, delivery.ID); return e },
		"delete": func() error { return svc.Delete(ctx, other.ID, otherWorkspace.ID, endpoint.ID) },
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) { require.ErrorIs(t, operation(), service.ErrWorkspaceNotFound) })
	}
	var unchanged string
	require.NoError(t, integrationDB.QueryRow(`SELECT name FROM workspace_webhooks WHERE id=$1`, endpoint.ID).Scan(&unchanged))
	require.Equal(t, endpoint.Name, unchanged)
}

func TestWorkspaceWebhookRBACServiceOperations(t *testing.T) {
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		t.Run(role, func(t *testing.T) {
			ctx, workspaces, svc, repo, owner, workspace := workspaceWebhookFixture(t)
			actor := owner
			if role != "owner" {
				actor = workspaceJoin(t, ctx, workspaces, owner.ID, workspace.ID, role)
			}
			endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
			delivery, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
			require.NoError(t, err)
			claims, err := repo.ClaimDeliveries(ctx, "rbac-test", 1, 2*time.Minute)
			require.NoError(t, err)
			require.Len(t, claims, 1)
			require.NoError(t, repo.MarkDeliveryDead(ctx, delivery.ID, claims[0].LockOwner, 400, "", "HTTP 400"))
			_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_webhooks SET last_test_at=NULL WHERE id=$1`, endpoint.ID)
			require.NoError(t, err)
			name := "RBAC update"
			full := role == "owner" || role == "admin"
			for _, operation := range []struct {
				name    string
				allowed bool
				run     func() error
			}{
				{"read", role != "viewer", func() error { _, e := svc.List(ctx, actor.ID, workspace.ID); return e }},
				{"delivery.read", full || role == "developer", func() error { _, _, e := svc.Deliveries(ctx, actor.ID, workspace.ID, endpoint.ID, 1, 20); return e }},
				{"create", full, func() error {
					_, _, e := svc.Create(ctx, actor.ID, workspace.ID, service.CreateWorkspaceWebhookInput{Name: "RBAC create", URL: "https://8.8.8.8/events", EventTypes: []string{service.EventWorkspaceUpdated}})
					return e
				}},
				{"update", full, func() error {
					_, e := svc.Update(ctx, actor.ID, workspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{Name: &name})
					return e
				}},
				{"secret.rotate", full, func() error { _, _, e := svc.Rotate(ctx, actor.ID, workspace.ID, endpoint.ID); return e }},
				{"test", full, func() error { _, e := svc.Test(ctx, actor.ID, workspace.ID, endpoint.ID); return e }},
				{"delivery.retry", full, func() error { _, e := svc.Retry(ctx, actor.ID, workspace.ID, endpoint.ID, delivery.ID); return e }},
				{"delete", full, func() error { return svc.Delete(ctx, actor.ID, workspace.ID, endpoint.ID) }},
			} {
				t.Run(operation.name, func(t *testing.T) {
					err := operation.run()
					if operation.allowed {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
					}
				})
			}
		})
	}
}

func TestWorkspaceWebhookConcurrentTestsEnforceCooldown(t *testing.T) {
	ctx, _, svc, _, owner, workspace := workspaceWebhookFixture(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			require.Equal(t, 429, infraerrors.Code(err))
		}
	}
	require.Equal(t, 1, successes)
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_webhook_deliveries WHERE webhook_id=$1`, endpoint.ID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestWorkspaceWebhookClaimBatchIsBoundedAndRetryUsesRotatedSecret(t *testing.T) {
	ctx, _, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	for i := 0; i < 24; i++ {
		event, err := service.NewDomainEvent(service.EventWorkspaceUpdated, workspace.ID, 0, owner.ID, "workspace", fmt.Sprint(workspace.ID), service.DomainEventData{"name": fmt.Sprintf("Batch %d", i)})
		require.NoError(t, err)
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		require.NoError(t, InsertDomainEventTx(ctx, tx, event, ""))
		require.NoError(t, tx.Commit())
		require.NoError(t, repo.EnqueueEventDeliveries(ctx, event))
	}
	claims, err := repo.ClaimDeliveries(ctx, "bounded", 8, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 8)
	first := claims[0]
	oldSecret, err := svcEncryptorFixture(t).Decrypt(first.SecretCurrent)
	require.NoError(t, err)
	_, newSecret, err := svc.Rotate(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	require.NotEqual(t, oldSecret, newSecret)
	require.NoError(t, repo.MarkDeliveryRetry(ctx, first.ID, first.LockOwner, time.Now().Add(-time.Minute), "timeout", 0, ""))
	reclaimed, err := repo.ClaimDeliveries(ctx, "bounded", 1, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, reclaimed, 1)
	require.Equal(t, first.ID, reclaimed[0].ID)
	current, err := svcEncryptorFixture(t).Decrypt(reclaimed[0].SecretCurrent)
	require.NoError(t, err)
	require.Equal(t, newSecret, current)
	previous, err := svcEncryptorFixture(t).Decrypt(reclaimed[0].SecretPrevious)
	require.NoError(t, err)
	require.Equal(t, oldSecret, previous)
	require.Equal(t, first.Payload, reclaimed[0].Payload)
	require.Equal(t, "delivery_retryable", reclaimed[0].ErrorCode)
}

func TestWorkspaceWebhookConcurrentEndpointLimitCountsDisabled(t *testing.T) {
	ctx, _, svc, _, owner, workspace := workspaceWebhookFixture(t)
	var wg sync.WaitGroup
	errors := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := svc.Create(ctx, owner.ID, workspace.ID, service.CreateWorkspaceWebhookInput{Name: "Concurrent", URL: "https://8.8.8.8/events", EventTypes: []string{service.EventWorkspaceUpdated}})
			errors <- err
		}()
	}
	wg.Wait()
	close(errors)
	successes := 0
	for err := range errors {
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrWorkspaceConflict)
		}
	}
	require.Equal(t, service.WorkspaceWebhookMaxEndpoints, successes)
	endpoints, err := svc.List(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	disabled := false
	_, err = svc.Update(ctx, owner.ID, workspace.ID, endpoints[0].ID, service.UpdateWorkspaceWebhookInput{Enabled: &disabled})
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, owner.ID, workspace.ID, service.CreateWorkspaceWebhookInput{Name: "Overflow", URL: "https://8.8.8.8/events", EventTypes: []string{service.EventWorkspaceUpdated}})
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id,enabled) VALUES($1,'SQL overflow','https://8.8.8.8/events','encrypted',$2,false)`, workspace.ID, owner.ID)
	require.Error(t, err, "database trigger must enforce the endpoint cap even for disabled inserts")
}

func TestWorkspaceWebhookTestCooldownAndWorkspaceRateLimit(t *testing.T) {
	ctx, _, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoints := make([]*service.WorkspaceWebhook, 0, 10)
	for i := 0; i < 10; i++ {
		endpoints = append(endpoints, createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID))
	}
	for _, endpoint := range endpoints {
		_, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
		require.NoError(t, err)
	}
	_, err := svc.Test(ctx, owner.ID, workspace.ID, endpoints[0].ID)
	require.Equal(t, 429, infraerrors.Code(err), "endpoint cooldown is enforced")
	require.NoError(t, svc.Delete(ctx, owner.ID, workspace.ID, endpoints[0].ID))
	fresh := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	_, err = svc.Test(ctx, owner.ID, workspace.ID, fresh.ID)
	require.Equal(t, 429, infraerrors.Code(err), "deletion must not reset the workspace rate limit")
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_webhook_test_limits SET window_started_at=now()-interval '2 minutes' WHERE workspace_id=$1`, workspace.ID)
	require.NoError(t, err)
	_, err = svc.Test(ctx, owner.ID, workspace.ID, fresh.ID)
	require.NoError(t, err)
	var eventPayload []byte
	require.NoError(t, integrationDB.QueryRow(`SELECT payload FROM domain_events WHERE event_type=$1 AND workspace_id=$2 ORDER BY created_at DESC LIMIT 1`, service.EventWebhookTest, workspace.ID).Scan(&eventPayload))
	var event service.DomainEvent
	require.NoError(t, json.Unmarshal(eventPayload, &event))
	require.NoError(t, repo.EnqueueEventDeliveries(ctx, &event))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_webhook_deliveries WHERE event_id=$1`, event.ID).Scan(&count))
	require.Equal(t, 1, count, "targeted tests never fan out to other subscribed endpoints")
}

func TestWorkspaceWebhookTwoWorkersAndLeaseRecoveryFenceStaleOwner(t *testing.T) {
	ctx, _, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	delivery, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	type result struct {
		claims []service.WebhookDeliveryClaim
		err    error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			claims, err := repo.ClaimDeliveries(ctx, "worker", 8, 2*time.Minute)
			results <- result{claims, err}
		}()
	}
	wg.Wait()
	close(results)
	all := []service.WebhookDeliveryClaim{}
	for res := range results {
		require.NoError(t, res.err)
		all = append(all, res.claims...)
	}
	require.Len(t, all, 1)
	old := all[0]
	require.Equal(t, delivery.ID, old.ID)
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_webhook_deliveries SET locked_at=now()-interval '3 minutes' WHERE id=$1`, old.ID)
	require.NoError(t, err)
	recovered, err := repo.ClaimDeliveries(ctx, "worker", 8, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, recovered, 1)
	require.NotEqual(t, old.LockOwner, recovered[0].LockOwner, "same worker identity still uses a fresh claim token")
	require.Equal(t, old.Payload, recovered[0].Payload)
	require.ErrorIs(t, repo.MarkDeliverySuccess(ctx, old.ID, old.LockOwner, 204, ""), service.ErrWebhookDeliveryLeaseLost)
	require.NoError(t, repo.MarkDeliverySuccess(ctx, old.ID, recovered[0].LockOwner, 204, "accepted"))
}

func TestWorkspaceWebhookDisabledClaimRecheckAndImmutablePayload(t *testing.T) {
	ctx, _, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	delivery, err := svc.Test(ctx, owner.ID, workspace.ID, endpoint.ID)
	require.NoError(t, err)
	claims, err := repo.ClaimDeliveries(ctx, "worker", 8, 2*time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	disabled := false
	_, err = svc.Update(ctx, owner.ID, workspace.ID, endpoint.ID, service.UpdateWorkspaceWebhookInput{Enabled: &disabled})
	require.NoError(t, err)
	active, err := repo.DeliveryEndpointActive(ctx, delivery.ID, claims[0].LockOwner)
	require.NoError(t, err)
	require.False(t, active)
	require.NoError(t, repo.MarkDeliveryRetry(ctx, delivery.ID, claims[0].LockOwner, time.Now().Add(-time.Minute), "endpoint disabled", 0, ""))
	claims, err = repo.ClaimDeliveries(ctx, "worker", 8, 2*time.Minute)
	require.NoError(t, err)
	require.Empty(t, claims, "disabled endpoints are not claimed")
	_, err = integrationDB.ExecContext(ctx, `UPDATE workspace_webhook_deliveries SET payload='{}'::jsonb WHERE id=$1`, delivery.ID)
	require.Error(t, err, "event payload remains immutable across delivery state changes")
}

func TestWorkspaceWebhookDuplicateFanoutAndMigrationReapply(t *testing.T) {
	ctx, _, svc, repo, owner, workspace := workspaceWebhookFixture(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, workspace.ID)
	event, err := service.NewDomainEvent(service.EventWorkspaceUpdated, workspace.ID, 0, owner.ID, "workspace", fmt.Sprint(workspace.ID), service.DomainEventData{"name": "Stable"})
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, InsertDomainEventTx(ctx, tx, event, ""))
	require.NoError(t, tx.Commit())
	require.NoError(t, repo.EnqueueEventDeliveries(ctx, event))
	require.NoError(t, repo.EnqueueEventDeliveries(ctx, event))
	items, total, err := svc.Deliveries(ctx, owner.ID, workspace.ID, endpoint.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	sql, err := migrations.FS.ReadFile("281_workspace_webhooks.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = integrationDB.ExecContext(ctx, string(sql))
		require.NoError(t, err)
	}
	endpoints, err := svc.List(ctx, owner.ID, workspace.ID)
	require.NoError(t, err)
	require.Len(t, endpoints, 1, "reapplying the migration preserves endpoint data")
	_, total, err = svc.Deliveries(ctx, owner.ID, workspace.ID, endpoint.ID, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
}
