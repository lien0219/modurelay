-- Explicit opt-in retry events carry only bounded scalar state evidence.
-- This additive constraint supplements the existing envelope/data allowlist;
-- existing event types and subscriptions retain their original behavior.
DO $$
BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='domain_events'::regclass AND conname='domain_events_admin_webhook_retry_safe') THEN
  ALTER TABLE domain_events ADD CONSTRAINT domain_events_admin_webhook_retry_safe
   CHECK (event_type <> 'webhook.administrator_retried' OR (
    workspace_id > 0 AND actor_user_id > 0 AND project_id IS NULL
    AND subject_type = 'webhook_delivery'
    AND subject_id = payload#>>'{data,delivery_id}'
    AND payload->'data' = jsonb_build_object('previous_status','dead','status','retrying','delivery_id',payload#>'{data,delivery_id}')
    AND CASE WHEN jsonb_typeof(payload#>'{data,delivery_id}') = 'number'
      AND (payload#>>'{data,delivery_id}') ~ '^[1-9][0-9]{0,18}$'
     THEN (payload#>>'{data,delivery_id}')::numeric <= 9223372036854775807
     ELSE FALSE END
   ) IS TRUE);
 END IF;
END $$;
