package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	balanceInvalidationAttempts = 3
	balanceInvalidationBackoff  = 50 * time.Millisecond
	balanceInvalidationTimeout  = balanceInvalidationAttempts*cacheWriteTimeout + 3*balanceInvalidationBackoff
)

var errBalanceCacheInvalidationPending = errors.New("user balance cache invalidation is unconfirmed")

// Non-zero size preserves pointer identity for concurrent invalidations.
type balanceInvalidation struct{ userID int64 }

func retryUserBalanceCacheInvalidation(ctx context.Context, invalidator UserBalanceCacheInvalidator, userID int64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// The debit is already committed. A canceled request must not cancel its
	// invalidation, and a timed-out attempt must not cancel subsequent attempts.
	retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), balanceInvalidationTimeout)
	defer cancel()
	var lastErr error
	for attempt := 0; attempt < balanceInvalidationAttempts; attempt++ {
		attemptCtx, attemptCancel := context.WithTimeout(retryCtx, cacheWriteTimeout)
		lastErr = invalidator.InvalidateUserBalance(attemptCtx, userID)
		attemptCancel()
		if lastErr == nil {
			// The Redis implementation atomically changes the fill fence and DELs
			// the balance. Its successful reply confirms deletion, including a
			// missing key; checking GET afterwards would race a legitimate refill.
			return nil
		}
		if attempt+1 == balanceInvalidationAttempts {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * balanceInvalidationBackoff)
		select {
		case <-timer.C:
		case <-retryCtx.Done():
			timer.Stop()
			return fmt.Errorf("%w for user %d: %w", errBalanceCacheInvalidationPending, userID, lastErr)
		}
	}
	return fmt.Errorf("%w for user %d: %w", errBalanceCacheInvalidationPending, userID, lastErr)
}

func (s *BillingCacheService) confirmPendingBalanceInvalidation(ctx context.Context, userID int64) error {
	if _, pending := s.balanceInvalidations.Load(userID); !pending {
		return nil
	}
	if err := s.InvalidateUserBalance(ctx, userID); err != nil {
		return err
	}
	if _, pending := s.balanceInvalidations.Load(userID); pending {
		return errBalanceCacheInvalidationPending
	}
	return nil
}

// UserBalanceCacheInvalidator is deliberately narrower than BillingCacheService
// so auth, refund and batch flows can invalidate the same distributed fence.
type UserBalanceCacheInvalidator interface {
	InvalidateUserBalance(context.Context, int64) error
}

// invalidateUserBalanceCache runs synchronously after a committed balance
// change, even when the originating request has already been canceled.
func invalidateUserBalanceCache(ctx context.Context, invalidator UserBalanceCacheInvalidator, userID int64) error {
	if invalidator == nil || userID <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), balanceInvalidationTimeout)
	defer cancel()
	if err := invalidator.InvalidateUserBalance(cacheCtx, userID); err != nil {
		logger.LegacyPrintf("service.billing_cache", "Warning: invalidate committed user balance failed for user %d: %v", userID, err)
		return err
	}
	return nil
}

// invalidateUserBalanceCacheOnCommit must be called before a transaction is
// committed. Invalidation before COMMIT would allow another pod to repopulate
// the old balance while the writer is still uncommitted.
func invalidateUserBalanceCacheOnCommit(ctx context.Context, invalidator UserBalanceCacheInvalidator, userID int64) {
	if invalidator == nil || userID <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		tx.OnCommit(func(next dbent.Committer) dbent.Committer {
			return dbent.CommitFunc(func(commitCtx context.Context, tx *dbent.Tx) error {
				if err := next.Commit(commitCtx, tx); err != nil {
					return err
				}
				invalidateUserBalanceCache(commitCtx, invalidator, userID)
				return nil
			})
		})
		return
	}
	invalidateUserBalanceCache(ctx, invalidator, userID)
}
