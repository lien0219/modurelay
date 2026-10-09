package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type durableImageTaskStore struct {
	*imageTaskStore
	db *sql.DB
}

func (s *imageTaskStore) WithFinancialDatabase(db *sql.DB) service.ImageTaskStore {
	return &durableImageTaskStore{imageTaskStore: s, db: db}
}
func NewDurableImageTaskStore(rdb *redis.Client, db *sql.DB) service.ImageTaskStore {
	return (&imageTaskStore{rdb: rdb}).WithFinancialDatabase(db)
}
func imageFinancialSnapshot(task *service.ImageTaskRecord) *service.GrokVideoPendingBilling {
	return &service.GrokVideoPendingBilling{RequestID: task.ID, UserID: task.UserID, ServiceAccountID: task.ServiceAccountID, APIKeyID: task.APIKeyID,
		WorkspaceID: task.WorkspaceID, ProjectID: task.ProjectID, BillingPrincipalUserID: task.BillingPrincipalUserID, BudgetReservationID: task.BudgetReservationID}
}

func (s *durableImageTaskStore) Save(ctx context.Context, task *service.ImageTaskRecord, ttl time.Duration) error {
	if task == nil {
		return service.ErrImageTaskUnavailable
	}
	payload, err := json.Marshal(task)
	if err != nil {
		return err
	}
	var createdAt *time.Time
	if task.CreatedAt > 0 {
		frozenCreatedAt := time.Unix(task.CreatedAt, 0).UTC()
		createdAt = &frozenCreatedAt
	}
	if err = putFinancialMediaAt(ctx, s.db, "image", task.ID, imageFinancialSnapshot(task), payload, task.Status, createdAt); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE financial_media_records SET payload=$2::jsonb,state=$3,updated_at=now()
	 WHERE kind='image' AND record_key=$1 AND (state IN ('processing','unknown') OR payload=$2::jsonb)`, task.ID, string(payload), task.Status)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrBudgetReservationConflict
	}
	return nil
}
func (s *durableImageTaskStore) Get(ctx context.Context, id string) (*service.ImageTaskRecord, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM financial_media_records WHERE kind='image' AND record_key=$1`, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrImageTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	var task service.ImageTaskRecord
	if err = json.Unmarshal(payload, &task); err != nil {
		return nil, err
	}
	if task.ExpiresAt < time.Now().Unix() && task.Status != "processing" && task.Status != "unknown" {
		return nil, service.ErrImageTaskNotFound
	}
	return &task, nil
}
func (s *durableImageTaskStore) MarkImageProviderStarted(ctx context.Context, id string, account int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE financial_media_records SET provider_started_at=COALESCE(provider_started_at,now()),account_id=COALESCE(account_id,$2),updated_at=now()
	 WHERE kind='image' AND record_key=$1 AND state='processing' AND (account_id IS NULL OR account_id=$2)`, id, account)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrImageTaskUnavailable
	}
	return nil
}

// Restart recovery records uncertainty after the provider boundary. A task
// proven never sent can release its budget in the same SQL transaction as its
// terminal state. Generation bodies are deliberately not stored or replayed.
func (s *durableImageTaskStore) RecoverImageTasks(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT record_key,payload,provider_started_at FROM financial_media_records
	 WHERE kind='image' AND state='processing' AND created_at<=$1 ORDER BY created_at,record_key LIMIT $2 FOR UPDATE SKIP LOCKED`, cutoff, limit)
	if err != nil {
		return 0, err
	}
	type stalled struct {
		id      string
		payload []byte
		started sql.NullTime
	}
	records := []stalled{}
	for rows.Next() {
		var r stalled
		if err = rows.Scan(&r.id, &r.payload, &r.started); err != nil {
			break
		}
		records = append(records, r)
	}
	err = errors.Join(err, rows.Err(), rows.Close())
	if err != nil {
		return 0, err
	}
	for _, r := range records {
		var task service.ImageTaskRecord
		if err = json.Unmarshal(r.payload, &task); err != nil {
			return 0, err
		}
		if r.started.Valid {
			task.Status = "unknown"
			task.Error = json.RawMessage(`{"type":"upstream_outcome_unknown","message":"Provider outcome is unknown; the accepted task is retained for reconciliation."}`)
		} else {
			task.Status = service.ImageTaskStatusFailed
			task.HTTPStatus = 503
			task.Error = json.RawMessage(`{"type":"execution_interrupted","message":"Image task was interrupted before provider submission."}`)
			if task.BudgetReservationID != "" {
				cmd := &service.UsageBillingCommand{UserID: task.UserID, ServiceAccountID: task.ServiceAccountID, APIKeyID: task.APIKeyID, WorkspaceID: task.WorkspaceID, ProjectID: task.ProjectID, BillingPrincipalUserID: task.BillingPrincipalUserID, BudgetReservationID: task.BudgetReservationID}
				if err = releaseTenantBudgetTx(ctx, tx, cmd); err != nil {
					return 0, err
				}
			}
		}
		payload, err := json.Marshal(task)
		if err != nil {
			return 0, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE financial_media_records SET state=$2,payload=$3::jsonb,updated_at=now() WHERE kind='image' AND record_key=$1`, r.id, task.Status, string(payload)); err != nil {
			return 0, err
		}
	}
	return len(records), tx.Commit()
}
