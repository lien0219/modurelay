package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPolicyUpdateEventIsWorkspaceVisibleAndUsesSafeScalarPayload(t *testing.T) {
	event, err := NewDomainEvent(EventPolicyUpdated, 11, 42, 7, "policy", "project:42", DomainEventData{
		"scope_type":      "project",
		"scope_id":        int64(42),
		"policy_revision": int64(4),
	})
	require.NoError(t, err)
	require.Equal(t, EventPolicyUpdated, event.Type)
	require.True(t, IsWorkspaceVisibleEvent(EventPolicyUpdated))
	category, title, body := notificationPresentation(EventPolicyUpdated)
	require.Equal(t, "policy", category)
	require.Equal(t, "notifications.policy.title", title)
	require.Equal(t, "notifications.policy.body", body)
	_, err = NewDomainEvent(EventPolicyUpdated, 11, 42, 7, "policy", "project:42", DomainEventData{"allowed_models": []string{"secret"}})
	require.Error(t, err)
}

func TestPolicyActorContextRoundTripsAuthenticatedID(t *testing.T) {
	ctx := WithPolicyActor(context.Background(), 73)
	require.EqualValues(t, 73, PolicyActorID(ctx))
	require.Zero(t, PolicyActorID(context.Background()))
}
