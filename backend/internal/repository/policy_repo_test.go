package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func expectPolicyAuthorization(mock sqlmock.Sqlmock, actorID, workspaceID, projectID int64, ref domain.PolicyRef) {
	mock.ExpectExec("SET LOCAL statement_timeout='20s'; SET LOCAL lock_timeout='5s'").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT id FROM users WHERE id=\\$1 AND status='active' AND deleted_at IS NULL FOR SHARE").WithArgs(actorID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(actorID))
	switch ref.Scope {
	case domain.PolicyScopeWorkspace:
		mock.ExpectQuery("SELECT id FROM workspaces WHERE id=\\$1$").WithArgs(ref.ScopeID).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(workspaceID))
	case domain.PolicyScopeProject:
		mock.ExpectQuery("SELECT workspace_id FROM projects WHERE id=\\$1$").WithArgs(ref.ScopeID).
			WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(workspaceID))
	case domain.PolicyScopeServiceAccount:
		mock.ExpectQuery("SELECT workspace_id,project_id FROM service_accounts WHERE id=\\$1$").WithArgs(ref.ScopeID).
			WillReturnRows(sqlmock.NewRows([]string{"workspace_id", "project_id"}).AddRow(workspaceID, projectID))
	}
	mock.ExpectQuery("SELECT id FROM workspaces WHERE id=\\$1 FOR UPDATE").WithArgs(workspaceID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(workspaceID))
	now := time.Unix(100, 0).UTC()
	mock.ExpectQuery("SELECT m.id,m.workspace_id,m.user_id,m.role,m.status.*FROM workspace_members m JOIN users").WithArgs(workspaceID, actorID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "user_id", "role", "status", "invited_by_user_id", "joined_at", "created_at", "updated_at"}).AddRow(int64(1), workspaceID, actorID, "admin", "active", nil, now, now, now))
	mock.ExpectQuery("SELECT id,name,slug,type,status,project_access_mode.*FROM workspaces WHERE id=\\$1 AND EXISTS").WithArgs(workspaceID, actorID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "type", "status", "project_access_mode", "owner_user_id", "billing_owner_user_id", "created_at", "updated_at"}).AddRow(workspaceID, "Organization", "organization", "organization", "active", "all_projects", actorID, actorID, now, now))
	if projectID > 0 {
		mock.ExpectQuery("SELECT id,workspace_id,name,slug,description,status.*FROM projects WHERE workspace_id=\\$1 AND id=\\$2").WithArgs(workspaceID, projectID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "name", "slug", "description", "status", "is_default", "created_by_user_id", "allowed_group_ids", "allowed_models", "created_at", "updated_at"}).AddRow(projectID, workspaceID, "Project", "project", "", "active", false, actorID, "{}", "{}", now, now))
	}
	if ref.Scope == domain.PolicyScopeServiceAccount {
		mock.ExpectQuery("SELECT id FROM service_accounts WHERE workspace_id=\\$1 AND project_id=\\$2 AND id=\\$3 FOR SHARE").WithArgs(workspaceID, projectID, ref.ScopeID).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(ref.ScopeID))
	}
	mock.ExpectQuery("SELECT w.id,COALESCE.*require_sso.*FROM workspaces w LEFT JOIN workspace_security_policies").WithArgs(workspaceID).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id", "require_sso", "sso_grace_until", "revision", "updated_by", "updated_at", "require_mfa", "session_max_age_seconds", "invitation_policy", "allow_external_members", "workspace_jit_enabled", "approved_identity_provider_mode", "approved_identity_provider_ids"}).AddRow(workspaceID, false, nil, int64(1), nil, now, false, nil, "any", true, true, "any_active", "{}"))
	mock.ExpectQuery("SELECT totp_enabled FROM users WHERE id=\\$1 AND status='active' AND deleted_at IS NULL$").WithArgs(actorID).
		WillReturnRows(sqlmock.NewRows([]string{"totp_enabled"}).AddRow(false))
}

func TestPolicyRepositoryGetPreservesNullAndEmptyAllowlists(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
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
	defer func() { _ = db.Close() }()
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
	defer func() { _ = db.Close() }()
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
	defer func() { _ = db.Close() }()
	rpm := int64(10)
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	expectPolicyAuthorization(mock, 7, 11, 42, domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42})
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
	defer func() { _ = db.Close() }()
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	expectPolicyAuthorization(mock, 7, 7, 0, domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 7})
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
	defer func() { _ = db.Close() }()
	repo := NewPolicyRepository(db)
	_, err = repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 0})
	require.ErrorIs(t, err, domain.ErrInvalidPolicy)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryUpdateCreateConflictDoesNotOverwriteExisting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	expectPolicyAuthorization(mock, 7, 1, 2, domain.PolicyRef{Scope: domain.PolicyScopeServiceAccount, ScopeID: 9})
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
	defer func() { _ = db.Close() }()
	dbErr := errors.New("db down")
	mock.ExpectQuery("SELECT .* FROM project_policies WHERE project_id=\\$1").WithArgs(int64(42)).WillReturnError(dbErr)
	repo := NewPolicyRepository(db)
	_, err = repo.GetPolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42})
	require.ErrorIs(t, err, dbErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
