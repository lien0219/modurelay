package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type durableGatewayCache struct {
	*gatewayCache
	db     *sql.DB
	claims sync.Map
}

func (c *gatewayCache) WithFinancialDatabase(db *sql.DB) service.GatewayCache {
	return &durableGatewayCache{gatewayCache: c, db: db}
}

func NewDurableGatewayCache(rdb *redis.Client, db *sql.DB) service.GatewayCache {
	return (&gatewayCache{rdb: rdb}).WithFinancialDatabase(db)
}

func financialVideoKey(task string, owner, key int64) string {
	if owner < 0 {
		return fmt.Sprintf("sa:%d:%d:%s", -owner, key, strings.TrimSpace(task))
	}
	return fmt.Sprintf("%d:%d:%s", owner, key, strings.TrimSpace(task))
}

func putFinancialMedia(ctx context.Context, db *sql.DB, kind, key string, p *service.GrokVideoPendingBilling, payload []byte, state string) error {
	return putFinancialMediaAt(ctx, db, kind, key, p, payload, state, nil)
}

func putFinancialMediaAt(ctx context.Context, db *sql.DB, kind, key string, p *service.GrokVideoPendingBilling, payload []byte, state string, createdAt *time.Time) error {
	if p == nil || !service.ValidExecutionAttribution(p.UserID, p.ServiceAccountID) || p.APIKeyID <= 0 || len(payload) == 0 || len(payload) > 1800000 || len(key) > 512 {
		return service.ErrBudgetReservationInvalid
	}
	_, err := db.ExecContext(ctx, `INSERT INTO financial_media_records(kind,record_key,api_key_id,actor_user_id,service_account_id,workspace_id,project_id,billing_principal_user_id,reservation_id,account_id,payload,state,created_at)
		 VALUES($1,$2,$3,NULLIF($4,0),NULLIF($5,0),NULLIF($6,0),NULLIF($7,0),NULLIF($8,0),NULLIF($9,'')::uuid,NULLIF($10,0),$11::jsonb,$12,COALESCE($13::timestamptz,now()))
		 ON CONFLICT(kind,record_key) DO NOTHING`, kind, key, p.APIKeyID, p.UserID, p.ServiceAccountID, p.WorkspaceID, p.ProjectID, p.BillingPrincipalUserID, p.BudgetReservationID, p.AccountID, string(payload), state, createdAt)
	if err != nil {
		return err
	}
	var matches bool
	err = db.QueryRowContext(ctx, `SELECT api_key_id=$3 AND actor_user_id IS NOT DISTINCT FROM NULLIF($4,0) AND service_account_id IS NOT DISTINCT FROM NULLIF($5,0)
	 AND workspace_id IS NOT DISTINCT FROM NULLIF($6,0) AND project_id IS NOT DISTINCT FROM NULLIF($7,0) AND billing_principal_user_id IS NOT DISTINCT FROM NULLIF($8,0)
	 AND reservation_id IS NOT DISTINCT FROM NULLIF($9,'')::uuid AND (account_id IS NOT DISTINCT FROM NULLIF($10,0) OR $1='image')
	 FROM financial_media_records WHERE kind=$1 AND record_key=$2`, kind, key, p.APIKeyID, p.UserID, p.ServiceAccountID, p.WorkspaceID, p.ProjectID, p.BillingPrincipalUserID, p.BudgetReservationID, p.AccountID).Scan(&matches)
	if err != nil {
		return err
	}
	if !matches {
		return service.ErrBudgetReservationConflict
	}
	return nil
}

func (c *durableGatewayCache) SetGrokVideoPendingBilling(ctx context.Context, key string, payload []byte, ttl time.Duration) error {
	var p service.GrokVideoPendingBilling
	if err := json.Unmarshal(payload, &p); err != nil {
		return err
	}
	if financialVideoKey(p.RequestID, p.OwnershipID(), p.APIKeyID) != key {
		return service.ErrBudgetReservationConflict
	}
	return putFinancialMedia(ctx, c.db, "video", key, &p, payload, "pending")
}

func (c *durableGatewayCache) GetGrokVideoPendingBilling(ctx context.Context, key string) ([]byte, error) {
	var payload []byte
	err := c.db.QueryRowContext(ctx, `SELECT payload FROM financial_media_records WHERE kind='video' AND record_key=$1`, key).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return payload, err
}

func (c *durableGatewayCache) PrepareGrokVideoSettlement(ctx context.Context, key string, payload []byte) ([]byte, error) {
	if err := c.SetGrokVideoPendingBilling(ctx, key, payload, 0); err != nil {
		return nil, err
	}
	_, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET payload=$2::jsonb,due_at=now()+interval '30 seconds',updated_at=now()
	 WHERE kind='video' AND record_key=$1 AND payload->'settlement' IS NULL AND COALESCE((payload->>'cancelled')::boolean,false)=false`, key, string(payload))
	if err != nil {
		return nil, err
	}
	return c.GetGrokVideoPendingBilling(ctx, key)
}

func (c *durableGatewayCache) DeleteGrokVideoPendingBilling(ctx context.Context, key string) error {
	_, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET payload=payload||'{"cancelled":true}'::jsonb,due_at=now(),updated_at=now()
	 WHERE kind='video' AND record_key=$1 AND payload->'settlement' IS NULL`, key)
	return err
}

func (c *durableGatewayCache) ScheduleGrokVideoRecovery(ctx context.Context, key string, due time.Time, ttl time.Duration) error {
	_, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET due_at=$2 WHERE kind='video' AND record_key=$1 AND state='pending'`, key, due)
	return err
}
func (c *durableGatewayCache) ListDueGrokVideoRecovery(ctx context.Context, now time.Time, limit int) ([]string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := c.db.QueryContext(ctx, `SELECT record_key FROM financial_media_records WHERE kind='video' AND state='pending' AND due_at<=$1 ORDER BY due_at,record_key LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	keys := []string{}
	for rows.Next() {
		var key string
		if err = rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}
func (c *durableGatewayCache) CompleteGrokVideoRecovery(ctx context.Context, key string, ttl time.Duration) error {
	_, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET state='settled',updated_at=now() WHERE kind='video' AND record_key=$1`, key)
	return err
}
func (c *durableGatewayCache) RemoveGrokVideoRecovery(ctx context.Context, key string) error {
	return c.CompleteGrokVideoRecovery(ctx, key, 0)
}

func (c *durableGatewayCache) ClaimGrokVideoBilled(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	token := uuid.NewString()
	var got string
	err := c.db.QueryRowContext(ctx, `UPDATE financial_media_records SET billing_claim_token=$2::uuid,billing_claim_until=now()+$3*interval '1 millisecond'
	 WHERE kind='video' AND record_key=$1 AND (billing_claim_until IS NULL OR billing_claim_until<=now()) RETURNING billing_claim_token::text`, key, token, ttl.Milliseconds()).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	c.claims.Store(key, got)
	return true, nil
}
func (c *durableGatewayCache) ReleaseGrokVideoBilled(ctx context.Context, key string) error {
	token, ok := c.claims.LoadAndDelete(key)
	if !ok {
		return nil
	}
	_, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET billing_claim_token=NULL,billing_claim_until=NULL WHERE kind='video' AND record_key=$1 AND billing_claim_token=$2::uuid`, key, token)
	return err
}

func (c *durableGatewayCache) bindingSnapshot(ctx context.Context, task string, owner, key, account int64) (*service.GrokVideoPendingBilling, error) {
	p := &service.GrokVideoPendingBilling{RequestID: task, UserID: owner, APIKeyID: key, AccountID: account}
	if owner < 0 {
		p.UserID = 0
		p.ServiceAccountID = -owner
	}
	if hold := service.BudgetReservationFromContext(ctx); hold != nil {
		p.BudgetReservationID = hold.ID()
		var actor, machine sql.NullInt64
		err := c.db.QueryRowContext(ctx, `SELECT actor_user_id,service_account_id,workspace_id,project_id,billing_principal_user_id FROM budget_reservations WHERE id=$1::uuid AND api_key_id=$2`, p.BudgetReservationID, key).Scan(&actor, &machine, &p.WorkspaceID, &p.ProjectID, &p.BillingPrincipalUserID)
		if err != nil {
			return nil, err
		}
		if actor.Int64 != p.UserID || machine.Int64 != p.ServiceAccountID {
			return nil, service.ErrBudgetReservationConflict
		}
	}
	return p, nil
}
func (c *durableGatewayCache) BindVideoTaskAccount(ctx context.Context, task string, owner, key, group, account int64) error {
	p, err := c.bindingSnapshot(ctx, task, owner, key, account)
	if err != nil {
		return err
	}
	p.GroupID = group
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return putFinancialMedia(ctx, c.db, "binding", financialVideoKey(task, owner, key), p, payload, "accepted")
}
func (c *durableGatewayCache) GetVideoTaskAccount(ctx context.Context, task string, owner, key, group int64) (int64, error) {
	var account int64
	err := c.db.QueryRowContext(ctx, `SELECT account_id FROM financial_media_records WHERE kind='binding' AND record_key=$1 AND api_key_id=$2 AND COALESCE((payload->>'group_id')::bigint,0)=$3`, financialVideoKey(task, owner, key), key, group).Scan(&account)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, service.ErrStickySessionNotFound
	}
	return account, err
}
func (c *durableGatewayCache) StartMediaAttempt(ctx context.Context, p *service.GrokVideoPendingBilling) error {
	if p == nil || p.AttemptID == "" {
		return service.ErrBudgetReservationInvalid
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err = putFinancialMedia(ctx, c.db, "attempt", p.AttemptID, p, payload, "started"); err != nil {
		return err
	}
	var state string
	if err = c.db.QueryRowContext(ctx, `SELECT state FROM financial_media_records WHERE kind='attempt' AND record_key=$1`, p.AttemptID).Scan(&state); err != nil {
		return err
	}
	if state != "started" {
		return service.ErrBudgetReservationConflict
	}
	result, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET provider_started_at=COALESCE(provider_started_at,now()) WHERE kind='attempt' AND record_key=$1 AND state='started'`, p.AttemptID)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		if err != nil {
			return err
		}
		return service.ErrBudgetReservationConflict
	}
	return err
}

func (c *durableGatewayCache) CompleteMediaAttempt(ctx context.Context, p *service.GrokVideoPendingBilling) error {
	if p == nil || p.AttemptID == "" || p.RequestID == "" {
		return service.ErrBudgetReservationInvalid
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var frozenPayload []byte
	var state string
	var frozenAccountID sql.NullInt64
	var providerStartedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT payload,state,account_id,provider_started_at
		FROM financial_media_records
		WHERE kind='attempt' AND record_key=$1
		FOR UPDATE`, strings.TrimSpace(p.AttemptID)).Scan(&frozenPayload, &state, &frozenAccountID, &providerStartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrBudgetReservationConflict
	}
	if err != nil {
		return err
	}
	if state != "started" || !providerStartedAt.Valid || !frozenAccountID.Valid || frozenAccountID.Int64 != p.AccountID {
		return service.ErrBudgetReservationConflict
	}
	var frozen service.GrokVideoPendingBilling
	if err = json.Unmarshal(frozenPayload, &frozen); err != nil {
		return err
	}
	if frozen.AttemptID != strings.TrimSpace(p.AttemptID) || frozen.AccountID != p.AccountID || !service.ValidExecutionAttribution(frozen.UserID, frozen.ServiceAccountID) || frozen.APIKeyID <= 0 {
		return service.ErrBudgetReservationConflict
	}
	// The provider task ID is the only completion-time value that comes from
	// the forwarding path. Billing identity, tenant, reservation, model, and
	// media dimensions remain the admission-time snapshot.
	frozen.RequestID = strings.TrimSpace(p.RequestID)
	payload, err := json.Marshal(&frozen)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE financial_media_records SET payload=$2::jsonb,state='accepted',updated_at=now()
		WHERE kind='attempt' AND record_key=$1 AND state='started'`, strings.TrimSpace(p.AttemptID), string(payload))
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		if err != nil {
			return err
		}
		return service.ErrBudgetReservationConflict
	}
	key := financialVideoKey(frozen.RequestID, frozen.OwnershipID(), frozen.APIKeyID)
	if err = upsertPendingFinancialMediaTx(ctx, tx, key, &frozen, string(payload)); err != nil {
		return err
	}
	return tx.Commit()
}

func upsertPendingFinancialMediaTx(ctx context.Context, tx *sql.Tx, key string, p *service.GrokVideoPendingBilling, payload string) error {
	if p == nil || !service.ValidExecutionAttribution(p.UserID, p.ServiceAccountID) || p.APIKeyID <= 0 || len(payload) == 0 || len(payload) > 1800000 || len(key) > 512 {
		return service.ErrBudgetReservationInvalid
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO financial_media_records(kind,record_key,api_key_id,actor_user_id,service_account_id,workspace_id,project_id,billing_principal_user_id,reservation_id,account_id,payload,state)
		VALUES('video',$1,$2,NULLIF($3,0),NULLIF($4,0),NULLIF($5,0),NULLIF($6,0),NULLIF($7,0),NULLIF($8,'')::uuid,NULLIF($9,0),$10::jsonb,'pending')
		ON CONFLICT(kind,record_key) DO UPDATE
		SET payload=EXCLUDED.payload,
			state=CASE WHEN financial_media_records.state='settled' THEN financial_media_records.state ELSE 'pending' END,
			updated_at=now()
		WHERE financial_media_records.payload->'settlement' IS NULL`, key, p.APIKeyID, p.UserID, p.ServiceAccountID, p.WorkspaceID, p.ProjectID, p.BillingPrincipalUserID, p.BudgetReservationID, p.AccountID, payload)
	return err
}

func (c *durableGatewayCache) RejectMediaAttempt(ctx context.Context, attemptID string) error {
	if strings.TrimSpace(attemptID) == "" {
		return service.ErrBudgetReservationInvalid
	}
	result, err := c.db.ExecContext(ctx, `UPDATE financial_media_records SET state='failed',updated_at=now()
		WHERE kind='attempt' AND record_key=$1 AND state='started' AND provider_started_at IS NOT NULL`, strings.TrimSpace(attemptID))
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n != 1 {
		if err != nil {
			return err
		}
		return service.ErrBudgetReservationConflict
	}
	return nil
}

func (c *durableGatewayCache) RecoverMediaAttempts(ctx context.Context, cutoff time.Time, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT record_key,payload FROM financial_media_records
		WHERE kind='attempt' AND state='started' AND created_at<=$1 ORDER BY created_at,record_key LIMIT $2 FOR UPDATE SKIP LOCKED`, cutoff, limit)
	if err != nil {
		return 0, err
	}
	type attempt struct {
		key     string
		payload []byte
	}
	var attempts []attempt
	for rows.Next() {
		var a attempt
		if err = rows.Scan(&a.key, &a.payload); err != nil {
			_ = rows.Close()
			return 0, err
		}
		attempts = append(attempts, a)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	for _, a := range attempts {
		var payload map[string]any
		if err = json.Unmarshal(a.payload, &payload); err != nil {
			return 0, err
		}
		payload["provider_outcome"] = "unknown"
		payload["recovery_reason"] = "attempt marker exceeded recovery cutoff"
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return 0, marshalErr
		}
		if _, err = tx.ExecContext(ctx, `UPDATE financial_media_records SET state='unknown',payload=$2::jsonb,updated_at=now() WHERE kind='attempt' AND record_key=$1`, a.key, string(encoded)); err != nil {
			return 0, err
		}
	}
	return len(attempts), tx.Commit()
}
