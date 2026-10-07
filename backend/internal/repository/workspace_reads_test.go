package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

func TestListWorkspacesScansProjectAccessMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL")).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT w.id,w.name,w.slug,w.type,w.status,w.project_access_mode,w.owner_user_id,w.billing_owner_user_id,w.created_at,w.updated_at,m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id JOIN users u ON u.id=m.user_id WHERE m.user_id=$1 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL ORDER BY w.id LIMIT $2 OFFSET $3")).
		WithArgs(int64(7), pagination.DefaultPagination().Limit(), pagination.DefaultPagination().Offset()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "slug", "type", "status", "project_access_mode", "owner_user_id", "billing_owner_user_id", "created_at", "updated_at", "role"}).
			AddRow(int64(11), "Acme", "acme", "organization", "active", "assigned_projects", int64(7), int64(7), now, now, "developer"))

	repo := &workspaceRepository{db: db}
	items, total, err := repo.ListWorkspaces(context.Background(), 7, pagination.DefaultPagination())
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].ProjectAccessMode != "assigned_projects" {
		t.Fatalf("unexpected workspace result: total=%d items=%+v", total, items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
