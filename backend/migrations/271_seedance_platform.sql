-- Add Seedance to the persisted platform enums used by quotas and explicit
-- Composite routes. Account platform values and historical usage rows are not
-- rewritten; legacy OpenAI Seedance accounts remain OpenAI accounts.

ALTER TABLE user_platform_quotas
    DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

ALTER TABLE user_platform_quotas
    ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                        'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'seedance'));

ALTER TABLE composite_model_routes
    DROP CONSTRAINT IF EXISTS composite_model_routes_target_platform_check;

ALTER TABLE composite_model_routes
    ADD CONSTRAINT composite_model_routes_target_platform_check
    CHECK (target_platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'grok',
                               'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'seedance'));

-- The API and admin route model already support endpoint-specific video routes.
-- Persisting endpoint='videos' must be allowed by the database as well.
ALTER TABLE composite_model_routes
    DROP CONSTRAINT IF EXISTS composite_model_routes_endpoint_check;

ALTER TABLE composite_model_routes
    ADD CONSTRAINT composite_model_routes_endpoint_check
    CHECK (endpoint IN ('any', 'messages', 'count_tokens', 'responses', 'chat_completions',
                        'embeddings', 'images', 'videos', 'gemini'));
