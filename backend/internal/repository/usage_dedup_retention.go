package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type usageDedupRetentionRow struct {
	RequestID   string    `json:"request_id"`
	APIKeyID    int64     `json:"api_key_id"`
	Fingerprint string    `json:"request_fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
}

// Process one indexed keyset page per worker invocation. Tenant markers stay
// hot with their immutable graph; eligible legacy markers keep the existing
// cold authority. Progress crosses retained rows without an unbounded scan.
func cleanupUsageBillingDedupPage(ctx context.Context, q sqlExecutor, cutoff time.Time) error {
	var afterTime sql.NullTime
	var afterID sql.NullString
	var afterKey sql.NullInt64
	err := scanSingleRow(ctx, q, `SELECT after_time,after_id,after_key FROM domain_event_retention_cursors WHERE category='dedup' FOR UPDATE SKIP LOCKED`, nil, &afterTime, &afterID, &afterKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := q.QueryContext(ctx, `SELECT request_id,api_key_id,request_fingerprint,created_at,tenant_settlement_marker_protected(request_id,api_key_id)
	 FROM usage_billing_dedup WHERE created_at<$1 AND ($3::timestamptz IS NULL OR (created_at,request_id,api_key_id)>($3,$4::text,$5::bigint))
	 ORDER BY created_at,request_id,api_key_id LIMIT $2 FOR UPDATE SKIP LOCKED`, cutoff.UTC(), usageBillingDedupCleanupBatchSize, afterTime, afterID, afterKey)
	if err != nil {
		return err
	}
	eligible := make([]usageDedupRetentionRow, 0)
	var last usageDedupRetentionRow
	scanned := 0
	for rows.Next() {
		var protected bool
		if err = rows.Scan(&last.RequestID, &last.APIKeyID, &last.Fingerprint, &last.CreatedAt, &protected); err != nil {
			_ = rows.Close()
			return err
		}
		scanned++
		if !protected {
			eligible = append(eligible, last)
		}
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	if len(eligible) > 0 {
		payload, marshalErr := json.Marshal(eligible)
		if marshalErr != nil {
			return marshalErr
		}
		if _, err = q.ExecContext(ctx, `INSERT INTO usage_billing_dedup_archive(request_id,api_key_id,request_fingerprint,created_at)
		 SELECT request_id,api_key_id,request_fingerprint,created_at FROM jsonb_to_recordset($1::jsonb) AS r(request_id text,api_key_id bigint,request_fingerprint text,created_at timestamptz)
		 ON CONFLICT(request_id,api_key_id) DO NOTHING`, string(payload)); err != nil {
			return err
		}
		// A conflicting archive never authorizes erasing the original marker.
		if _, err = q.ExecContext(ctx, `DELETE FROM usage_billing_dedup d USING usage_billing_dedup_archive a,jsonb_to_recordset($1::jsonb) AS r(request_id text,api_key_id bigint)
		 WHERE d.request_id=r.request_id AND d.api_key_id=r.api_key_id AND a.request_id=d.request_id AND a.api_key_id=d.api_key_id AND a.request_fingerprint=d.request_fingerprint AND a.created_at=d.created_at`, string(payload)); err != nil {
			return err
		}
	}
	if scanned < usageBillingDedupCleanupBatchSize {
		afterTime, afterID, afterKey = sql.NullTime{}, sql.NullString{}, sql.NullInt64{}
	} else {
		afterTime = sql.NullTime{Time: last.CreatedAt, Valid: true}
		afterID = sql.NullString{String: last.RequestID, Valid: true}
		afterKey = sql.NullInt64{Int64: last.APIKeyID, Valid: true}
	}
	_, err = q.ExecContext(ctx, `UPDATE domain_event_retention_cursors SET after_time=$1,after_id=$2,after_key=$3 WHERE category='dedup'`, afterTime, afterID, afterKey)
	return err
}
