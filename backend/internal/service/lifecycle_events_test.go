package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecycleEventsAreWorkspaceVisibleAndSubscribable(t *testing.T) {
	for _, eventType := range []string{
		EventWorkspaceRestored, EventWorkspaceRetentionUpdated,
		EventWorkspaceExportRequested, EventWorkspaceExportCompleted, EventWorkspaceExportFailed, EventWorkspaceExportCancelled,
		EventWorkspaceDeletionRequested, EventWorkspaceDeletionCancelled, EventWorkspaceDeletionBlocked, EventWorkspaceDeletionCompleted,
	} {
		t.Run(eventType, func(t *testing.T) {
			event, err := NewDomainEvent(eventType, 11, 0, 7, "workspace", "11", DomainEventData{"status": "pending"})
			require.NoError(t, err)
			require.True(t, IsWorkspaceVisibleEvent(event.Type))
			subscriptions, err := normalizeWebhookEvents([]string{eventType})
			require.NoError(t, err)
			require.Equal(t, []string{eventType}, subscriptions)
		})
	}
}
