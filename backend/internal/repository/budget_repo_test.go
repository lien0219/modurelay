package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBudgetReserveRejectsInvalidAmountsBeforeSQL(t *testing.T) {
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), 1e12} {
		t.Run(amountName(amount), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			a := service.BudgetAttribution{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, ActorUserID: 4, APIKeyID: 5}
			reservation, err := NewBudgetRepository(db).Reserve(context.Background(), a, "request", amount)
			require.ErrorIs(t, err, service.ErrBudgetUnpriced)
			require.Nil(t, reservation)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func amountName(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "positive_infinity"
	case math.IsInf(v, -1):
		return "negative_infinity"
	case v < 0:
		return "negative"
	default:
		return "numeric_overflow"
	}
}

func TestBudgetReserveRejectsMissingIdentityBeforeSQL(t *testing.T) {
	a := service.BudgetAttribution{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, ActorUserID: 4, APIKeyID: 5}
	for _, field := range []string{"workspace", "project", "principal", "actor", "key", "request"} {
		t.Run(field, func(t *testing.T) {
			invalid, requestID := a, "request"
			switch field {
			case "workspace":
				invalid.WorkspaceID = 0
			case "project":
				invalid.ProjectID = 0
			case "principal":
				invalid.BillingPrincipalUserID = 0
			case "actor":
				invalid.ActorUserID = 0
			case "key":
				invalid.APIKeyID = 0
			case "request":
				requestID = " \t "
			}
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			reservation, err := NewBudgetRepository(db).Reserve(context.Background(), invalid, requestID, 1)
			require.ErrorIs(t, err, service.ErrBudgetReservationInvalid)
			require.Nil(t, reservation)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBudgetFinalizeRejectsInvalidAmountsBeforeSQL(t *testing.T) {
	for _, amount := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1), 1e12} {
		t.Run(amountName(amount), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			err = NewBudgetRepository(db).Finalize(context.Background(), "00000000-0000-0000-0000-000000000001", amount)
			require.ErrorIs(t, err, service.ErrBudgetReservationInvalid)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBudgetMonthStartUsesLocalCalendarMonth(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 30, 0, 0, time.UTC)
	period, err := budgetMonthStart(now, "America/Los_Angeles")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), period)
	period, err = budgetMonthStart(now, "Asia/Shanghai")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), period)
}

func TestBudgetCounterLookupFailsClosed(t *testing.T) {
	for _, failure := range []string{"missing", "underflow", "query_error"} {
		t.Run(failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			mock.ExpectBegin()
			tx, err := db.Begin()
			require.NoError(t, err)
			period := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			queryErr := errors.New("counter read unavailable")
			query := mock.ExpectQuery(`SELECT spent>=0`).WithArgs("workspace", int64(12), period, float64(2))
			switch failure {
			case "missing":
				query.WillReturnError(sql.ErrNoRows)
			case "underflow":
				query.WillReturnRows(sqlmock.NewRows([]string{"valid"}).AddRow(false))
			case "query_error":
				query.WillReturnError(queryErr)
			}
			mock.ExpectRollback()
			err = validatePendingBudgetCounter(context.Background(), tx, budgetScope{"workspace", 12, period}, 2)
			if failure == "query_error" {
				require.ErrorIs(t, err, queryErr)
			} else {
				require.ErrorIs(t, err, service.ErrBudgetCounterCorrupt)
			}
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBudgetSecondScopeSQLFailureRollsBackAdmission(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	a := service.BudgetAttribution{WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, ActorUserID: 4, APIKeyID: 5}
	period := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	failure := errors.New("project counter database unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users`).WithArgs(a.ActorUserID, a.BillingPrincipalUserID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3).AddRow(4))
	mock.ExpectQuery(`SELECT billing_owner_user_id,status FROM workspaces`).WithArgs(a.WorkspaceID).
		WillReturnRows(sqlmock.NewRows([]string{"billing_owner_user_id", "status"}).AddRow(3, "active"))
	mock.ExpectQuery(`SELECT status FROM projects`).WithArgs(a.ProjectID, a.WorkspaceID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("active"))
	mock.ExpectQuery(`SELECT id FROM api_keys`).WithArgs(a.APIKeyID, a.ActorUserID, a.ProjectID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	mock.ExpectQuery(`SELECT user_id FROM workspace_members`).WithArgs(a.WorkspaceID, a.ActorUserID, a.BillingPrincipalUserID).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(3).AddRow(4))
	mock.ExpectQuery(`SELECT .* FROM budget_reservations`).WithArgs("request", a.APIKeyID).WillReturnError(sql.ErrNoRows)
	for _, policy := range []string{"workspace_budget_policies", "project_budget_policies"} {
		id := a.WorkspaceID
		if policy == "project_budget_policies" {
			id = a.ProjectID
		}
		mock.ExpectQuery(`SELECT amount,hard_limit,enabled,timezone FROM ` + policy).WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"amount", "hard_limit", "enabled", "timezone"}).AddRow(10, true, true, "UTC"))
	}
	mock.ExpectQuery(`INSERT INTO budget_counters`).WithArgs("workspace", a.WorkspaceID, period, float64(2), true, float64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id"}).AddRow(1))
	mock.ExpectQuery(`INSERT INTO budget_counters`).WithArgs("project", a.ProjectID, period, float64(2), true, float64(10)).WillReturnError(failure)
	mock.ExpectRollback()
	repo := &budgetRepository{db: db, now: func() time.Time { return period }}
	res, err := repo.Reserve(context.Background(), a, "request", 2)
	require.ErrorIs(t, err, failure)
	require.Nil(t, res)
	require.NoError(t, mock.ExpectationsWereMet())
}
