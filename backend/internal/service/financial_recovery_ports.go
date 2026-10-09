package service

import (
	"context"
	"time"
)

type mediaProviderStartContextKey struct{}

// WithMediaProviderStart attaches the durable marker that must succeed before
// an image request crosses the upstream provider boundary. Legacy callers that
// do not provide a marker retain the existing behavior.
func WithMediaProviderStart(ctx context.Context, marker func(context.Context, int64) error) context.Context {
	if marker == nil {
		return ctx
	}
	return context.WithValue(ctx, mediaProviderStartContextKey{}, marker)
}

// Optional ports keep legacy gateway constructors and test doubles compatible.
type FrozenTenantUsageRecovery interface {
	RecoverFrozenTenantUsage(context.Context, int) (int, error)
	ListFrozenUsageCacheInvalidations(context.Context, int) ([]*UsageLog, error)
	AcknowledgeFrozenUsageCacheInvalidation(context.Context, string, int64) error
}

type VideoTaskBindingStore interface {
	BindVideoTaskAccount(context.Context, string, int64, int64, int64, int64) error
	GetVideoTaskAccount(context.Context, string, int64, int64, int64) (int64, error)
}

type MediaAttemptStore interface {
	StartMediaAttempt(context.Context, *GrokVideoPendingBilling) error
}

type MediaAttemptFinalizer interface {
	CompleteMediaAttempt(context.Context, *GrokVideoPendingBilling) error
}

type MediaAttemptRejector interface {
	RejectMediaAttempt(context.Context, string) error
}

type MediaAttemptRecovery interface {
	RecoverMediaAttempts(context.Context, time.Time, int) (int, error)
}

type BatchImageSQLRecovery interface {
	ListRecoverableBatchImageJobs(context.Context, int) ([]*BatchImageJob, error)
}
