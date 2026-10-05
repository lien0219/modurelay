package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepositoryCreateForRecipientsIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	insert := regexp.QuoteMeta("INSERT INTO user_notifications(event_id,recipient_user_id,workspace_id,project_id,category,title_key,body_key,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8),($9,$10,$11,$12,$13,$14,$15,$16) ON CONFLICT(event_id,recipient_user_id) DO NOTHING")
	mock.ExpectExec(insert).
		WithArgs("evt_1", int64(7), int64(11), int64(13), "workspace", "workspace.updated.title", "workspace.updated.body", []byte(`{"status":"active"}`), "evt_1", int64(8), int64(11), int64(13), "workspace", "workspace.updated.title", "workspace.updated.body", []byte(`{"status":"active"}`)).
		WillReturnResult(sqlmock.NewResult(1, 2))
	mock.ExpectCommit()

	repo := &notificationRepository{db: db}
	err = repo.CreateForRecipients(context.Background(), []service.UserNotification{
		{EventID: "evt_1", RecipientUserID: 7, WorkspaceID: ptrInt64(11), ProjectID: ptrInt64(13), Category: "workspace", TitleKey: "workspace.updated.title", BodyKey: "workspace.updated.body", Data: map[string]any{"status": "active"}},
		{EventID: "evt_1", RecipientUserID: 8, WorkspaceID: ptrInt64(11), ProjectID: ptrInt64(13), Category: "workspace", TitleKey: "workspace.updated.title", BodyKey: "workspace.updated.body", Data: map[string]any{"status": "active"}},
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepositoryCreateForRecipientsRollsBackOnInsertError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO user_notifications")).WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	repo := &notificationRepository{db: db}
	err = repo.CreateForRecipients(context.Background(), []service.UserNotification{{
		EventID: "evt_1", RecipientUserID: 7, Category: "workspace", TitleKey: "title", BodyKey: "body",
	}})
	require.EqualError(t, err, "insert failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepositoryCreateForRecipientsValidatesBeforeBegin(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &notificationRepository{db: db}
	err = repo.CreateForRecipients(context.Background(), []service.UserNotification{{
		EventID: "", RecipientUserID: 7, Category: "workspace", TitleKey: "title", BodyKey: "body",
	}})
	require.EqualError(t, err, "notification event is required")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNotificationRepositoryFanoutUsesBoundedBatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	rows := make([]service.UserNotification, 10000)
	for i := range rows {
		rows[i] = service.UserNotification{EventID: "evt_1", RecipientUserID: int64(i + 1), Category: "system", TitleKey: "title", BodyKey: "body"}
	}
	mock.ExpectBegin()
	// 10,000 recipients must not become 10,000 statements or exceed PostgreSQL
	// parameter limits; a batch contains at most 500 rows / 4,000 parameters.
	for i := 0; i < 20; i++ {
		mock.ExpectExec(`INSERT INTO user_notifications.*VALUES.*\$4000\) ON CONFLICT\(event_id,recipient_user_id\) DO NOTHING`).WillReturnResult(sqlmock.NewResult(0, 500))
	}
	mock.ExpectCommit()
	repo := &notificationRepository{db: db}
	require.NoError(t, repo.CreateForRecipients(context.Background(), rows))
	require.NoError(t, mock.ExpectationsWereMet())
}

func ptrInt64(value int64) *int64 { return &value }
