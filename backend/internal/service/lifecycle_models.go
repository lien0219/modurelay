package service

import (
	"context"
	"io"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrLifecycleDisabled    = infraerrors.Conflict("LIFECYCLE_DISABLED", "workspace lifecycle operations are disabled")
	ErrLifecycleLeaseLost   = infraerrors.Conflict("LIFECYCLE_LEASE_LOST", "lifecycle job lease was lost")
	ErrLifecycleExportLimit = infraerrors.BadRequest("EXPORT_LIMIT_EXCEEDED", "export exceeds the configured snapshot size, row or time limit")
	ErrLifecycleStorage     = infraerrors.Conflict("EXPORT_STORAGE_UNAVAILABLE", "encrypted tenant export storage is not configured")
	ErrLifecycleBlocked     = infraerrors.Conflict("DELETION_BLOCKED", "workspace cleanup is blocked; inspect deletion preflight reasons")
	ErrLifecycleChallenge   = infraerrors.Conflict("DELETION_CONFIRMATION_INVALID", "deletion confirmation is invalid, expired or already used")
	ErrLifecycleRateLimit   = infraerrors.TooManyRequests("LIFECYCLE_RATE_LIMITED", "please wait before requesting another lifecycle operation")
)

type LifecycleExportJob struct {
	ID                string     `json:"id"`
	WorkspaceID       int64      `json:"workspace_id"`
	RequestedByUserID int64      `json:"requested_by_user_id"`
	State             string     `json:"state"`
	Attempts          int        `json:"attempts"`
	Progress          int64      `json:"progress"`
	ObjectKey         string     `json:"-"`
	LeaseToken        string     `json:"-"`
	LeaseExpiresAt    *time.Time `json:"-"`
	DataCutoff        *time.Time `json:"data_cutoff"`
	ArtifactSHA256    string     `json:"artifact_sha256,omitempty"`
	SizeBytes         int64      `json:"size_bytes"`
	FailureCode       string     `json:"failure_code,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	ExpiresAt         *time.Time `json:"expires_at"`
}

type LifecycleDeletionJob struct {
	ID                        string             `json:"id"`
	WorkspaceID               int64              `json:"workspace_id"`
	RequestedByUserID         int64              `json:"requested_by_user_id"`
	State                     string             `json:"state"`
	PreviousStatus            string             `json:"previous_status"`
	Phase                     string             `json:"phase"`
	Cursor                    int64              `json:"cursor"`
	Attempts                  int                `json:"attempts"`
	LeaseToken                string             `json:"-"`
	LeaseExpiresAt            *time.Time         `json:"-"`
	Progress                  int64              `json:"progress"`
	FailureCode               string             `json:"failure_code,omitempty"`
	BlockingReasons           []LifecycleBlocker `json:"blocking_reasons"`
	ProtectedEvidenceRetained bool               `json:"protected_evidence_retained"`
	BusinessClosed            bool               `json:"business_closed"`
	EarliestPurgeAt           time.Time          `json:"earliest_purge_at"`
	CreatedAt                 time.Time          `json:"created_at"`
	UpdatedAt                 time.Time          `json:"updated_at"`
	CompletedAt               *time.Time         `json:"completed_at"`
}

type LifecycleBlocker struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}
type LifecyclePreflight struct {
	Eligible                  bool               `json:"eligible"`
	BlockingReasons           []LifecycleBlocker `json:"blocking_reasons"`
	ResourceCounts            map[string]int64   `json:"resource_counts"`
	CountsCapped              bool               `json:"counts_capped"`
	ProtectedRecords          map[string]int64   `json:"protected_records"`
	EstimatedPurgeScope       []string           `json:"estimated_purge_scope"`
	RetainedScope             []string           `json:"retained_scope"`
	EarliestPurgeAt           time.Time          `json:"earliest_purge_at"`
	ProtectedEvidenceRetained bool               `json:"protected_evidence_retained"`
}
type LifecycleChallenge struct {
	Token     string              `json:"token"`
	ExpiresAt time.Time           `json:"expires_at"`
	Preflight *LifecyclePreflight `json:"preflight"`
}
type LifecycleDownloadGrant struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}
type LifecycleExportResult struct {
	DataCutoff time.Time
	Records    int64
	SHA256     string
	SizeBytes  int64
}

type WorkspaceLifecycleRepository interface {
	LifecyclePreflight(context.Context, int64, int64) (*LifecyclePreflight, error)
	LifecycleDeletionChallenge(context.Context, int64, int64) (*LifecycleChallenge, error)
	RequestLifecycleDeletion(context.Context, int64, int64, string, string) (*LifecycleDeletionJob, error)
	CancelLifecycleDeletion(context.Context, int64, int64, string) error
	RetryLifecycleDeletion(context.Context, int64, int64, string) (*LifecycleDeletionJob, error)
	GetLifecycleDeletion(context.Context, int64, int64) (*LifecycleDeletionJob, error)
	CreateLifecycleExport(context.Context, int64, int64) (*LifecycleExportJob, error)
	ListLifecycleExports(context.Context, int64, int64, int, int) ([]LifecycleExportJob, int64, error)
	GetLifecycleExport(context.Context, int64, int64, string) (*LifecycleExportJob, error)
	CancelLifecycleExport(context.Context, int64, int64, string) error
	AuthorizeLifecycleDownload(context.Context, int64, int64, string) (*LifecycleDownloadGrant, error)
	RedeemLifecycleDownload(context.Context, int64, int64, string, string) (*LifecycleExportJob, error)
	ClaimLifecycleExport(context.Context) (*LifecycleExportJob, error)
	WriteLifecycleExportSnapshot(context.Context, *LifecycleExportJob, io.Writer) (*LifecycleExportResult, error)
	FinishLifecycleExport(context.Context, *LifecycleExportJob, *LifecycleExportResult, error) error
	CleanupLifecycleExports(context.Context, func(context.Context, string) error) error
	ClaimLifecycleDeletion(context.Context) (*LifecycleDeletionJob, error)
	RunLifecyclePurgeBatch(context.Context, *LifecycleDeletionJob, int) error
	ReleaseLifecycleDeletion(context.Context, *LifecycleDeletionJob, error) error
}

// Enrollment only raises the required proof strength. It never supplies proof.
func RequireLifecycleStrongAuthentication(ctx context.Context, now time.Time) error {
	if err := RequireRecentAuthentication(ctx, now, 10*time.Minute); err != nil {
		return err
	}
	auth, _ := SessionAuthenticationFromContext(ctx)
	proof, ok := RecentAuthenticationProofFromContext(ctx)
	if auth.MFAEnrolled && !auth.MFASatisfied && (!ok || !proof.MFASatisfied || proof.VerifiedAt.Before(now.Add(-10*time.Minute)) || proof.VerifiedAt.After(now.Add(time.Minute))) {
		return ErrMFARequired
	}
	return nil
}
