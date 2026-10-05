package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceWebhookDeliveryScanAcceptsPendingSQLNulls(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UTC()
	columns := []string{"id", "webhook_id", "event_id", "event_type", "status", "attempts", "next_attempt_at", "response_status", "response_preview", "last_error", "created_at", "delivered_at", "last_attempt_at", "error_code"}
	mock.ExpectQuery("pending_delivery").WillReturnRows(sqlmock.NewRows(columns).AddRow(1, 2, "evt_pending", service.EventWebhookTest, "pending", 0, now, nil, nil, nil, now, nil, nil, nil))
	var item service.WorkspaceWebhookDelivery
	require.NoError(t, scanWorkspaceWebhookDelivery(db.QueryRow("pending_delivery"), &item))
	require.Equal(t, "pending", item.Status)
	require.Nil(t, item.ResponseStatus)
	require.Empty(t, item.ResponsePreview)
	require.Empty(t, item.LastError)
	require.Nil(t, item.DeliveredAt)
	require.Nil(t, item.LastAttemptAt)
	require.Empty(t, item.ErrorCode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceWebhookDeliveryScanKeepsCompletedFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now().UTC()
	mock.ExpectQuery("completed_delivery").WillReturnRows(sqlmock.NewRows([]string{"id", "webhook_id", "event_id", "event_type", "status", "attempts", "next_attempt_at", "response_status", "response_preview", "last_error", "created_at", "delivered_at", "last_attempt_at", "error_code"}).AddRow(1, 2, "evt_completed", service.EventWebhookTest, "succeeded", 2, now, 204, "accepted", nil, now, now, now, nil))
	var item service.WorkspaceWebhookDelivery
	require.NoError(t, scanWorkspaceWebhookDelivery(db.QueryRow("completed_delivery"), &item))
	require.NotNil(t, item.ResponseStatus)
	require.Equal(t, 204, *item.ResponseStatus)
	require.Equal(t, "accepted", item.ResponsePreview)
	require.NotNil(t, item.DeliveredAt)
	require.Equal(t, now, *item.DeliveredAt)
	require.NotNil(t, item.LastAttemptAt)
	require.Equal(t, now, *item.LastAttemptAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceWebhookStatusUpdatesRejectLostLease(t *testing.T) {
	for _, operation := range []string{"success", "retry", "dead"} {
		t.Run(operation, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			repo := &workspaceWebhookRepository{db: db}
			mock.ExpectExec("UPDATE workspace_webhook_deliveries").WillReturnResult(sqlmock.NewResult(0, 0))
			switch operation {
			case "success":
				err = repo.MarkDeliverySuccess(context.Background(), 17, "stale-owner", 204, "")
			case "retry":
				err = repo.MarkDeliveryRetry(context.Background(), 17, "stale-owner", time.Now(), "timeout", 0, "")
			case "dead":
				err = repo.MarkDeliveryDead(context.Background(), 17, "stale-owner", 400, "", "HTTP 400")
			}
			require.ErrorIs(t, err, service.ErrWebhookDeliveryLeaseLost)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWorkspaceWebhookStatusUpdateReportsRowsAffectedError(t *testing.T) {
	errRows := errors.New("rows unavailable")
	require.ErrorIs(t, checkWebhookDeliveryLeaseResult(sqlmock.NewErrorResult(errRows), nil, 17), errRows)
}

func TestWorkspaceWebhookTestFanoutOnlyUsesRequestedEndpoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	event, err := service.NewDomainEvent(service.EventWebhookTest, 1, 0, 7, "webhook", "17", service.DomainEventData{"category": "test"})
	require.NoError(t, err)
	repo := &workspaceWebhookRepository{db: db}
	// A test delivery is inserted transactionally for one endpoint by the test
	// API. Dispatching its event must not issue a second broad fanout query.
	require.NoError(t, repo.EnqueueEventDeliveries(context.Background(), event))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceWebhookListDeliveryRejectsMissingEndpoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	expectWebhookScope(t, mock, false)
	mock.ExpectQuery("SELECT id FROM workspace_webhooks WHERE workspace_id=\\$1 AND id=\\$2").WithArgs(int64(11), int64(99)).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	_, _, err = (&workspaceWebhookRepository{db: db}).ListDeliveries(context.Background(), 7, 11, 99, 1, 20)
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestWorkspaceWebhookUpdateCountsDisabledEndpointsAgainstWorkspaceLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	expectWebhookScope(t, mock, true)
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT id,workspace_id,name,url,enabled,created_by_user_id,created_at,updated_at,previous_secret_until,disabled_at FROM workspace_webhooks WHERE workspace_id=\\$1 AND id=\\$2 FOR UPDATE").
		WithArgs(int64(11), int64(23)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "name", "url", "enabled", "created_by_user_id", "created_at", "updated_at", "previous_secret_until", "disabled_at"}).
			AddRow(23, 11, "endpoint", "https://example.com/hook", true, 7, now, now, nil, nil))
	mock.ExpectQuery("SELECT COALESCE\\(array_agg\\(event_type ORDER BY event_type\\),'\\{\\}'\\) FROM workspace_webhook_subscriptions WHERE webhook_id=\\$1").
		WithArgs(int64(23)).
		WillReturnRows(sqlmock.NewRows([]string{"coalesce"}).AddRow("{workspace.created}"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM workspace_webhooks WHERE workspace_id=\\$1 AND id<>\\$2").
		WithArgs(int64(11), int64(23)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(service.WorkspaceWebhookMaxEndpoints))
	mock.ExpectRollback()

	_, err = (&workspaceWebhookRepository{db: db}).UpdateWebhook(context.Background(), 7, 11, 23, service.UpdateWorkspaceWebhookInput{}, nil)
	require.ErrorIs(t, err, service.ErrWorkspaceConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectWebhookScope(t *testing.T, mock sqlmock.Sqlmock, write bool) {
	t.Helper()
	now := time.Now().UTC()
	lock := "SHARE"
	if write {
		lock = "UPDATE"
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM workspaces WHERE id=\\$1 FOR " + lock).WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery("SELECT m.id,m.workspace_id").WithArgs(int64(11), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "user_id", "role", "status", "invited_by_user_id", "joined_at", "created_at", "updated_at"}).AddRow(1, 11, 7, "owner", "active", nil, now, now, now))
	mock.ExpectQuery("SELECT id,name,slug,type,status,owner_user_id").WithArgs(int64(11), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "type", "status", "owner_user_id", "billing_owner_user_id", "created_at", "updated_at"}).AddRow(11, "Test", "test", "organization", "active", 7, 7, now, now))
}
