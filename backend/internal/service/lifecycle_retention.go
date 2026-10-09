package service

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var ErrLifecycleRetentionInvalid = infraerrors.BadRequest("LIFECYCLE_RETENTION_INVALID", "retention must respect the platform floor and protected records")

type LifecycleRetentionPolicy struct {
	Category      string `json:"category"`
	RetentionDays int    `json:"retention_days"`
	MinimumDays   int    `json:"minimum_days"`
	Protected     bool   `json:"protected"`
}

// Optional so legacy integrations and focused Workspace test doubles stay
// source compatible; production implements both live authorization checks.
type WorkspaceLifecycleRetentionRepository interface {
	LifecycleRetention(context.Context, int64, int64) ([]LifecycleRetentionPolicy, error)
	UpdateLifecycleRetention(context.Context, int64, int64, string, int) ([]LifecycleRetentionPolicy, error)
}

func ValidLifecycleRetentionCategory(category string) bool {
	switch category {
	case "operational", "financial", "audit", "security", "temporary":
		return true
	default:
		return false
	}
}

func (s *WorkspaceService) LifecycleRetention(ctx context.Context, actor, workspace int64) ([]LifecycleRetentionPolicy, error) {
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "lifecycle.read"); err != nil {
		return nil, err
	}
	repo, ok := s.repo.(WorkspaceLifecycleRetentionRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	return repo.LifecycleRetention(ctx, actor, workspace)
}

func (s *WorkspaceService) UpdateLifecycleRetention(ctx context.Context, actor, workspace int64, category string, days int) ([]LifecycleRetentionPolicy, error) {
	if !ValidLifecycleRetentionCategory(category) || days < 0 || days > 36500 {
		return nil, ErrLifecycleRetentionInvalid
	}
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "lifecycle.manage"); err != nil {
		return nil, err
	}
	repo, ok := s.repo.(WorkspaceLifecycleRetentionRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	return repo.UpdateLifecycleRetention(ctx, actor, workspace, category, days)
}
