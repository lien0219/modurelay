-- Phase B: workspace-scoped enterprise identity foundation.
-- This migration only adds control-plane identity state. It does not rewrite
-- usage, billing, API keys, service credentials, or historical memberships.

ALTER TABLE workspace_members
    ADD COLUMN IF NOT EXISTS membership_source VARCHAR(20) NOT NULL DEFAULT 'manual'
    CHECK (membership_source IN ('manual', 'oidc', 'scim'));

ALTER TABLE workspace_team_members
    ADD COLUMN IF NOT EXISTS membership_source VARCHAR(20) NOT NULL DEFAULT 'manual'
    CHECK (membership_source IN ('manual', 'oidc', 'scim'));

CREATE TABLE IF NOT EXISTS workspace_domains (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain VARCHAR(255) NOT NULL CHECK (length(trim(domain)) BETWEEN 1 AND 255),
    normalized_domain VARCHAR(255) NOT NULL CHECK (length(normalized_domain) BETWEEN 1 AND 255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'failed', 'revoked')),
    verification_token_hash BYTEA NOT NULL CHECK (octet_length(verification_token_hash) = 32),
    verified_at TIMESTAMPTZ,
    verification_method VARCHAR(20) NOT NULL DEFAULT 'dns_txt' CHECK (verification_method IN ('dns_txt')),
    last_checked_at TIMESTAMPTZ,
    last_error_code VARCHAR(80),
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, normalized_domain)
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_domains_one_claim
    ON workspace_domains(normalized_domain)
    WHERE status <> 'revoked';
CREATE INDEX IF NOT EXISTS workspace_domains_workspace_status
    ON workspace_domains(workspace_id, status, id DESC);

CREATE TABLE IF NOT EXISTS workspace_identity_providers (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL DEFAULT 'oidc' CHECK (type IN ('oidc')),
    provider_key VARCHAR(80) NOT NULL CHECK (provider_key ~ '^[a-z0-9][a-z0-9_-]{0,79}$'),
    name VARCHAR(120) NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    is_default BOOLEAN NOT NULL DEFAULT false,
    issuer_url VARCHAR(2048) NOT NULL,
    client_id VARCHAR(512) NOT NULL,
    encrypted_client_secret TEXT,
    token_auth_method VARCHAR(32) NOT NULL DEFAULT 'client_secret_basic'
        CHECK (token_auth_method IN ('client_secret_basic', 'client_secret_post', 'none')),
    CHECK ((token_auth_method='none' AND encrypted_client_secret IS NULL)
        OR (token_auth_method IN ('client_secret_basic','client_secret_post')
            AND encrypted_client_secret IS NOT NULL AND length(encrypted_client_secret)>0)),
    scopes TEXT[] NOT NULL DEFAULT ARRAY['openid', 'profile', 'email']::TEXT[],
    authorization_endpoint VARCHAR(2048),
    token_endpoint VARCHAR(2048),
    jwks_uri VARCHAR(2048),
    userinfo_endpoint VARCHAR(2048),
    discovery_enabled BOOLEAN NOT NULL DEFAULT true,
    claim_mapping JSONB NOT NULL DEFAULT '{}',
    jit_config JSONB NOT NULL DEFAULT '{"enabled":false,"default_role":"viewer"}',
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disabled_at TIMESTAMPTZ,
    last_validated_at TIMESTAMPTZ,
    last_validation_code VARCHAR(80)
        CHECK (last_validation_code IN ('SUCCESS','DISCOVERY_FAILED','ISSUER_MISMATCH','ENDPOINT_INVALID','CONFIGURATION_INVALID','VALIDATION_FAILED','PROVIDER_DISABLED')),
    UNIQUE (workspace_id, provider_key),
    UNIQUE (workspace_id, id)
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_identity_providers_one_default
    ON workspace_identity_providers(workspace_id)
    WHERE is_default AND status = 'active';
CREATE INDEX IF NOT EXISTS workspace_identity_providers_workspace_status
    ON workspace_identity_providers(workspace_id, status, id DESC);

ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS membership_provider_id BIGINT;
ALTER TABLE workspace_team_members ADD COLUMN IF NOT EXISTS membership_provider_id BIGINT;
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='workspace_members_identity_provider_scope') THEN
        ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_identity_provider_scope
            FOREIGN KEY (workspace_id,membership_provider_id) REFERENCES workspace_identity_providers(workspace_id,id);
        ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_identity_source
            CHECK (membership_source='oidc' OR membership_provider_id IS NULL);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='workspace_team_members_identity_provider_scope') THEN
        ALTER TABLE workspace_team_members ADD CONSTRAINT workspace_team_members_identity_provider_scope
            FOREIGN KEY (workspace_id,membership_provider_id) REFERENCES workspace_identity_providers(workspace_id,id);
        ALTER TABLE workspace_team_members ADD CONSTRAINT workspace_team_members_identity_source
            CHECK (membership_source='oidc' OR membership_provider_id IS NULL);
    END IF;
END $$;

-- A team can be granted by several providers while retaining a manual/SCIM
-- membership. Reconciliation removes one provider's attribution, then removes
-- the materialized OIDC membership only when no attribution remains.
CREATE TABLE IF NOT EXISTS workspace_identity_team_grants (
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    team_id BIGINT NOT NULL,
    workspace_member_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id,provider_id,team_id,workspace_member_id),
    FOREIGN KEY (workspace_id,provider_id) REFERENCES workspace_identity_providers(workspace_id,id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id,team_id) REFERENCES workspace_teams(workspace_id,id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id,workspace_member_id) REFERENCES workspace_members(workspace_id,id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS workspace_user_identities (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject VARCHAR(1024) NOT NULL CHECK (length(trim(subject)) BETWEEN 1 AND 1024),
    email_at_link VARCHAR(320),
    display_name VARCHAR(512),
    email_verified BOOLEAN NOT NULL DEFAULT false,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider_id, subject),
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, provider_id)
        REFERENCES workspace_identity_providers(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_user_identities_user
    ON workspace_user_identities(workspace_id, user_id, id DESC);

CREATE TABLE IF NOT EXISTS workspace_identity_team_mappings (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    claim_value VARCHAR(512) NOT NULL,
    team_id BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider_id, claim_value, team_id),
    FOREIGN KEY (workspace_id, provider_id)
        REFERENCES workspace_identity_providers(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, team_id)
        REFERENCES workspace_teams(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_identity_team_mappings_lookup
    ON workspace_identity_team_mappings(workspace_id, provider_id, claim_value, enabled);

CREATE TABLE IF NOT EXISTS workspace_identity_role_mappings (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    claim_value VARCHAR(512) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('viewer', 'developer', 'admin', 'billing')),
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN -100000 AND 100000),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by_user_id BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider_id, claim_value, role),
    FOREIGN KEY (workspace_id, provider_id)
        REFERENCES workspace_identity_providers(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_identity_role_mappings_lookup
    ON workspace_identity_role_mappings(workspace_id, provider_id, claim_value, priority DESC);

CREATE TABLE IF NOT EXISTS workspace_identity_auth_states (
    id BIGSERIAL PRIMARY KEY,
    state_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(state_hash) = 32),
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    provider_revision BIGINT NOT NULL CHECK (provider_revision > 0),
    browser_session_hash BYTEA NOT NULL CHECK (octet_length(browser_session_hash) = 32),
    nonce_hash BYTEA NOT NULL CHECK (octet_length(nonce_hash) = 32),
    pkce_verifier_ciphertext TEXT NOT NULL,
    nonce_ciphertext TEXT NOT NULL,
    return_to VARCHAR(1024) NOT NULL DEFAULT '/workspaces',
    intent VARCHAR(20) NOT NULL DEFAULT 'login' CHECK (intent IN ('login', 'link')),
    link_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((intent='login' AND link_user_id IS NULL) OR (intent='link' AND link_user_id IS NOT NULL)),
    CHECK (expires_at>created_at AND expires_at<=created_at+interval '15 minutes 5 seconds'),
    FOREIGN KEY (workspace_id, provider_id)
        REFERENCES workspace_identity_providers(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_identity_auth_states_expiry
    ON workspace_identity_auth_states(expires_at);

CREATE TABLE IF NOT EXISTS workspace_identity_link_requests (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject VARCHAR(1024) NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, provider_id)
        REFERENCES workspace_identity_providers(workspace_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_identity_link_requests_expiry
    ON workspace_identity_link_requests(expires_at);

CREATE TABLE IF NOT EXISTS workspace_security_policies (
    workspace_id BIGINT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    require_sso BOOLEAN NOT NULL DEFAULT false,
    sso_grace_until TIMESTAMPTZ,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    updated_by_user_id BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO workspace_security_policies(workspace_id)
SELECT id FROM workspaces
ON CONFLICT (workspace_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS workspace_identity_break_glass_limits (
    workspace_id BIGINT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    last_used_at TIMESTAMPTZ NOT NULL,
    actor_user_id BIGINT NOT NULL REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS workspace_identity_login_completions (
    token_hash BYTEA PRIMARY KEY CHECK (octet_length(token_hash)=32),
    browser_session_hash BYTEA NOT NULL CHECK (octet_length(browser_session_hash)=32),
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider_id BIGINT NOT NULL,
    provider_revision BIGINT NOT NULL CHECK (provider_revision>0),
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    return_to VARCHAR(1024) NOT NULL,
    authenticated_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (expires_at>created_at AND expires_at<=created_at+interval '2 minutes 5 seconds'),
    FOREIGN KEY (workspace_id,provider_id) REFERENCES workspace_identity_providers(workspace_id,id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS workspace_identity_login_completions_expiry ON workspace_identity_login_completions(expires_at);

-- Keep the existing bounded scalar allowlist, adding only identity IDs and
-- configuration status. Subjects, emails, arbitrary claims and secrets stay
-- outside audit/event payloads.
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
     'require_sso','role_source','source_provider_id','role_count','team_count','mapping_revision')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;
