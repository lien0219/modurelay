-- Phase C1: protocol-neutral SSO, independent encrypted SAML key material.
-- No historical machine credential, usage, billing or membership rewrites.
ALTER TABLE workspace_identity_providers
 ADD COLUMN IF NOT EXISTS saml_config JSONB,
 ADD COLUMN IF NOT EXISTS encrypted_saml_sp_keys TEXT,
 ADD COLUMN IF NOT EXISTS saml_public_id VARCHAR(64);
ALTER TABLE workspace_identity_providers DROP CONSTRAINT IF EXISTS workspace_identity_providers_type_check;
ALTER TABLE workspace_identity_providers
 ALTER COLUMN issuer_url DROP NOT NULL,
 ALTER COLUMN client_id DROP NOT NULL,
 ALTER COLUMN token_auth_method DROP NOT NULL,
 ALTER COLUMN scopes DROP NOT NULL,
 ALTER COLUMN discovery_enabled DROP NOT NULL;

-- Remove only the Phase B OIDC-only type and client-auth checks. PostgreSQL
-- assigns names to inline/anonymous checks; select by their precise columns.
DO $$ DECLARE c RECORD; BEGIN
 FOR c IN SELECT conname FROM pg_constraint
  WHERE conrelid='workspace_identity_providers'::regclass AND contype='c'
   AND (pg_get_constraintdef(oid) LIKE '%type %oidc%'
     OR (pg_get_constraintdef(oid) LIKE '%token_auth_method%'
       AND conname <> 'workspace_identity_providers_protocol_config'))
 LOOP EXECUTE format('ALTER TABLE workspace_identity_providers DROP CONSTRAINT %I',c.conname); END LOOP;
END $$;

CREATE OR REPLACE FUNCTION enterprise_saml_config_valid(value JSONB) RETURNS BOOLEAN
LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE cert JSONB; k TEXT;
BEGIN
 IF value IS NULL OR jsonb_typeof(value) IS DISTINCT FROM 'object' THEN RETURN false; END IF;
 FOR k IN SELECT jsonb_object_keys(value) LOOP
  IF k NOT IN ('idp_entity_id','sso_url','signing_certificates','metadata_url','metadata_source',
   'subject_attribute','allow_unspecified_name_id','email_attribute','name_attribute','groups_attribute',
   'sp_certificate','next_sp_certificate','authn_requests_signed') THEN RETURN false; END IF;
 END LOOP;
 IF jsonb_typeof(value->'idp_entity_id') IS DISTINCT FROM 'string'
  OR length(trim(value->>'idp_entity_id')) NOT BETWEEN 1 AND 2048
  OR jsonb_typeof(value->'sso_url') IS DISTINCT FROM 'string'
  OR length(value->>'sso_url') NOT BETWEEN 1 AND 2048
  OR (value->>'sso_url') !~ '^https://[^[:space:]]+$'
  OR jsonb_typeof(value->'signing_certificates') IS DISTINCT FROM 'array'
  OR jsonb_typeof(value->'sp_certificate') IS DISTINCT FROM 'string'
  OR length(trim(value->>'sp_certificate')) NOT BETWEEN 1 AND 16384
  OR value->'authn_requests_signed' IS DISTINCT FROM 'true'::jsonb
  OR jsonb_typeof(value->'allow_unspecified_name_id') IS DISTINCT FROM 'boolean'
 THEN RETURN false; END IF;
 IF jsonb_array_length(value->'signing_certificates') NOT BETWEEN 1 AND 8 THEN RETURN false; END IF;
 FOR cert IN SELECT jsonb_array_elements(value->'signing_certificates') LOOP
  IF jsonb_typeof(cert) IS DISTINCT FROM 'string'
   OR length(trim(cert#>>'{}')) NOT BETWEEN 1 AND 16384 THEN RETURN false; END IF;
 END LOOP;
 FOR k IN SELECT unnest(ARRAY['metadata_url','metadata_source','subject_attribute','email_attribute','name_attribute','groups_attribute','next_sp_certificate']) LOOP
  IF value ? k AND (jsonb_typeof(value->k) IS DISTINCT FROM 'string'
   OR length(value->>k)>CASE WHEN k='next_sp_certificate' THEN 16384 WHEN k='metadata_url' THEN 2048 ELSE 512 END)
  THEN RETURN false; END IF;
 END LOOP;
 RETURN true;
END $$;

ALTER TABLE workspace_identity_providers DROP CONSTRAINT IF EXISTS workspace_identity_providers_protocol_config;
ALTER TABLE workspace_identity_providers ADD CONSTRAINT workspace_identity_providers_protocol_config CHECK (
 (type='oidc' AND issuer_url IS NOT NULL AND length(trim(issuer_url))>0
  AND client_id IS NOT NULL AND length(trim(client_id))>0
  AND token_auth_method IS NOT NULL AND scopes IS NOT NULL AND discovery_enabled IS NOT NULL
  AND ((token_auth_method='none' AND encrypted_client_secret IS NULL)
   OR (token_auth_method IN ('client_secret_basic','client_secret_post')
    AND encrypted_client_secret IS NOT NULL AND length(encrypted_client_secret)>0))
  AND saml_config IS NULL AND encrypted_saml_sp_keys IS NULL AND saml_public_id IS NULL)
 OR (type='saml' AND issuer_url IS NULL AND client_id IS NULL
  AND encrypted_client_secret IS NULL AND token_auth_method IS NULL
  AND scopes IS NULL AND discovery_enabled IS NULL AND authorization_endpoint IS NULL
  AND token_endpoint IS NULL AND jwks_uri IS NULL AND userinfo_endpoint IS NULL
  AND enterprise_saml_config_valid(saml_config)
  AND encrypted_saml_sp_keys IS NOT NULL AND length(encrypted_saml_sp_keys)>0
  AND saml_public_id IS NOT NULL AND saml_public_id ~ '^[A-Za-z0-9_-]{43}$')
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_identity_providers_saml_public_id
 ON workspace_identity_providers(saml_public_id) WHERE saml_public_id IS NOT NULL;

ALTER TABLE workspace_members DROP CONSTRAINT IF EXISTS workspace_members_membership_source_check;
ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_membership_source_check CHECK (membership_source IN ('manual','oidc','saml','scim'));
ALTER TABLE workspace_members DROP CONSTRAINT IF EXISTS workspace_members_identity_source;
ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_identity_source CHECK (membership_source IN ('oidc','saml') OR membership_provider_id IS NULL);
ALTER TABLE workspace_team_members DROP CONSTRAINT IF EXISTS workspace_team_members_membership_source_check;
ALTER TABLE workspace_team_members ADD CONSTRAINT workspace_team_members_membership_source_check CHECK (membership_source IN ('manual','oidc','saml','scim'));
ALTER TABLE workspace_team_members DROP CONSTRAINT IF EXISTS workspace_team_members_identity_source;
ALTER TABLE workspace_team_members ADD CONSTRAINT workspace_team_members_identity_source CHECK (membership_source IN ('oidc','saml') OR membership_provider_id IS NULL);
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_signup_source_check;
ALTER TABLE users ADD CONSTRAINT users_signup_source_check CHECK (signup_source IN ('email','linuxdo','wechat','oidc','saml','github','google','dingtalk'));

ALTER TABLE workspace_identity_auth_states
 ADD COLUMN IF NOT EXISTS protocol VARCHAR(20) NOT NULL DEFAULT 'oidc',
 ADD COLUMN IF NOT EXISTS request_id VARCHAR(256),
 ALTER COLUMN nonce_hash DROP NOT NULL,
 ALTER COLUMN nonce_ciphertext DROP NOT NULL,
 ALTER COLUMN pkce_verifier_ciphertext DROP NOT NULL;
ALTER TABLE workspace_identity_auth_states DROP CONSTRAINT IF EXISTS workspace_identity_auth_states_protocol_config;
ALTER TABLE workspace_identity_auth_states ADD CONSTRAINT workspace_identity_auth_states_protocol_config CHECK (
 (protocol='oidc' AND request_id IS NULL AND nonce_hash IS NOT NULL
  AND octet_length(nonce_hash)=32 AND nonce_ciphertext IS NOT NULL AND length(nonce_ciphertext)>0
  AND pkce_verifier_ciphertext IS NOT NULL AND length(pkce_verifier_ciphertext)>0)
 OR (protocol='saml' AND request_id IS NOT NULL AND length(trim(request_id)) BETWEEN 1 AND 256
  AND nonce_hash IS NULL AND nonce_ciphertext IS NULL AND pkce_verifier_ciphertext IS NULL)
);
ALTER TABLE workspace_identity_login_completions
 ADD COLUMN IF NOT EXISTS auth_method VARCHAR(20) NOT NULL DEFAULT 'oidc';
ALTER TABLE workspace_identity_login_completions DROP CONSTRAINT IF EXISTS workspace_identity_login_completions_auth_method_check;
ALTER TABLE workspace_identity_login_completions ADD CONSTRAINT workspace_identity_login_completions_auth_method_check CHECK (auth_method IN ('oidc','saml'));
CREATE TABLE IF NOT EXISTS workspace_identity_saml_replays (
 id BIGSERIAL PRIMARY KEY,
 workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 provider_id BIGINT NOT NULL,
 id_hash BYTEA NOT NULL CHECK (octet_length(id_hash)=32),
 expires_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (workspace_id,provider_id,id_hash),
 CHECK (expires_at>created_at AND expires_at<=created_at+interval '24 hours 5 minutes'),
 FOREIGN KEY (workspace_id,provider_id) REFERENCES workspace_identity_providers(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_identity_saml_replays_expiry ON workspace_identity_saml_replays(expires_at,id);
