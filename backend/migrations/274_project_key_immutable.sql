-- Established project assignments are immutable, including direct SQL writes.
-- The only transition is lazy assignment of a legacy NULL key to its creator's
-- personal default project. Organizational keys must be created in their scope.
CREATE OR REPLACE FUNCTION guard_api_key_project_assignment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF OLD.project_id IS NOT NULL AND NEW.project_id IS DISTINCT FROM OLD.project_id THEN
  RAISE EXCEPTION 'API key project assignment is immutable' USING ERRCODE='23514';
 END IF;
 IF OLD.project_id IS NULL AND NEW.project_id IS NOT NULL AND NOT EXISTS(
  SELECT 1 FROM projects p JOIN workspaces w ON w.id=p.workspace_id
  WHERE p.id=NEW.project_id AND p.is_default AND w.type='personal' AND w.owner_user_id=OLD.user_id
 ) THEN
  RAISE EXCEPTION 'legacy API key assignment must use personal default project' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS api_key_project_assignment_guard ON api_keys;
CREATE TRIGGER api_key_project_assignment_guard BEFORE UPDATE OF project_id ON api_keys
 FOR EACH ROW EXECUTE FUNCTION guard_api_key_project_assignment();

-- The hot-table index is created online by 279_usage_tenant_indexes_notx.sql.
