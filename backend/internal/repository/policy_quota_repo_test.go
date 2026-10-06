package repository

import (
	"context"
	"math"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPolicyQuotaReservationItemsMatchRequiresCompleteSnapshot(t *testing.T) {
	limit := int64(100)
	actual := []policyQuotaItem{{
		ScopeType:    domain.PolicyScopeWorkspace,
		ScopeID:      10,
		Revision:     3,
		PeriodType:   "daily",
		PeriodStart:  time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC),
		RequestLimit: &limit,
		RequestHeld:  1,
		TokenHeld:    0,
	}}

	require.True(t, policyQuotaReservationItemsMatch(actual, actual))

	changed := append([]policyQuotaItem(nil), actual...)
	changed[0].Revision = 4
	require.False(t, policyQuotaReservationItemsMatch(actual, changed))

	changed = append([]policyQuotaItem(nil), actual...)
	changed[0].RequestLimit = serviceInt64Ptr(99)
	require.False(t, policyQuotaReservationItemsMatch(actual, changed))
}

func TestPolicyQuotaThresholdUnitsUsesCeilingWithoutOverflow(t *testing.T) {
	for _, tc := range []struct {
		name      string
		limit     int64
		threshold int64
		want      int64
	}{
		{name: "eighty percent rounds up", limit: 101, threshold: 80, want: 81},
		{name: "full limit", limit: 101, threshold: 100, want: 101},
		{name: "maximum int", limit: math.MaxInt64, threshold: 80, want: 7378697629483820646},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, policyQuotaThresholdUnits(tc.limit, tc.threshold))
		})
	}
}

func TestPolicyQuotaAlertDedupeKeyIncludesImmutableWindow(t *testing.T) {
	key := policyQuotaAlertDedupeKey(domain.PolicyScopeWorkspace, 10, 3, "daily", time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC), "tokens", 80)
	require.Equal(t, "policy-quota:workspace:10:3:daily:2026-10-06:tokens:80", key)
}

func serviceInt64Ptr(value int64) *int64 { return &value }

func TestPolicyQuotaFinalizeRecordsTokenOverageAndExhaustionAlert(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &policyQuotaRepository{db: db}
	reservationID := "reservation-overage"
	periodStart := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status,workspace_id,project_id FROM policy_quota_reservations WHERE id=$1 FOR UPDATE")).
		WithArgs(reservationID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "workspace_id", "project_id"}).AddRow(service.PolicyQuotaReservationPending, int64(11), int64(22)))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT reservation_id,scope_type,scope_id,policy_revision,period_type,period_start,request_limit,token_limit,request_reserved,token_reserved FROM policy_quota_reservation_items WHERE reservation_id=$1")).
		WithArgs(reservationID).
		WillReturnRows(sqlmock.NewRows([]string{"reservation_id", "scope_type", "scope_id", "policy_revision", "period_type", "period_start", "request_limit", "token_limit", "request_reserved", "token_reserved"}).
			AddRow(reservationID, domain.PolicyScopeWorkspace, int64(11), int64(3), "daily", periodStart, nil, int64(6), int64(0), int64(2)))
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).
		WithArgs("policy-quota:workspace:11:daily:2026-10-06").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COALESCE(SUM(requests_used),0),COALESCE(SUM(requests_reserved),0),COALESCE(SUM(tokens_used),0),COALESCE(SUM(tokens_reserved),0) FROM policy_quota_counters WHERE scope_type=$1 AND scope_id=$2 AND period_type=$3 AND period_start=$4")).
		WithArgs(domain.PolicyScopeWorkspace, int64(11), "daily", periodStart).
		WillReturnRows(sqlmock.NewRows([]string{"requests_used", "requests_reserved", "tokens_used", "tokens_reserved"}).AddRow(int64(0), int64(0), int64(5), int64(2)))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE policy_quota_counters SET requests_reserved=requests_reserved-$1,requests_used=requests_used+$1,tokens_reserved=tokens_reserved-$2,tokens_used=tokens_used+$3,updated_at=now() WHERE scope_type=$4 AND scope_id=$5 AND policy_revision=$6 AND period_type=$7 AND period_start=$8")).
		WithArgs(int64(0), int64(2), int64(5), domain.PolicyScopeWorkspace, int64(11), int64(3), "daily", periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	expectPolicyQuotaAlertWrites(mock, periodStart, 80)
	expectPolicyQuotaAlertWrites(mock, periodStart, 100)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE policy_quota_reservations SET actual_tokens=$1,status=$2,updated_at=now() WHERE id=$3 AND status=$4")).
		WithArgs(int64(5), service.PolicyQuotaReservationFinalized, reservationID, service.PolicyQuotaReservationPending).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, repo.Finalize(context.Background(), reservationID, 5))
	require.NoError(t, mock.ExpectationsWereMet())
}

func expectPolicyQuotaAlertWrites(mock sqlmock.Sqlmock, periodStart time.Time, threshold int64) {
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO policy_quota_alerts(scope_type,scope_id,policy_revision,period_type,period_start,quota_type,threshold) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING RETURNING 1")).
		WithArgs(domain.PolicyScopeWorkspace, int64(11), int64(3), "daily", periodStart, "tokens", threshold).
		WillReturnRows(sqlmock.NewRows([]string{"inserted"}).AddRow(1))
	mock.ExpectExec("INSERT INTO domain_events\\(").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE dedupe_key=$1 ON CONFLICT(event_id) DO NOTHING")).
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestPolicyQuotaReleaseKeepsReservationPendingWhenCounterIsMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &policyQuotaRepository{db: db}
	reservationID := "reservation-missing-counter"
	periodStart := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT status FROM policy_quota_reservations WHERE id=$1 FOR UPDATE")).
		WithArgs(reservationID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(service.PolicyQuotaReservationPending))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT reservation_id,scope_type,scope_id,policy_revision,period_type,period_start,request_limit,token_limit,request_reserved,token_reserved FROM policy_quota_reservation_items WHERE reservation_id=$1")).
		WithArgs(reservationID).
		WillReturnRows(sqlmock.NewRows([]string{"reservation_id", "scope_type", "scope_id", "policy_revision", "period_type", "period_start", "request_limit", "token_limit", "request_reserved", "token_reserved"}).
			AddRow(reservationID, domain.PolicyScopeWorkspace, int64(11), int64(3), "daily", periodStart, int64(5), nil, int64(1), int64(0)))
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock(hashtextextended($1, 0))")).
		WithArgs("policy-quota:workspace:11:daily:2026-10-06").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE policy_quota_counters SET requests_reserved=requests_reserved-$1,tokens_reserved=tokens_reserved-$2,updated_at=now() WHERE scope_type=$3 AND scope_id=$4 AND policy_revision=$5 AND period_type=$6 AND period_start=$7 AND requests_reserved >= $1 AND tokens_reserved >= $2")).
		WithArgs(int64(1), int64(0), domain.PolicyScopeWorkspace, int64(11), int64(3), "daily", periodStart).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	require.ErrorIs(t, repo.Release(context.Background(), reservationID), service.ErrPolicyQuotaReservationConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}
