CREATE TABLE IF NOT EXISTS resource_categories (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS resource_posts (
    id BIGSERIAL PRIMARY KEY,
    category_id BIGINT NOT NULL REFERENCES resource_categories(id) ON DELETE RESTRICT,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    author_username VARCHAR(100) NOT NULL,
    author_role VARCHAR(20) NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'published',
    view_count INTEGER NOT NULL DEFAULT 0,
    like_count INTEGER NOT NULL DEFAULT 0,
    comment_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS resource_posts_category_idx ON resource_posts(category_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS resource_posts_author_idx ON resource_posts(author_id, created_at DESC);

CREATE TABLE IF NOT EXISTS resource_comments (
    id BIGSERIAL PRIMARY KEY,
    post_id BIGINT NOT NULL REFERENCES resource_posts(id) ON DELETE CASCADE,
    parent_id BIGINT NULL REFERENCES resource_comments(id) ON DELETE CASCADE,
    author_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    author_username VARCHAR(100) NOT NULL,
    author_role VARCHAR(20) NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'published',
    like_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS resource_comments_post_idx ON resource_comments(post_id, status, created_at ASC);

CREATE TABLE IF NOT EXISTS resource_likes (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id BIGINT NULL REFERENCES resource_posts(id) ON DELETE CASCADE,
    comment_id BIGINT NULL REFERENCES resource_comments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT resource_likes_one_target CHECK ((post_id IS NOT NULL) <> (comment_id IS NOT NULL)
));
CREATE UNIQUE INDEX IF NOT EXISTS resource_likes_user_post_idx ON resource_likes(user_id, post_id) WHERE post_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS resource_likes_user_comment_idx ON resource_likes(user_id, comment_id) WHERE comment_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS resource_notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    actor_username VARCHAR(100) NOT NULL,
    actor_role VARCHAR(20) NOT NULL,
    post_id BIGINT NOT NULL REFERENCES resource_posts(id) ON DELETE CASCADE,
    comment_id BIGINT NULL REFERENCES resource_comments(id) ON DELETE CASCADE,
    kind VARCHAR(20) NOT NULL,
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS resource_notifications_user_idx ON resource_notifications(user_id, read, created_at DESC);

INSERT INTO resource_categories (slug, name, description, sort_order)
VALUES
    ('guides', '使用指南', '配置、教程与实践经验', 10),
    ('tools', '工具资源', '实用工具、脚本与工作流', 20),
    ('models', '模型交流', '模型体验、评测与提示词', 30),
    ('community', '社区交流', '问题讨论与经验分享', 40)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO settings (key, value) VALUES
    ('resource_center_enabled', 'true'),
    ('resource_center_forbid_urls', 'false'),
    ('resource_center_banned_words', '[]')
ON CONFLICT (key) DO NOTHING;
