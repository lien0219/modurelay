package service

import (
	"context"
	"strings"
	"time"
)

func (s *WorkspaceService) anomalyRepository() (FinOpsAnomalyRepository, error) {
	r, ok := s.repo.(FinOpsAnomalyRepository)
	if !ok || r == nil {
		return nil, ErrWorkspaceConflict
	}
	return r, nil
}

func (s *WorkspaceService) ListFinOpsAnomalies(ctx context.Context, actorID, workspaceID, projectID int64, filter FinOpsAnomalyFilter) ([]FinOpsAnomalyFinding, int64, error) {
	if err := filter.Validate(); err != nil {
		return nil, 0, err
	}
	filter = filter.Normalized()
	scope := FinOpsScope{WorkspaceID: workspaceID, ProjectID: projectID}
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "finops_anomaly.read"); err != nil {
			return nil, 0, err
		}
	} else {
		access, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "finops_anomaly.read")
		if err != nil {
			return nil, 0, err
		}
		if access.Workspace != nil && access.Workspace.ProjectAccessMode == ProjectAccessModeAssigned &&
			access.Member != nil && access.Member.Role != "owner" && access.Member.Role != "admin" && access.Member.Role != "billing" {
			scope.WorkspaceOnly = true
		}
	}
	r, err := s.anomalyRepository()
	if err != nil {
		return nil, 0, err
	}
	return r.ListFinOpsAnomalies(ctx, scope, filter)
}

func (s *WorkspaceService) GetFinOpsAnomaly(ctx context.Context, actorID, workspaceID, projectID, anomalyID int64) (*FinOpsAnomalyFinding, error) {
	scope := FinOpsScope{WorkspaceID: workspaceID, ProjectID: projectID}
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "finops_anomaly.read"); err != nil {
			return nil, err
		}
	} else {
		access, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "finops_anomaly.read")
		if err != nil {
			return nil, err
		}
		if access.Workspace != nil && access.Workspace.ProjectAccessMode == ProjectAccessModeAssigned &&
			access.Member != nil && access.Member.Role != "owner" && access.Member.Role != "admin" && access.Member.Role != "billing" {
			scope.WorkspaceOnly = true
		}
	}
	r, err := s.anomalyRepository()
	if err != nil {
		return nil, err
	}
	return r.GetFinOpsAnomaly(ctx, scope, anomalyID)
}

func (s *WorkspaceService) TransitionFinOpsAnomaly(ctx context.Context, actorID, workspaceID, projectID, anomalyID int64, patch FinOpsAnomalyPatch) (*FinOpsAnomalyFinding, error) {
	if patch.Status != AnomalyStatusAcknowledged && patch.Status != AnomalyStatusResolved {
		return nil, ErrFinOpsAnomalyInvalidTransition
	}
	if patch.ExpectedVersion <= 0 || len([]rune(strings.TrimSpace(patch.ResolutionReason))) > 1000 {
		return nil, ErrWorkspaceInvalid
	}
	if patch.Status == AnomalyStatusResolved && strings.TrimSpace(patch.ResolutionReason) == "" {
		return nil, ErrWorkspaceInvalid
	}
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "finops_anomaly.manage"); err != nil {
			return nil, err
		}
	} else if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "finops_anomaly.manage"); err != nil {
		return nil, err
	}
	r, err := s.anomalyRepository()
	if err != nil {
		return nil, err
	}
	return r.TransitionFinOpsAnomaly(ctx, actorID, FinOpsScope{WorkspaceID: workspaceID, ProjectID: projectID}, anomalyID, patch)
}

func (s *WorkspaceService) GetFinOpsAnomalyStatus(ctx context.Context, actorID, workspaceID int64) (*FinOpsAnomalyDetectorStatus, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "finops_anomaly.read"); err != nil {
		return nil, err
	}
	r, err := s.anomalyRepository()
	if err != nil {
		return nil, err
	}
	return r.GetFinOpsAnomalyStatus(ctx)
}

func (s *WorkspaceService) StartAnomalyWorker() {
	if s == nil || s.anomalyWorker != nil {
		return
	}
	r, ok := s.repo.(FinOpsAnomalyRepository)
	if !ok || r == nil {
		return
	}
	s.anomalyWorker = NewFinOpsAnomalyWorker(r, DefaultFinOpsAnomalyConfig(), 5*time.Minute)
	s.anomalyWorker.Start()
}

func (s *WorkspaceService) StopAnomalyWorker() {
	if s == nil || s.anomalyWorker == nil {
		return
	}
	s.anomalyWorker.Stop()
	s.anomalyWorker = nil
}
