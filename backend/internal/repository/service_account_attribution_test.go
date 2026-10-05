package repository

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModerationPersistenceKeepsExecutionPrincipal(t *testing.T) {
	for _, machine := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		id := int64(31)
		log := &service.ContentModerationLog{RequestID: "principal"}
		args := make([]driver.Value, 27)
		for i := range args {
			args[i] = sqlmock.AnyArg()
		}
		args[0] = "principal"
		if machine {
			log.ServiceAccountID = &id
			args[1], args[2], args[3] = nil, id, ""
		} else {
			log.UserID, log.UserEmail = &id, "human@example.test"
			args[1], args[2], args[3] = id, nil, log.UserEmail
		}
		mock.ExpectQuery("(?s)INSERT INTO content_moderation_logs.*request_id, user_id, service_account_id").
			WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, time.Now()))
		require.NoError(t, NewContentModerationRepository(db).CreateLog(context.Background(), log))
		require.NoError(t, mock.ExpectationsWereMet())
		mock.ExpectClose()
		require.NoError(t, db.Close())
	}
}

func TestModerationReadKeepsMachineSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	id := int64(31)
	mock.ExpectQuery("SELECT COUNT.*l.service_account_id").WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	columns := []string{"id", "request_id", "user_id", "service_account_id", "user_email", "api_key_id", "api_key_name", "group_id", "group_name", "endpoint", "provider", "model", "mode", "action", "flagged", "highest_category", "highest_score", "category_scores", "threshold_snapshot", "input_excerpt", "upstream_latency_ms", "error", "violation_count", "auto_banned", "email_sent", "status", "queue_delay_ms", "matched_keyword", "created_at", "engine_meta"}
	mock.ExpectQuery("(?s)SELECT.*l.service_account_id.*FROM content_moderation_logs").
		WithArgs(id, 20, 0).WillReturnRows(sqlmock.NewRows(columns).AddRow(
		1, "machine", nil, id, "", 9, "machine key", nil, "", "/v1/responses", "openai", "model",
		"observe", "allow", false, "", 0, "{}", "{}", "", nil, "", 0, false, false, "", nil, "", time.Now(), nil))
	logs, _, err := NewContentModerationRepository(db).ListLogs(context.Background(), service.ContentModerationLogFilter{ServiceAccountID: &id})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Nil(t, logs[0].UserID)
	require.Empty(t, logs[0].UserEmail)
	require.Equal(t, &id, logs[0].ServiceAccountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsErrorPersistenceKeepsMachineSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	id := int64(31)
	args := make([]driver.Value, 39)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	args[2], args[3] = nil, id
	mock.ExpectQuery("(?s)INSERT INTO ops_error_logs.*user_id,.*service_account_id,.*api_key_id,").
		WithArgs(args...).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))
	got, err := NewOpsRepository(db).InsertErrorLog(context.Background(), &service.OpsInsertErrorLogInput{ServiceAccountID: &id})
	require.NoError(t, err)
	require.EqualValues(t, 4, got)
	where, filterArgs := buildOpsErrorLogsWhere(&service.OpsErrorLogFilter{ServiceAccountID: &id})
	require.Contains(t, where, "e.service_account_id = $1")
	require.Equal(t, []any{id}, filterArgs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsSystemLogReadAndCleanupKeepMachineScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	id := int64(31)
	mock.ExpectQuery("SELECT COUNT.*l.service_account_id").WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	columns := []string{"id", "created_at", "host", "level", "component", "message", "request_id", "client_request_id", "user_id", "service_account_id", "api_key_id", "account_id", "platform", "model", "extra"}
	mock.ExpectQuery("(?s)SELECT.*l.service_account_id.*FROM ops_system_logs").
		WithArgs(id, 50, 0).WillReturnRows(sqlmock.NewRows(columns).AddRow(
		4, time.Now(), "host", "warn", "gateway", "failure", "machine", "", nil, id, 9, nil, "openai", "model", "{}"))
	logs, err := NewOpsRepository(db).ListSystemLogs(context.Background(), &service.OpsSystemLogFilter{ServiceAccountID: &id})
	require.NoError(t, err)
	require.Len(t, logs.Logs, 1)
	require.Nil(t, logs.Logs[0].UserID)
	require.Equal(t, &id, logs.Logs[0].ServiceAccountID)
	mock.ExpectExec("DELETE FROM ops_system_logs l .*l.service_account_id").WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	count, err := NewOpsRepository(db).DeleteSystemLogs(context.Background(), &service.OpsSystemLogCleanupFilter{ServiceAccountID: &id})
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.NoError(t, mock.ExpectationsWereMet())
}
