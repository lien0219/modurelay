//go:build integration

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type financialCacheProvider interface {
	WithFinancialDatabase(*sql.DB) service.GatewayCache
}
type financialImageProvider interface {
	WithFinancialDatabase(*sql.DB) service.ImageTaskStore
}

func TestVideoSQLRecoverySurvivesRedisLossAndPreservesFirstSettlement(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a, res, cmd, log := tenantUsageFixture(t)
	provider, ok := NewGatewayCache(testRedis(t)).(financialCacheProvider)
	require.True(t, ok, "video pending evidence must have a SQL authority")
	cache := provider.WithFinancialDatabase(integrationDB)
	key := fmt.Sprintf("%d:%d:%s", cmd.UserID, cmd.APIKeyID, cmd.RequestID)
	pending := service.GrokVideoPendingBilling{RequestID: cmd.RequestID, UserID: cmd.UserID, APIKeyID: cmd.APIKeyID, AccountID: cmd.AccountID, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, BudgetReservationID: res.ID}
	payload, err := json.Marshal(pending)
	require.NoError(t, err)
	require.NoError(t, cache.SetGrokVideoPendingBilling(ctx, key, payload, 0))
	pending.Settlement = &service.VideoUsageSettlement{Command: cmd, UsageLog: log}
	payload, err = json.Marshal(pending)
	require.NoError(t, err)
	preparer := cache.(service.GrokVideoSettlementPreparer)
	first, err := preparer.PrepareGrokVideoSettlement(ctx, key, payload)
	require.NoError(t, err)
	// A fresh adapter with no Redis connection represents complete cache loss.
	restarted := NewGatewayCache(nil).(financialCacheProvider).WithFinancialDatabase(integrationDB)
	got, err := restarted.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.JSONEq(t, string(first), string(got))
	pending.Settlement.Command.BalanceCost = 99
	payload, err = json.Marshal(pending)
	require.NoError(t, err)
	got, err = restarted.(service.GrokVideoSettlementPreparer).PrepareGrokVideoSettlement(ctx, key, payload)
	require.NoError(t, err)
	require.JSONEq(t, string(first), string(got))
	require.NoError(t, restarted.(service.GrokVideoPendingBillingCleanup).DeleteGrokVideoPendingBilling(ctx, key))
	got, err = restarted.GetGrokVideoPendingBilling(ctx, key)
	require.NoError(t, err)
	require.JSONEq(t, string(first), string(got), "cancel cannot erase an observed payable outcome")
	bindings := restarted.(service.VideoTaskBindingStore)
	require.NoError(t, bindings.BindVideoTaskAccount(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, 0, cmd.AccountID))
	account, err := bindings.GetVideoTaskAccount(ctx, cmd.RequestID, cmd.UserID, cmd.APIKeyID, 0)
	require.NoError(t, err)
	require.Equal(t, cmd.AccountID, account)
	_, err = bindings.GetVideoTaskAccount(ctx, cmd.RequestID, a.BillingPrincipalUserID, cmd.APIKeyID, 0)
	require.Error(t, err, "another actor cannot use a task binding")
	due, err := restarted.(service.GrokVideoRecoveryCache).ListDueGrokVideoRecovery(ctx, time.Now().Add(time.Hour), 1)
	require.NoError(t, err)
	require.Equal(t, []string{key}, due)
}

func TestImageSQLStateAndProviderUncertaintySurviveRedisLoss(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a, res, cmd, _ := tenantUsageFixture(t)
	provider, ok := NewImageTaskStore(testRedis(t)).(financialImageProvider)
	require.True(t, ok, "async image state must have a SQL authority")
	store := provider.WithFinancialDatabase(integrationDB)
	record := &service.ImageTaskRecord{ID: "imgtask_durable", UserID: cmd.UserID, APIKeyID: cmd.APIKeyID, WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID, BudgetReservationID: res.ID, Status: service.ImageTaskStatusProcessing, CreatedAt: time.Now().Add(-time.Hour).Unix(), ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, store.Save(ctx, record, time.Hour))
	marker, ok := store.(interface {
		MarkImageProviderStarted(context.Context, string, int64) error
	})
	require.True(t, ok)
	require.NoError(t, marker.MarkImageProviderStarted(ctx, record.ID, cmd.AccountID))
	restarted := NewImageTaskStore(nil).(financialImageProvider).WithFinancialDatabase(integrationDB)
	got, err := restarted.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, cmd.UserID, got.UserID)
	recovery, ok := restarted.(interface {
		RecoverImageTasks(context.Context, time.Time, int) (int, error)
	})
	require.True(t, ok)
	count, err := recovery.RecoverImageTasks(ctx, time.Now(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	got, err = restarted.Get(ctx, record.ID)
	require.NoError(t, err)
	require.Equal(t, "unknown", got.Status, "a restart must not imply provider failure or trigger another image generation")
	var state string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM budget_reservations WHERE id=$1::uuid`, res.ID).Scan(&state))
	require.Equal(t, "pending", state)
}

func TestVideoMediaAttemptCompletionUsesFrozenAdmissionSnapshot(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a, res, cmd, _ := tenantUsageFixture(t)
	provider, ok := NewGatewayCache(testRedis(t)).(financialCacheProvider)
	require.True(t, ok)
	cache := provider.WithFinancialDatabase(integrationDB)
	attempts, ok := cache.(service.MediaAttemptStore)
	require.True(t, ok)
	finalizer, ok := cache.(service.MediaAttemptFinalizer)
	require.True(t, ok)
	attemptID := "video-attempt:" + uuid.NewString()
	pending := &service.GrokVideoPendingBilling{
		AttemptID:              attemptID,
		RequestID:              "admission:" + uuid.NewString(),
		UserID:                 cmd.UserID,
		APIKeyID:               cmd.APIKeyID,
		AccountID:              cmd.AccountID,
		WorkspaceID:            a.WorkspaceID,
		ProjectID:              a.ProjectID,
		BillingPrincipalUserID: a.BillingPrincipalUserID,
		BudgetReservationID:    res.ID,
		Model:                  "frozen-model",
		BillingModel:           "frozen-model",
		VideoResolution:        "720p",
		VideoDurationSeconds:   5,
	}
	require.NoError(t, attempts.StartMediaAttempt(ctx, pending))

	accepted := *pending
	accepted.RequestID = "video-task:" + uuid.NewString()
	accepted.Model = "live-model-must-not-win"
	accepted.BillingModel = "live-model-must-not-win"
	accepted.VideoResolution = "4k"
	accepted.VideoDurationSeconds = 99
	acceptedPayload, err := json.Marshal(&accepted)
	require.NoError(t, err)
	videoKey := fmt.Sprintf("%d:%d:%s", pending.UserID, pending.APIKeyID, accepted.RequestID)
	require.NoError(t, cache.SetGrokVideoPendingBilling(ctx, videoKey, acceptedPayload, 0))
	require.NoError(t, finalizer.CompleteMediaAttempt(ctx, &accepted))

	payload, err := cache.GetGrokVideoPendingBilling(ctx, videoKey)
	require.NoError(t, err)
	var got service.GrokVideoPendingBilling
	require.NoError(t, json.Unmarshal(payload, &got))
	require.Equal(t, accepted.RequestID, got.RequestID)
	require.Equal(t, pending.WorkspaceID, got.WorkspaceID)
	require.Equal(t, pending.ProjectID, got.ProjectID)
	require.Equal(t, pending.BillingPrincipalUserID, got.BillingPrincipalUserID)
	require.Equal(t, pending.BudgetReservationID, got.BudgetReservationID)
	require.Equal(t, pending.Model, got.Model)
	require.Equal(t, pending.BillingModel, got.BillingModel)
	require.Equal(t, pending.VideoResolution, got.VideoResolution)
	require.Equal(t, pending.VideoDurationSeconds, got.VideoDurationSeconds)
}

func TestVideoMediaAttemptRejectionIsTerminal(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a, res, cmd, _ := tenantUsageFixture(t)
	provider, ok := NewGatewayCache(testRedis(t)).(financialCacheProvider)
	require.True(t, ok)
	cache := provider.WithFinancialDatabase(integrationDB)
	attempts, ok := cache.(service.MediaAttemptStore)
	require.True(t, ok)
	rejector, ok := cache.(service.MediaAttemptRejector)
	require.True(t, ok)
	finalizer, ok := cache.(service.MediaAttemptFinalizer)
	require.True(t, ok)
	attemptID := "video-attempt:" + uuid.NewString()
	pending := &service.GrokVideoPendingBilling{
		AttemptID:              attemptID,
		RequestID:              "admission:" + uuid.NewString(),
		UserID:                 cmd.UserID,
		APIKeyID:               cmd.APIKeyID,
		AccountID:              cmd.AccountID,
		WorkspaceID:            a.WorkspaceID,
		ProjectID:              a.ProjectID,
		BillingPrincipalUserID: a.BillingPrincipalUserID,
		BudgetReservationID:    res.ID,
	}
	require.NoError(t, attempts.StartMediaAttempt(ctx, pending))
	require.NoError(t, rejector.RejectMediaAttempt(ctx, attemptID))
	var state string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT state FROM financial_media_records WHERE kind='attempt' AND record_key=$1`, attemptID).Scan(&state))
	require.Equal(t, "failed", state)
	require.Error(t, finalizer.CompleteMediaAttempt(ctx, &service.GrokVideoPendingBilling{
		AttemptID: attemptID,
		RequestID: "video-task:" + uuid.NewString(),
		UserID:    cmd.UserID,
		APIKeyID:  cmd.APIKeyID,
		AccountID: cmd.AccountID,
	}))
	var records int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM financial_media_records WHERE kind='video' AND api_key_id=$1`, cmd.APIKeyID).Scan(&records))
	require.Zero(t, records)
}

func TestVideoMediaAttemptRecoveryMarksUnknownWithoutResubmission(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a, res, cmd, _ := tenantUsageFixture(t)
	provider, ok := NewGatewayCache(testRedis(t)).(financialCacheProvider)
	require.True(t, ok)
	cache := provider.WithFinancialDatabase(integrationDB)
	attempts, ok := cache.(service.MediaAttemptStore)
	require.True(t, ok)
	recovery, ok := cache.(service.MediaAttemptRecovery)
	require.True(t, ok)
	attemptID := "video-attempt:" + uuid.NewString()
	pending := &service.GrokVideoPendingBilling{
		AttemptID:              attemptID,
		RequestID:              "admission:" + uuid.NewString(),
		UserID:                 cmd.UserID,
		APIKeyID:               cmd.APIKeyID,
		AccountID:              cmd.AccountID,
		WorkspaceID:            a.WorkspaceID,
		ProjectID:              a.ProjectID,
		BillingPrincipalUserID: a.BillingPrincipalUserID,
		BudgetReservationID:    res.ID,
	}
	require.NoError(t, attempts.StartMediaAttempt(ctx, pending))
	count, err := recovery.RecoverMediaAttempts(ctx, time.Now().Add(time.Hour), 10)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	var state string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT state FROM financial_media_records WHERE kind='attempt' AND record_key=$1`, attemptID).Scan(&state))
	require.Equal(t, "unknown", state)
	var outcome string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT payload->>'provider_outcome' FROM financial_media_records WHERE kind='attempt' AND record_key=$1`, attemptID).Scan(&outcome))
	require.Equal(t, "unknown", outcome)
	var records int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT count(*) FROM financial_media_records WHERE kind='video' AND api_key_id=$1`, cmd.APIKeyID).Scan(&records))
	require.Zero(t, records)
	var reservationState string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT status FROM budget_reservations WHERE id=$1::uuid`, res.ID).Scan(&reservationState))
	require.Equal(t, "pending", reservationState)
}
