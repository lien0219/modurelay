-- Synchronous immutable operation receipts, not a background queue. No leases
-- or scheduler; no financial, membership or historical attribution changes.
CREATE TABLE IF NOT EXISTS enterprise_admin_operations (
 id UUID PRIMARY KEY,
 actor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
 idempotency_key UUID NOT NULL,
 action TEXT NOT NULL CHECK(action IN ('suspend','resume','retry_webhook')),
 target_type TEXT NOT NULL CHECK(target_type IN ('workspace','webhook_delivery')),
 target_id BIGINT NOT NULL CHECK(target_id>0),
 request_fingerprint CHAR(64) NOT NULL CHECK(request_fingerprint ~ '^[0-9a-f]{64}$'),
 reason VARCHAR(500) NOT NULL CHECK(length(trim(reason)) BETWEEN 1 AND 500),
 previous_status VARCHAR(32) NOT NULL CHECK(length(trim(previous_status)) BETWEEN 1 AND 32),
 result_status VARCHAR(32) NOT NULL CHECK(length(trim(result_status)) BETWEEN 1 AND 32),
 result_updated_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(actor_user_id,idempotency_key),
 CHECK((action IN ('suspend','resume') AND target_type='workspace' AND target_id=workspace_id)
    OR (action='retry_webhook' AND target_type='webhook_delivery'))
);
CREATE INDEX IF NOT EXISTS enterprise_admin_operations_target
 ON enterprise_admin_operations(workspace_id,target_type,target_id,action,created_at DESC);
CREATE OR REPLACE FUNCTION guard_enterprise_admin_operation_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'administrator operation receipts are append-only' USING ERRCODE='23514';
END $$;
DROP TRIGGER IF EXISTS enterprise_admin_operation_immutable ON enterprise_admin_operations;
CREATE TRIGGER enterprise_admin_operation_immutable BEFORE UPDATE OR DELETE ON enterprise_admin_operations
 FOR EACH ROW EXECUTE FUNCTION guard_enterprise_admin_operation_receipt();
DROP TRIGGER IF EXISTS enterprise_admin_operation_truncate ON enterprise_admin_operations;
CREATE TRIGGER enterprise_admin_operation_truncate BEFORE TRUNCATE ON enterprise_admin_operations
 FOR EACH STATEMENT EXECUTE FUNCTION guard_enterprise_admin_operation_receipt();
