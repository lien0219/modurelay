-- A migration-time opening balance plus every subsequent monetary mutation.
-- Triggers cover direct SQL, holds, refunds, recharge and subscription resets.
-- These are observation tables, never a new balance authority or debit policy.
CREATE TABLE IF NOT EXISTS ops_account_ledger_heads (
 account_type TEXT NOT NULL, account_id BIGINT NOT NULL, field TEXT NOT NULL,
 opening_amount NUMERIC(30,10) NOT NULL,
 expected_amount NUMERIC(30,10) NOT NULL,
 actual_amount NUMERIC(30,10),
 checked_at TIMESTAMPTZ,
 mismatch BOOLEAN NOT NULL DEFAULT FALSE,
 PRIMARY KEY(account_type,account_id,field)
);
CREATE INDEX IF NOT EXISTS idx_ops_account_ledger_check ON ops_account_ledger_heads(checked_at NULLS FIRST,account_type,account_id,field);
CREATE TABLE IF NOT EXISTS ops_account_ledger_entries (
 id BIGSERIAL PRIMARY KEY,
 account_type TEXT NOT NULL, account_id BIGINT NOT NULL, field TEXT NOT NULL,
 before_amount NUMERIC(30,10) NOT NULL, after_amount NUMERIC(30,10) NOT NULL,
 delta NUMERIC(30,10) NOT NULL,
 intent_id BIGINT,
 transaction_id BIGINT NOT NULL DEFAULT txid_current(),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_ops_account_ledger_account ON ops_account_ledger_entries(account_type,account_id,field,id);
CREATE INDEX IF NOT EXISTS idx_ops_account_ledger_intent ON ops_account_ledger_entries(intent_id) WHERE intent_id IS NOT NULL;

LOCK TABLE users, user_subscriptions IN SHARE ROW EXCLUSIVE MODE;
INSERT INTO ops_account_ledger_heads(account_type,account_id,field,opening_amount,expected_amount)
SELECT 'user',id,'balance',balance,balance FROM users
UNION ALL SELECT 'user',id,'frozen_balance',COALESCE(frozen_balance,0),COALESCE(frozen_balance,0) FROM users
UNION ALL SELECT 'subscription',id,'daily_usage_usd',daily_usage_usd,daily_usage_usd FROM user_subscriptions
UNION ALL SELECT 'subscription',id,'weekly_usage_usd',weekly_usage_usd,weekly_usage_usd FROM user_subscriptions
UNION ALL SELECT 'subscription',id,'monthly_usage_usd',monthly_usage_usd,monthly_usage_usd FROM user_subscriptions
ON CONFLICT DO NOTHING;

CREATE OR REPLACE FUNCTION ops_capture_account_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE old_row JSONB; new_row JSONB; fields TEXT[]; field_name TEXT;
 before_value NUMERIC; after_value NUMERIC; entity_id BIGINT; kind TEXT; intent BIGINT;
BEGIN
 old_row := CASE WHEN TG_OP='INSERT' THEN '{}'::jsonb ELSE to_jsonb(OLD) END;
 new_row := CASE WHEN TG_OP='DELETE' THEN '{}'::jsonb ELSE to_jsonb(NEW) END;
 entity_id := COALESCE((new_row->>'id')::bigint,(old_row->>'id')::bigint);
 kind := CASE WHEN TG_TABLE_NAME='users' THEN 'user' ELSE 'subscription' END;
 fields := CASE WHEN kind='user' THEN ARRAY['balance','frozen_balance'] ELSE ARRAY['daily_usage_usd','weekly_usage_usd','monthly_usage_usd'] END;
 intent := NULLIF(current_setting('modurelay.billing_intent',true),'')::bigint;
 FOREACH field_name IN ARRAY fields LOOP
  before_value := COALESCE((old_row->>field_name)::numeric,0);
  after_value := COALESCE((new_row->>field_name)::numeric,0);
  IF before_value IS DISTINCT FROM after_value OR TG_OP='INSERT' THEN
   INSERT INTO ops_account_ledger_heads(account_type,account_id,field,opening_amount,expected_amount)
   VALUES(kind,entity_id,field_name,before_value,before_value) ON CONFLICT DO NOTHING;
   INSERT INTO ops_account_ledger_entries(account_type,account_id,field,before_amount,after_amount,delta,intent_id)
   VALUES(kind,entity_id,field_name,before_value,after_value,after_value-before_value,intent);
   UPDATE ops_account_ledger_heads SET expected_amount=expected_amount+(after_value-before_value), checked_at=NULL
   WHERE account_type=kind AND account_id=entity_id AND field=field_name;
  END IF;
 END LOOP;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
EXCEPTION WHEN OTHERS THEN
 -- A broken observation table must not change the original monetary mutation.
 -- The unchanged projection is detected by the independent reconciliation pass.
 RAISE WARNING 'account ledger observation failed (SQLSTATE %)', SQLSTATE;
 BEGIN
  INSERT INTO ops_billing_recording_gaps(instance_id,missed_count,first_missed_at,last_missed_at)
  VALUES('postgres-account-ledger',1,NOW(),NOW())
  ON CONFLICT(instance_id) DO UPDATE SET missed_count=ops_billing_recording_gaps.missed_count+1,last_missed_at=NOW();
 EXCEPTION WHEN OTHERS THEN
  RAISE WARNING 'account ledger gap persistence failed (SQLSTATE %)', SQLSTATE;
 END;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;
DROP TRIGGER IF EXISTS ops_account_ledger_user ON users;
CREATE TRIGGER ops_account_ledger_user AFTER INSERT OR DELETE OR UPDATE OF balance,frozen_balance ON users FOR EACH ROW EXECUTE FUNCTION ops_capture_account_ledger();
DROP TRIGGER IF EXISTS ops_account_ledger_subscription ON user_subscriptions;
CREATE TRIGGER ops_account_ledger_subscription AFTER INSERT OR DELETE OR UPDATE OF daily_usage_usd,weekly_usage_usd,monthly_usage_usd ON user_subscriptions FOR EACH ROW EXECUTE FUNCTION ops_capture_account_ledger();
