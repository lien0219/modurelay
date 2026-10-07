-- Phase C2: bounded control-plane provenance and hash-only SCIM credentials.
CREATE TABLE IF NOT EXISTS workspace_scim_connectors (
 id BIGSERIAL PRIMARY KEY, workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
 public_endpoint_id VARCHAR(43) NOT NULL UNIQUE CHECK(public_endpoint_id ~ '^[A-Za-z0-9_-]{43}$'),
 name VARCHAR(120) NOT NULL CHECK(length(trim(name))>0), status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 default_role VARCHAR(20) NOT NULL CHECK(default_role IN ('viewer','developer','admin','billing')),
 group_mode VARCHAR(20) NOT NULL DEFAULT 'explicit' CHECK(group_mode='explicit'), revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 created_by_user_id BIGINT NOT NULL REFERENCES users(id), created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 disabled_at TIMESTAMPTZ,last_sync_at TIMESTAMPTZ,last_error_code VARCHAR(80),failure_count INTEGER NOT NULL DEFAULT 0,
 failure_notified_at TIMESTAMPTZ,conflict_notified_at TIMESTAMPTZ, UNIQUE(workspace_id,id)
);
CREATE TABLE IF NOT EXISTS workspace_scim_tokens (
 id BIGSERIAL PRIMARY KEY,workspace_id BIGINT NOT NULL,connector_id BIGINT NOT NULL,
 token_hash BYTEA NOT NULL UNIQUE CHECK(octet_length(token_hash)=32),token_prefix VARCHAR(24) NOT NULL,
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK(status IN ('active','revoked')),
 created_by_user_id BIGINT NOT NULL REFERENCES users(id),created_at TIMESTAMPTZ NOT NULL DEFAULT now(),expires_at TIMESTAMPTZ,revoked_at TIMESTAMPTZ,last_used_at TIMESTAMPTZ,expiry_notified_at TIMESTAMPTZ,
 CHECK(expires_at IS NULL OR expires_at>created_at AND expires_at<=created_at+interval '366 days 5 seconds'),
 FOREIGN KEY(workspace_id,connector_id) REFERENCES workspace_scim_connectors(workspace_id,id) ON DELETE CASCADE,
 UNIQUE(workspace_id,connector_id,id)
);
ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS administratively_suspended BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS effective_membership_source_id BIGINT;
ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS administratively_removed BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE workspace_audit_logs ALTER COLUMN actor_user_id DROP NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS workspace_members_user_scope ON workspace_members(workspace_id,id,user_id);
CREATE TABLE IF NOT EXISTS workspace_scim_users (
 id VARCHAR(43) PRIMARY KEY CHECK(id ~ '^[A-Za-z0-9_-]{43}$'),workspace_id BIGINT NOT NULL,connector_id BIGINT NOT NULL,user_id BIGINT NOT NULL REFERENCES users(id),member_id BIGINT NOT NULL,
 external_id VARCHAR(255),username VARCHAR(255) NOT NULL,primary_email VARCHAR(255) NOT NULL,active BOOLEAN NOT NULL DEFAULT true,
 attributes JSONB NOT NULL CHECK(jsonb_typeof(attributes)='object' AND octet_length(attributes::text)<=65536),revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),deleted BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,connector_id) REFERENCES workspace_scim_connectors(workspace_id,id),
 FOREIGN KEY(workspace_id,member_id,user_id) REFERENCES workspace_members(workspace_id,id,user_id),
 UNIQUE(workspace_id,connector_id,id),UNIQUE(workspace_id,connector_id,id,member_id),UNIQUE(workspace_id,connector_id,user_id),UNIQUE(workspace_id,connector_id,username),UNIQUE(workspace_id,connector_id,external_id)
);
CREATE TABLE IF NOT EXISTS workspace_scim_groups (
 id VARCHAR(43) PRIMARY KEY CHECK(id ~ '^[A-Za-z0-9_-]{43}$'),workspace_id BIGINT NOT NULL,connector_id BIGINT NOT NULL,
 external_id VARCHAR(255),display_name VARCHAR(255) NOT NULL,team_id BIGINT,revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),deleted BOOLEAN NOT NULL DEFAULT false,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(workspace_id,connector_id) REFERENCES workspace_scim_connectors(workspace_id,id),
 FOREIGN KEY(workspace_id,team_id) REFERENCES workspace_teams(workspace_id,id),
 UNIQUE(workspace_id,connector_id,id),UNIQUE(workspace_id,connector_id,external_id)
);
CREATE TABLE IF NOT EXISTS workspace_scim_group_members (
 workspace_id BIGINT NOT NULL,connector_id BIGINT NOT NULL,group_id VARCHAR(43) NOT NULL,user_id VARCHAR(43) NOT NULL,
 PRIMARY KEY(workspace_id,connector_id,group_id,user_id),
 FOREIGN KEY(workspace_id,connector_id,group_id) REFERENCES workspace_scim_groups(workspace_id,connector_id,id),
 FOREIGN KEY(workspace_id,connector_id,user_id) REFERENCES workspace_scim_users(workspace_id,connector_id,id)
);
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='workspace_identity_providers_typed_scope') THEN
 ALTER TABLE workspace_identity_providers ADD CONSTRAINT workspace_identity_providers_typed_scope UNIQUE(workspace_id,id,type); END IF; END $$;
CREATE TABLE IF NOT EXISTS workspace_membership_sources (
 id BIGSERIAL PRIMARY KEY,workspace_id BIGINT NOT NULL,member_id BIGINT NOT NULL,source_type VARCHAR(20) NOT NULL CHECK(source_type IN('manual','oidc','saml','scim')),
 provider_id BIGINT,connector_id BIGINT,scim_user_id VARCHAR(43),active BOOLEAN NOT NULL DEFAULT true,
 role VARCHAR(20) NOT NULL CHECK(role IN('owner','admin','developer','billing','viewer')),created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK(source_type='manual' OR role<>'owner'),
 CHECK((source_type='manual' AND provider_id IS NULL AND connector_id IS NULL AND scim_user_id IS NULL)
 OR (source_type IN('oidc','saml') AND provider_id IS NOT NULL AND connector_id IS NULL AND scim_user_id IS NULL)
 OR (source_type='scim' AND provider_id IS NULL AND connector_id IS NOT NULL AND scim_user_id IS NOT NULL)),
 FOREIGN KEY(workspace_id,member_id) REFERENCES workspace_members(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,provider_id,source_type) REFERENCES workspace_identity_providers(workspace_id,id,type),
 FOREIGN KEY(workspace_id,connector_id,scim_user_id,member_id) REFERENCES workspace_scim_users(workspace_id,connector_id,id,member_id),
 UNIQUE(workspace_id,member_id,source_type,provider_id),UNIQUE(workspace_id,member_id,scim_user_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_membership_sources_effective_scope ON workspace_membership_sources(workspace_id,member_id,id);
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='workspace_members_effective_source_scope') THEN
 ALTER TABLE workspace_members ADD CONSTRAINT workspace_members_effective_source_scope FOREIGN KEY(workspace_id,id,effective_membership_source_id) REFERENCES workspace_membership_sources(workspace_id,member_id,id) DEFERRABLE INITIALLY DEFERRED; END IF; END $$;
CREATE UNIQUE INDEX IF NOT EXISTS workspace_membership_sources_manual ON workspace_membership_sources(workspace_id,member_id) WHERE source_type='manual';
CREATE TABLE IF NOT EXISTS workspace_team_membership_sources (
 id BIGSERIAL PRIMARY KEY,workspace_id BIGINT NOT NULL,member_id BIGINT NOT NULL,team_id BIGINT NOT NULL,
 source_type VARCHAR(20) NOT NULL CHECK(source_type IN('manual','oidc','saml','scim')),
 provider_id BIGINT,connector_id BIGINT,scim_group_id VARCHAR(43),active BOOLEAN NOT NULL DEFAULT true,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 CHECK((source_type='manual' AND provider_id IS NULL AND connector_id IS NULL AND scim_group_id IS NULL)
 OR(source_type IN('oidc','saml') AND provider_id IS NOT NULL AND connector_id IS NULL AND scim_group_id IS NULL)
 OR(source_type='scim' AND provider_id IS NULL AND connector_id IS NOT NULL AND scim_group_id IS NOT NULL)),
 FOREIGN KEY(workspace_id,member_id) REFERENCES workspace_members(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,team_id) REFERENCES workspace_teams(workspace_id,id) ON DELETE CASCADE,
 FOREIGN KEY(workspace_id,provider_id,source_type) REFERENCES workspace_identity_providers(workspace_id,id,type),
 FOREIGN KEY(workspace_id,connector_id,scim_group_id) REFERENCES workspace_scim_groups(workspace_id,connector_id,id),
 UNIQUE(workspace_id,member_id,team_id,provider_id),UNIQUE(workspace_id,member_id,team_id,scim_group_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS workspace_team_membership_sources_manual ON workspace_team_membership_sources(workspace_id,member_id,team_id) WHERE source_type='manual';
-- Backfill once: never recreate removed attribution or overwrite new admin state
-- on raw SQL reruns. Legacy unbound SCIM has no connector proof: preserve manual.
DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='workspace_member_insert_provenance') THEN
 UPDATE workspace_members SET administratively_suspended=true WHERE status='suspended';
 INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,provider_id,active,role)
 SELECT m.workspace_id,m.id,CASE WHEN m.membership_provider_id IS NOT NULL AND m.membership_source IN('oidc','saml') AND m.role<>'owner' THEN m.membership_source ELSE 'manual' END,
 CASE WHEN m.membership_provider_id IS NOT NULL AND m.membership_source IN('oidc','saml') AND m.role<>'owner' THEN m.membership_provider_id END,m.status='active',m.role FROM workspace_members m ON CONFLICT DO NOTHING;
 INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,provider_id,active,role)
 SELECT DISTINCT i.workspace_id,m.id,p.type,p.id,m.status='active',CASE WHEN m.role='owner' THEN 'viewer' ELSE m.role END FROM workspace_user_identities i JOIN workspace_members m ON m.workspace_id=i.workspace_id AND m.user_id=i.user_id JOIN workspace_identity_providers p ON p.workspace_id=i.workspace_id AND p.id=i.provider_id ON CONFLICT DO NOTHING;
 INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,active)
 SELECT workspace_id,workspace_member_id,team_id,'manual',true FROM workspace_team_members WHERE membership_source IN('manual','scim') OR membership_provider_id IS NULL ON CONFLICT DO NOTHING;
 INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,provider_id,active)
 SELECT workspace_id,workspace_member_id,team_id,membership_source,membership_provider_id,true FROM workspace_team_members WHERE membership_source IN('oidc','saml') AND membership_provider_id IS NOT NULL ON CONFLICT DO NOTHING;
 INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type,provider_id,active)
 SELECT g.workspace_id,g.workspace_member_id,g.team_id,p.type,g.provider_id,true FROM workspace_identity_team_grants g JOIN workspace_identity_providers p ON p.workspace_id=g.workspace_id AND p.id=g.provider_id ON CONFLICT DO NOTHING;
 END IF; END $$;
CREATE OR REPLACE FUNCTION workspace_member_provenance_on_insert() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.membership_source IN('manual','scim') AND NEW.membership_provider_id IS NULL AND current_setting('modurelay.scim_insert',true) IS DISTINCT FROM 'true' THEN
 INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,role,active) VALUES(NEW.workspace_id,NEW.id,'manual',NEW.role,NEW.status='active') ON CONFLICT DO NOTHING;
 ELSIF NEW.membership_source IN('oidc','saml') AND NEW.membership_provider_id IS NOT NULL THEN
 INSERT INTO workspace_membership_sources(workspace_id,member_id,source_type,provider_id,role,active) VALUES(NEW.workspace_id,NEW.id,NEW.membership_source,NEW.membership_provider_id,NEW.role,NEW.status='active') ON CONFLICT DO NOTHING;
 END IF; RETURN NEW; END $$;
DROP TRIGGER IF EXISTS workspace_member_insert_provenance ON workspace_members;
CREATE TRIGGER workspace_member_insert_provenance AFTER INSERT ON workspace_members FOR EACH ROW EXECUTE FUNCTION workspace_member_provenance_on_insert();
CREATE OR REPLACE FUNCTION workspace_team_provenance_on_insert() RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
 IF NEW.membership_source='manual' THEN INSERT INTO workspace_team_membership_sources(workspace_id,member_id,team_id,source_type) VALUES(NEW.workspace_id,NEW.workspace_member_id,NEW.team_id,'manual') ON CONFLICT DO NOTHING; END IF; RETURN NEW; END $$;
DROP TRIGGER IF EXISTS workspace_team_insert_provenance ON workspace_team_members;
CREATE TRIGGER workspace_team_insert_provenance AFTER INSERT ON workspace_team_members FOR EACH ROW EXECUTE FUNCTION workspace_team_provenance_on_insert();
-- SCIM materialized membership has a connector source, not an identity provider.

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
     'require_sso','role_source','source_provider_id','role_count','team_count','mapping_revision','connector_id','token_id','resource_id','operation','added_count','removed_count','failure_count')
     OR jsonb_typeof(field) NOT IN ('string','number','boolean','null')
     OR (jsonb_typeof(field)='string' AND octet_length(field#>>'{}')>4096))
 END
$$;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_signup_source_check;
ALTER TABLE users ADD CONSTRAINT users_signup_source_check CHECK (signup_source IN ('email','linuxdo','wechat','oidc','saml','scim','github','google','dingtalk'));
