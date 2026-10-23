package billingobserve

import (
	"sync"
	"testing"
)

func TestGapConcurrentPersistencePreservesNewFailures(t *testing.T) {
	const instance = "concurrent-test"
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() { defer wg.Done(); Failed(instance) }()
	}
	wg.Wait()
	before := Snapshot(instance)
	if before.MissedCount != 100 {
		t.Fatalf("missed=%d", before.MissedCount)
	}
	Failed(instance)
	Persisted(instance, before.MissedCount)
	if Pending() != 1 {
		t.Fatalf("new failure disappeared: pending=%d", Pending())
	}
	Persisted(instance, 100) // duplicate flush must not decrement again
	if Pending() != 1 {
		t.Fatalf("duplicate flush changed pending=%d", Pending())
	}
	Persisted(instance, 101)
	if Pending() != 0 {
		t.Fatalf("pending=%d", Pending())
	}
	if Snapshot(instance).MissedCount != 101 {
		t.Fatal("cumulative evidence cleared")
	}
}
