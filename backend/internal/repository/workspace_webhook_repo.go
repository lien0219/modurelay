package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type workspaceWebhookRepository struct{ db *sql.DB }

func NewWorkspaceWebhookRepository(db *sql.DB) service.WorkspaceWebhookRepository {
	return &workspaceWebhookRepository{db: db}
}

const workspaceWebhookColumns = `w.id,w.workspace_id,w.name,w.url,w.enabled,w.created_by_user_id,w.created_at,w.updated_at,w.previous_secret_until,w.disabled_at`

const workspaceWebhookStatusUpdateTimeout = 5 * time.Second

func scanWorkspaceWebhookDelivery(row workspaceScanner, item *service.WorkspaceWebhookDelivery, extra ...any) error {
	var (
		responseStatus  sql.NullInt64
		responsePreview sql.NullString
		lastError       sql.NullString
		errorCode       sql.NullString
		deliveredAt     sql.NullTime
		lastAttemptAt   sql.NullTime
	)
	scanArgs := []any{
		&item.ID, &item.WebhookID, &item.EventID, &item.EventType, &item.Status,
		&item.Attempts, &item.NextAttemptAt, &responseStatus, &responsePreview,
		&lastError, &item.CreatedAt, &deliveredAt, &lastAttemptAt, &errorCode,
	}
	scanArgs = append(scanArgs, extra...)
	if err := row.Scan(scanArgs...); err != nil {
		return workspaceError(err)
	}
	if responseStatus.Valid {
		value := int(responseStatus.Int64)
		item.ResponseStatus = &value
	}
	if responsePreview.Valid {
		item.ResponsePreview = responsePreview.String
	}
	if lastError.Valid {
		item.LastError = lastError.String
	}
	if deliveredAt.Valid {
		value := deliveredAt.Time
		item.DeliveredAt = &value
	}
	if lastAttemptAt.Valid {
		value := lastAttemptAt.Time
		item.LastAttemptAt = &value
	}
	if errorCode.Valid {
		item.ErrorCode = errorCode.String
	}
	return nil
}

func scanWorkspaceWebhook(row workspaceScanner, events *[]string) (*service.WorkspaceWebhook, error) {
	item := &service.WorkspaceWebhook{}
	var eventValues []string
	if err := row.Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.URL, &item.Enabled, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt, &item.PreviousSecretUntil, &item.DisabledAt, pq.Array(&eventValues)); err != nil {
		return nil, workspaceError(err)
	}
	item.EventTypes = eventValues
	if events != nil {
		*events = eventValues
	}
	return item, nil
}

func (r *workspaceWebhookRepository) beginScoped(ctx context.Context, actorID, workspaceID int64, permission string) (*sql.Tx, *service.WorkspaceAccess, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("workspace webhook database is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	if err = lockWorkspace(ctx, tx, workspaceID, !strings.HasSuffix(permission, ".read")); err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	access, err := workspaceAccess(ctx, tx, actorID, workspaceID, 0)
	if err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	if err = service.CheckWorkspacePermission(access, permission); err != nil {
		_ = tx.Rollback()
		return nil, nil, err
	}
	return tx, access, nil
}

func (r *workspaceWebhookRepository) ListWebhooks(ctx context.Context, actorID, workspaceID int64) ([]service.WorkspaceWebhook, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT `+workspaceWebhookColumns+`,COALESCE(array_agg(s.event_type ORDER BY s.event_type) FILTER (WHERE s.event_type IS NOT NULL),'{}') FROM workspace_webhooks w LEFT JOIN workspace_webhook_subscriptions s ON s.webhook_id=w.id WHERE w.workspace_id=$1 GROUP BY w.id ORDER BY w.id`, workspaceID)
	if err != nil {
		return nil, workspaceError(err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.WorkspaceWebhook, 0)
	for rows.Next() {
		item, scanErr := scanWorkspaceWebhook(rows, nil)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return items, tx.Commit()
}

func (r *workspaceWebhookRepository) GetWebhook(ctx context.Context, actorID, workspaceID, webhookID int64) (*service.WorkspaceWebhookRecord, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.read")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item := &service.WorkspaceWebhookRecord{}
	var events []string
	err = tx.QueryRowContext(ctx, `SELECT `+workspaceWebhookColumns+`,w.secret_current_encrypted,COALESCE(w.secret_previous_encrypted,''),COALESCE(array_agg(s.event_type ORDER BY s.event_type) FILTER (WHERE s.event_type IS NOT NULL),'{}') FROM workspace_webhooks w LEFT JOIN workspace_webhook_subscriptions s ON s.webhook_id=w.id WHERE w.workspace_id=$1 AND w.id=$2 GROUP BY w.id`, workspaceID, webhookID).Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.URL, &item.Enabled, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt, &item.PreviousSecretUntil, &item.DisabledAt, &item.SecretCurrentEncrypted, &item.SecretPreviousEncrypted, pq.Array(&events))
	if err != nil {
		return nil, workspaceError(err)
	}
	item.EventTypes = events
	return item, tx.Commit()
}

func (r *workspaceWebhookRepository) CreateWebhook(ctx context.Context, actorID, workspaceID int64, input service.CreateWorkspaceWebhookInput, encryptedSecret string, _ []string) (*service.WorkspaceWebhook, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.create")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var endpointCount int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhooks WHERE workspace_id=$1`, workspaceID).Scan(&endpointCount); err != nil {
		return nil, err
	}
	if endpointCount >= service.WorkspaceWebhookMaxEndpoints {
		return nil, service.ErrWorkspaceConflict
	}
	item := &service.WorkspaceWebhook{}
	err = tx.QueryRowContext(ctx, `INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id) VALUES($1,$2,$3,$4,$5) RETURNING id,workspace_id,name,url,enabled,created_by_user_id,created_at,updated_at,previous_secret_until,disabled_at`, workspaceID, input.Name, input.URL, encryptedSecret, actorID).Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.URL, &item.Enabled, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt, &item.PreviousSecretUntil, &item.DisabledAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = replaceWebhookSubscriptions(ctx, tx, item.ID, input.EventTypes); err != nil {
		return nil, err
	}
	item.EventTypes = append([]string(nil), input.EventTypes...)
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "webhook_created", "webhook", item.ID, map[string]any{"name": item.Name}); err != nil {
		return nil, err
	}
	return item, workspaceError(tx.Commit())
}

func (r *workspaceWebhookRepository) UpdateWebhook(ctx context.Context, actorID, workspaceID, webhookID int64, input service.UpdateWorkspaceWebhookInput, _ []string) (*service.WorkspaceWebhook, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.update")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var current service.WorkspaceWebhook
	err = tx.QueryRowContext(ctx, `SELECT id,workspace_id,name,url,enabled,created_by_user_id,created_at,updated_at,previous_secret_until,disabled_at FROM workspace_webhooks WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, workspaceID, webhookID).Scan(&current.ID, &current.WorkspaceID, &current.Name, &current.URL, &current.Enabled, &current.CreatedByUserID, &current.CreatedAt, &current.UpdatedAt, &current.PreviousSecretUntil, &current.DisabledAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if input.Name != nil {
		current.Name = strings.TrimSpace(*input.Name)
	}
	if input.URL != nil {
		current.URL = *input.URL
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}
	if input.EventTypes != nil {
		if err = replaceWebhookSubscriptions(ctx, tx, webhookID, *input.EventTypes); err != nil {
			return nil, err
		}
		current.EventTypes = append([]string(nil), (*input.EventTypes)...)
	} else {
		var eventValues []string
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(array_agg(event_type ORDER BY event_type),'{}') FROM workspace_webhook_subscriptions WHERE webhook_id=$1`, webhookID).Scan(pq.Array(&eventValues)); err != nil {
			return nil, err
		}
		current.EventTypes = eventValues
	}
	if current.Enabled {
		var n int
		if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhooks WHERE workspace_id=$1 AND id<>$2`, workspaceID, webhookID).Scan(&n); err != nil {
			return nil, err
		}
		if n >= service.WorkspaceWebhookMaxEndpoints {
			return nil, service.ErrWorkspaceConflict
		}
	}
	err = tx.QueryRowContext(ctx, `UPDATE workspace_webhooks SET name=$3,url=$4,enabled=$5,disabled_at=CASE WHEN $5 THEN NULL ELSE COALESCE(disabled_at,clock_timestamp()) END,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING id,workspace_id,name,url,enabled,created_by_user_id,created_at,updated_at,previous_secret_until,disabled_at`, workspaceID, webhookID, current.Name, current.URL, current.Enabled).Scan(&current.ID, &current.WorkspaceID, &current.Name, &current.URL, &current.Enabled, &current.CreatedByUserID, &current.CreatedAt, &current.UpdatedAt, &current.PreviousSecretUntil, &current.DisabledAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "webhook_updated", "webhook", webhookID, nil); err != nil {
		return nil, err
	}
	return &current, workspaceError(tx.Commit())
}

func (r *workspaceWebhookRepository) DeleteWebhook(ctx context.Context, actorID, workspaceID, webhookID int64) error {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.delete")
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `DELETE FROM workspace_webhooks WHERE workspace_id=$1 AND id=$2`, workspaceID, webhookID)
	if err != nil {
		return workspaceError(err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return service.ErrWorkspaceNotFound
	}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "webhook_deleted", "webhook", webhookID, nil); err != nil {
		return err
	}
	return workspaceError(tx.Commit())
}

func (r *workspaceWebhookRepository) RotateWebhookSecret(ctx context.Context, actorID, workspaceID, webhookID int64, encryptedSecret string, _ []string, previousUntil time.Time) (*service.WorkspaceWebhook, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.secret.rotate")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item := &service.WorkspaceWebhook{}
	err = tx.QueryRowContext(ctx, `UPDATE workspace_webhooks SET secret_previous_encrypted=secret_current_encrypted,previous_secret_until=$3,secret_current_encrypted=$4,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING id,workspace_id,name,url,enabled,created_by_user_id,created_at,updated_at,previous_secret_until,disabled_at`, workspaceID, webhookID, previousUntil, encryptedSecret).Scan(&item.ID, &item.WorkspaceID, &item.Name, &item.URL, &item.Enabled, &item.CreatedByUserID, &item.CreatedAt, &item.UpdatedAt, &item.PreviousSecretUntil, &item.DisabledAt)
	if err != nil {
		return nil, workspaceError(err)
	}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, nil, "webhook_secret_rotated", "webhook", webhookID, nil); err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(array_agg(event_type ORDER BY event_type),'{}') FROM workspace_webhook_subscriptions WHERE webhook_id=$1`, webhookID).Scan(pq.Array(&item.EventTypes)); err != nil {
		return nil, err
	}
	return item, workspaceError(tx.Commit())
}

func replaceWebhookSubscriptions(ctx context.Context, tx *sql.Tx, webhookID int64, events []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspace_webhook_subscriptions WHERE webhook_id=$1`, webhookID); err != nil {
		return err
	}
	for _, eventType := range events {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workspace_webhook_subscriptions(webhook_id,event_type) VALUES($1,$2)`, webhookID, eventType); err != nil {
			return workspaceError(err)
		}
	}
	return nil
}

func (r *workspaceWebhookRepository) CreateTestDelivery(ctx context.Context, actorID, workspaceID, webhookID int64, event *service.DomainEvent) (*service.WorkspaceWebhookDelivery, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.test")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var enabled, coolingDown bool
	if err = tx.QueryRowContext(ctx, `SELECT enabled,COALESCE(last_test_at>clock_timestamp()-($3 * interval '1 second'),false) FROM workspace_webhooks WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, workspaceID, webhookID, service.WebhookTestCooldown.Seconds()).Scan(&enabled, &coolingDown); err != nil {
		return nil, workspaceError(err)
	}
	if !enabled {
		return nil, service.ErrWorkspaceConflict
	}
	if coolingDown {
		return nil, service.ErrWebhookTestRateLimited
	}
	var count int
	err = tx.QueryRowContext(ctx, `INSERT INTO workspace_webhook_test_limits(workspace_id,window_started_at,test_count) VALUES($1,clock_timestamp(),1) ON CONFLICT(workspace_id) DO UPDATE SET window_started_at=CASE WHEN workspace_webhook_test_limits.window_started_at<=clock_timestamp()-interval '1 minute' THEN clock_timestamp() ELSE workspace_webhook_test_limits.window_started_at END,test_count=CASE WHEN workspace_webhook_test_limits.window_started_at<=clock_timestamp()-interval '1 minute' THEN 1 ELSE workspace_webhook_test_limits.test_count+1 END WHERE workspace_webhook_test_limits.window_started_at<=clock_timestamp()-interval '1 minute' OR workspace_webhook_test_limits.test_count<$2 RETURNING test_count`, workspaceID, service.WebhookWorkspaceTestsPerMinute).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrWebhookTestRateLimited
	}
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_webhooks SET last_test_at=clock_timestamp() WHERE workspace_id=$1 AND id=$2`, workspaceID, webhookID); err != nil {
		return nil, err
	}
	if err = insertDomainEventTx(ctx, tx, event, ""); err != nil {
		return nil, workspaceError(err)
	}
	item, err := createWebhookDeliveryTx(ctx, tx, workspaceID, webhookID, event)
	if err != nil {
		return nil, err
	}
	return item, workspaceError(tx.Commit())
}

func createWebhookDeliveryTx(ctx context.Context, q workspaceSQL, workspaceID, webhookID int64, event *service.DomainEvent) (*service.WorkspaceWebhookDelivery, error) {
	payload, err := event.MarshalPayload()
	if err != nil {
		return nil, err
	}
	item := &service.WorkspaceWebhookDelivery{}
	err = scanWorkspaceWebhookDelivery(q.QueryRowContext(ctx, `INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload) VALUES($1,$2,$3,$4,$5) ON CONFLICT(webhook_id,event_id) DO UPDATE SET webhook_id=EXCLUDED.webhook_id RETURNING id,webhook_id,event_id,event_type,status,attempts,next_attempt_at,response_status,response_preview,last_error,created_at,delivered_at,last_attempt_at,error_code`, workspaceID, webhookID, event.ID, event.Type, payload), item)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *workspaceWebhookRepository) ListDeliveries(ctx context.Context, actorID, workspaceID, webhookID int64, page, pageSize int) ([]service.WorkspaceWebhookDelivery, int64, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.delivery.read")
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var endpointID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM workspace_webhooks WHERE workspace_id=$1 AND id=$2`, workspaceID, webhookID).Scan(&endpointID); err != nil {
		return nil, 0, workspaceError(err)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int64
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND webhook_id=$2`, workspaceID, webhookID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,webhook_id,event_id,event_type,status,attempts,next_attempt_at,response_status,response_preview,last_error,created_at,delivered_at,last_attempt_at,error_code FROM workspace_webhook_deliveries WHERE workspace_id=$1 AND webhook_id=$2 ORDER BY created_at DESC,id DESC LIMIT $3 OFFSET $4`, workspaceID, webhookID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.WorkspaceWebhookDelivery, 0, pageSize)
	for rows.Next() {
		var item service.WorkspaceWebhookDelivery
		if err = scanWorkspaceWebhookDelivery(rows, &item); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *workspaceWebhookRepository) RetryDelivery(ctx context.Context, actorID, workspaceID, webhookID, deliveryID int64) (*service.WorkspaceWebhookDelivery, error) {
	tx, _, err := r.beginScoped(ctx, actorID, workspaceID, "webhook.delivery.retry")
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item := &service.WorkspaceWebhookDelivery{}
	err = scanWorkspaceWebhookDelivery(tx.QueryRowContext(ctx, `UPDATE workspace_webhook_deliveries SET status='retrying',attempts=0,next_attempt_at=now(),last_error=NULL,error_code=NULL,lock_owner=NULL,locked_at=NULL,delivered_at=NULL,finished_at=NULL WHERE workspace_id=$1 AND webhook_id=$2 AND id=$3 AND status='dead' RETURNING id,webhook_id,event_id,event_type,status,attempts,next_attempt_at,response_status,response_preview,last_error,created_at,delivered_at,last_attempt_at,error_code`, workspaceID, webhookID, deliveryID), item)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *workspaceWebhookRepository) ClaimDeliveries(ctx context.Context, owner string, limit int, lease time.Duration) ([]service.WebhookDeliveryClaim, error) {
	if limit <= 0 {
		limit = 50
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	leaseSeconds := lease.Seconds()
	claimToken := strings.TrimSpace(owner) + ":" + uuid.NewString()
	rows, err := tx.QueryContext(ctx, `WITH due AS (SELECT d.id FROM workspace_webhook_deliveries d JOIN workspace_webhooks w ON w.id=d.webhook_id AND w.workspace_id=d.workspace_id AND w.enabled WHERE (d.status IN ('pending','retrying') AND d.next_attempt_at<=now()) OR (d.status='delivering' AND d.locked_at<now()-($3 * INTERVAL '1 second')) ORDER BY d.created_at,d.id LIMIT $1 FOR UPDATE OF d SKIP LOCKED), claimed AS (UPDATE workspace_webhook_deliveries d SET status='delivering',attempts=d.attempts+1,lock_owner=$2,locked_at=now(),last_attempt_at=clock_timestamp(),finished_at=NULL FROM due WHERE d.id=due.id RETURNING d.*) SELECT c.id,c.webhook_id,c.event_id,c.event_type,c.status,c.attempts,c.next_attempt_at,c.response_status,c.response_preview,c.last_error,c.created_at,c.delivered_at,c.last_attempt_at,c.error_code,c.workspace_id,c.payload,w.url,w.secret_current_encrypted,COALESCE(w.secret_previous_encrypted,''),w.previous_secret_until FROM claimed c JOIN workspace_webhooks w ON w.id=c.webhook_id AND w.workspace_id=c.workspace_id ORDER BY c.created_at,c.id`, limit, claimToken, leaseSeconds)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	claims := make([]service.WebhookDeliveryClaim, 0, limit)
	for rows.Next() {
		var c service.WebhookDeliveryClaim
		if err = scanWorkspaceWebhookDelivery(rows, &c.WorkspaceWebhookDelivery, &c.WorkspaceID, &c.Payload, &c.URL, &c.SecretCurrent, &c.SecretPrevious, &c.PreviousExpiresAt); err != nil {
			return nil, err
		}
		c.LockOwner = claimToken
		claims = append(claims, c)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	return claims, tx.Commit()
}

func (r *workspaceWebhookRepository) DeliveryEndpointActive(ctx context.Context, deliveryID int64, owner string) (bool, error) {
	ctx, cancel := boundedWebhookStatusContext(ctx)
	defer cancel()
	var active bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_webhook_deliveries d JOIN workspace_webhooks w ON w.id=d.webhook_id AND w.workspace_id=d.workspace_id WHERE d.id=$1 AND d.lock_owner=$2 AND d.status='delivering' AND w.enabled)`, deliveryID, owner).Scan(&active)
	return active, err
}

func (r *workspaceWebhookRepository) MarkDeliverySuccess(ctx context.Context, deliveryID int64, owner string, status int, preview string) error {
	ctx, cancel := boundedWebhookStatusContext(ctx)
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_webhook_deliveries SET status='succeeded',response_status=$3,response_preview=$4,last_error=NULL,error_code=NULL,delivered_at=now(),finished_at=now(),lock_owner=NULL,locked_at=NULL WHERE id=$1 AND lock_owner=$2 AND status='delivering'`, deliveryID, owner, status, preview)
	return checkWebhookDeliveryLeaseResult(result, err, deliveryID)
}

func (r *workspaceWebhookRepository) MarkDeliveryRetry(ctx context.Context, deliveryID int64, owner string, next time.Time, reason string, status int, preview string) error {
	ctx, cancel := boundedWebhookStatusContext(ctx)
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_webhook_deliveries SET status='retrying',next_attempt_at=$3,response_status=NULLIF($4,0),response_preview=$5,last_error=$6,error_code='delivery_retryable',lock_owner=NULL,locked_at=NULL WHERE id=$1 AND lock_owner=$2 AND status='delivering'`, deliveryID, owner, next, status, preview, reason)
	return checkWebhookDeliveryLeaseResult(result, err, deliveryID)
}

func (r *workspaceWebhookRepository) MarkDeliveryDead(ctx context.Context, deliveryID int64, owner string, status int, preview, reason string) error {
	ctx, cancel := boundedWebhookStatusContext(ctx)
	defer cancel()
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_webhook_deliveries SET status='dead',response_status=NULLIF($3,0),response_preview=$4,last_error=$5,error_code='delivery_terminal',finished_at=now(),lock_owner=NULL,locked_at=NULL WHERE id=$1 AND lock_owner=$2 AND status='delivering'`, deliveryID, owner, status, preview, reason)
	return checkWebhookDeliveryLeaseResult(result, err, deliveryID)
}

func boundedWebhookStatusContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, workspaceWebhookStatusUpdateTimeout)
}

func checkWebhookDeliveryLeaseResult(result sql.Result, err error, deliveryID int64) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%w: delivery=%d", service.ErrWebhookDeliveryLeaseLost, deliveryID)
	}
	return nil
}

// EnqueueEventDeliveries fans an immutable domain event out to matching active endpoints.
// It is intentionally idempotent on (webhook,event), so event retries cannot duplicate rows.
func (r *workspaceWebhookRepository) EnqueueEventDeliveries(ctx context.Context, event *service.DomainEvent) error {
	if event == nil || event.WorkspaceID == nil {
		return errors.New("workspace domain event is required")
	}
	if event.Type == service.EventWebhookTest {
		// The API inserted this test delivery for its explicitly selected
		// endpoint. Its outbox event may notify the actor but must not fan out.
		return nil
	}
	payload, err := event.MarshalPayload()
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize fan-out with lifecycle's Workspace lock. After waiting for a
	// concurrent purge, recheck its committed state before admitting work that
	// would need an endpoint secret already scheduled for destruction.
	var state string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM workspaces WHERE id=$1 FOR SHARE`, *event.WorkspaceID).Scan(&state); err != nil {
		return err
	}
	if state == "purging" || state == "deleted" {
		return tx.Commit()
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload) SELECT w.workspace_id,w.id,$1,$2,$3 FROM workspace_webhooks w JOIN workspace_webhook_subscriptions s ON s.webhook_id=w.id WHERE w.workspace_id=$4 AND w.enabled AND s.event_type=$2 ON CONFLICT(webhook_id,event_id) DO NOTHING`, event.ID, event.Type, payload, *event.WorkspaceID); err != nil {
		return err
	}
	return tx.Commit()
}
