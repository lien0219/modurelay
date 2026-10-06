package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPolicyRepositoryGetPreservesNullAndEmptyAllowlists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	rev := int64(3)
	modelLimit := int64(25)
	mock.ExpectQuery("SELECT .* FROM project_policies WHERE project_id=\\$1").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"revision", "allowed_models", "allowed_platforms", "rpm_limit", "daily_request_limit", "monthly_request_limit", "daily_token_limit", "monthly_token_limit"}).
			AddRow(rev, "{}", nil, modelLimit, nil, nil, nil, nil))

	repo := NewPolicyRepository(db)
	got, err := repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.NotNil(t, got.AllowedModels)
	require.Empty(t, got.AllowedModels)
	require.Nil(t, got.AllowedPlatforms)
	require.Equal(t, modelLimit, *got.RPMLimit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryGetMissingReturnsNilWithoutError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT .* FROM workspace_policies WHERE workspace_id=\\$1").
		WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
	repo := NewPolicyRepository(db)
	got, err := repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 7})
	require.NoError(t, err)
	require.Nil(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryTreatsLegacyGroupAndCredentialScopesAsInherited(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewPolicyRepository(db)
	for _, scope := range []domain.PolicyScope{domain.PolicyScopeGroup, domain.PolicyScopeCredential} {
		got, err := repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: scope, ScopeID: 7})
		require.NoError(t, err)
		require.Nil(t, got)
	}
}

func TestPolicyRepositoryUpdateUsesRevisionCompareAndSwap(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	rpm := int64(10)
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT workspace_id FROM projects WHERE id=\\$1").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(int64(11)))
	mock.ExpectQuery("SELECT .* FROM project_policies WHERE project_id=\\$1 FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"revision", "allowed_models", "allowed_platforms", "rpm_limit", "daily_request_limit", "monthly_request_limit", "daily_token_limit", "monthly_token_limit"}).
			AddRow(int64(3), "{}", nil, int64(5), nil, nil, nil, nil))
	mock.ExpectQuery("UPDATE project_policies SET .*updated_by_user_id=\\$8.*RETURNING revision").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), rpm, nil, nil, nil, nil, int64(7), int64(42), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(4)))
	mock.ExpectExec("INSERT INTO workspace_audit_logs").
		WithArgs(int64(11), int64(42), int64(7), "project_policy.updated", "project_policy", int64(42), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO domain_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO domain_event_outbox").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewPolicyRepository(db)
	got, err := repo.UpdatePolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42}, 3, domain.Policy{AllowedModels: []string{}, RPMLimit: &rpm})
	require.NoError(t, err)
	require.Equal(t, int64(4), got.Revision)
	require.Empty(t, got.AllowedModels)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryUpdateReturnsRevisionConflictWhenCASMisses(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM workspaces WHERE id=\\$1").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery("SELECT .* FROM workspace_policies WHERE workspace_id=\\$1 FOR UPDATE").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"revision", "allowed_models", "allowed_platforms", "rpm_limit", "daily_request_limit", "monthly_request_limit", "daily_token_limit", "monthly_token_limit"}).
			AddRow(int64(9), nil, nil, nil, nil, nil, nil, nil))
	mock.ExpectRollback()
	repo := NewPolicyRepository(db)
	_, err = repo.UpdatePolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 7}, 8, domain.Policy{})
	require.ErrorIs(t, err, domain.ErrPolicyRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryRejectsInvalidReferenceBeforeQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewPolicyRepository(db)
	_, err = repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 0})
	require.ErrorIs(t, err, domain.ErrInvalidPolicy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryUpdateCreateConflictDoesNotOverwriteExisting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT workspace_id,project_id FROM service_accounts WHERE id=\\$1").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id", "project_id"}).AddRow(int64(1), int64(2)))
	mock.ExpectQuery("INSERT INTO service_account_policies").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	repo := NewPolicyRepository(db)
	_, err = repo.UpdatePolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: 9}, 0, domain.Policy{})
	require.ErrorIs(t, err, domain.ErrPolicyRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryMapsUnexpectedDBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	dbErr := errors.New("db down")
	mock.ExpectQuery("SELECT .* FROM project_policies WHERE project_id=\\$1").WithArgs(int64(42)).WillReturnError(dbErr)
	repo := NewPolicyRepository(db)
	_, err = repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42})
	require.ErrorIs(t, err, dbErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
