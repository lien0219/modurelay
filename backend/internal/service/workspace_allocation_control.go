package service

import (
	"context"
	"strings"
	"time"
)

func (s *WorkspaceService) allocationRepo() (WorkspaceAllocationRepository, error) {
	r, ok := s.repo.(WorkspaceAllocationRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	return r, nil
}

func (s *WorkspaceService) ListAllocationCostCenters(ctx context.Context, actor, workspace int64, archived bool) ([]WorkspaceCostCenter, error) {
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "workspace.read"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	return r.ListAllocationCostCenters(ctx, actor, workspace, archived)
}
func (s *WorkspaceService) CreateAllocationCostCenter(ctx context.Context, actor, workspace int64, code, name, description string) (*WorkspaceCostCenter, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if code == "" || name == "" {
		return nil, ErrWorkspaceAllocationInvalid
	}
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "workspace.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.CreateAllocationCostCenter(ctx, actor, workspace, code, name, description)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}
func (s *WorkspaceService) UpdateAllocationCostCenter(ctx context.Context, actor, workspace, id int64, code, name, description string) (*WorkspaceCostCenter, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if id <= 0 || code == "" || name == "" {
		return nil, ErrWorkspaceAllocationInvalid
	}
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "workspace.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.UpdateAllocationCostCenter(ctx, actor, workspace, id, code, name, description)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}
func (s *WorkspaceService) ArchiveAllocationCostCenter(ctx context.Context, actor, workspace, id int64) error {
	if id <= 0 {
		return ErrWorkspaceAllocationInvalid
	}
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "workspace.update"); err != nil {
		return err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return err
	}
	err = r.ArchiveAllocationCostCenter(ctx, actor, workspace, id)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return err
}

func (s *WorkspaceService) ListAllocationTags(ctx context.Context, actor, workspace int64, archived bool) ([]WorkspaceAllocationTag, error) {
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "workspace.read"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	return r.ListAllocationTags(ctx, actor, workspace, archived)
}
func (s *WorkspaceService) CreateAllocationTag(ctx context.Context, actor, workspace int64, key, value, description string) (*WorkspaceAllocationTag, error) {
	key, err := NormalizeAllocationTagKey(key)
	if err != nil {
		return nil, err
	}
	value, err = NormalizeAllocationTagValue(value)
	if err != nil {
		return nil, err
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) > 500 {
		return nil, ErrWorkspaceAllocationInvalid
	}
	if _, err = s.access.RequireWorkspace(ctx, actor, workspace, "workspace.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.CreateAllocationTag(ctx, actor, workspace, key, value, description)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}
func (s *WorkspaceService) UpdateAllocationTag(ctx context.Context, actor, workspace, id int64, key, value, description string) (*WorkspaceAllocationTag, error) {
	if id <= 0 {
		return nil, ErrWorkspaceAllocationInvalid
	}
	key, err := NormalizeAllocationTagKey(key)
	if err != nil {
		return nil, err
	}
	value, err = NormalizeAllocationTagValue(value)
	if err != nil {
		return nil, err
	}
	description = strings.TrimSpace(description)
	if len([]rune(description)) > 500 {
		return nil, ErrWorkspaceAllocationInvalid
	}
	if _, err = s.access.RequireWorkspace(ctx, actor, workspace, "workspace.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.UpdateAllocationTag(ctx, actor, workspace, id, key, value, description)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}
func (s *WorkspaceService) ArchiveAllocationTag(ctx context.Context, actor, workspace, id int64) error {
	if id <= 0 {
		return ErrWorkspaceAllocationInvalid
	}
	if _, err := s.access.RequireWorkspace(ctx, actor, workspace, "workspace.update"); err != nil {
		return err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return err
	}
	err = r.ArchiveAllocationTag(ctx, actor, workspace, id)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return err
}

func (s *WorkspaceService) GetProjectAllocation(ctx context.Context, actor, workspace, project int64) (*ProjectAllocation, error) {
	if _, err := s.access.RequireProject(ctx, actor, workspace, project, "project.read"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	return r.GetProjectAllocation(ctx, actor, workspace, project)
}
func (s *WorkspaceService) SetProjectAllocation(ctx context.Context, actor, workspace, project int64, config AllocationConfig) (*ProjectAllocation, error) {
	normalized, err := config.NormalizeAndValidate()
	if err != nil {
		return nil, err
	}
	if _, err = s.access.RequireProject(ctx, actor, workspace, project, "project.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.SetProjectAllocation(ctx, actor, workspace, project, normalized)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}

func (s *WorkspaceService) GetAPIKeyAllocationOverride(ctx context.Context, actor, workspace, project, key int64) (*APIKeyAllocationOverride, error) {
	if _, err := s.access.RequireProject(ctx, actor, workspace, project, "project.read"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	return r.GetAPIKeyAllocationOverride(ctx, actor, workspace, project, key)
}
func (s *WorkspaceService) SetAPIKeyAllocationOverride(ctx context.Context, actor, workspace, project, key int64, config AllocationConfig) (*APIKeyAllocationOverride, error) {
	normalized, err := config.NormalizeAndValidate()
	if err != nil {
		return nil, err
	}
	if _, err = s.access.RequireProject(ctx, actor, workspace, project, "project.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.SetAPIKeyAllocationOverride(ctx, actor, workspace, project, key, normalized)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}
func (s *WorkspaceService) GetServiceAccountAllocationOverride(ctx context.Context, actor, workspace, project, account int64) (*ServiceAccountAllocationOverride, error) {
	if _, err := s.access.RequireProject(ctx, actor, workspace, project, "project.read"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	return r.GetServiceAccountAllocationOverride(ctx, actor, workspace, project, account)
}
func (s *WorkspaceService) SetServiceAccountAllocationOverride(ctx context.Context, actor, workspace, project, account int64, config AllocationConfig) (*ServiceAccountAllocationOverride, error) {
	normalized, err := config.NormalizeAndValidate()
	if err != nil {
		return nil, err
	}
	if _, err = s.access.RequireProject(ctx, actor, workspace, project, "project.update"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	out, err := r.SetServiceAccountAllocationOverride(ctx, actor, workspace, project, account, normalized)
	if err == nil {
		s.invalidate(ctx, workspace)
	}
	return out, err
}

func (s *WorkspaceService) GetAllocationReport(ctx context.Context, actor int64, filter AllocationFilter) (*AllocationReport, error) {
	filter.Timezone = strings.TrimSpace(filter.Timezone)
	if filter.Environment != "" && filter.Environment != AllocationEnvironmentUnallocated {
		if normalized, err := NormalizeAllocationEnvironment(filter.Environment); err == nil {
			filter.Environment = normalized
		}
	}
	if filter.TagKey != "" {
		if normalized, err := NormalizeAllocationTagKey(filter.TagKey); err == nil {
			filter.TagKey = normalized
		}
	}
	if filter.TagValue != "" {
		if normalized, err := NormalizeAllocationTagValue(filter.TagValue); err == nil {
			filter.TagValue = normalized
		}
	}
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if filter.Timezone == "" {
		filter.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(filter.Timezone); err != nil {
		return nil, ErrWorkspaceAllocationInvalid
	}
	if filter.From != "" || filter.To != "" {
		start, err := time.Parse(time.RFC3339, filter.From)
		if err != nil {
			return nil, ErrWorkspaceAllocationInvalid
		}
		end, err := time.Parse(time.RFC3339, filter.To)
		if err != nil || !end.After(start) || end.Sub(start) > 366*24*time.Hour {
			return nil, ErrWorkspaceAllocationInvalid
		}
	}
	if filter.ProjectID > 0 {
		if _, err := s.access.RequireProject(ctx, actor, filter.WorkspaceID, filter.ProjectID, "usage.read"); err != nil {
			return nil, err
		}
	} else if _, err := s.access.RequireWorkspace(ctx, actor, filter.WorkspaceID, "usage.read"); err != nil {
		return nil, err
	}
	r, err := s.allocationRepo()
	if err != nil {
		return nil, err
	}
	return r.GetAllocationReport(ctx, filter)
}
