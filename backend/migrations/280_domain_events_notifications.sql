-- Transactional domain events, durable dispatch leases, and user inbox rows.
-- Event payloads are immutable; all delivery work is performed from the outbox.
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
     'project_id','workspace_id','scope_type','scope_id','period_start','policy_revision','threshold','amount',
     'spent','reserved','estimated_amount','actual_amount','reason_code','request_id','task_id','model',
     'platform','previous_status','delivery_id','category')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;

CREATE TABLE IF NOT EXISTS domain_events (
 id TEXT PRIMARY KEY,
 event_type TEXT NOT NULL CHECK (event_type=trim(event_type) AND length(event_type)>0),
 event_version INTEGER NOT NULL CHECK (event_version=1),
 created_at TIMESTAMPTZ NOT NULL,
 workspace_id BIGINT REFERENCES workspaces(id),
 project_id BIGINT,
 actor_user_id BIGINT REFERENCES users(id),
 subject_type TEXT NOT NULL CHECK (subject_type=trim(subject_type) AND length(subject_type)>0),
 subject_id TEXT NOT NULL CHECK (subject_id=trim(subject_id) AND length(subject_id)>0),
 payload JSONB NOT NULL,
 dedupe_key TEXT,
 UNIQUE(dedupe_key),
 CHECK ((domain_event_payload_is_safe(payload)
  AND payload->>'id'=id AND payload->>'type'=event_type
  AND payload->'version'=to_jsonb(event_version)
  AND (payload->>'created_at')::timestamptz=created_at
  AND payload->'subject'->>'type'=subject_type AND payload->'subject'->>'id'=subject_id
  AND (payload->>'workspace_id') IS NOT DISTINCT FROM workspace_id::text
  AND (payload->>'project_id') IS NOT DISTINCT FROM project_id::text
  AND (payload->>'actor_user_id') IS NOT DISTINCT FROM actor_user_id::text) IS TRUE),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id)
);
CREATE INDEX IF NOT EXISTS domain_events_scope_created ON domain_events(workspace_id,created_at DESC);
CREATE INDEX IF NOT EXISTS domain_events_type_created ON domain_events(event_type,created_at DESC);
CREATE INDEX IF NOT EXISTS domain_events_created ON domain_events(created_at,id);

CREATE OR REPLACE FUNCTION guard_domain_event_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'domain event envelope is immutable' USING ERRCODE='23514';
END $$;
DROP TRIGGER IF EXISTS domain_event_immutable_guard ON domain_events;
CREATE TRIGGER domain_event_immutable_guard BEFORE UPDATE ON domain_events
 FOR EACH ROW EXECUTE FUNCTION guard_domain_event_immutable();

CREATE TABLE IF NOT EXISTS domain_event_outbox (
 event_id TEXT PRIMARY KEY REFERENCES domain_events(id) ON DELETE CASCADE,
 available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts>=0),
 locked_until TIMESTAMPTZ,
 lock_token TEXT,
 last_error TEXT,
 delivered_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS domain_event_outbox_due ON domain_event_outbox(available_at,created_at,event_id)
 WHERE delivered_at IS NULL;
CREATE INDEX IF NOT EXISTS domain_event_outbox_retention ON domain_event_outbox(delivered_at,event_id)
 WHERE delivered_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS user_notifications (
 id BIGSERIAL PRIMARY KEY,
 event_id TEXT NOT NULL REFERENCES domain_events(id) ON DELETE CASCADE,
 recipient_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 workspace_id BIGINT,
 project_id BIGINT,
 category TEXT NOT NULL,
 title_key TEXT NOT NULL,
 body_key TEXT NOT NULL,
 data JSONB NOT NULL DEFAULT '{}',
 read_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(event_id,recipient_user_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id)
);
CREATE INDEX IF NOT EXISTS user_notifications_recipient_created ON user_notifications(recipient_user_id,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS user_notifications_unread ON user_notifications(recipient_user_id,id DESC) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS user_notifications_retention ON user_notifications(created_at,id);

CREATE TABLE IF NOT EXISTS budget_alert_transitions (
 scope_type TEXT NOT NULL CHECK (scope_type IN ('workspace','project')),
 scope_id BIGINT NOT NULL CHECK (scope_id>0),
 period_start DATE NOT NULL CHECK (extract(day FROM period_start)=1),
 policy_revision BIGINT NOT NULL CHECK (policy_revision>0),
 threshold INTEGER NOT NULL CHECK (threshold IN (50,80,100)),
 crossed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(scope_type,scope_id,period_start,policy_revision,threshold)
);
CREATE INDEX IF NOT EXISTS budget_alert_transitions_scope ON budget_alert_transitions(scope_type,scope_id,period_start);

-- Revision increments are independent of policy values so an edited policy can
-- emit its own threshold transition for the same calendar period.
ALTER TABLE workspace_budget_policies ADD COLUMN IF NOT EXISTS policy_revision BIGINT NOT NULL DEFAULT 1 CHECK (policy_revision>0);
ALTER TABLE project_budget_policies ADD COLUMN IF NOT EXISTS policy_revision BIGINT NOT NULL DEFAULT 1 CHECK (policy_revision>0);

-- Deploying the event system must not replay historical threshold crossings.
-- Seed the real finalized counters directly without creating events/outbox rows.
INSERT INTO budget_alert_transitions(scope_type,scope_id,period_start,policy_revision,threshold)
SELECT c.scope_type,c.scope_id,c.period_start,p.policy_revision,t.threshold
FROM budget_counters c JOIN workspace_budget_policies p ON c.scope_type='workspace' AND p.workspace_id=c.scope_id
CROSS JOIN (VALUES(50),(80),(100)) AS t(threshold)
WHERE p.enabled AND p.amount>0 AND c.spent*100>=p.amount*t.threshold
ON CONFLICT DO NOTHING;
INSERT INTO budget_alert_transitions(scope_type,scope_id,period_start,policy_revision,threshold)
SELECT c.scope_type,c.scope_id,c.period_start,p.policy_revision,t.threshold
FROM budget_counters c JOIN project_budget_policies p ON c.scope_type='project' AND p.project_id=c.scope_id
CROSS JOIN (VALUES(50),(80),(100)) AS t(threshold)
WHERE p.enabled AND p.amount>0 AND c.spent*100>=p.amount*t.threshold
ON CONFLICT DO NOTHING;
