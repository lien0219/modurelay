package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// policyRepository is intentionally separate from WorkspaceRepository. Policy
// rows are tenant-owned and this adapter keeps scope-to-table resolution in one
// place so callers cannot accidentally issue an unscoped GetByID query.
type policyRepository struct{ db *sql.DB }

func NewPolicyRepository(db *sql.DB) domain.PolicyRepository {
	return &policyRepository{db: db}
}

type policyTable struct {
	table  string
	column string
}

func policyTableFor(ref domain.PolicyRef) (policyTable, error) {
	if ref.ScopeID <= 0 {
		return policyTable{}, domain.ErrInvalidPolicy
	}
	switch ref.Scope {
	case domain.PolicyScopeWorkspace:
		return policyTable{"workspace_policies", "workspace_id"}, nil
	case domain.PolicyScopeProject:
		return policyTable{"project_policies", "project_id"}, nil
	case domain.PolicyScopeServiceAccount:
		return policyTable{"service_account_policies", "service_account_id"}, nil
	default:
		// Group and credential restrictions remain owned by their existing
		// systems in this migration; they are still composed by the resolver.
		return policyTable{}, fmt.Errorf("%w: unsupported persisted scope %q", domain.ErrInvalidPolicy, ref.Scope)
	}
}

const policyColumns = `revision,allowed_models,allowed_platforms,rpm_limit,daily_request_limit,monthly_request_limit,daily_token_limit,monthly_token_limit`
const policyMutableColumns = `allowed_models,allowed_platforms,rpm_limit,daily_request_limit,monthly_request_limit,daily_token_limit,monthly_token_limit`

func scanPolicy(row interface{ Scan(...any) error }, ref domain.PolicyRef) (*domain.Policy, error) {
	p := &domain.Policy{Scope: ref.Scope, ScopeID: ref.ScopeID}
	var models, platforms pq.StringArray
	if err := row.Scan(&p.Revision, &models, &platforms, &p.RPMLimit, &p.DailyRequestLimit, &p.MonthlyRequestLimit, &p.DailyTokenLimit, &p.MonthlyTokenLimit); err != nil {
		return nil, err
	}
	// pq.StringArray preserves SQL NULL as nil and '{}' as a non-nil empty
	// slice, which is the API's inheritance versus deny-all contract.
	if models != nil {
		p.AllowedModels = append([]string{}, models...)
	}
	if platforms != nil {
		p.AllowedPlatforms = append([]string{}, platforms...)
	}
	return p, nil
}

func (r *policyRepository) GetPolicy(ctx context.Context, ref domain.PolicyRef) (*domain.Policy, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("%w: nil policy database", domain.ErrInvalidPolicy)
	}
	if (ref.Scope == domain.PolicyScopeGroup || ref.Scope == domain.PolicyScopeCredential) && ref.ScopeID <= 0 {
		return nil, domain.ErrInvalidPolicy
	}
	// Group and credential restrictions are still owned by their legacy
	// runtime objects. The policy resolver treats the absent persisted row as
	// inheritance; middleware applies those runtime predicates separately.
	if ref.Scope == domain.PolicyScopeGroup || ref.Scope == domain.PolicyScopeCredential {
		return nil, nil
	}
	info, err := policyTableFor(ref)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1", policyColumns, info.table, info.column)
	p, err := scanPolicy(r.db.QueryRowContext(ctx, query, ref.ScopeID), ref)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func policyValues(value domain.Policy) []any {
	return []any{
		pq.Array(value.AllowedModels),
		pq.Array(value.AllowedPlatforms),
		value.RPMLimit,
		value.DailyRequestLimit,
		value.MonthlyRequestLimit,
		value.DailyTokenLimit,
		value.MonthlyTokenLimit,
	}
}

func (r *policyRepository) UpdatePolicy(ctx context.Context, ref domain.PolicyRef, expectedRevision int64, value domain.Policy) (*domain.Policy, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("%w: nil policy database", domain.ErrInvalidPolicy)
	}
	if expectedRevision < 0 {
		return nil, fmt.Errorf("%w: expected revision cannot be negative", domain.ErrInvalidPolicy)
	}
	info, err := policyTableFor(ref)
	if err != nil {
		return nil, err
	}
	value.Scope, value.ScopeID, value.Revision = ref.Scope, ref.ScopeID, 0
	if err = value.Validate(); err != nil {
		return nil, err
	}
	actorID := service.PolicyActorID(ctx)
	if actorID <= 0 {
		return nil, fmt.Errorf("%w: authenticated policy actor is required", domain.ErrInvalidPolicy)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	workspaceID, projectID, err := policyTenantScope(ctx, tx, ref)
	if err != nil {
		return nil, err
	}
	args := policyValues(value)
	oldRevision := int64(0)
	changedFields := []string{"allowed_models", "allowed_platforms", "rpm_limit", "daily_request_limit", "monthly_request_limit", "daily_token_limit", "monthly_token_limit"}
	if expectedRevision == 0 {
		query := fmt.Sprintf(`INSERT INTO %s (%s,%s,revision,created_by_user_id,updated_by_user_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,$9,$10) ON CONFLICT (%s) DO NOTHING RETURNING revision`, info.table, info.column, policyMutableColumns, info.column)
		insertArgs := append([]any{ref.ScopeID}, args...)
		insertArgs = append(insertArgs, actorID, actorID)
		row := tx.QueryRowContext(ctx, query, insertArgs...)
		var revision int64
		if err = row.Scan(&revision); err == nil {
			value.Revision = revision
		} else if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		} else {
			return nil, domain.ErrPolicyRevisionConflict
		}
	} else {
		query := fmt.Sprintf("SELECT %s FROM %s WHERE %s=$1 FOR UPDATE", policyColumns, info.table, info.column)
		current, scanErr := scanPolicy(tx.QueryRowContext(ctx, query, ref.ScopeID), ref)
		if errors.Is(scanErr, sql.ErrNoRows) || (scanErr == nil && current.Revision != expectedRevision) {
			return nil, domain.ErrPolicyRevisionConflict
		}
		if scanErr != nil {
			return nil, scanErr
		}
		oldRevision = current.Revision
		changedFields = policyChangedFields(*current, value)
		updateQuery := fmt.Sprintf(`UPDATE %s SET allowed_models=$1, allowed_platforms=$2, rpm_limit=$3, daily_request_limit=$4, monthly_request_limit=$5, daily_token_limit=$6, monthly_token_limit=$7, revision=revision+1, updated_by_user_id=$8, updated_at=now() WHERE %s=$9 AND revision=$10 RETURNING revision`, info.table, info.column)
		row := tx.QueryRowContext(ctx, updateQuery, append(args, actorID, ref.ScopeID, expectedRevision)...)
		if err = row.Scan(&value.Revision); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, domain.ErrPolicyRevisionConflict
			}
			return nil, err
		}
	}
	var auditProjectID *int64
	if projectID > 0 {
		auditProjectID = &projectID
	}
	action, target := policyAuditScope(ref.Scope)
	metadata := map[string]any{"old_revision": oldRevision, "new_revision": value.Revision, "changed_fields": changedFields}
	if err = appendWorkspaceAudit(ctx, tx, workspaceID, actorID, auditProjectID, action, target, ref.ScopeID, metadata); err != nil {
		return nil, err
	}
	eventData := service.DomainEventData{"scope_type": string(ref.Scope), "scope_id": ref.ScopeID, "policy_revision": value.Revision}
	if err = insertWorkspaceMutationEvent(ctx, tx, workspaceID, projectID, actorID, "policy_updated", "policy", ref.ScopeID, eventData); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &value, nil
}

func policyTenantScope(ctx context.Context, q workspaceSQL, ref domain.PolicyRef) (int64, int64, error) {
	var workspaceID, projectID int64
	switch ref.Scope {
	case domain.PolicyScopeWorkspace:
		err := q.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE id=$1`, ref.ScopeID).Scan(&workspaceID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, domain.ErrPolicyNotFound
		}
		return workspaceID, 0, err
	case domain.PolicyScopeProject:
		err := q.QueryRowContext(ctx, `SELECT workspace_id FROM projects WHERE id=$1`, ref.ScopeID).Scan(&workspaceID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, domain.ErrPolicyNotFound
		}
		return workspaceID, ref.ScopeID, err
	case domain.PolicyScopeServiceAccount:
		err := q.QueryRowContext(ctx, `SELECT workspace_id,project_id FROM service_accounts WHERE id=$1`, ref.ScopeID).Scan(&workspaceID, &projectID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, domain.ErrPolicyNotFound
		}
		return workspaceID, projectID, err
	default:
		return 0, 0, domain.ErrInvalidPolicy
	}
}

func policyAuditScope(scope domain.PolicyScope) (string, string) {
	switch scope {
	case domain.PolicyScopeWorkspace:
		return "workspace_policy.updated", "workspace_policy"
	case domain.PolicyScopeProject:
		return "project_policy.updated", "project_policy"
	case domain.PolicyScopeServiceAccount:
		return "service_account_policy.updated", "service_account_policy"
	default:
		return "policy.updated", "policy"
	}
}

func policyChangedFields(old, next domain.Policy) []string {
	changed := make([]string, 0, 7)
	if !reflect.DeepEqual(old.AllowedModels, next.AllowedModels) {
		changed = append(changed, "allowed_models")
	}
	if !reflect.DeepEqual(old.AllowedPlatforms, next.AllowedPlatforms) {
		changed = append(changed, "allowed_platforms")
	}
	for _, field := range []struct {
		name string
		old  *int64
		new  *int64
	}{{"rpm_limit", old.RPMLimit, next.RPMLimit}, {"daily_request_limit", old.DailyRequestLimit, next.DailyRequestLimit}, {"monthly_request_limit", old.MonthlyRequestLimit, next.MonthlyRequestLimit}, {"daily_token_limit", old.DailyTokenLimit, next.DailyTokenLimit}, {"monthly_token_limit", old.MonthlyTokenLimit, next.MonthlyTokenLimit}} {
		if !reflect.DeepEqual(field.old, field.new) {
			changed = append(changed, field.name)
		}
	}
	return changed
}
