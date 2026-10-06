-- Durable request/token quota reservations. Counters are revision-scoped so
-- policy updates never rewrite historical usage. Admission aggregates all
-- revisions for the current UTC period before adding a new reservation.
CREATE TABLE IF NOT EXISTS policy_quota_counters (
    scope_type TEXT NOT NULL CHECK (scope_type IN ('group', 'workspace', 'project', 'service_account', 'credential')),
    scope_id BIGINT NOT NULL CHECK (scope_id > 0),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    period_type TEXT NOT NULL CHECK (period_type IN ('daily', 'monthly')),
    period_start DATE NOT NULL,
    requests_used BIGINT NOT NULL DEFAULT 0 CHECK (requests_used >= 0),
    requests_reserved BIGINT NOT NULL DEFAULT 0 CHECK (requests_reserved >= 0),
    tokens_used BIGINT NOT NULL DEFAULT 0 CHECK (tokens_used >= 0),
    tokens_reserved BIGINT NOT NULL DEFAULT 0 CHECK (tokens_reserved >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_type, scope_id, policy_revision, period_type, period_start)
);

CREATE TABLE IF NOT EXISTS policy_quota_reservations (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE RESTRICT,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE RESTRICT,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
    service_account_id BIGINT REFERENCES service_accounts(id) ON DELETE RESTRICT,
    estimated_tokens BIGINT NOT NULL DEFAULT 0 CHECK (estimated_tokens >= 0),
    request_units BIGINT NOT NULL DEFAULT 1 CHECK (request_units > 0),
    actual_tokens BIGINT CHECK (actual_tokens IS NULL OR actual_tokens >= 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'finalized', 'released')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (request_id, api_key_id)
);

CREATE INDEX IF NOT EXISTS policy_quota_reservations_status
    ON policy_quota_reservations(status, updated_at);

CREATE TABLE IF NOT EXISTS policy_quota_reservation_items (
    reservation_id TEXT NOT NULL REFERENCES policy_quota_reservations(id) ON DELETE CASCADE,
    scope_type TEXT NOT NULL CHECK (scope_type IN ('group', 'workspace', 'project', 'service_account', 'credential')),
    scope_id BIGINT NOT NULL CHECK (scope_id > 0),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    period_type TEXT NOT NULL CHECK (period_type IN ('daily', 'monthly')),
    period_start DATE NOT NULL,
    request_limit BIGINT CHECK (request_limit IS NULL OR request_limit > 0),
    token_limit BIGINT CHECK (token_limit IS NULL OR token_limit > 0),
    request_reserved BIGINT NOT NULL DEFAULT 0 CHECK (request_reserved >= 0),
    token_reserved BIGINT NOT NULL DEFAULT 0 CHECK (token_reserved >= 0),
    PRIMARY KEY (reservation_id, scope_type, scope_id, policy_revision, period_type, period_start)
);

CREATE INDEX IF NOT EXISTS policy_quota_reservation_items_scope
    ON policy_quota_reservation_items(scope_type, scope_id, period_type, period_start);
