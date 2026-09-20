-- Enroll existing deployments in the administrator-controlled Toolbox feature.
-- Preserve an administrator's explicit value when this migration is replayed.
INSERT INTO settings (key, value)
VALUES ('tool_center_enabled', 'true')
ON CONFLICT (key) DO NOTHING;
