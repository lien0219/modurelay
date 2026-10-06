package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
)

type policyQuotaRepository struct {
	db  *sql.DB
	now func() time.Time
}

func NewPolicyQuotaRepository(db *sql.DB) service.PolicyQuotaRepository {
	return &policyQuotaRepository{db: db, now: time.Now}
}

const policyQuotaReservationColumns = `id,request_id,api_key_id,workspace_id,project_id,service_account_id,estimated_tokens,request_units,actual_tokens,status`

func scanPolicyQuotaReservation(row interface{ Scan(...any) error }) (*service.PolicyQuotaReservation, error) {
	var reservation service.PolicyQuotaReservation
	var serviceAccount sql.NullInt64
	var actual sql.NullInt64
	if err := row.Scan(&reservation.ID, &reservation.RequestID, &reservation.APIKeyID, &reservation.WorkspaceID, &reservation.ProjectID, &serviceAccount, &reservation.EstimatedTokens, &reservation.RequestUnits, &actual, &reservation.Status); err != nil {
		return nil, err
	}
	if serviceAccount.Valid {
		reservation.ServiceAccountID = serviceAccount.Int64
	}
	return &reservation, nil
}

type policyQuotaItem struct {
	ReservationID string
	ScopeType     domain.PolicyScope
	ScopeID       int64
	Revision      int64
	PeriodType    string
	PeriodStart   time.Time
	RequestLimit  *int64
	TokenLimit    *int64
	RequestHeld   int64
	TokenHeld     int64
}

func (r *policyQuotaRepository) Reserve(ctx context.Context, request service.PolicyQuotaReservationRequest) (*service.PolicyQuotaReservation, error) {
	if r == nil || r.db == nil {
		return nil, service.ErrPolicyQuotaUnavailable
	}
	if strings.TrimSpace(request.RequestID) == "" || request.APIKeyID <= 0 || request.WorkspaceID <= 0 || request.ProjectID <= 0 || request.EstimatedTokens < 0 || request.RequestUnits <= 0 || len(request.Scopes) == 0 {
		return nil, service.ErrPolicyQuotaReservationInvalid
	}
	scopes := append([]service.PolicyQuotaScope(nil), request.Scopes...)
	for _, scope := range scopes {
		if err := validatePolicyQuotaScopeRepository(scope); err != nil {
			return nil, err
		}
	}
	servicePolicyQuotaScopeSort(scopes)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := scanPolicyQuotaReservation(tx.QueryRowContext(ctx,
		`SELECT `+policyQuotaReservationColumns+` FROM policy_quota_reservations WHERE request_id=$1 AND api_key_id=$2 FOR UPDATE`,
		strings.TrimSpace(request.RequestID), request.APIKeyID))
	if err == nil {
		if !policyQuotaReservationMatches(ctx, tx, existing, request, scopes) {
			return nil, service.ErrPolicyQuotaReservationConflict
		}
		if existing.Status != service.PolicyQuotaReservationPending {
			return nil, service.ErrPolicyQuotaReservationClosed
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	now := request.Now
	if now.IsZero() {
		now = time.Now()
		if r.now != nil {
			now = r.now()
		}
	}
	day, month := service.PolicyQuotaPeriodStarts(now)
	items := buildPolicyQuotaItems("", scopes, request.EstimatedTokens, request.RequestUnits, day, month)
	if len(items) == 0 {
		return nil, service.ErrPolicyQuotaReservationInvalid
	}
	if err := lockPolicyQuotaPeriods(ctx, tx, items); err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := ensurePolicyQuotaCounter(ctx, tx, item); err != nil {
			return nil, err
		}
		requestsUsed, requestsReserved, tokensUsed, tokensReserved, err := policyQuotaTotals(ctx, tx, item)
		if err != nil {
			return nil, err
		}
		if item.RequestLimit != nil && requestsUsed+requestsReserved+item.RequestHeld > *item.RequestLimit {
			return nil, fmt.Errorf("%w: %s request quota exhausted", service.ErrPolicyQuotaExceeded, item.ScopeType)
		}
		if item.TokenLimit != nil && tokensUsed+tokensReserved+item.TokenHeld > *item.TokenLimit {
			return nil, fmt.Errorf("%w: %s token quota exhausted", service.ErrPolicyQuotaExceeded, item.ScopeType)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE policy_quota_counters SET requests_reserved=requests_reserved+$1,tokens_reserved=tokens_reserved+$2,updated_at=now() WHERE scope_type=$3 AND scope_id=$4 AND policy_revision=$5 AND period_type=$6 AND period_start=$7`, item.RequestHeld, item.TokenHeld, item.ScopeType, item.ScopeID, item.Revision, item.PeriodType, item.PeriodStart); err != nil {
			return nil, err
		}
	}

	reservation := &service.PolicyQuotaReservation{
		ID:               uuid.NewString(),
		RequestID:        strings.TrimSpace(request.RequestID),
		APIKeyID:         request.APIKeyID,
		WorkspaceID:      request.WorkspaceID,
		ProjectID:        request.ProjectID,
		ServiceAccountID: request.ServiceAccountID,
		EstimatedTokens:  request.EstimatedTokens,
		RequestUnits:     request.RequestUnits,
		Status:           service.PolicyQuotaReservationPending,
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO policy_quota_reservations(id,request_id,api_key_id,workspace_id,project_id,service_account_id,estimated_tokens,request_units,status) VALUES($1,$2,$3,$4,$5,NULLIF($6,0),$7,$8,$9)`, reservation.ID, reservation.RequestID, reservation.APIKeyID, reservation.WorkspaceID, reservation.ProjectID, reservation.ServiceAccountID, reservation.EstimatedTokens, reservation.RequestUnits, reservation.Status); err != nil {
		return nil, err
	}
	for i := range items {
		items[i].ReservationID = reservation.ID
		if _, err := tx.ExecContext(ctx, `INSERT INTO policy_quota_reservation_items(reservation_id,scope_type,scope_id,policy_revision,period_type,period_start,request_limit,token_limit,request_reserved,token_reserved) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, reservation.ID, items[i].ScopeType, items[i].ScopeID, items[i].Revision, items[i].PeriodType, items[i].PeriodStart, items[i].RequestLimit, items[i].TokenLimit, items[i].RequestHeld, items[i].TokenHeld); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return reservation, nil
}

func (r *policyQuotaRepository) Finalize(ctx context.Context, reservationID string, actualTokens int64) error {
	if r == nil || r.db == nil {
		return service.ErrPolicyQuotaUnavailable
	}
	if strings.TrimSpace(reservationID) == "" || actualTokens < 0 {
		return service.ErrPolicyQuotaReservationInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var workspaceID, projectID int64
	if err = tx.QueryRowContext(ctx, `SELECT status,workspace_id,project_id FROM policy_quota_reservations WHERE id=$1 FOR UPDATE`, reservationID).Scan(&status, &workspaceID, &projectID); errors.Is(err, sql.ErrNoRows) {
		return service.ErrPolicyQuotaReservationInvalid
	} else if err != nil {
		return err
	}
	if status == service.PolicyQuotaReservationFinalized {
		return tx.Commit()
	}
	if status != service.PolicyQuotaReservationPending {
		return service.ErrPolicyQuotaReservationClosed
	}
	items, err := loadPolicyQuotaItems(ctx, tx, reservationID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return service.ErrPolicyQuotaReservationConflict
	}
	if err := lockPolicyQuotaPeriods(ctx, tx, items); err != nil {
		return err
	}
	for _, item := range items {
		requestsUsed, requestsReserved, tokensUsed, tokensReserved, err := policyQuotaTotals(ctx, tx, item)
		if err != nil {
			return err
		}
		if requestsReserved < item.RequestHeld || tokensReserved < item.TokenHeld {
			return service.ErrPolicyQuotaReservationConflict
		}
		finalTokens := int64(0)
		tokenTotal := int64(0)
		if item.TokenLimit != nil {
			finalTokens = actualTokens
			var ok bool
			tokenTotal, ok = policyQuotaTokenTotalAfterFinalize(tokensUsed, tokensReserved, item.TokenHeld, finalTokens)
			if !ok {
				return service.ErrPolicyQuotaReservationConflict
			}
		}
		result, err := tx.ExecContext(ctx, `UPDATE policy_quota_counters SET requests_reserved=requests_reserved-$1,requests_used=requests_used+$1,tokens_reserved=tokens_reserved-$2,tokens_used=tokens_used+$3,updated_at=now() WHERE scope_type=$4 AND scope_id=$5 AND policy_revision=$6 AND period_type=$7 AND period_start=$8`, item.RequestHeld, item.TokenHeld, finalTokens, item.ScopeType, item.ScopeID, item.Revision, item.PeriodType, item.PeriodStart)
		if err := requirePolicyQuotaOneRow(result, err); err != nil {
			return err
		}
		requestTotal := requestsUsed + requestsReserved
		if item.RequestLimit != nil {
			if err := emitPolicyQuotaAlerts(ctx, tx, workspaceID, projectID, item, requestTotal, *item.RequestLimit, "requests"); err != nil {
				return err
			}
		}
		if item.TokenLimit != nil {
			if err := emitPolicyQuotaAlerts(ctx, tx, workspaceID, projectID, item, tokenTotal, *item.TokenLimit, "tokens"); err != nil {
				return err
			}
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE policy_quota_reservations SET actual_tokens=$1,status=$2,updated_at=now() WHERE id=$3 AND status=$4`, actualTokens, service.PolicyQuotaReservationFinalized, reservationID, service.PolicyQuotaReservationPending)
	if err := requirePolicyQuotaOneRow(result, err); err != nil {
		return err
	}
	return tx.Commit()
}

// policyQuotaTokenTotalAfterFinalize records actual usage even when it exceeds
// the configured limit. The next admission sees the exhausted counter and is
// denied; actual provider usage cannot be rolled back after the response.
func policyQuotaTokenTotalAfterFinalize(tokensUsed, tokensReserved, tokenHeld, finalTokens int64) (int64, bool) {
	if tokensUsed < 0 || tokensReserved < 0 || tokenHeld < 0 || finalTokens < 0 || tokensReserved < tokenHeld {
		return 0, false
	}
	otherReserved := tokensReserved - tokenHeld
	if tokensUsed > math.MaxInt64-otherReserved {
		return 0, false
	}
	total := tokensUsed + otherReserved
	if finalTokens > math.MaxInt64-total {
		return 0, false
	}
	return total + finalTokens, true
}

func requirePolicyQuotaOneRow(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrPolicyQuotaReservationConflict
	}
	return nil
}

func (r *policyQuotaRepository) Release(ctx context.Context, reservationID string) error {
	if r == nil || r.db == nil {
		return service.ErrPolicyQuotaUnavailable
	}
	if strings.TrimSpace(reservationID) == "" {
		return service.ErrPolicyQuotaReservationInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM policy_quota_reservations WHERE id=$1 FOR UPDATE`, reservationID).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		return service.ErrPolicyQuotaReservationInvalid
	} else if err != nil {
		return err
	}
	if status == service.PolicyQuotaReservationFinalized || status == service.PolicyQuotaReservationReleased {
		return tx.Commit()
	}
	items, err := loadPolicyQuotaItems(ctx, tx, reservationID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return service.ErrPolicyQuotaReservationConflict
	}
	if err := lockPolicyQuotaPeriods(ctx, tx, items); err != nil {
		return err
	}
	for _, item := range items {
		result, err := tx.ExecContext(ctx, `UPDATE policy_quota_counters SET requests_reserved=requests_reserved-$1,tokens_reserved=tokens_reserved-$2,updated_at=now() WHERE scope_type=$3 AND scope_id=$4 AND policy_revision=$5 AND period_type=$6 AND period_start=$7 AND requests_reserved >= $1 AND tokens_reserved >= $2`, item.RequestHeld, item.TokenHeld, item.ScopeType, item.ScopeID, item.Revision, item.PeriodType, item.PeriodStart)
		if err := requirePolicyQuotaOneRow(result, err); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE policy_quota_reservations SET status=$1,updated_at=now() WHERE id=$2 AND status=$3`, service.PolicyQuotaReservationReleased, reservationID, service.PolicyQuotaReservationPending)
	if err := requirePolicyQuotaOneRow(result, err); err != nil {
		return err
	}
	return tx.Commit()
}

func policyQuotaReservationMatches(ctx context.Context, tx *sql.Tx, existing *service.PolicyQuotaReservation, request service.PolicyQuotaReservationRequest, scopes []service.PolicyQuotaScope) bool {
	if existing == nil || existing.APIKeyID != request.APIKeyID || existing.WorkspaceID != request.WorkspaceID || existing.ProjectID != request.ProjectID || existing.ServiceAccountID != request.ServiceAccountID || existing.EstimatedTokens != request.EstimatedTokens || existing.RequestUnits != request.RequestUnits {
		return false
	}
	items, err := loadPolicyQuotaItems(ctx, tx, existing.ID)
	if err != nil {
		return false
	}
	now := request.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	day, month := service.PolicyQuotaPeriodStarts(now)
	expected := buildPolicyQuotaItems(existing.ID, scopes, request.EstimatedTokens, request.RequestUnits, day, month)
	return policyQuotaReservationItemsMatch(items, expected)
}

// policyQuotaReservationItemsMatch compares the immutable admission snapshot,
// rather than only the number of rows. This prevents an idempotency replay from
// reusing a reservation created for a different policy revision, limit, period,
// or token hold.
func policyQuotaReservationItemsMatch(actual, expected []policyQuotaItem) bool {
	if len(actual) != len(expected) {
		return false
	}
	canonical := func(items []policyQuotaItem) []policyQuotaItem {
		copyItems := append([]policyQuotaItem(nil), items...)
		sort.Slice(copyItems, func(i, j int) bool {
			a, b := copyItems[i], copyItems[j]
			if a.ScopeType != b.ScopeType {
				return a.ScopeType < b.ScopeType
			}
			if a.ScopeID != b.ScopeID {
				return a.ScopeID < b.ScopeID
			}
			if a.Revision != b.Revision {
				return a.Revision < b.Revision
			}
			if a.PeriodType != b.PeriodType {
				return a.PeriodType < b.PeriodType
			}
			return a.PeriodStart.Before(b.PeriodStart)
		})
		return copyItems
	}
	left, right := canonical(actual), canonical(expected)
	for i := range left {
		a, b := left[i], right[i]
		if a.ScopeType != b.ScopeType || a.ScopeID != b.ScopeID || a.Revision != b.Revision ||
			a.PeriodType != b.PeriodType || !samePolicyQuotaPeriod(a.PeriodStart, b.PeriodStart) ||
			a.RequestHeld != b.RequestHeld || a.TokenHeld != b.TokenHeld ||
			!samePolicyQuotaLimit(a.RequestLimit, b.RequestLimit) || !samePolicyQuotaLimit(a.TokenLimit, b.TokenLimit) {
			return false
		}
	}
	return true
}

func samePolicyQuotaPeriod(a, b time.Time) bool {
	return a.UTC().Format("2006-01-02") == b.UTC().Format("2006-01-02")
}

func samePolicyQuotaLimit(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func buildPolicyQuotaItems(reservationID string, scopes []service.PolicyQuotaScope, estimatedTokens, requestUnits int64, day, month time.Time) []policyQuotaItem {
	items := make([]policyQuotaItem, 0, len(scopes)*2)
	for _, scope := range scopes {
		periods := []struct {
			kind         string
			start        time.Time
			requestLimit *int64
			tokenLimit   *int64
		}{{"daily", day, scope.DailyRequestLimit, scope.DailyTokenLimit}, {"monthly", month, scope.MonthlyRequestLimit, scope.MonthlyTokenLimit}}
		for _, period := range periods {
			if period.requestLimit == nil && period.tokenLimit == nil {
				continue
			}
			items = append(items, policyQuotaItem{
				ReservationID: reservationID,
				ScopeType:     scope.Scope,
				ScopeID:       scope.ID,
				Revision:      scope.Revision,
				PeriodType:    period.kind,
				PeriodStart:   period.start,
				RequestLimit:  cloneQuotaInt(period.requestLimit),
				TokenLimit:    cloneQuotaInt(period.tokenLimit),
				RequestHeld:   boolToInt64(period.requestLimit != nil) * positivePolicyQuotaRequestUnits(requestUnits),
				TokenHeld:     boolToInt64(period.tokenLimit != nil) * estimatedTokens,
			})
		}
	}
	return items
}

func cloneQuotaInt(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func boolToInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func positivePolicyQuotaRequestUnits(value int64) int64 {
	if value <= 0 {
		return 1
	}
	return value
}

func loadPolicyQuotaItems(ctx context.Context, tx *sql.Tx, reservationID string) ([]policyQuotaItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT reservation_id,scope_type,scope_id,policy_revision,period_type,period_start,request_limit,token_limit,request_reserved,token_reserved FROM policy_quota_reservation_items WHERE reservation_id=$1`, reservationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]policyQuotaItem, 0)
	for rows.Next() {
		var item policyQuotaItem
		if err := rows.Scan(&item.ReservationID, &item.ScopeType, &item.ScopeID, &item.Revision, &item.PeriodType, &item.PeriodStart, &item.RequestLimit, &item.TokenLimit, &item.RequestHeld, &item.TokenHeld); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func lockPolicyQuotaPeriods(ctx context.Context, tx *sql.Tx, items []policyQuotaItem) error {
	keys := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		key := policyQuotaAdvisoryKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
			return err
		}
	}
	return nil
}

func policyQuotaAdvisoryKey(item policyQuotaItem) string {
	return fmt.Sprintf("policy-quota:%s:%d:%s:%s", item.ScopeType, item.ScopeID, item.PeriodType, item.PeriodStart.UTC().Format("2006-01-02"))
}

// policyQuotaThresholdUnits returns ceil(limit*threshold/100) without
// multiplying the full limit, which keeps the calculation safe at MaxInt64.
func policyQuotaThresholdUnits(limit, threshold int64) int64 {
	if limit <= 0 || threshold <= 0 {
		return 0
	}
	if threshold >= 100 {
		return limit
	}
	base, remainder := limit/100, limit%100
	units := base*threshold + (remainder*threshold)/100
	if remainder*threshold%100 != 0 {
		units++
	}
	return units
}

func policyQuotaAlertDedupeKey(scope domain.PolicyScope, scopeID, revision int64, periodType string, periodStart time.Time, quotaType string, threshold int64) string {
	return fmt.Sprintf("policy-quota:%s:%d:%d:%s:%s:%s:%d", scope, scopeID, revision, periodType, periodStart.UTC().Format("2006-01-02"), quotaType, threshold)
}

func emitPolicyQuotaAlerts(ctx context.Context, tx *sql.Tx, workspaceID, projectID int64, item policyQuotaItem, used, limit int64, quotaType string) error {
	if used < 0 || limit <= 0 {
		return service.ErrPolicyQuotaReservationConflict
	}
	for _, threshold := range []int64{80, 100} {
		thresholdUnits := policyQuotaThresholdUnits(limit, threshold)
		if used < thresholdUnits {
			continue
		}
		key := policyQuotaAlertDedupeKey(item.ScopeType, item.ScopeID, item.Revision, item.PeriodType, item.PeriodStart, quotaType, threshold)
		var inserted int
		err := tx.QueryRowContext(ctx, `INSERT INTO policy_quota_alerts(scope_type,scope_id,policy_revision,period_type,period_start,quota_type,threshold) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING RETURNING 1`, item.ScopeType, item.ScopeID, item.Revision, item.PeriodType, item.PeriodStart, quotaType, threshold).Scan(&inserted)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		periodEnd := item.PeriodStart
		if item.PeriodType == "monthly" {
			periodEnd = periodEnd.AddDate(0, 1, 0)
		} else {
			periodEnd = periodEnd.AddDate(0, 0, 1)
		}
		eventType := service.EventQuotaThreshold
		if threshold == 100 {
			eventType = service.EventQuotaExhausted
		}
		event, err := service.NewDomainEvent(eventType, workspaceID, projectID, 0, "policy_quota", fmt.Sprintf("%s:%d", item.ScopeType, item.ScopeID), service.DomainEventData{
			"scope_type":      string(item.ScopeType),
			"scope_id":        item.ScopeID,
			"policy_revision": item.Revision,
			"period_start":    item.PeriodStart.UTC().Format("2006-01-02"),
			"period_end":      periodEnd.UTC().Format("2006-01-02"),
			"quota_type":      quotaType,
			"threshold":       threshold,
			"used":            used,
			"limit":           limit,
		})
		if err != nil {
			return err
		}
		if err := insertDomainEventTx(ctx, tx, event, key); err != nil {
			return err
		}
	}
	return nil
}

func ensurePolicyQuotaCounter(ctx context.Context, tx *sql.Tx, item policyQuotaItem) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO policy_quota_counters(scope_type,scope_id,policy_revision,period_type,period_start) VALUES($1,$2,$3,$4,$5) ON CONFLICT(scope_type,scope_id,policy_revision,period_type,period_start) DO NOTHING`, item.ScopeType, item.ScopeID, item.Revision, item.PeriodType, item.PeriodStart)
	return err
}

func policyQuotaTotals(ctx context.Context, tx *sql.Tx, item policyQuotaItem) (requestsUsed, requestsReserved, tokensUsed, tokensReserved int64, err error) {
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(requests_used),0),COALESCE(SUM(requests_reserved),0),COALESCE(SUM(tokens_used),0),COALESCE(SUM(tokens_reserved),0) FROM policy_quota_counters WHERE scope_type=$1 AND scope_id=$2 AND period_type=$3 AND period_start=$4`, item.ScopeType, item.ScopeID, item.PeriodType, item.PeriodStart).Scan(&requestsUsed, &requestsReserved, &tokensUsed, &tokensReserved)
	return
}

func validatePolicyQuotaScopeRepository(scope service.PolicyQuotaScope) error {
	if scope.Scope == "" || scope.ID <= 0 || scope.Revision <= 0 {
		return service.ErrPolicyQuotaReservationInvalid
	}
	for _, value := range []*int64{scope.DailyRequestLimit, scope.MonthlyRequestLimit, scope.DailyTokenLimit, scope.MonthlyTokenLimit} {
		if value != nil && *value <= 0 {
			return service.ErrPolicyQuotaReservationInvalid
		}
	}
	return nil
}

func servicePolicyQuotaScopeSort(scopes []service.PolicyQuotaScope) {
	sort.Slice(scopes, func(i, j int) bool {
		if scopes[i].Scope != scopes[j].Scope {
			return scopes[i].Scope < scopes[j].Scope
		}
		if scopes[i].ID != scopes[j].ID {
			return scopes[i].ID < scopes[j].ID
		}
		return scopes[i].Revision < scopes[j].Revision
	})
}
