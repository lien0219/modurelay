package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTOTPDisableDependencyIsCheckedInsideUserTransaction(t *testing.T) {
	for _, required := range []bool{false, true} {
		t.Run(map[bool]string{false: "unrequired", true: "required"}[required], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT password_hash,totp_enabled FROM users .* FOR UPDATE").WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"password_hash", "totp_enabled"}).AddRow("current-hash", true))
			mock.ExpectQuery("SELECT EXISTS.*workspace_members.*workspace_security_policies.*require_mfa").WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"required"}).AddRow(required))
			if required {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec("UPDATE users SET totp_enabled=false,totp_enabled_at=NULL,totp_secret_encrypted=NULL.*WHERE id=\\$1").WithArgs(int64(42)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}
			err = NewUserFactorDependencyRepository(db).DisableTOTP(context.Background(), 42, "current-hash")
			if required {
				require.ErrorIs(t, err, service.ErrWorkspaceMFARequired)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestTOTPDisableDependencyReadFailureDoesNotDisableFactor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	dependencyErr := errors.New("policy read unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT password_hash,totp_enabled FROM users .* FOR UPDATE").WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"password_hash", "totp_enabled"}).AddRow("current-hash", true))
	mock.ExpectQuery("SELECT EXISTS.*workspace_members.*workspace_security_policies.*require_mfa").WithArgs(int64(42)).WillReturnError(dependencyErr)
	mock.ExpectRollback()
	err = NewUserFactorDependencyRepository(db).DisableTOTP(context.Background(), 42, "current-hash")
	require.ErrorIs(t, err, dependencyErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestTOTPDisableRejectsChangedIdentityUnderUserLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT password_hash,totp_enabled FROM users .* FOR UPDATE").WithArgs(int64(42)).WillReturnRows(sqlmock.NewRows([]string{"password_hash", "totp_enabled"}).AddRow("new-password-hash", true))
	mock.ExpectRollback()
	err = NewUserFactorDependencyRepository(db).DisableTOTP(context.Background(), 42, "old-password-hash")
	require.ErrorIs(t, err, service.ErrPasswordIncorrect)
	require.NoError(t, mock.ExpectationsWereMet())
}
