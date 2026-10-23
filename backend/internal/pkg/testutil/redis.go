package testutil

import (
	"testing"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient creates a client for a test's Redis fixture and closes it at cleanup.
func NewRedisClient(t testing.TB, address string) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: address})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close test Redis client: %v", err)
		}
	})
	return client
}
