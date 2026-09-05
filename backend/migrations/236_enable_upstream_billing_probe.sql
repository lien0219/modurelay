-- Balance/rate discovery is automatic for eligible static-key upstreams.
-- Preserve an administrator's explicit false opt-out; only legacy rows where
-- the setting did not exist are enrolled by this one-time migration.
UPDATE accounts
SET extra = jsonb_set(
        COALESCE(extra, '{}'::jsonb),
        '{upstream_billing_probe_enabled}',
        'true'::jsonb,
        true
    ),
    updated_at = NOW()
WHERE deleted_at IS NULL
  AND (
      (type = 'apikey' AND platform IN (
          'openai', 'anthropic', 'gemini', 'antigravity',
          'grok', 'kimi', 'zhipu', 'deepseek'
      ))
      OR (type = 'upstream' AND platform = 'antigravity')
  )
  AND NOT (COALESCE(extra, '{}'::jsonb) ? 'upstream_billing_probe_enabled');
