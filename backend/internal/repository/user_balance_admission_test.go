package repository

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAdmissionBalanceReadsPrimaryWithReportingReplica(t *testing.T) {
	primary, primaryMock, err := sqlmock.New()
	require.NoError(t, err)
	replica, replicaMock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(&primaryWithReadReplicaDriver{
		Driver:    entsql.OpenDB(dialect.Postgres, primary),
		reporting: &reportingQueryer{primary: primary, replica: replica},
	}))
	t.Cleanup(func() { _ = client.Close() })
	repo := newUserRepositoryWithSQL(client, primary)
	// Any accidental replica read is unexpected and fails. Only the primary
	// knows this user's last debit exhausted the balance.
	primaryMock.ExpectQuery(`SELECT balance FROM users WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-1))
	balance, err := repo.GetBalance(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, -1.0, balance)
	require.NoError(t, primaryMock.ExpectationsWereMet())
	require.NoError(t, replicaMock.ExpectationsWereMet())
}

func TestAdmissionBalanceDoesNotMaskPrimaryFailureOrMissingUser(t *testing.T) {
	for _, missing := range []bool{false, true} {
		name := "primary-unavailable"
		if missing {
			name = "missing-or-deleted-user"
		}
		t.Run(name, func(t *testing.T) {
			repo, mock := newRedeemAdjustmentRepoMock(t)
			query := mock.ExpectQuery(`SELECT balance FROM users WHERE id = \$1 AND deleted_at IS NULL`).WithArgs(int64(42))
			wantErr := errors.New("primary unavailable")
			if missing {
				query.WillReturnRows(sqlmock.NewRows([]string{"balance"}))
				wantErr = service.ErrUserNotFound
			} else {
				query.WillReturnError(wantErr)
			}
			_, err := repo.GetBalance(context.Background(), 42)
			require.ErrorIs(t, err, wantErr)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
