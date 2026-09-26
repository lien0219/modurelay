-- Remove login identifiers from historical public resource-center snapshots.
-- Public forum identity is deliberately separate from account/login identity.
-- Keep author_id for authorization/moderation, but never expose phone/email-like
-- usernames through public author_username/actor_username snapshots.

UPDATE resource_posts
SET author_username = CASE
    WHEN author_role = 'admin' THEN '官方管理员'
    ELSE '用户 ' || author_id::text
END
WHERE
    author_username ~* '^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$'
    OR author_username ~ '^[0-9]{6,}$'
    OR (
        author_username ~ '^\+?[0-9][0-9[:space:]().-]{5,}[0-9]$'
        AND length(regexp_replace(author_username, '[^0-9]', '', 'g')) >= 7
    );

UPDATE resource_comments
SET author_username = CASE
    WHEN author_role = 'admin' THEN '官方管理员'
    ELSE '用户 ' || author_id::text
END
WHERE
    author_username ~* '^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$'
    OR author_username ~ '^[0-9]{6,}$'
    OR (
        author_username ~ '^\+?[0-9][0-9[:space:]().-]{5,}[0-9]$'
        AND length(regexp_replace(author_username, '[^0-9]', '', 'g')) >= 7
    );

UPDATE resource_notifications
SET actor_username = CASE
    WHEN actor_role = 'admin' THEN '官方管理员'
    ELSE '用户 ' || actor_id::text
END
WHERE
    actor_username ~* '^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$'
    OR actor_username ~ '^[0-9]{6,}$'
    OR (
        actor_username ~ '^\+?[0-9][0-9[:space:]().-]{5,}[0-9]$'
        AND length(regexp_replace(actor_username, '[^0-9]', '', 'g')) >= 7
    );
