-- Video task billing IDs include provider prefixes and can exceed 64 characters.
-- Match the billing dedup tables while preserving existing request IDs and keys.
ALTER TABLE usage_logs
    ALTER COLUMN request_id TYPE VARCHAR(255);
