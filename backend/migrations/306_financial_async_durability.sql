-- Additive recovery evidence. No historical usage or attribution is rewritten.
CREATE TABLE frozen_usage_recovery (
 id UUID PRIMARY KEY,
 request_id TEXT NOT NULL CHECK (length(request_id) BETWEEN 1 AND 256),
 api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
 actor_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 service_account_id BIGINT,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
 project_id BIGINT NOT NULL,
 billing_principal_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 reservation_id UUID NOT NULL REFERENCES budget_reservations(id) ON DELETE RESTRICT,
 account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
 fingerprint TEXT NOT NULL CHECK (length(fingerprint)=64),
 envelope JSONB NOT NULL CHECK (jsonb_typeof(envelope)='object' AND octet_length(envelope::text)<=65536 AND envelope->>'version'='1'),
 state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending','settled')),
 due_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 lease_token UUID,
 lease_until TIMESTAMPTZ,
 attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts>=0),
 cache_invalidated_at TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 settled_at TIMESTAMPTZ,
 UNIQUE(request_id,api_key_id),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(service_account_id,workspace_id,project_id) REFERENCES service_accounts(id,workspace_id,project_id) ON DELETE RESTRICT,
 CHECK ((actor_user_id IS NOT NULL)<>(service_account_id IS NOT NULL)),
 CHECK ((lease_token IS NULL)=(lease_until IS NULL)),
 CHECK ((state='settled')=(settled_at IS NOT NULL))
);
CREATE INDEX frozen_usage_due ON frozen_usage_recovery(due_at,id) WHERE state='pending';
CREATE INDEX frozen_usage_workspace_pending ON frozen_usage_recovery(workspace_id,id) WHERE state='pending';
CREATE INDEX frozen_usage_cache_invalidation ON frozen_usage_recovery(settled_at,id) WHERE state='settled' AND cache_invalidated_at IS NULL;

CREATE TABLE financial_media_records (
 kind TEXT NOT NULL CHECK (kind IN ('video','binding','attempt','image')),
 record_key TEXT NOT NULL CHECK (length(record_key) BETWEEN 1 AND 512),
 api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
 actor_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 service_account_id BIGINT REFERENCES service_accounts(id) ON DELETE RESTRICT,
 workspace_id BIGINT REFERENCES workspaces(id) ON DELETE RESTRICT,
 project_id BIGINT,
 billing_principal_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
 reservation_id UUID REFERENCES budget_reservations(id) ON DELETE RESTRICT,
 account_id BIGINT REFERENCES accounts(id) ON DELETE RESTRICT,
 payload JSONB NOT NULL CHECK (jsonb_typeof(payload)='object' AND octet_length(payload::text)<=2097152),
 state TEXT NOT NULL CHECK (state IN ('pending','settled','processing','completed','failed','unknown','cancelled','started','accepted')),
 due_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 provider_started_at TIMESTAMPTZ,
 billing_claim_token UUID,
 billing_claim_until TIMESTAMPTZ,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(kind,record_key),
 FOREIGN KEY(workspace_id,project_id) REFERENCES projects(workspace_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(service_account_id,workspace_id,project_id) REFERENCES service_accounts(id,workspace_id,project_id) ON DELETE RESTRICT,
 CHECK ((actor_user_id IS NOT NULL)<>(service_account_id IS NOT NULL)),
 CHECK ((workspace_id IS NULL AND project_id IS NULL AND billing_principal_user_id IS NULL AND reservation_id IS NULL)
     OR (workspace_id IS NOT NULL AND project_id IS NOT NULL AND billing_principal_user_id IS NOT NULL AND reservation_id IS NOT NULL)),
 CHECK (service_account_id IS NULL OR workspace_id IS NOT NULL)
);
CREATE INDEX financial_video_due ON financial_media_records(due_at,record_key) WHERE kind='video' AND state='pending';
CREATE INDEX financial_image_recovery ON financial_media_records(created_at,record_key) WHERE kind='image' AND state='processing';
CREATE INDEX financial_media_attempt_recovery ON financial_media_records(created_at,record_key) WHERE kind='attempt' AND state='started';
CREATE INDEX financial_media_pending_workspace ON financial_media_records(workspace_id,kind,state);

CREATE FUNCTION guard_financial_recovery_record() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='UPDATE' THEN
  IF ROW(NEW.api_key_id,NEW.actor_user_id,NEW.service_account_id,NEW.workspace_id,NEW.project_id,NEW.billing_principal_user_id,NEW.reservation_id,NEW.created_at)
   IS DISTINCT FROM ROW(OLD.api_key_id,OLD.actor_user_id,OLD.service_account_id,OLD.workspace_id,OLD.project_id,OLD.billing_principal_user_id,OLD.reservation_id,OLD.created_at) THEN
   RAISE EXCEPTION 'financial recovery identity is immutable' USING ERRCODE='23514';
  END IF;
  IF TG_TABLE_NAME='frozen_usage_recovery' THEN
   IF ROW(NEW.id,NEW.request_id,NEW.fingerprint,NEW.envelope,NEW.account_id) IS DISTINCT FROM ROW(OLD.id,OLD.request_id,OLD.fingerprint,OLD.envelope,OLD.account_id)
    OR (OLD.state='settled' AND NEW.state<>'settled') THEN
    RAISE EXCEPTION 'frozen accounting evidence is immutable' USING ERRCODE='23514';
   END IF;
  ELSE
   IF ROW(NEW.kind,NEW.record_key) IS DISTINCT FROM ROW(OLD.kind,OLD.record_key)
    OR (OLD.account_id IS NOT NULL AND NEW.account_id IS DISTINCT FROM OLD.account_id)
    OR (OLD.provider_started_at IS NOT NULL AND NEW.provider_started_at IS DISTINCT FROM OLD.provider_started_at)
    OR (OLD.payload->'settlement' IS NOT NULL AND NEW.payload->'settlement' IS DISTINCT FROM OLD.payload->'settlement') THEN
    RAISE EXCEPTION 'media ownership and frozen settlement are immutable' USING ERRCODE='23514';
   END IF;
  END IF;
 END IF;
 IF NEW.reservation_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM budget_reservations r WHERE r.id=NEW.reservation_id AND r.api_key_id=NEW.api_key_id
   AND r.actor_user_id IS NOT DISTINCT FROM NEW.actor_user_id AND r.service_account_id IS NOT DISTINCT FROM NEW.service_account_id
   AND r.workspace_id=NEW.workspace_id AND r.project_id=NEW.project_id AND r.billing_principal_user_id=NEW.billing_principal_user_id
 ) THEN
  RAISE EXCEPTION 'financial recovery reservation ownership mismatch' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER frozen_usage_identity BEFORE INSERT OR UPDATE ON frozen_usage_recovery FOR EACH ROW EXECUTE FUNCTION guard_financial_recovery_record();
CREATE TRIGGER media_recovery_identity BEFORE INSERT OR UPDATE ON financial_media_records FOR EACH ROW EXECUTE FUNCTION guard_financial_recovery_record();
CREATE TRIGGER frozen_usage_delete_guard BEFORE DELETE ON frozen_usage_recovery FOR EACH ROW EXECUTE FUNCTION guard_financial_record_deletion();
CREATE TRIGGER frozen_usage_truncate_guard BEFORE TRUNCATE ON frozen_usage_recovery FOR EACH STATEMENT EXECUTE FUNCTION guard_financial_record_deletion();
CREATE TRIGGER media_recovery_delete_guard BEFORE DELETE ON financial_media_records FOR EACH ROW EXECUTE FUNCTION guard_financial_record_deletion();
CREATE TRIGGER media_recovery_truncate_guard BEFORE TRUNCATE ON financial_media_records FOR EACH STATEMENT EXECUTE FUNCTION guard_financial_record_deletion();

-- SQL rehydration scans accepted jobs and safe failed-hold releases in bounded pages.
CREATE INDEX batch_image_sql_recovery ON batch_image_jobs(updated_at,id)
 WHERE status IN ('submitted','running','indexing','settling','failed','cancelled');
