package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSCIMOperationalEventsUseScalarPayloadAndSecurityPresentation(t *testing.T) {
	for _, eventType := range []string{"workspace.scim.connector.created", "workspace.scim.connector.disabled", "workspace.scim.token.created", "workspace.scim.token.revoked", "workspace.scim.sync.failed", "workspace.scim.token.expiring", "workspace.scim.security.conflict"} {
		event, err := NewDomainEvent(eventType, 11, 0, 0, "scim_connector", "3", DomainEventData{"connector_id": int64(3), "token_id": int64(4), "resource_id": "opaque", "operation": "patch", "added_count": 2, "removed_count": 1, "failure_count": 3})
		require.NoError(t, err)
		require.Nil(t, event.ActorUserID)
		require.True(t, IsWorkspaceVisibleEvent(event.Type))
	}
	for _, typ := range []string{"workspace.scim.connector.disabled", "workspace.scim.sync.failed", "workspace.scim.token.expiring", "workspace.scim.security.conflict"} {
		category, title, body := notificationPresentation(typ)
		require.Equal(t, "security", category)
		require.Equal(t, "notifications.scim.title", title)
		require.Equal(t, "notifications.scim.body", body)
	}
	_, err := NewDomainEvent("workspace.scim.token.created", 11, 0, 3, "scim_token", "4", DomainEventData{"secret": "canary"})
	require.Error(t, err)
}

func TestSCIMProvisioningPermissionsBelongOnlyToWorkspaceOwnerAndAdmin(t *testing.T) {
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		for _, permission := range []string{"provisioning.read", "provisioning.manage", "provisioning.token.rotate"} {
			require.Equal(t, role == "owner" || role == "admin", HasWorkspacePermission(role, permission), role+":"+permission)
		}
	}
}
