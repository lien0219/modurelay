-- Durable policy quota threshold state. The primary key makes a threshold
-- transition unique for one immutable scope/revision/window/type, while the
-- domain event and outbox rows are written in the same transaction.
CREATE OR REPLACE FUNCTION domain_event_payload_is_safe(value JSONB) RETURNS BOOLEAN
LANGUAGE SQL IMMUTABLE STRICT AS $$
 SELECT CASE
  WHEN jsonb_typeof(value) IS DISTINCT FROM 'object'
    OR jsonb_typeof(value->'data') IS DISTINCT FROM 'object'
    OR jsonb_typeof(value->'subject') IS DISTINCT FROM 'object' THEN FALSE
  ELSE
   NOT EXISTS (SELECT 1 FROM jsonb_object_keys(value) AS k(key)
    WHERE key NOT IN ('id','type','version','created_at','workspace_id','project_id','actor_user_id','subject','data'))
   AND NOT EXISTS (SELECT 1 FROM jsonb_object_keys(value->'subject') AS k(key) WHERE key NOT IN ('type','id'))
   AND NOT EXISTS (SELECT 1 FROM jsonb_each(value->'data') AS d(key,field)
    WHERE key NOT IN ('name','slug','status','role','user_id','member_id','invitation_id','key_id','key_name',
     'service_account_id','credential_id','credential_name','old_credential_id','new_credential_id','expires_at',
     'project_id','workspace_id','scope_type','scope_id','period_start','period_end','policy_revision','threshold',
     'quota_type','used','limit','amount','spent','reserved','estimated_amount','actual_amount','reason_code',
     'request_id','task_id','model','platform','previous_status','delivery_id','category')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;

CREATE TABLE IF NOT EXISTS policy_quota_alerts (
    scope_type TEXT NOT NULL CHECK (scope_type IN ('group', 'workspace', 'project', 'service_account', 'credential')),
    scope_id BIGINT NOT NULL CHECK (scope_id > 0),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    period_type TEXT NOT NULL CHECK (period_type IN ('daily', 'monthly')),
    period_start DATE NOT NULL,
    quota_type TEXT NOT NULL CHECK (quota_type IN ('requests', 'tokens')),
    threshold INTEGER NOT NULL CHECK (threshold IN (80, 100)),
    crossed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_type, scope_id, policy_revision, period_type, period_start, quota_type, threshold)
);

CREATE INDEX IF NOT EXISTS policy_quota_alerts_scope_period
    ON policy_quota_alerts(scope_type, scope_id, period_start);
