-- Remove observation-table I/O from monetary transactions. Fixed-size account
-- projections are updated in the already locked source row; the background
-- observer writes aggregate revision ranges, not a complete per-event journal.
DROP TRIGGER IF EXISTS ops_account_ledger_user ON users;
DROP TRIGGER IF EXISTS ops_account_ledger_subscription ON user_subscriptions;
ALTER TABLE users ADD COLUMN IF NOT EXISTS ops_balance_expected NUMERIC(30,10);
ALTER TABLE users ADD COLUMN IF NOT EXISTS ops_frozen_expected NUMERIC(30,10);
ALTER TABLE users ADD COLUMN IF NOT EXISTS ops_ledger_revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS ops_ledger_coverage_gap BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS ops_daily_expected NUMERIC(30,10);
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS ops_weekly_expected NUMERIC(30,10);
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS ops_monthly_expected NUMERIC(30,10);
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS ops_ledger_revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE user_subscriptions ADD COLUMN IF NOT EXISTS ops_ledger_coverage_gap BOOLEAN NOT NULL DEFAULT FALSE;

-- Preserve existing discrepancies from 279. Missing historical heads are an
-- explicit coverage gap, never silently rebaselined into a healthy account.
UPDATE users u SET
 ops_balance_expected=COALESCE((SELECT expected_amount FROM ops_account_ledger_heads WHERE account_type='user' AND account_id=u.id AND field='balance'),u.balance),
 ops_frozen_expected=COALESCE((SELECT expected_amount FROM ops_account_ledger_heads WHERE account_type='user' AND account_id=u.id AND field='frozen_balance'),u.frozen_balance),
 ops_ledger_coverage_gap=(SELECT COUNT(*) FROM ops_account_ledger_heads WHERE account_type='user' AND account_id=u.id)<2;
UPDATE user_subscriptions s SET
 ops_daily_expected=COALESCE((SELECT expected_amount FROM ops_account_ledger_heads WHERE account_type='subscription' AND account_id=s.id AND field='daily_usage_usd'),s.daily_usage_usd),
 ops_weekly_expected=COALESCE((SELECT expected_amount FROM ops_account_ledger_heads WHERE account_type='subscription' AND account_id=s.id AND field='weekly_usage_usd'),s.weekly_usage_usd),
 ops_monthly_expected=COALESCE((SELECT expected_amount FROM ops_account_ledger_heads WHERE account_type='subscription' AND account_id=s.id AND field='monthly_usage_usd'),s.monthly_usage_usd),
 ops_ledger_coverage_gap=(SELECT COUNT(*) FROM ops_account_ledger_heads WHERE account_type='subscription' AND account_id=s.id)<3;

ALTER TABLE ops_account_ledger_heads ADD COLUMN IF NOT EXISTS source_revision BIGINT NOT NULL DEFAULT 0;
ALTER TABLE ops_account_ledger_heads ADD COLUMN IF NOT EXISTS coverage_incomplete BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE ops_account_ledger_entries ADD COLUMN IF NOT EXISTS observation_kind TEXT NOT NULL DEFAULT 'transaction';
ALTER TABLE ops_account_ledger_entries ADD COLUMN IF NOT EXISTS previous_revision BIGINT;
ALTER TABLE ops_account_ledger_entries ADD COLUMN IF NOT EXISTS source_revision BIGINT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_account_ledger_aggregate_revision ON ops_account_ledger_entries(account_type,account_id,field,source_revision) WHERE source_revision IS NOT NULL;

CREATE OR REPLACE FUNCTION ops_capture_account_projection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='users' THEN
  IF TG_OP='INSERT' THEN
   NEW.ops_balance_expected:=NEW.balance;
   NEW.ops_frozen_expected:=COALESCE(NEW.frozen_balance,0);
   NEW.ops_ledger_revision:=1;
   NEW.ops_ledger_coverage_gap:=FALSE;
  ELSIF NEW.balance IS DISTINCT FROM OLD.balance OR NEW.frozen_balance IS DISTINCT FROM OLD.frozen_balance THEN
   NEW.ops_balance_expected:=OLD.ops_balance_expected+(NEW.balance-OLD.balance);
   NEW.ops_frozen_expected:=OLD.ops_frozen_expected+(COALESCE(NEW.frozen_balance,0)-COALESCE(OLD.frozen_balance,0));
   NEW.ops_ledger_revision:=OLD.ops_ledger_revision+1;
   NEW.ops_ledger_coverage_gap:=OLD.ops_ledger_coverage_gap;
  END IF;
 ELSE
  IF TG_OP='INSERT' THEN
   NEW.ops_daily_expected:=NEW.daily_usage_usd;
   NEW.ops_weekly_expected:=NEW.weekly_usage_usd;
   NEW.ops_monthly_expected:=NEW.monthly_usage_usd;
   NEW.ops_ledger_revision:=1;
   NEW.ops_ledger_coverage_gap:=FALSE;
  ELSIF NEW.daily_usage_usd IS DISTINCT FROM OLD.daily_usage_usd OR NEW.weekly_usage_usd IS DISTINCT FROM OLD.weekly_usage_usd OR NEW.monthly_usage_usd IS DISTINCT FROM OLD.monthly_usage_usd THEN
   NEW.ops_daily_expected:=OLD.ops_daily_expected+(NEW.daily_usage_usd-OLD.daily_usage_usd);
   NEW.ops_weekly_expected:=OLD.ops_weekly_expected+(NEW.weekly_usage_usd-OLD.weekly_usage_usd);
   NEW.ops_monthly_expected:=OLD.ops_monthly_expected+(NEW.monthly_usage_usd-OLD.monthly_usage_usd);
   NEW.ops_ledger_revision:=OLD.ops_ledger_revision+1;
   NEW.ops_ledger_coverage_gap:=OLD.ops_ledger_coverage_gap;
  END IF;
 END IF;
 RETURN NEW;
EXCEPTION WHEN OTHERS OR query_canceled THEN
 -- No SQL/monitoring-table dependency, even on error. Keep monetary fields and
 -- preserve an in-row coverage marker for the independent observer to report.
 NEW.ops_ledger_coverage_gap:=TRUE;
 RETURN NEW;
END $$;
CREATE TRIGGER ops_account_ledger_user BEFORE INSERT OR UPDATE OF balance,frozen_balance ON users FOR EACH ROW EXECUTE FUNCTION ops_capture_account_projection();
CREATE TRIGGER ops_account_ledger_subscription BEFORE INSERT OR UPDATE OF daily_usage_usd,weekly_usage_usd,monthly_usage_usd ON user_subscriptions FOR EACH ROW EXECUTE FUNCTION ops_capture_account_projection();
