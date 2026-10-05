package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDomainEventWriterUsesExecAndPreservesDedupePayload(t *testing.T) {
	for _, dedupe := range []string{"", "retry-transition"} {
		t.Run(dedupe, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			event, err := service.NewDomainEvent(service.EventWorkspaceUpdated, 11, 0, 7, "workspace", "11", service.DomainEventData{"status": "active"})
			require.NoError(t, err)
			payload, err := event.MarshalPayload()
			require.NoError(t, err)
			if dedupe == "" {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)")).WithArgs(event.ID, event.Type, 1, event.CreatedAt, int64(11), nil, int64(7), "workspace", "11", payload).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE id=$1 ON CONFLICT(event_id) DO NOTHING")).WithArgs(event.ID).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,project_id,actor_user_id,subject_type,subject_id,payload,dedupe_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(dedupe_key) DO NOTHING")).WithArgs(event.ID, event.Type, 1, event.CreatedAt, int64(11), nil, int64(7), "workspace", "11", payload, dedupe).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE dedupe_key=$1 ON CONFLICT(event_id) DO NOTHING")).WithArgs(dedupe).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			require.NoError(t, insertDomainEventTx(context.Background(), db, event, dedupe))
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDomainEventOutboxRejectsExpiredLeaseWrites(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &domainEventOutboxRepository{db: db}
	mock.ExpectExec(`UPDATE domain_event_outbox .*locked_until>now\(\)`).WithArgs("evt_1", "old-lease").WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.Ack(context.Background(), "evt_1", "old-lease"))
	mock.ExpectExec(`UPDATE domain_event_outbox .*locked_until>now\(\)`).WithArgs("evt_1", "old-lease", float64(0), "dispatch_failed").WillReturnResult(sqlmock.NewResult(0, 0))
	require.Error(t, repo.Retry(context.Background(), "evt_1", "old-lease", -time.Second, errors.New("contains sk-private-provider-secret")))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDomainEventOutboxRetryPersistsSafeFailureCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectExec("UPDATE domain_event_outbox").WithArgs("evt_1", "lease", float64(1), "dispatch_failed").WillReturnResult(sqlmock.NewResult(0, 1))
	repo := &domainEventOutboxRepository{db: db}
	require.NoError(t, repo.Retry(context.Background(), "evt_1", "lease", time.Second, errors.New("password=postgres sk-private-provider-secret")))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDomainEventOutboxClaimClampsOversizedBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectQuery("WITH due AS").WithArgs(1000, float64(60), sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"event_id", "event_type", "payload", "attempts", "lock_token", "locked_until"}))
	mock.ExpectCommit()
	repo := &domainEventOutboxRepository{db: db}
	items, err := repo.Claim(context.Background(), 10000, time.Minute)
	require.NoError(t, err)
	require.Empty(t, items)
	require.NoError(t, mock.ExpectationsWereMet())
}
