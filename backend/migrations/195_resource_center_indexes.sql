CREATE INDEX IF NOT EXISTS resource_posts_status_created_idx
    ON resource_posts(status, created_at DESC);

CREATE INDEX IF NOT EXISTS resource_posts_created_idx
    ON resource_posts(created_at DESC);

CREATE INDEX IF NOT EXISTS resource_comments_created_idx
    ON resource_comments(created_at DESC);
