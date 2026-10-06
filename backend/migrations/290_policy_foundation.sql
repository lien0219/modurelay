-- Hierarchical access policies are stored per tenant-owned scope. Nullable
-- arrays retain inheritance (NULL) separately from deny-all ({}); numeric
-- limits use NULL for inheritance and strictly positive values for limits.
CREATE TABLE IF NOT EXISTS workspace_policies (
    workspace_id BIGINT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    allowed_models TEXT[],
    allowed_platforms TEXT[],
    rpm_limit BIGINT,
    daily_request_limit BIGINT,
    monthly_request_limit BIGINT,
    daily_token_limit BIGINT,
    monthly_token_limit BIGINT,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (allowed_models IS NULL OR cardinality(allowed_models) <= 1000),
    CHECK (allowed_platforms IS NULL OR cardinality(allowed_platforms) <= 256),
    CHECK (rpm_limit IS NULL OR rpm_limit > 0),
    CHECK (daily_request_limit IS NULL OR daily_request_limit > 0),
    CHECK (monthly_request_limit IS NULL OR monthly_request_limit > 0),
    CHECK (daily_token_limit IS NULL OR daily_token_limit > 0),
    CHECK (monthly_token_limit IS NULL OR monthly_token_limit > 0)
);

CREATE TABLE IF NOT EXISTS project_policies (
    project_id BIGINT PRIMARY KEY REFERENCES projects(id) ON DELETE CASCADE,
    allowed_models TEXT[],
    allowed_platforms TEXT[],
    rpm_limit BIGINT,
    daily_request_limit BIGINT,
    monthly_request_limit BIGINT,
    daily_token_limit BIGINT,
    monthly_token_limit BIGINT,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (allowed_models IS NULL OR cardinality(allowed_models) <= 1000),
    CHECK (allowed_platforms IS NULL OR cardinality(allowed_platforms) <= 256),
    CHECK (rpm_limit IS NULL OR rpm_limit > 0),
    CHECK (daily_request_limit IS NULL OR daily_request_limit > 0),
    CHECK (monthly_request_limit IS NULL OR monthly_request_limit > 0),
    CHECK (daily_token_limit IS NULL OR daily_token_limit > 0),
    CHECK (monthly_token_limit IS NULL OR monthly_token_limit > 0)
);

CREATE TABLE IF NOT EXISTS service_account_policies (
    service_account_id BIGINT PRIMARY KEY REFERENCES service_accounts(id) ON DELETE CASCADE,
    allowed_models TEXT[],
    allowed_platforms TEXT[],
    rpm_limit BIGINT,
    daily_request_limit BIGINT,
    monthly_request_limit BIGINT,
    daily_token_limit BIGINT,
    monthly_token_limit BIGINT,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (allowed_models IS NULL OR cardinality(allowed_models) <= 1000),
    CHECK (allowed_platforms IS NULL OR cardinality(allowed_platforms) <= 256),
    CHECK (rpm_limit IS NULL OR rpm_limit > 0),
    CHECK (daily_request_limit IS NULL OR daily_request_limit > 0),
    CHECK (monthly_request_limit IS NULL OR monthly_request_limit > 0),
    CHECK (daily_token_limit IS NULL OR daily_token_limit > 0),
    CHECK (monthly_token_limit IS NULL OR monthly_token_limit > 0)
);

CREATE INDEX IF NOT EXISTS project_policies_workspace ON project_policies(project_id);
CREATE INDEX IF NOT EXISTS service_account_policies_service_account ON service_account_policies(service_account_id);
