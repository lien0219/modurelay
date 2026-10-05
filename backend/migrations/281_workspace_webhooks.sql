-- Workspace webhook endpoints, event subscriptions, and durable deliveries.
-- Secrets are encrypted application values and are never returned by reads.
CREATE TABLE IF NOT EXISTS workspace_webhooks (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 name VARCHAR(100) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 100),
 url TEXT NOT NULL CHECK (length(trim(url)) BETWEEN 1 AND 2048),
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 disabled_at TIMESTAMPTZ,
 secret_current_encrypted TEXT NOT NULL,
 secret_previous_encrypted TEXT,
 previous_secret_until TIMESTAMPTZ,
 last_test_at TIMESTAMPTZ,
 created_by_user_id BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
 ,UNIQUE(workspace_id,id)
);
CREATE INDEX IF NOT EXISTS workspace_webhooks_scope ON workspace_webhooks(workspace_id,id);
CREATE TABLE IF NOT EXISTS workspace_webhook_subscriptions (
 webhook_id BIGINT NOT NULL REFERENCES workspace_webhooks(id) ON DELETE CASCADE,
 event_type TEXT NOT NULL CHECK (length(trim(event_type)) > 0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(webhook_id,event_type)
);
CREATE INDEX IF NOT EXISTS workspace_webhook_subscriptions_event ON workspace_webhook_subscriptions(event_type,webhook_id);

CREATE OR REPLACE FUNCTION guard_workspace_webhook_limit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 PERFORM 1 FROM workspaces WHERE id=NEW.workspace_id FOR UPDATE;
 IF (SELECT count(*) FROM workspace_webhooks WHERE workspace_id=NEW.workspace_id AND (TG_OP='INSERT' OR id<>NEW.id)) >= 10 THEN
  RAISE EXCEPTION 'workspace webhook endpoint limit exceeded' USING ERRCODE='23514', CONSTRAINT='workspace_webhooks_limit';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS workspace_webhook_limit_guard ON workspace_webhooks;
DROP TRIGGER IF EXISTS workspace_webhook_limit_update_guard ON workspace_webhooks;
CREATE TRIGGER workspace_webhook_limit_guard BEFORE INSERT ON workspace_webhooks
 FOR EACH ROW EXECUTE FUNCTION guard_workspace_webhook_limit();
CREATE TRIGGER workspace_webhook_limit_update_guard BEFORE UPDATE OF workspace_id ON workspace_webhooks
 FOR EACH ROW WHEN (NEW.workspace_id IS DISTINCT FROM OLD.workspace_id) EXECUTE FUNCTION guard_workspace_webhook_limit();

-- The workspace limit survives endpoint deletion so repeatedly creating new
-- endpoints cannot bypass the test-send rate limit.
CREATE TABLE IF NOT EXISTS workspace_webhook_test_limits (
 workspace_id BIGINT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
 window_started_at TIMESTAMPTZ NOT NULL,
 test_count INTEGER NOT NULL CHECK (test_count BETWEEN 1 AND 10)
);

CREATE TABLE IF NOT EXISTS workspace_webhook_deliveries (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 webhook_id BIGINT NOT NULL REFERENCES workspace_webhooks(id) ON DELETE CASCADE,
 event_id TEXT NOT NULL REFERENCES domain_events(id) ON DELETE CASCADE,
 event_type TEXT NOT NULL,
 payload JSONB NOT NULL,
 status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','delivering','succeeded','retrying','dead')),
 attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 last_attempt_at TIMESTAMPTZ,
 response_status INTEGER,
 response_preview TEXT,
 last_error TEXT,
 error_code TEXT,
 lock_owner TEXT,
 locked_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 delivered_at TIMESTAMPTZ,
 finished_at TIMESTAMPTZ,
 UNIQUE(webhook_id,event_id),
 FOREIGN KEY(workspace_id,webhook_id) REFERENCES workspace_webhooks(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_webhook_deliveries_due ON workspace_webhook_deliveries(next_attempt_at,created_at)
 WHERE status IN ('pending','retrying','delivering');
CREATE INDEX IF NOT EXISTS workspace_webhook_deliveries_scope ON workspace_webhook_deliveries(workspace_id,webhook_id,created_at DESC);
CREATE INDEX IF NOT EXISTS workspace_webhook_deliveries_finished ON workspace_webhook_deliveries(status,finished_at,id)
 WHERE status IN ('succeeded','dead');

CREATE OR REPLACE FUNCTION guard_workspace_webhook_delivery_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.event_id IS DISTINCT FROM NEW.event_id OR OLD.event_type IS DISTINCT FROM NEW.event_type OR OLD.payload IS DISTINCT FROM NEW.payload OR OLD.workspace_id IS DISTINCT FROM NEW.workspace_id OR OLD.webhook_id IS DISTINCT FROM NEW.webhook_id THEN
  RAISE EXCEPTION 'workspace webhook delivery event envelope is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS workspace_webhook_delivery_immutable_guard ON workspace_webhook_deliveries;
CREATE TRIGGER workspace_webhook_delivery_immutable_guard BEFORE UPDATE ON workspace_webhook_deliveries
 FOR EACH ROW EXECUTE FUNCTION guard_workspace_webhook_delivery_immutable();
