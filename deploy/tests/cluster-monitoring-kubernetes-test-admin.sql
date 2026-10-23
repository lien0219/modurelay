-- ONLY for the disposable audit@example.test administrator in the dedicated
-- monitoring acceptance database. This is automated test fixture state, not
-- a real person's legal acknowledgement. Never execute on a business database.
BEGIN;
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM users WHERE email='audit@example.test')
     OR EXISTS (SELECT 1 FROM users WHERE email <> 'audit@example.test') THEN
    RAISE EXCEPTION 'expected a dedicated acceptance database with only the test admin';
  END IF;
END $$;
INSERT INTO settings(key,value)
SELECT 'admin_compliance_acknowledgement:' || id,
       json_build_object('version','v2026.06.10',
                         'admin_user_id',id,
                         'user_agent','disposable-automated-test-fixture',
                         'accepted_at',now())::text
FROM users WHERE email='audit@example.test'
ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value;
COMMIT;
