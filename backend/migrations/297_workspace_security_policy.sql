-- Human control-plane security only. Preserve all existing policy and hot data.
ALTER TABLE workspace_security_policies
 ADD COLUMN IF NOT EXISTS require_mfa BOOLEAN NOT NULL DEFAULT false,
 ADD COLUMN IF NOT EXISTS session_max_age_seconds INTEGER,
 ADD COLUMN IF NOT EXISTS invitation_policy TEXT NOT NULL DEFAULT 'any',
 ADD COLUMN IF NOT EXISTS allow_external_members BOOLEAN NOT NULL DEFAULT true,
 ADD COLUMN IF NOT EXISTS workspace_jit_enabled BOOLEAN NOT NULL DEFAULT true,
 ADD COLUMN IF NOT EXISTS approved_identity_provider_mode TEXT NOT NULL DEFAULT 'any_active';

DO $$ BEGIN
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='workspace_security_policies'::regclass AND conname='workspace_security_session_age') THEN
  ALTER TABLE workspace_security_policies ADD CONSTRAINT workspace_security_session_age CHECK (session_max_age_seconds IS NULL OR session_max_age_seconds BETWEEN 900 AND 2592000);
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='workspace_security_policies'::regclass AND conname='workspace_security_invitation_policy') THEN
  ALTER TABLE workspace_security_policies ADD CONSTRAINT workspace_security_invitation_policy CHECK (invitation_policy IN ('any','verified_domains_only','disabled'));
 END IF;
 IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='workspace_security_policies'::regclass AND conname='workspace_security_provider_mode') THEN
  ALTER TABLE workspace_security_policies ADD CONSTRAINT workspace_security_provider_mode CHECK (approved_identity_provider_mode IN ('any_active','selected'));
 END IF;
END $$;

CREATE TABLE IF NOT EXISTS workspace_security_approved_providers (
 workspace_id BIGINT NOT NULL REFERENCES workspace_security_policies(workspace_id) ON DELETE CASCADE,
 provider_id BIGINT NOT NULL,
 PRIMARY KEY (workspace_id,provider_id),
 FOREIGN KEY (workspace_id,provider_id) REFERENCES workspace_identity_providers(workspace_id,id) ON DELETE CASCADE
);

-- Assertion deadlines survive the browser completion exchange and refresh.
ALTER TABLE workspace_identity_login_completions ADD COLUMN IF NOT EXISTS valid_until TIMESTAMPTZ;

-- Scalar-only event data. Changed fields are bounded names, not claims/secrets.
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
     'request_id','task_id','model','platform','previous_status','delivery_id','category','team_id','grant_id',
     'subject_type','project_access_mode','domain_id','domain','normalized_domain','provider_id','provider_revision',
     'require_sso','role_source','source_provider_id','role_count','team_count','mapping_revision','connector_id','token_id','resource_id','operation','added_count','removed_count','failure_count',
     'previous_revision','require_mfa','session_max_age_seconds','invitation_policy','allow_external_members','workspace_jit_enabled','approved_identity_provider_mode','changed_fields')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;
