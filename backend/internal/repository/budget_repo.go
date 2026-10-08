package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type budgetRepository struct {
	db  *sql.DB
	now func() time.Time
}

func NewBudgetRepository(db *sql.DB) service.BudgetRepository {
	return &budgetRepository{db: db, now: time.Now}
}

// Budget amounts share the wallet's NUMERIC(20,8) range and rounding. Reject
// non-finite values before they reach either PostgreSQL or decimal conversion.
func validBudgetAmount(amount float64) bool {
	return !math.IsNaN(amount) && !math.IsInf(amount, 0) && amount >= 0 && amount < 1e12 && service.QuantizeUsageBillingAmount(amount) < 1e12
}

func (r *budgetRepository) Reserve(ctx context.Context, a service.BudgetAttribution, requestID string, estimate float64) (*service.BudgetReservation, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrBudgetUnavailable
	}
	requestID = strings.TrimSpace(requestID)
	if a.WorkspaceID <= 0 || a.ProjectID <= 0 || a.BillingPrincipalUserID <= 0 || !service.ValidExecutionAttribution(a.ActorUserID, a.ServiceAccountID) || a.APIKeyID <= 0 || requestID == "" {
		return nil, service.ErrBudgetReservationInvalid
	}
	if !validBudgetAmount(estimate) {
		return nil, service.ErrBudgetUnpriced
	}
	estimate = service.QuantizeUsageBillingAmount(estimate)
	allocation := service.UnallocatedAllocation()
	if a.Allocation != nil {
		allocation = *a.Allocation
	}
	normalizedAllocation, err := allocation.NormalizeAndValidate()
	if err != nil {
		return nil, service.ErrBudgetReservationInvalid
	}
	a.Allocation = &normalizedAllocation
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockBudgetAdmissionUsers(ctx, tx, a); err != nil {
		return nil, err
	}
	if err = lockBudgetScopes(ctx, tx, a.WorkspaceID, a.ProjectID, &a); err != nil {
		return nil, err
	}
	if err = validateBudgetAdmissionKey(ctx, tx, a); err != nil {
		return nil, err
	}

	existing, err := scanBudgetReservation(tx.QueryRowContext(ctx, `SELECT `+budgetReservationColumns+` FROM budget_reservations WHERE request_id=$1 AND api_key_id=$2 FOR UPDATE`, requestID, a.APIKeyID))
	if err == nil {
		if !budgetAttributionMatches(existing, a) || existing.Estimate != estimate {
			return nil, service.ErrBudgetReservationConflict
		}
		if existing.Status != "pending" {
			return nil, service.ErrBudgetReservationClosed
		}
		existing.Allocation, err = loadReservationAllocationSnapshot(ctx, tx, existing.ID)
		if err != nil {
			return nil, err
		}
		if existing.Allocation == nil {
			fallback := service.UnallocatedAllocation()
			existing.Allocation = &fallback
		}
		if !allocationSnapshotsEqual(existing.Allocation, a.Allocation) {
			return nil, service.ErrBudgetReservationConflict
		}
		for _, scope := range reservationBudgetScopes(existing) {
			if err = validatePendingBudgetCounter(ctx, tx, scope, existing.Estimate); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	workspacePolicy, err := loadBudgetPolicy(ctx, tx, "workspace", a.WorkspaceID)
	if err != nil {
		return nil, err
	}
	projectPolicy, err := loadBudgetPolicy(ctx, tx, "project", a.ProjectID)
	if err != nil {
		return nil, err
	}
	workspacePeriod, err := budgetMonthStart(now, workspacePolicy.timezone)
	if err != nil {
		return nil, err
	}
	projectPeriod, err := budgetMonthStart(now, projectPolicy.timezone)
	if err != nil {
		return nil, err
	}
	for _, scope := range []budgetScope{{"workspace", a.WorkspaceID, workspacePeriod}, {"project", a.ProjectID, projectPeriod}} {
		policy := workspacePolicy
		if scope.typ == "project" {
			policy = projectPolicy
		}
		if err = reserveBudgetCounter(ctx, tx, scope, estimate, policy); err != nil {
			if errors.Is(err, budgetExceeded(scope.typ)) {
				_ = tx.Rollback()
				if persistErr := r.recordHardBudgetReject(ctx, scope, a.WorkspaceID, a.ProjectID, estimate); persistErr != nil {
					return nil, fmt.Errorf("%w (persist hard budget transition: %v)", err, persistErr)
				}
			}
			return nil, err
		}
	}
	reservation := &service.BudgetReservation{
		ID: uuid.NewString(), RequestID: requestID, ActorUserID: a.ActorUserID, ServiceAccountID: a.ServiceAccountID, APIKeyID: a.APIKeyID,
		WorkspaceID: a.WorkspaceID, ProjectID: a.ProjectID, BillingPrincipalUserID: a.BillingPrincipalUserID,
		PeriodStart: workspacePeriod, PeriodEnd: workspacePeriod.AddDate(0, 1, 0),
		ProjectPeriodStart: projectPeriod, ProjectPeriodEnd: projectPeriod.AddDate(0, 1, 0),
		Estimate: estimate, Status: "pending",
		Allocation: &normalizedAllocation,
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO budget_reservations(id,request_id,actor_user_id,api_key_id,workspace_id,project_id,billing_principal_user_id,period_start,period_end,project_period_start,project_period_end,estimate,status,service_account_id) VALUES($1,$2,NULLIF($3,0),$4,$5,$6,$7,$8,$9,$10,$11,$12,'pending',NULLIF($13,0))`,
		reservation.ID, requestID, a.ActorUserID, a.APIKeyID, a.WorkspaceID, a.ProjectID, a.BillingPrincipalUserID,
		reservation.PeriodStart, reservation.PeriodEnd, reservation.ProjectPeriodStart, reservation.ProjectPeriodEnd, estimate, a.ServiceAccountID)
	if err != nil {
		return nil, err
	}
	tagsJSON, err := normalizedAllocation.TagsJSON()
	if err != nil {
		return nil, service.ErrBudgetReservationInvalid
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO budget_reservation_allocation_snapshots(reservation_id,workspace_id,project_id,billing_principal_user_id,cost_center_id,environment,allocation_tags,policy_revision) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8)`,
		reservation.ID, reservation.WorkspaceID, reservation.ProjectID, reservation.BillingPrincipalUserID, nullableAllocationCenter(normalizedAllocation.CostCenterID), normalizedAllocation.Environment, tagsJSON, normalizedAllocation.PolicyRevision)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return reservation, nil
}

func nullableAllocationCenter(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}

func loadReservationAllocationSnapshot(ctx context.Context, tx *sql.Tx, reservationID string) (*service.AllocationSnapshot, error) {
	var center sql.NullInt64
	var environment string
	var tagsRaw []byte
	var revision int64
	err := tx.QueryRowContext(ctx, `SELECT cost_center_id,environment,allocation_tags,policy_revision FROM budget_reservation_allocation_snapshots WHERE reservation_id=$1`, reservationID).Scan(&center, &environment, &tagsRaw, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tags map[string]string
	if err := json.Unmarshal(tagsRaw, &tags); err != nil {
		return nil, service.ErrBudgetReservationInvalid
	}
	snapshot := &service.AllocationSnapshot{Environment: environment, Tags: tags, AllocationTags: tags, PolicyRevision: revision}
	if center.Valid {
		snapshot.CostCenterID = &center.Int64
	}
	normalized, err := snapshot.NormalizeAndValidate()
	if err != nil {
		return nil, service.ErrBudgetReservationInvalid
	}
	return &normalized, nil
}

func allocationSnapshotsEqual(a, b *service.AllocationSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Environment != b.Environment || a.PolicyRevision != b.PolicyRevision {
		return false
	}
	if (a.CostCenterID == nil) != (b.CostCenterID == nil) {
		return false
	}
	if a.CostCenterID != nil && *a.CostCenterID != *b.CostCenterID {
		return false
	}
	if len(a.Tags) != len(b.Tags) {
		return false
	}
	for key, value := range a.Tags {
		if b.Tags[key] != value {
			return false
		}
	}
	return true
}

func budgetMonthStart(now time.Time, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return time.Time{}, err
	}
	local := now.In(loc)
	// DATE counters use the local calendar's month label, also used by GetBudget.
	return time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

func lockBudgetAdmissionUsers(ctx context.Context, tx *sql.Tx, a service.BudgetAttribution) error {
	// User lifecycle mutations lock users before workspaces. Preserve that order
	// and use ID order when the creator and wallet owner are different users.
	rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE id IN ($1,$2) AND status='active' AND deleted_at IS NULL ORDER BY id FOR SHARE`, a.ActorUserID, a.BillingPrincipalUserID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	want := 2
	if a.ActorUserID == a.BillingPrincipalUserID || a.ActorUserID == 0 {
		want = 1
	}
	if count != want {
		return service.ErrWorkspaceForbidden
	}
	return nil
}

// A nil admission validates only durable scope identity, so archived projects
// and changed billing owners can still settle the snapshot they admitted.
func lockBudgetScopes(ctx context.Context, tx *sql.Tx, workspaceID, projectID int64, admission *service.BudgetAttribution) error {
	var principal int64
	var status string
	err := tx.QueryRowContext(ctx, `SELECT billing_owner_user_id,status FROM workspaces WHERE id=$1 FOR UPDATE`, workspaceID).Scan(&principal, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrWorkspaceNotFound
	}
	if err != nil {
		return err
	}
	if admission != nil && (status != "active" || principal != admission.BillingPrincipalUserID) {
		return service.ErrWorkspaceForbidden
	}
	err = tx.QueryRowContext(ctx, `SELECT status FROM projects WHERE id=$1 AND workspace_id=$2 FOR UPDATE`, projectID, workspaceID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrWorkspaceNotFound
	}
	if err != nil {
		return err
	}
	if admission != nil && status != "active" {
		return service.ErrWorkspaceForbidden
	}
	return nil
}

func validateBudgetAdmissionKey(ctx context.Context, tx *sql.Tx, a service.BudgetAttribution) error {
	var id int64
	var err error
	if a.ServiceAccountID > 0 {
		err = tx.QueryRowContext(ctx, `SELECT k.id FROM api_keys k JOIN service_accounts sa ON sa.id=k.service_account_id AND sa.project_id=k.project_id
 WHERE k.id=$1 AND k.user_id IS NULL AND k.service_account_id=$2 AND k.project_id=$3 AND sa.workspace_id=$4 AND sa.status='active'
 AND k.deleted_at IS NULL AND k.status='active' AND (k.expires_at IS NULL OR k.expires_at>now()) AND (k.quota=0 OR k.quota_used<k.quota) FOR SHARE OF k,sa`, a.APIKeyID, a.ServiceAccountID, a.ProjectID, a.WorkspaceID).Scan(&id)
	} else {
		err = tx.QueryRowContext(ctx, `SELECT id FROM api_keys WHERE id=$1 AND user_id=$2 AND service_account_id IS NULL AND project_id=$3 AND deleted_at IS NULL AND status='active' AND (expires_at IS NULL OR expires_at>now()) AND (quota=0 OR quota_used<quota) FOR SHARE`, a.APIKeyID, a.ActorUserID, a.ProjectID).Scan(&id)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrWorkspaceForbidden
	}
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT user_id FROM workspace_members WHERE workspace_id=$1 AND user_id IN ($2,$3) AND status='active' AND (user_id<>$2 OR role IN ('owner','admin','developer')) ORDER BY user_id FOR SHARE`, a.WorkspaceID, a.ActorUserID, a.BillingPrincipalUserID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	count := 0
	for rows.Next() {
		if err = rows.Scan(&id); err != nil {
			return err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	want := 2
	if a.ActorUserID == a.BillingPrincipalUserID || a.ActorUserID == 0 {
		want = 1
	}
	if count != want {
		return service.ErrWorkspaceForbidden
	}
	return nil
}

type budgetScope struct {
	typ    string
	id     int64
	period time.Time
}

type budgetPolicyState struct {
	amount        float64
	hard, enabled bool
	timezone      string
}

func (r *budgetRepository) recordHardBudgetReject(ctx context.Context, scope budgetScope, workspaceID, projectID int64, estimate float64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = lockBudgetScopes(ctx, tx, workspaceID, projectID, nil); err != nil {
		return err
	}
	policy, err := loadBudgetPolicy(ctx, tx, scope.typ, scope.id)
	if err != nil {
		return err
	}
	if !policy.enabled || !policy.hard {
		return tx.Commit()
	}
	now := time.Now()
	if r.now != nil {
		now = r.now()
	}
	if scope.period, err = budgetMonthStart(now, policy.timezone); err != nil {
		return err
	}
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT policy_revision FROM `+budgetPolicyTable(scope.typ)+` WHERE `+budgetPolicyColumn(scope.typ)+`=$1 FOR SHARE`, scope.id).Scan(&revision); err != nil {
		return err
	}
	var spent, reserved float64
	err = tx.QueryRowContext(ctx, `SELECT spent,reserved FROM budget_counters WHERE scope_type=$1 AND scope_id=$2 AND period_start=$3 FOR UPDATE`, scope.typ, scope.id, scope.period).Scan(&spent, &reserved)
	if errors.Is(err, sql.ErrNoRows) {
		// A rejected first request must not create an empty counter. The scope
		// locks above serialize its zero snapshot and alert transition instead.
		spent, reserved, err = 0, 0, nil
	}
	if err != nil {
		return err
	}
	if spent+reserved+estimate <= policy.amount {
		return tx.Commit()
	}
	if err = insertBudgetTransitionTx(ctx, tx, scope, workspaceID, projectID, revision, 100, service.EventBudgetHardLimit, spent, reserved, policy.amount, "admission_rejected"); err != nil {
		return err
	}
	return tx.Commit()
}

func budgetPolicyTable(scopeType string) string {
	if scopeType == "project" {
		return "project_budget_policies"
	}
	return "workspace_budget_policies"
}

func budgetPolicyColumn(scopeType string) string {
	if scopeType == "project" {
		return "project_id"
	}
	return "workspace_id"
}

func loadBudgetPolicy(ctx context.Context, tx *sql.Tx, typ string, id int64) (budgetPolicyState, error) {
	policy := budgetPolicyState{timezone: "UTC"}
	table, column := "workspace_budget_policies", "workspace_id"
	if typ == "project" {
		table, column = "project_budget_policies", "project_id"
	}
	err := tx.QueryRowContext(ctx, `SELECT amount,hard_limit,enabled,timezone FROM `+table+` WHERE `+column+`=$1 FOR SHARE`, id).Scan(&policy.amount, &policy.hard, &policy.enabled, &policy.timezone)
	if errors.Is(err, sql.ErrNoRows) {
		return policy, nil
	}
	if err != nil {
		return policy, err
	}
	if !validBudgetAmount(policy.amount) || strings.TrimSpace(policy.timezone) == "" {
		return policy, service.ErrBudgetReservationInvalid
	}
	return policy, nil
}

func budgetExceeded(typ string) error {
	if typ == "project" {
		return service.ErrProjectBudgetExceeded
	}
	return service.ErrWorkspaceBudgetExceeded
}

func reserveBudgetCounter(ctx context.Context, tx *sql.Tx, scope budgetScope, estimate float64, policy budgetPolicyState) error {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO budget_counters(scope_type,scope_id,period_start,reserved)
		SELECT $1,$2,$3,$4::numeric WHERE NOT $5::boolean OR $4::numeric<=$6::numeric
		ON CONFLICT(scope_type,scope_id,period_start) DO UPDATE
		SET reserved=budget_counters.reserved+EXCLUDED.reserved
		WHERE (NOT $5::boolean OR budget_counters.spent+budget_counters.reserved+EXCLUDED.reserved<=$6::numeric)
			AND budget_counters.spent>=0 AND budget_counters.spent<'Infinity'::numeric
			AND budget_counters.reserved>=0 AND budget_counters.reserved<'Infinity'::numeric
		RETURNING scope_id`, scope.typ, scope.id, scope.period, estimate, policy.hard && policy.enabled, policy.amount).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return budgetExceeded(scope.typ)
	}
	return err
}

const budgetReservationColumns = `id,request_id,actor_user_id,service_account_id,api_key_id,workspace_id,project_id,billing_principal_user_id,period_start,period_end,project_period_start,project_period_end,estimate,actual,status`

func scanBudgetReservation(row interface{ Scan(...any) error }) (*service.BudgetReservation, error) {
	var out service.BudgetReservation
	var actor, machine sql.NullInt64
	err := row.Scan(&out.ID, &out.RequestID, &actor, &machine, &out.APIKeyID, &out.WorkspaceID, &out.ProjectID, &out.BillingPrincipalUserID,
		&out.PeriodStart, &out.PeriodEnd, &out.ProjectPeriodStart, &out.ProjectPeriodEnd, &out.Estimate, &out.Actual, &out.Status)
	out.ActorUserID, out.ServiceAccountID = actor.Int64, machine.Int64
	return &out, err
}

func budgetAttributionMatches(res *service.BudgetReservation, a service.BudgetAttribution) bool {
	return res.ActorUserID == a.ActorUserID && res.ServiceAccountID == a.ServiceAccountID && res.APIKeyID == a.APIKeyID && res.WorkspaceID == a.WorkspaceID && res.ProjectID == a.ProjectID && res.BillingPrincipalUserID == a.BillingPrincipalUserID
}

func reservationBudgetScopes(res *service.BudgetReservation) []budgetScope {
	return []budgetScope{{"workspace", res.WorkspaceID, res.PeriodStart}, {"project", res.ProjectID, res.ProjectPeriodStart}}
}

func validatePendingBudgetCounter(ctx context.Context, tx *sql.Tx, scope budgetScope, estimate float64) error {
	var valid bool
	err := tx.QueryRowContext(ctx, `SELECT spent>=0 AND spent<'Infinity'::numeric AND reserved>=$4::numeric AND reserved<'Infinity'::numeric FROM budget_counters WHERE scope_type=$1 AND scope_id=$2 AND period_start=$3 FOR UPDATE`, scope.typ, scope.id, scope.period, estimate).Scan(&valid)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !valid) {
		return fmt.Errorf("%w: %s/%d", service.ErrBudgetCounterCorrupt, scope.typ, scope.id)
	}
	return err
}

func loadBudgetReservationForTransition(ctx context.Context, tx *sql.Tx, id string) (*service.BudgetReservation, error) {
	// Billing can already hold a key quota or subscription row. Lock only the
	// immutable reservation here, then workspace/project counters in that order;
	// acquiring admission's live scope locks would invert key-to-workspace order.
	res, err := scanBudgetReservation(tx.QueryRowContext(ctx, `SELECT `+budgetReservationColumns+` FROM budget_reservations WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrBudgetReservationInvalid
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func settleBudgetReservationTx(ctx context.Context, tx *sql.Tx, res *service.BudgetReservation, actual float64, status string, allowTerminalRetry bool) error {
	if !validBudgetAmount(res.Estimate) || !validBudgetAmount(res.Actual) {
		return service.ErrBudgetReservationInvalid
	}
	if res.Status != "pending" {
		if status == "released" && allowTerminalRetry && (res.Status == "released" || res.Status == "finalized") {
			return nil
		}
		if status == "finalized" && res.Status == "finalized" {
			if res.Actual != actual {
				return service.ErrBudgetReservationConflict
			}
			if allowTerminalRetry {
				return nil
			}
		}
		return service.ErrBudgetReservationClosed
	}
	for _, scope := range reservationBudgetScopes(res) {
		policy := budgetPolicyState{}
		var err error
		if status == "finalized" {
			policy, err = loadBudgetPolicy(ctx, tx, scope.typ, scope.id)
			if err != nil {
				return err
			}
		}
		if err = validatePendingBudgetCounter(ctx, tx, scope, res.Estimate); err != nil {
			return err
		}
		enforceCapacity := status == "finalized" && actual > res.Estimate && policy.hard && policy.enabled
		var id int64
		var spentAfter, reservedAfter float64
		err = tx.QueryRowContext(ctx, `UPDATE budget_counters SET reserved=reserved-$1::numeric,spent=spent+$2::numeric
			WHERE scope_type=$3 AND scope_id=$4 AND period_start=$5 AND reserved>=$1::numeric
				AND (NOT $6::boolean OR spent+reserved-$1::numeric+$2::numeric<=$7::numeric)
			RETURNING scope_id,spent,reserved`, res.Estimate, actual, scope.typ, scope.id, scope.period, enforceCapacity, policy.amount).Scan(&id, &spentAfter, &reservedAfter)
		if errors.Is(err, sql.ErrNoRows) {
			if enforceCapacity {
				return budgetExceeded(scope.typ)
			}
			return service.ErrBudgetCounterCorrupt
		}
		if err != nil {
			return err
		}
		if status == "finalized" && policy.enabled && policy.amount > 0 && spentAfter >= 0 {
			var revision int64
			if err = tx.QueryRowContext(ctx, `SELECT policy_revision FROM `+budgetPolicyTable(scope.typ)+` WHERE `+budgetPolicyColumn(scope.typ)+`=$1 FOR SHARE`, scope.id).Scan(&revision); err != nil {
				return err
			}
			spentBefore := spentAfter - actual
			for _, threshold := range []int{50, 80, 100} {
				if spentBefore*100 < policy.amount*float64(threshold) && spentAfter*100 >= policy.amount*float64(threshold) {
					eventType := service.EventBudgetThreshold
					if threshold == 100 && policy.hard {
						eventType = service.EventBudgetHardLimit
					} else if threshold == 100 {
						eventType = service.EventBudgetSoftLimit
					}
					if err = insertBudgetTransitionTx(ctx, tx, scope, res.WorkspaceID, res.ProjectID, revision, threshold, eventType, spentAfter, reservedAfter, policy.amount, "finalized_spend"); err != nil {
						return err
					}
				}
			}
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE budget_reservations SET actual=$2,status=$3,finalized_at=now() WHERE id=$1 AND status='pending'`, res.ID, actual, status)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return service.ErrBudgetReservationClosed
	}
	return nil
}

func insertBudgetTransitionTx(ctx context.Context, tx *sql.Tx, scope budgetScope, workspaceID, projectID, revision int64, threshold int, eventType string, spent, reserved, amount float64, reason string) error {
	result, err := tx.ExecContext(ctx, `INSERT INTO budget_alert_transitions(scope_type,scope_id,period_start,policy_revision,threshold) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, scope.typ, scope.id, scope.period, revision, threshold)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	if eventType == "" {
		eventType = service.EventBudgetThreshold
	}
	project := int64(0)
	if scope.typ == "project" {
		project = projectID
	}
	event, err := service.NewDomainEvent(eventType, workspaceID, project, 0, scope.typ, fmt.Sprintf("%d", scope.id), service.DomainEventData{
		"scope_type": scope.typ, "scope_id": scope.id, "period_start": scope.period.Format("2006-01-02"),
		"policy_revision": revision, "threshold": threshold, "amount": amount,
		"spent": spent, "reserved": reserved, "reason_code": reason,
	})
	if err != nil {
		return err
	}
	dedupeKey := fmt.Sprintf("budget:%s:%d:%s:%d:%d", scope.typ, scope.id, scope.period.Format("2006-01"), revision, threshold)
	return insertDomainEventTx(ctx, tx, event, dedupeKey)
}

func (r *budgetRepository) Finalize(ctx context.Context, id string, actual float64) error {
	return r.transition(ctx, id, actual, "finalized")
}

func (r *budgetRepository) Release(ctx context.Context, id string) error {
	return r.transition(ctx, id, 0, "released")
}

func (r *budgetRepository) transition(ctx context.Context, id string, actual float64, status string) error {
	if r == nil || r.db == nil {
		return service.ErrBudgetUnavailable
	}
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil || !validBudgetAmount(actual) {
		return service.ErrBudgetReservationInvalid
	}
	actual = service.QuantizeUsageBillingAmount(actual)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := loadBudgetReservationForTransition(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = settleBudgetReservationTx(ctx, tx, res, actual, status, true); err != nil {
		return err
	}
	return tx.Commit()
}
