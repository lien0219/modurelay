package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type jsonFieldMatcher struct {
	key  string
	want any
}

func (m jsonFieldMatcher) Match(v driver.Value) bool {
	var got map[string]any
	b, ok := v.([]byte)
	if !ok || json.Unmarshal(b, &got) != nil {
		return false
	}
	return got[m.key] == m.want
}

func TestPolicyRepositoryCreateAtomicallyAttributesAuditAndOutbox(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT workspace_id FROM projects WHERE id=\\$1").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(int64(11)))
	mock.ExpectQuery("INSERT INTO project_policies").
		WithArgs(int64(42), sqlmock.AnyArg(), sqlmock.AnyArg(), nil, nil, nil, nil, nil, int64(7), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(1)))
	mock.ExpectExec("INSERT INTO workspace_audit_logs").
		WithArgs(int64(11), int64(42), int64(7), "project_policy.updated", "project_policy", int64(42), jsonFieldMatcher{key: "old_revision", want: float64(0)}).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO domain_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO domain_event_outbox").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := NewPolicyRepository(db)
	got, err := repo.UpdatePolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42}, 0, domain.Policy{})
	require.NoError(t, err)
	require.Equal(t, int64(1), got.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryRevisionMissRollsBackWithoutAudit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	ctx := service.WithPolicyActor(context.Background(), 7)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT workspace_id FROM projects WHERE id=\\$1").WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(int64(11)))
	mock.ExpectQuery("SELECT .* FROM project_policies WHERE project_id=\\$1 FOR UPDATE").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"revision", "allowed_models", "allowed_platforms", "rpm_limit", "daily_request_limit", "monthly_request_limit", "daily_token_limit", "monthly_token_limit"}).
			AddRow(int64(8), nil, nil, nil, nil, nil, nil, nil))
	mock.ExpectRollback()
	repo := NewPolicyRepository(db)
	_, err = repo.UpdatePolicy(ctx, domain.PolicyRef{Scope: domain.PolicyScopeProject, ScopeID: 42}, 9, domain.Policy{})
	require.ErrorIs(t, err, domain.ErrPolicyRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPolicyRepositoryUpdateRequiresActorAttribution(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := NewPolicyRepository(db)
	_, err = repo.UpdatePolicy(context.Background(), domain.PolicyRef{Scope: domain.PolicyScopeWorkspace, ScopeID: 1}, 0, domain.Policy{})
	require.ErrorIs(t, err, domain.ErrInvalidPolicy)
	require.NoError(t, mock.ExpectationsWereMet())
}
