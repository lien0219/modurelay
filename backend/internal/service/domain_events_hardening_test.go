package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type boundedEventRecipients func(context.Context, *DomainEvent) ([]int64, error)

func (f boundedEventRecipients) Resolve(ctx context.Context, event *DomainEvent) ([]int64, error) {
	return f(ctx, event)
}

func TestDomainEventDispatcherBoundsClaimAndConsumerContexts(t *testing.T) {
	claim := dispatcherClaim(t)
	claim.LockedUntil = time.Now().Add(time.Second)
	outbox := &eventDispatcherOutboxStub{claim: func(ctx context.Context) ([]DomainEventOutboxRecord, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok, "queue claiming needs its own bounded context")
		require.Less(t, time.Until(deadline), domainEventDispatcherLease)
		return []DomainEventOutboxRecord{claim}, nil
	}}
	resolver := boundedEventRecipients(func(ctx context.Context, _ *DomainEvent) ([]int64, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok, "consumer processing must fit the live claim")
		require.True(t, deadline.Before(claim.LockedUntil))
		return nil, nil
	})
	d := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, resolver, nil)
	require.NoError(t, d.ProcessBatch(context.Background()))
	require.Equal(t, []string{claim.EventID}, outbox.ackIDs)
}

func TestDomainEventDispatcherExpiredClaimNeverInvokesConsumer(t *testing.T) {
	claim := dispatcherClaim(t)
	claim.LockedUntil = time.Now().Add(-time.Second)
	outbox := &eventDispatcherOutboxStub{claims: []DomainEventOutboxRecord{claim}}
	var calls atomic.Int32
	resolver := boundedEventRecipients(func(context.Context, *DomainEvent) ([]int64, error) { calls.Add(1); return nil, nil })
	d := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, resolver, nil)
	require.ErrorIs(t, d.ProcessBatch(context.Background()), ErrDomainEventLeaseLost)
	require.Zero(t, calls.Load())
	require.Empty(t, outbox.ackIDs)
	require.Empty(t, outbox.retryReasons)
}

func TestDomainEventDispatcherSlowConsumerDoesNotStarveFollowingEvent(t *testing.T) {
	poison, healthy := dispatcherClaim(t), dispatcherClaim(t)
	poison.LockedUntil = time.Now().Add(250 * time.Millisecond)
	outbox := &eventDispatcherOutboxStub{claims: []DomainEventOutboxRecord{poison, healthy}}
	resolver := boundedEventRecipients(func(ctx context.Context, event *DomainEvent) ([]int64, error) {
		if event.ID == poison.EventID {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return nil, nil
	})
	d := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, resolver, nil)
	started := time.Now()
	require.ErrorIs(t, d.ProcessBatch(context.Background()), context.DeadlineExceeded)
	require.Less(t, time.Since(started), time.Second)
	require.Len(t, outbox.retryReasons, 1)
	require.Equal(t, []string{healthy.EventID}, outbox.ackIDs)
}

func TestDomainEventDispatcherDrainCancelsConsumerAndLeavesRemainingClaimsRecoverable(t *testing.T) {
	entered, finished := make(chan struct{}), make(chan struct{})
	var claims, consumed atomic.Int32
	outbox := &eventDispatcherOutboxStub{claim: func(context.Context) ([]DomainEventOutboxRecord, error) {
		claims.Add(1)
		return []DomainEventOutboxRecord{dispatcherClaim(t), dispatcherClaim(t)}, nil
	}}
	resolver := boundedEventRecipients(func(ctx context.Context, _ *DomainEvent) ([]int64, error) {
		consumed.Add(1)
		close(entered)
		<-ctx.Done()
		close(finished)
		return nil, ctx.Err()
	})
	d := NewDomainEventDispatcher(outbox, &eventDispatcherNotificationsStub{}, resolver, nil)
	d.Start()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("consumer did not enter")
	}
	d.Stop()
	select {
	case <-finished:
	default:
		t.Fatal("drain returned before canceled consumer exited")
	}
	require.Equal(t, int32(1), claims.Load())
	require.Equal(t, int32(1), consumed.Load())
	require.Empty(t, outbox.ackIDs)
	require.Empty(t, outbox.retryReasons, "cancellation leaves immutable durable claims for lease recovery")
}
