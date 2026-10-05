package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type notificationRepository struct{ db *sql.DB }

const notificationInsertBatchSize = 500

func NewNotificationRepository(db *sql.DB) service.NotificationRepository {
	return &notificationRepository{db: db}
}

// CreateForRecipients persists the already-resolved inbox rows for one domain
// event fanout. Recipient selection and message localization belong to the
// service layer; this repository only serializes the row and makes retries
// idempotent on (event_id, recipient_user_id).
func (r *notificationRepository) CreateForRecipients(ctx context.Context, notifications []service.UserNotification) error {
	if len(notifications) == 0 {
		return nil
	}
	for _, item := range notifications {
		if strings.TrimSpace(item.EventID) == "" {
			return errors.New("notification event is required")
		}
		if item.RecipientUserID <= 0 {
			return errors.New("notification recipient is required")
		}
		if strings.TrimSpace(item.Category) == "" {
			return errors.New("notification category is required")
		}
		if strings.TrimSpace(item.TitleKey) == "" || strings.TrimSpace(item.BodyKey) == "" {
			return errors.New("notification message keys are required")
		}
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for start := 0; start < len(notifications); start += notificationInsertBatchSize {
		end := min(start+notificationInsertBatchSize, len(notifications))
		var query strings.Builder
		_, _ = query.WriteString(`INSERT INTO user_notifications(event_id,recipient_user_id,workspace_id,project_id,category,title_key,body_key,data) VALUES`)
		args := make([]any, 0, (end-start)*8)
		for i, item := range notifications[start:end] {
			if item.Data == nil {
				item.Data = map[string]any{}
			}
			payload, marshalErr := json.Marshal(item.Data)
			if marshalErr != nil {
				return marshalErr
			}
			if i > 0 {
				_ = query.WriteByte(',')
			}
			_ = query.WriteByte('(')
			for column := 0; column < 8; column++ {
				if column > 0 {
					_ = query.WriteByte(',')
				}
				_ = query.WriteByte('$')
				_, _ = query.WriteString(itoa(len(args) + column + 1))
			}
			_ = query.WriteByte(')')
			args = append(args, item.EventID, item.RecipientUserID, item.WorkspaceID, item.ProjectID, item.Category, item.TitleKey, item.BodyKey, payload)
		}
		_, _ = query.WriteString(` ON CONFLICT(event_id,recipient_user_id) DO NOTHING`)
		if _, err = tx.ExecContext(ctx, query.String(), args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *notificationRepository) List(ctx context.Context, userID int64, f service.NotificationListFilter) ([]service.UserNotification, int64, error) {
	if userID <= 0 {
		return nil, 0, errors.New("notification recipient is required")
	}
	f = f.Normalized()
	page, size := f.Page, f.PageSize
	where := []string{"recipient_user_id=$1"}
	args := []any{userID}
	arg := 2
	if strings.TrimSpace(f.Category) != "" {
		where = append(where, "category=$"+itoa(arg))
		args = append(args, strings.TrimSpace(f.Category))
		arg++
	}
	if f.Unread != nil {
		if *f.Unread {
			where = append(where, "read_at IS NULL")
		} else {
			where = append(where, "read_at IS NOT NULL")
		}
	}
	if f.WorkspaceID != nil {
		where = append(where, "workspace_id=$"+itoa(arg))
		args = append(args, *f.WorkspaceID)
		arg++
	}
	if f.ProjectID != nil {
		where = append(where, "project_id=$"+itoa(arg))
		args = append(args, *f.ProjectID)
		arg++
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM user_notifications WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := r.db.QueryContext(ctx, `SELECT id,event_id,recipient_user_id,workspace_id,project_id,category,title_key,body_key,data,read_at,created_at FROM user_notifications WHERE `+clause+` ORDER BY created_at DESC,id DESC LIMIT $`+itoa(arg)+` OFFSET $`+itoa(arg+1), args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.UserNotification, 0, size)
	for rows.Next() {
		var item service.UserNotification
		var payload []byte
		if err = rows.Scan(&item.ID, &item.EventID, &item.RecipientUserID, &item.WorkspaceID, &item.ProjectID, &item.Category, &item.TitleKey, &item.BodyKey, &payload, &item.ReadAt, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if len(payload) > 0 {
			if err = json.Unmarshal(payload, &item.Data); err != nil {
				return nil, 0, err
			}
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *notificationRepository) UnreadCount(ctx context.Context, userID int64, workspaceID, projectID *int64) (int64, error) {
	if userID <= 0 {
		return 0, errors.New("notification recipient is required")
	}
	where := []string{"recipient_user_id=$1", "read_at IS NULL"}
	args := []any{userID}
	arg := 2
	if workspaceID != nil {
		where = append(where, "workspace_id=$"+itoa(arg))
		args = append(args, *workspaceID)
		arg++
	}
	if projectID != nil {
		where = append(where, "project_id=$"+itoa(arg))
		args = append(args, *projectID)
	}
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT count(*) FROM user_notifications WHERE "+strings.Join(where, " AND "), args...).Scan(&count)
	return count, err
}

func (r *notificationRepository) MarkRead(ctx context.Context, userID, id int64) error {
	if userID <= 0 || id <= 0 {
		return service.ErrNotificationNotFound
	}
	result, err := r.db.ExecContext(ctx, `UPDATE user_notifications SET read_at=COALESCE(read_at,now()) WHERE id=$1 AND recipient_user_id=$2`, id, userID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return service.ErrNotificationNotFound
	}
	return nil
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID int64, workspaceID, projectID *int64) (int64, error) {
	if userID <= 0 {
		return 0, errors.New("notification recipient is required")
	}
	where := []string{"recipient_user_id=$1", "read_at IS NULL"}
	args := []any{userID}
	arg := 2
	if workspaceID != nil {
		where = append(where, "workspace_id=$"+itoa(arg))
		args = append(args, *workspaceID)
		arg++
	}
	if projectID != nil {
		where = append(where, "project_id=$"+itoa(arg))
		args = append(args, *projectID)
	}
	result, err := r.db.ExecContext(ctx, "UPDATE user_notifications SET read_at=now() WHERE "+strings.Join(where, " AND "), args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
