package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type frozenUsageEnvelope struct {
	Version int                          `json:"version"`
	Command *service.UsageBillingCommand `json:"command"`
	Usage   *service.UsageLog            `json:"usage"`
}

func scrubFrozenUsage(log *service.UsageLog) *service.UsageLog {
	frozen := *log
	frozen.User, frozen.APIKey, frozen.Account, frozen.Group, frozen.Subscription = nil, nil, nil, nil, nil
	frozen.UserAgent, frozen.IPAddress, frozen.SessionID, frozen.UpstreamRequestID = nil, nil, nil, nil
	return &frozen
}

func (r *usageBillingRepository) stageFrozenTenantUsage(ctx context.Context, cmd *service.UsageBillingCommand, log *service.UsageLog) (string, error) {
	payload, err := json.Marshal(frozenUsageEnvelope{Version: 1, Command: cmd, Usage: scrubFrozenUsage(log)})
	if err != nil || len(payload) > 60000 {
		return "", fmt.Errorf("invalid or oversized frozen billing envelope: %v", err)
	}
	var id, fingerprint string
	err = r.db.QueryRowContext(ctx, `INSERT INTO frozen_usage_recovery(id,request_id,api_key_id,actor_user_id,service_account_id,workspace_id,project_id,billing_principal_user_id,reservation_id,account_id,fingerprint,envelope)
	 VALUES($1,$2,$3,NULLIF($4,0),NULLIF($5,0),$6,$7,$8,$9::uuid,$10,$11,$12::jsonb)
	 ON CONFLICT(request_id,api_key_id) DO UPDATE SET due_at=frozen_usage_recovery.due_at
	 RETURNING id::text,fingerprint`, uuid.NewString(), cmd.RequestID, cmd.APIKeyID, cmd.UserID, cmd.ServiceAccountID, cmd.WorkspaceID, cmd.ProjectID, cmd.BillingPrincipalUserID, cmd.BudgetReservationID, cmd.AccountID, cmd.RequestFingerprint, string(payload)).Scan(&id, &fingerprint)
	if err != nil {
		return "", err
	}
	if fingerprint != cmd.RequestFingerprint {
		return "", service.ErrUsageBillingRequestConflict
	}
	return id, nil
}

func (r *usageBillingRepository) settleFrozenReceiptTx(ctx context.Context, tx *sql.Tx, id, token string) error {
	result, err := tx.ExecContext(ctx, `UPDATE frozen_usage_recovery SET state='settled',settled_at=COALESCE(settled_at,now()),lease_token=NULL,lease_until=NULL WHERE id=$1::uuid
	 AND ($2='' OR (lease_token::text=$2 AND lease_until>clock_timestamp()))`, id, token)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("frozen accounting lease lost")
	}
	return nil
}

func (r *usageBillingRepository) RecoverFrozenTenantUsage(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	token := uuid.NewString()
	rows, err := r.db.QueryContext(ctx, `WITH due AS (SELECT id FROM frozen_usage_recovery WHERE state='pending' AND due_at<=now() AND (lease_until IS NULL OR lease_until<=now()) ORDER BY due_at,id LIMIT $1 FOR UPDATE SKIP LOCKED)
	 UPDATE frozen_usage_recovery r SET lease_token=$2::uuid,lease_until=now()+interval '60 seconds',attempts=attempts+1 FROM due WHERE r.id=due.id RETURNING r.id::text,r.envelope`, limit, token)
	if err != nil {
		return 0, err
	}
	type claim struct {
		id       string
		envelope []byte
	}
	claims := []claim{}
	for rows.Next() {
		var c claim
		if err = rows.Scan(&c.id, &c.envelope); err != nil {
			break
		}
		claims = append(claims, c)
	}
	err = errors.Join(err, rows.Err(), rows.Close())
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures error
	for _, c := range claims {
		var envelope frozenUsageEnvelope
		if err = json.Unmarshal(c.envelope, &envelope); err == nil && envelope.Version == 1 && envelope.Command != nil && envelope.Usage != nil {
			_, err = r.applyFrozenTenantUsage(ctx, envelope.Command, envelope.Usage, c.id, token)
		} else {
			err = errors.New("invalid frozen usage envelope")
		}
		if err != nil {
			failures = errors.Join(failures, err)
			_, releaseErr := r.db.ExecContext(ctx, `UPDATE frozen_usage_recovery SET lease_token=NULL,lease_until=NULL,due_at=now()+interval '30 seconds' WHERE id=$1::uuid AND state='pending' AND lease_token=$2::uuid AND lease_until>now()`, c.id, token)
			failures = errors.Join(failures, releaseErr)
			continue
		}
		completed++
	}
	return completed, failures
}

func (r *usageBillingRepository) ListFrozenUsageCacheInvalidations(ctx context.Context, limit int) ([]*service.UsageLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT envelope FROM frozen_usage_recovery WHERE state='settled' AND cache_invalidated_at IS NULL ORDER BY settled_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	logs := []*service.UsageLog{}
	for rows.Next() {
		var payload []byte
		if err = rows.Scan(&payload); err != nil {
			return nil, err
		}
		var env frozenUsageEnvelope
		if err = json.Unmarshal(payload, &env); err != nil {
			return nil, err
		}
		logs = append(logs, env.Usage)
	}
	return logs, rows.Err()
}

func (r *usageBillingRepository) AcknowledgeFrozenUsageCacheInvalidation(ctx context.Context, requestID string, keyID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE frozen_usage_recovery SET cache_invalidated_at=$3 WHERE request_id=$1 AND api_key_id=$2 AND state='settled'`, requestID, keyID, time.Now().UTC())
	return err
}
