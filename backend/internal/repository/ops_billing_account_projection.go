package repository

import (
	"context"
	"database/sql"
	"time"
)

// Reads source rows without acquiring account locks. All writes target observer
// tables, so a slow/locked observer cannot delay a monetary UPDATE or its commit.
// Entries describe net changes over a revision range, not individual mutations.
func reconcileAccountProjections(ctx context.Context, tx *sql.Tx, now time.Time) error {
	_, err := tx.ExecContext(ctx, `WITH sources AS (
 SELECT 'user'::text AS kind,u.id,v.field FROM users u CROSS JOIN LATERAL (VALUES ('balance'),('frozen_balance')) v(field)
 UNION ALL
 SELECT 'subscription',s.id,v.field FROM user_subscriptions s CROSS JOIN LATERAL (VALUES ('daily_usage_usd'),('weekly_usage_usd'),('monthly_usage_usd')) v(field)
), missing AS (
 SELECT s.* FROM sources s LEFT JOIN ops_account_ledger_heads h ON h.account_type=s.kind AND h.account_id=s.id AND h.field=s.field
 WHERE h.account_id IS NULL LIMIT 10000
) INSERT INTO ops_account_ledger_heads(account_type,account_id,field,opening_amount,expected_amount)
 SELECT kind,id,field,0,0 FROM missing ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `WITH candidates AS MATERIALIZED (
 SELECT h.account_type,h.account_id,h.field,h.expected_amount AS previous_amount,h.source_revision AS previous_revision,
 CASE WHEN h.account_type='user' THEN CASE h.field WHEN 'balance' THEN u.ops_balance_expected ELSE u.ops_frozen_expected END
 ELSE CASE h.field WHEN 'daily_usage_usd' THEN s.ops_daily_expected WHEN 'weekly_usage_usd' THEN s.ops_weekly_expected ELSE s.ops_monthly_expected END END AS projected,
 CASE WHEN h.account_type='user' THEN (to_jsonb(u)->>h.field)::numeric ELSE (to_jsonb(s)->>h.field)::numeric END AS actual,
 CASE WHEN h.account_type='user' THEN u.ops_ledger_revision ELSE s.ops_ledger_revision END AS revision,
 CASE WHEN h.account_type='user' THEN u.id IS NULL OR u.ops_ledger_coverage_gap ELSE s.id IS NULL OR s.ops_ledger_coverage_gap END AS incomplete
 FROM ops_account_ledger_heads h
 LEFT JOIN users u ON h.account_type='user' AND h.account_id=u.id
 LEFT JOIN user_subscriptions s ON h.account_type='subscription' AND h.account_id=s.id
 ORDER BY h.checked_at NULLS FIRST,h.account_type,h.account_id,h.field LIMIT 10000
), evidence AS (
 INSERT INTO ops_account_ledger_entries(account_type,account_id,field,before_amount,after_amount,delta,observation_kind,previous_revision,source_revision)
 SELECT account_type,account_id,field,previous_amount,projected,projected-previous_amount,'aggregate',previous_revision,revision
 FROM candidates WHERE projected IS NOT NULL AND revision>previous_revision
 ON CONFLICT(account_type,account_id,field,source_revision) WHERE source_revision IS NOT NULL DO NOTHING
) UPDATE ops_account_ledger_heads h SET expected_amount=COALESCE(c.projected,h.expected_amount),actual_amount=c.actual,
 mismatch=c.projected IS DISTINCT FROM c.actual OR c.incomplete OR c.projected IS NULL,
 coverage_incomplete=c.incomplete OR c.projected IS NULL,
 source_revision=GREATEST(h.source_revision,COALESCE(c.revision,h.source_revision)),checked_at=$1
 FROM candidates c WHERE h.account_type=c.account_type AND h.account_id=c.account_id AND h.field=c.field
 AND h.source_revision=c.previous_revision AND h.expected_amount=c.previous_amount`, now)
	return err
}
