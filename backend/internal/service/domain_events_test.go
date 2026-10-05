package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type secretDomainEventValue struct{}

func (secretDomainEventValue) MarshalJSON() ([]byte, error) {
	return []byte(`{"credential":"sk-private-provider-secret"}`), nil
}

func TestDomainEventRejectsObjectsAndNonFiniteValues(t *testing.T) {
	for name, value := range map[string]any{
		"map":            map[string]any{"api_key": "sk-private-provider-secret"},
		"array":          []string{"sk-private-provider-secret"},
		"bytes":          []byte("sk-private-provider-secret"),
		"struct":         User{Email: "secret@example.com"},
		"marshaler":      secretDomainEventValue{},
		"pointer":        new(string),
		"nan":            math.NaN(),
		"infinity":       math.Inf(1),
		"invalid_number": json.Number("{\"credential\":\"secret\"}"),
	} {
		t.Run(name, func(t *testing.T) {
			event, err := NewDomainEvent(EventAPIKeyCreated, 11, 13, 7, "api_key", "17", DomainEventData{"key_name": value})
			require.Error(t, err)
			require.Nil(t, event)
			require.NotContains(t, err.Error(), "sk-private-provider-secret")
			require.NotContains(t, err.Error(), "secret@example.com")
		})
	}
}

func TestDomainEventMarshalRevalidatesPayloadAndEnvelope(t *testing.T) {
	for _, mutate := range []func(*DomainEvent){
		func(e *DomainEvent) { e.Data["key_name"] = map[string]any{"credential": "secret"} },
		func(e *DomainEvent) { e.Data["api_key"] = "sk-private-provider-secret" },
		func(e *DomainEvent) { e.Type = "unknown.event" },
		func(e *DomainEvent) { e.Version = 2 },
		func(e *DomainEvent) { e.Subject = EventSubject{} },
		func(e *DomainEvent) { e.ProjectID = new(int64) },
	} {
		event, err := NewDomainEvent(EventAPIKeyCreated, 11, 13, 7, "api_key", "17", DomainEventData{"key_id": int64(17)})
		require.NoError(t, err)
		mutate(event)
		payload, err := event.MarshalPayload()
		require.Error(t, err)
		require.Nil(t, payload)
		require.NotContains(t, err.Error(), "sk-private-provider-secret")
	}
}

func TestDomainEventCopiesScalarInputAndPreservesV1Scope(t *testing.T) {
	data := DomainEventData{"key_id": int64(17), "key_name": "Client", "amount": 1.25, "threshold": 80, "status": "active"}
	event, err := NewDomainEvent(EventAPIKeyCreated, 11, 13, 7, "api_key", "17", data)
	require.NoError(t, err)
	data["key_name"] = "later edit"
	data["credential"] = "sk-private-provider-secret"
	payload, err := event.MarshalPayload()
	require.NoError(t, err)
	var actual map[string]any
	require.NoError(t, json.Unmarshal(payload, &actual))
	require.Equal(t, float64(1), actual["version"])
	require.Equal(t, float64(11), actual["workspace_id"])
	require.Equal(t, float64(13), actual["project_id"])
	require.Equal(t, float64(7), actual["actor_user_id"])
	require.Equal(t, map[string]any{"type": "api_key", "id": "17"}, actual["subject"])
	actualData, ok := actual["data"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "Client", actualData["key_name"])
	require.NotContains(t, string(payload), "credential")
	require.NotContains(t, string(payload), "sk-private-provider-secret")
	other, err := NewDomainEvent(EventAPIKeyCreated, 11, 13, 7, "api_key", "17", dataWithoutCredentials())
	require.NoError(t, err)
	require.NotEqual(t, event.ID, other.ID)
}

func dataWithoutCredentials() DomainEventData { return DomainEventData{"key_id": int64(17)} }

type eventDispatcherOutboxStub struct {
	claims       []DomainEventOutboxRecord
	claimErr     error
	ackErr       error
	retryErr     error
	ackIDs       []string
	retryReasons []error
	claim        func(context.Context) ([]DomainEventOutboxRecord, error)
}

type eventDispatcherRetentionStub struct {
	eventDispatcherOutboxStub
	cleanup func(context.Context, int) (DomainEventRetentionResult, error)
}

func (s *eventDispatcherRetentionStub) Cleanup(ctx context.Context, limit int) (DomainEventRetentionResult, error) {
	return s.cleanup(ctx, limit)
}

func (s *eventDispatcherOutboxStub) Claim(ctx context.Context, limit int, lease time.Duration) ([]DomainEventOutboxRecord, error) {
	if s.claim != nil {
		return s.claim(ctx)
	}
	return s.claims, s.claimErr
}
func (s *eventDispatcherOutboxStub) Ack(_ context.Context, id, token string) error {
	s.ackIDs = append(s.ackIDs, id)
	return s.ackErr
}
func (s *eventDispatcherOutboxStub) Retry(_ context.Context, id, token string, delay time.Duration, err error) error {
	s.retryReasons = append(s.retryReasons, err)
	return s.retryErr
}

type eventDispatcherNotificationsStub struct {
	NotificationRepository
	rows []UserNotification
	err  error
}

func (s *eventDispatcherNotificationsStub) CreateForRecipients(_ context.Context, rows []UserNotification) error {
	s.rows = append(s.rows, rows...)
	return s.err
}

type eventDispatcherRecipientsStub struct {
	ids []int64
	err error
}

func (s eventDispatcherRecipientsStub) Resolve(context.Context, *DomainEvent) ([]int64, error) {
	return s.ids, s.err
}

type eventDispatcherWebhooksStub struct {
	calls int
	err   error
}

func (s *eventDispatcherWebhooksStub) EnqueueEventDeliveries(context.Context, *DomainEvent) error {
	s.calls++
	return s.err
}

func dispatcherClaim(t *testing.T) DomainEventOutboxRecord {
	t.Helper()
	event, err := NewDomainEvent(EventProjectArchived, 11, 13, 7, "project", "13", DomainEventData{"status": "archived"})
	require.NoError(t, err)
	payload, err := event.MarshalPayload()
	require.NoError(t, err)
	return DomainEventOutboxRecord{EventID: event.ID, EventType: event.Type, Payload: payload, Attempts: 1, LockToken: "lease", LockedUntil: time.Now().Add(time.Minute)}
}

func TestDomainEventDispatcherRejectsUnsafePersistedPayload(t *testing.T) {
	claim := dispatcherClaim(t)
	claim.Payload = []byte(strings.Replace(string(claim.Payload), `"status":"archived"`, `"status":{"credential":"sk-private-provider-secret"}`, 1))
	outbox := &eventDispatcherOutboxStub{claims: []DomainEventOutboxRecord{claim}}
	notifications := &eventDispatcherNotificationsStub{}
	webhooks := &eventDispatcherWebhooksStub{}
	dispatcher := NewDomainEventDispatcher(outbox, notifications, eventDispatcherRecipientsStub{ids: []int64{7}}, webhooks)
	err := dispatcher.ProcessBatch(context.Background())
	require.Error(t, err)
	require.Empty(t, notifications.rows)
	require.Zero(t, webhooks.calls)
	require.Empty(t, outbox.ackIDs)
	require.Len(t, outbox.retryReasons, 1)
	require.NotContains(t, outbox.retryReasons[0].Error(), "sk-private-provider-secret")
}

func TestDomainEventDispatcherReportsSafeConsumerFailure(t *testing.T) {
	claim := dispatcherClaim(t)
	outbox := &eventDispatcherOutboxStub{claims: []DomainEventOutboxRecord{claim}}
	notifications := &eventDispatcherNotificationsStub{err: errors.New("SQL payload includes sk-private-provider-secret")}
	dispatcher := NewDomainEventDispatcher(outbox, notifications, eventDispatcherRecipientsStub{ids: []int64{7}}, nil)
	err := dispatcher.ProcessBatch(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "sk-private-provider-secret")
	require.Len(t, outbox.retryReasons, 1)
	require.NotContains(t, outbox.retryReasons[0].Error(), "sk-private-provider-secret")
	require.Empty(t, outbox.ackIDs)
}

func TestDomainEventDispatcherLogsOnlySafeDiagnostics(t *testing.T) {
	claim := dispatcherClaim(t)
	claim.EventID = "sk-private-provider-secret"
	claim.EventType = "https://user:password@private.example/hook"
	outbox := &eventDispatcherOutboxStub{claims: []DomainEventOutboxRecord{claim}}
	dispatcher := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, eventDispatcherRecipientsStub{ids: []int64{7}}, nil)
	core, logs := observer.New(zap.WarnLevel)
	dispatcher.log = zap.New(core)
	require.Error(t, dispatcher.ProcessBatch(context.Background()))
	require.Len(t, logs.All(), 1)
	entry := logs.All()[0]
	encoded, err := json.Marshal(entry.ContextMap())
	require.NoError(t, err)
	require.Contains(t, string(encoded), "event_invalid")
	require.NotContains(t, string(encoded), "sk-private-provider-secret")
	require.NotContains(t, string(encoded), "password")
	require.NotContains(t, string(encoded), "private.example")
}

func TestDomainEventDispatcherLogsBackgroundClaimFailure(t *testing.T) {
	outbox := &eventDispatcherOutboxStub{claimErr: errors.New("database parameters sk-private-provider-secret")}
	dispatcher := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, eventDispatcherRecipientsStub{}, nil)
	core, logs := observer.New(zap.WarnLevel)
	dispatcher.log = zap.New(core)
	dispatcher.Start()
	require.Eventually(t, func() bool { return logs.Len() > 0 }, 2*time.Second, 10*time.Millisecond)
	dispatcher.Stop()
	entries := logs.All()
	require.Equal(t, "outbox_claim_failed", entries[0].ContextMap()["failure_code"])
	encoded, err := json.Marshal(entries[0].ContextMap())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "sk-private-provider-secret")
}

func TestDomainEventDispatcherStopCancelsAndWaitsForClaim(t *testing.T) {
	entered := make(chan struct{})
	finished := make(chan struct{})
	outbox := &eventDispatcherOutboxStub{claim: func(ctx context.Context) ([]DomainEventOutboxRecord, error) {
		close(entered)
		<-ctx.Done()
		close(finished)
		return nil, ctx.Err()
	}}
	dispatcher := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, eventDispatcherRecipientsStub{}, nil)
	dispatcher.Start()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not claim")
	}
	dispatcher.Stop()
	select {
	case <-finished:
	default:
		t.Fatal("Stop returned before claim exited")
	}
	dispatcher.Stop()
}

func TestDomainEventDispatcherStopBeforeStartPreventsLaunch(t *testing.T) {
	entered := make(chan struct{}, 1)
	outbox := &eventDispatcherOutboxStub{claim: func(context.Context) ([]DomainEventOutboxRecord, error) { entered <- struct{}{}; return nil, nil }}
	dispatcher := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, eventDispatcherRecipientsStub{}, nil)
	dispatcher.Stop()
	dispatcher.Start()
	dispatcher.Stop()
	// Stop and Start are allowed to race; after both finish no work remains.
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); dispatcher.Start() }()
	go func() { defer wg.Done(); dispatcher.Stop() }()
	wg.Wait()
	select {
	case <-entered:
		t.Fatal("stopped dispatcher started work")
	case <-time.After(20 * time.Millisecond):
	}
}

func TestDomainEventDispatcherOwnsRetentionAndCancelsCleanup(t *testing.T) {
	entered := make(chan int, 1)
	finished := make(chan struct{})
	outbox := &eventDispatcherRetentionStub{cleanup: func(ctx context.Context, limit int) (DomainEventRetentionResult, error) {
		entered <- limit
		<-ctx.Done()
		close(finished)
		return DomainEventRetentionResult{}, ctx.Err()
	}}
	dispatcher := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, eventDispatcherRecipientsStub{}, nil)
	dispatcher.Start()
	select {
	case limit := <-entered:
		require.Equal(t, 500, limit)
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher did not perform retention")
	}
	dispatcher.Stop()
	select {
	case <-finished:
	default:
		t.Fatal("Stop did not wait for canceled retention cleanup")
	}
}

func TestDomainEventDispatcherObservesRetentionErrorWithoutSecrets(t *testing.T) {
	outbox := &eventDispatcherRetentionStub{cleanup: func(context.Context, int) (DomainEventRetentionResult, error) {
		return DomainEventRetentionResult{}, errors.New("cleanup parameters include sk-private-provider-secret")
	}}
	dispatcher := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, eventDispatcherRecipientsStub{}, nil)
	core, logs := observer.New(zap.WarnLevel)
	dispatcher.log = zap.New(core)
	dispatcher.Start()
	require.Eventually(t, func() bool { return logs.Len() > 0 }, 2*time.Second, 10*time.Millisecond)
	dispatcher.Stop()
	entry := logs.All()[0]
	require.Equal(t, "retention_cleanup_failed", entry.ContextMap()["failure_code"])
	encoded, err := json.Marshal(entry.ContextMap())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "sk-private-provider-secret")
}
