//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type userBalanceInvalidatorRecorder struct {
	users []int64
}

func (r *userBalanceInvalidatorRecorder) InvalidateUserBalance(ctx context.Context, userID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.users = append(r.users, userID)
	return nil
}

func TestUserAuthInvalidationAlsoFencesBalanceWithoutAPIKeys(t *testing.T) {
	for _, lookupErr := range []error{nil, errors.New("key lookup unavailable")} {
		cache := &userBalanceInvalidatorRecorder{}
		repo := &authRepoStub{listKeysByUserID: func(context.Context, int64) ([]string, error) { return nil, lookupErr }}
		svc := &APIKeyService{apiKeyRepo: repo}
		svc.SetBalanceCacheInvalidator(cache)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		svc.InvalidateAuthCacheByUserID(ctx, 42)
		require.Equal(t, []int64{42}, cache.users, "balance invalidation must survive cancellation and failed/empty key lookups")
	}
}

func TestUsageCacheInvalidationCascadesToBalanceFence(t *testing.T) {
	cache := &userBalanceInvalidatorRecorder{}
	keys := &APIKeyService{
		apiKeyRepo:          &authRepoStub{listKeysByUserID: func(context.Context, int64) ([]string, error) { return nil, nil }},
		balanceCacheInvalid: cache,
	}
	usage := &UsageService{authCacheInvalidator: keys}
	usage.invalidateUsageCaches(context.Background(), 42, true)
	usage.invalidateUsageCaches(context.Background(), 43, false)
	require.Equal(t, []int64{42}, cache.users)
}

func TestUsageBalanceInvalidationWaitsForOuterTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer client.Close()
	cache := &userBalanceInvalidatorRecorder{}
	keys := &APIKeyService{
		apiKeyRepo:          &authRepoStub{listKeysByUserID: func(context.Context, int64) ([]string, error) { return nil, nil }},
		balanceCacheInvalid: cache,
	}
	usage := &UsageService{authCacheInvalidator: keys}
	mock.ExpectBegin()
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	usage.invalidateUsageCaches(dbent.NewTxContext(context.Background(), tx), 42, true)
	require.Empty(t, cache.users)
	mock.ExpectCommit()
	require.NoError(t, tx.Commit())
	require.Equal(t, []int64{42}, cache.users)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchImageCacheInvalidationCascadesToBalanceFence(t *testing.T) {
	cache := &userBalanceInvalidatorRecorder{}
	keys := &APIKeyService{
		apiKeyRepo:          &authRepoStub{listKeysByUserID: func(context.Context, int64) ([]string, error) { return nil, nil }},
		balanceCacheInvalid: cache,
	}
	(&BatchImagePublicService{AuthCache: keys}).invalidateAuthCache(context.Background(), 42)
	(&BatchImageSettlementService{AuthCache: keys}).invalidateAuthCache(context.Background(), 43)
	require.Equal(t, []int64{42, 43}, cache.users)
}

func TestRefundBalanceMutationInvalidatesAfterCommit(t *testing.T) {
	for _, end := range []string{"commit", "rollback", "commit_failure"} {
		t.Run(end, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			defer client.Close()
			cache := &userBalanceInvalidatorRecorder{}
			svc := &PaymentService{userRepo: &mockUserRepo{}, balanceCacheInvalidator: cache}
			mock.ExpectBegin()
			tx, err := client.Tx(context.Background())
			require.NoError(t, err)
			deducted, err := svc.deductAvailableBalance(dbent.NewTxContext(context.Background(), tx), 42, 3)
			require.NoError(t, err)
			require.Equal(t, 3.0, deducted)
			require.Empty(t, cache.users, "a pre-commit invalidation lets another pod refill the old balance")
			switch end {
			case "commit":
				mock.ExpectCommit()
				require.NoError(t, tx.Commit())
				require.Equal(t, []int64{42}, cache.users)
			case "rollback":
				mock.ExpectRollback()
				require.NoError(t, tx.Rollback())
				require.Empty(t, cache.users)
			case "commit_failure":
				mock.ExpectCommit().WillReturnError(errors.New("commit failed"))
				require.Error(t, tx.Commit())
				require.Empty(t, cache.users)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRefundDirectDeductionAndRollbackInvalidateBalance(t *testing.T) {
	cache := &userBalanceInvalidatorRecorder{}
	svc := &PaymentService{userRepo: &mockUserRepo{}, balanceCacheInvalidator: cache}
	_, err := svc.deductAvailableBalance(context.Background(), 42, 3)
	require.NoError(t, err)
	require.Equal(t, []int64{42}, cache.users)
	require.True(t, svc.RollbackRefund(context.Background(), &RefundPlan{
		DeductionType: payment.DeductionTypeBalance, BalanceToDeduct: 3, Order: &dbent.PaymentOrder{UserID: 42},
	}, errors.New("gateway rejected refund")))
	require.Equal(t, []int64{42, 42}, cache.users)
}
