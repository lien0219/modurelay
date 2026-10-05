-- Preserve the actor/key owner while freezing the admitted payer and budget.
-- Historical jobs remain legacy jobs and are never attributed from current key state.
ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS workspace_id BIGINT REFERENCES workspaces(id),
    ADD COLUMN IF NOT EXISTS project_id BIGINT REFERENCES projects(id),
    ADD COLUMN IF NOT EXISTS billing_principal_user_id BIGINT REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS budget_reservation_id UUID REFERENCES budget_reservations(id),
    ADD COLUMN IF NOT EXISTS provider_create_started_at TIMESTAMPTZ;

ALTER TABLE batch_image_jobs
    ADD CONSTRAINT batch_image_jobs_tenant_snapshot CHECK (
        (workspace_id IS NULL AND project_id IS NULL AND billing_principal_user_id IS NULL AND budget_reservation_id IS NULL)
        OR (workspace_id IS NOT NULL AND workspace_id > 0
            AND project_id IS NOT NULL AND project_id > 0
            AND billing_principal_user_id IS NOT NULL AND billing_principal_user_id > 0)
    ),
    ADD CONSTRAINT batch_image_jobs_tenant_project FOREIGN KEY (workspace_id, project_id)
        REFERENCES projects(workspace_id, id),
    ADD CONSTRAINT batch_image_jobs_admitted_before_create CHECK (
        workspace_id IS NULL OR provider_create_started_at IS NULL OR budget_reservation_id IS NOT NULL
    );

CREATE INDEX IF NOT EXISTS batch_image_jobs_workspace_created
    ON batch_image_jobs(workspace_id, created_at) WHERE workspace_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS batch_image_jobs_budget_reservation
    ON batch_image_jobs(budget_reservation_id) WHERE budget_reservation_id IS NOT NULL;

CREATE OR REPLACE FUNCTION validate_batch_image_tenant_snapshot() RETURNS trigger AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF ROW(NEW.batch_id, NEW.user_id, NEW.api_key_id, NEW.workspace_id, NEW.project_id, NEW.billing_principal_user_id)
            IS DISTINCT FROM ROW(OLD.batch_id, OLD.user_id, OLD.api_key_id, OLD.workspace_id, OLD.project_id, OLD.billing_principal_user_id) THEN
            RAISE EXCEPTION 'batch image attribution is immutable' USING ERRCODE = '23514';
        END IF;
        IF OLD.budget_reservation_id IS NOT NULL AND NEW.budget_reservation_id IS DISTINCT FROM OLD.budget_reservation_id THEN
            RAISE EXCEPTION 'batch image budget reservation is immutable' USING ERRCODE = '23514';
        END IF;
        IF OLD.provider_create_started_at IS NOT NULL AND NEW.provider_create_started_at IS DISTINCT FROM OLD.provider_create_started_at THEN
            RAISE EXCEPTION 'batch image provider create marker is immutable' USING ERRCODE = '23514';
        END IF;
    END IF;
    IF NEW.budget_reservation_id IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM budget_reservations r
        WHERE r.id = NEW.budget_reservation_id
            AND r.request_id = 'batch_image_hold:' || NEW.batch_id
            AND r.actor_user_id = NEW.user_id AND r.api_key_id = NEW.api_key_id
            AND r.workspace_id = NEW.workspace_id AND r.project_id = NEW.project_id
            AND r.billing_principal_user_id = NEW.billing_principal_user_id
            AND r.estimate = round(NEW.estimated_cost, 8)
    ) THEN
        RAISE EXCEPTION 'batch image budget attribution mismatch' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER batch_image_jobs_tenant_snapshot_guard
BEFORE INSERT OR UPDATE OF batch_id, user_id, api_key_id, workspace_id, project_id,
    billing_principal_user_id, budget_reservation_id, provider_create_started_at
ON batch_image_jobs FOR EACH ROW EXECUTE FUNCTION validate_batch_image_tenant_snapshot();
