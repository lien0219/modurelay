-- Registration and lazy bootstrap share the personal-workspace creation
-- function. Capture its actual INSERT rather than a racy repository pre-check.
-- Existing personal workspaces are deliberately not replayed on deployment.
CREATE OR REPLACE FUNCTION emit_personal_workspace_created() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE event_id TEXT; event_payload JSONB;
BEGIN
 event_id := 'evt_' || replace(gen_random_uuid()::text,'-','');
 event_payload := jsonb_build_object(
  'id',event_id,'type','workspace.created','version',1,'created_at',NEW.created_at,
  'workspace_id',NEW.id,'actor_user_id',NEW.owner_user_id,
  'subject',jsonb_build_object('type','workspace','id',NEW.id::text),
  'data',jsonb_build_object('name',NEW.name,'slug',NEW.slug,'status',NEW.status)
 );
 INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,actor_user_id,subject_type,subject_id,payload,dedupe_key)
 VALUES(event_id,'workspace.created',1,NEW.created_at,NEW.id,NEW.owner_user_id,'workspace',NEW.id::text,event_payload,'workspace.created:'||NEW.id);
 INSERT INTO domain_event_outbox(event_id) VALUES(event_id);
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS personal_workspace_created_event ON workspaces;
CREATE TRIGGER personal_workspace_created_event AFTER INSERT ON workspaces
 FOR EACH ROW WHEN (NEW.type='personal') EXECUTE FUNCTION emit_personal_workspace_created();
