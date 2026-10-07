package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSCIMNotificationFanoutSilencesSuccessAndKeepsOwnerAdminAlerts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	resolver := NewNotificationRecipientResolver(db)
	for _, typ := range []string{service.EventMemberJoined, service.EventWorkspaceTeamMemberAdded, service.EventSCIMTokenCreated} {
		event, e := service.NewDomainEvent(typ, 1, 0, 0, "scim_connector", "2", service.DomainEventData{"category": "scim", "connector_id": int64(2)})
		require.NoError(t, e)
		ids, e := resolver.Resolve(context.Background(), event)
		require.NoError(t, e)
		require.Empty(t, ids)
	}
	for _, typ := range []string{service.EventSCIMConnectorDisabled, service.EventSCIMSyncFailed, service.EventSCIMTokenExpiring, service.EventSCIMSecurityConflict} {
		mock.ExpectQuery(regexp.QuoteMeta(`SELECT m.user_id,m.role FROM workspace_members m JOIN users u ON u.id=m.user_id WHERE m.workspace_id=$1 AND m.status='active' AND u.status='active' AND u.deleted_at IS NULL`)).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}).AddRow(1, "owner").AddRow(2, "admin").AddRow(3, "billing").AddRow(4, "developer").AddRow(5, "viewer"))
		event, e := service.NewDomainEvent(typ, 1, 0, 0, "scim_connector", "2", service.DomainEventData{"category": "scim", "connector_id": int64(2)})
		require.NoError(t, e)
		ids, e := resolver.Resolve(context.Background(), event)
		require.NoError(t, e)
		require.Equal(t, []int64{1, 2}, ids)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
