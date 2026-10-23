//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAlertTransitionRollsBackIfNotificationIntentCannotBeSaved(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &opsRepository{db: db}
	now := time.Now()
	event := &service.OpsAlertEvent{ID: 10, Status: service.OpsAlertStatusResolved, ResolvedAt: &now}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT i.id FROM ops_alert_incidents").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec("UPDATE ops_alert_events SET status").WithArgs(int64(10), service.OpsAlertStatusResolved, &now).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE ops_alert_incidents").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ops_alert_deliveries").WillReturnError(errors.New("disk full"))
	mock.ExpectRollback()
	updated, err := repo.TransitionAlertEvent(context.Background(), event, []*service.OpsAlertDelivery{{Channel: "email", Transition: "resolved", Target: "ops@example.com", Payload: []byte(`{}`)}})
	require.Error(t, err)
	require.False(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestRepeatedRecoveryDoesNotEnqueueAnotherDelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &opsRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT i.id FROM ops_alert_incidents").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectExec("UPDATE ops_alert_events SET status").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	updated, err := repo.TransitionAlertEvent(context.Background(), &service.OpsAlertEvent{ID: 10, Status: service.OpsAlertStatusResolved}, []*service.OpsAlertDelivery{{Channel: "email"}})
	require.NoError(t, err)
	require.False(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestOutboxClaimUsesLeaseAndSkipLocked(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &opsRepository{db: db}
	now := time.Now()
	mock.ExpectQuery("UPDATE ops_alert_deliveries SET status='sending'.*FOR UPDATE OF d SKIP LOCKED").WithArgs(now, 10, sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "event_id", "channel", "transition", "target", "status", "attempts", "last_error", "next_attempt_at", "created_at", "sent_at", "payload", "lease_token"}).AddRow(7, 11, "webhook", "firing", "https://example.com", "sending", 2, "", now, now, nil, []byte(`{}`), "lease-1"))
	rows, err := repo.ClaimAlertDeliveries(context.Background(), now, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, 2, rows[0].Attempts)
	require.Equal(t, "lease-1", rows[0].LeaseToken)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestOutboxFinalFailureIsAuditedAndStaleWorkerCannotMarkEmailSent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &opsRepository{db: db}
	now := time.Now()
	d := &service.OpsAlertDelivery{ID: 7, EventID: 11, Channel: "email", Attempts: 8, LeaseToken: "old-lease"}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE ops_alert_deliveries SET status").WithArgs(int64(7), "old-lease", "failed", "SMTP delivery failed", now, nil).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO ops_alert_delivery_attempts").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	require.NoError(t, repo.CompleteAlertDelivery(context.Background(), d, errors.New("SMTP delivery failed"), now))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE ops_alert_deliveries SET status").WithArgs(int64(7), "old-lease", "sent", "", now, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	require.NoError(t, repo.CompleteAlertDelivery(context.Background(), d, nil, now))
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestOutboxManualRetryCannotResetSendingOrSentDelivery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &opsRepository{db: db}
	mock.ExpectExec("UPDATE ops_alert_deliveries .*status='failed'").WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.RetryAlertDelivery(context.Background(), 7))
	require.NoError(t, mock.ExpectationsWereMet())
}
