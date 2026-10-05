package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMachineBudgetReservationScansNullActor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Now().UTC()
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "request_id", "actor_user_id", "service_account_id", "api_key_id", "workspace_id", "project_id", "billing_principal_user_id", "period_start", "period_end", "project_period_start", "project_period_end", "estimate", "actual", "status"}).AddRow("r", "req", nil, 9, 5, 1, 2, 3, now, now, now, now, 1, 0, "pending"))
	res, err := scanBudgetReservation(db.QueryRow("SELECT"))
	require.NoError(t, err)
	require.Zero(t, res.ActorUserID)
	require.Equal(t, int64(9), res.ServiceAccountID)
	a := service.BudgetAttribution{ServiceAccountID: 9, APIKeyID: 5, WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3}
	require.True(t, budgetAttributionMatches(res, a))
	a.ServiceAccountID = 10
	require.False(t, budgetAttributionMatches(res, a))
}

func TestMachineTenantUsageSnapshotRejectsDifferentMachine(t *testing.T) {
	id := int64(9)
	w, p, payer := int64(1), int64(2), int64(3)
	reservation := "00000000-0000-0000-0000-000000000001"
	cmd := &service.UsageBillingCommand{ServiceAccountID: id, APIKeyID: 5, AccountID: 6, RequestID: "req", WorkspaceID: w, ProjectID: p, BillingPrincipalUserID: payer, BudgetReservationID: reservation, BudgetActualCost: 1}
	log := &service.UsageLog{ServiceAccountID: &id, APIKeyID: 5, AccountID: 6, RequestID: "req", WorkspaceID: &w, ProjectID: &p, BillingPrincipalUserID: &payer, BudgetReservationID: &reservation, ActualCost: 1, TotalCost: 1}
	require.NoError(t, validateTenantUsageSnapshot(cmd, log))
	different := int64(10)
	log.ServiceAccountID = &different
	require.ErrorIs(t, validateTenantUsageSnapshot(cmd, log), service.ErrBudgetReservationConflict)
	log.ServiceAccountID = &id
	cmd.UserID = 3
	require.ErrorIs(t, validateTenantUsageSnapshot(cmd, log), service.ErrBudgetReservationConflict)
}

func TestMachineAdmissionLocksOnlyPayerUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	require.NoError(t, err)
	a := service.BudgetAttribution{ServiceAccountID: 9, BillingPrincipalUserID: 3}
	mock.ExpectQuery("SELECT id FROM users").WithArgs(int64(0), int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))
	require.NoError(t, lockBudgetAdmissionUsers(context.Background(), tx, a))
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
