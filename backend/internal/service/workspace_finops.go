package service

import (
	"context"
	"time"
)

type FinOpsScope struct {
	WorkspaceID      int64
	ProjectID        int64
	ServiceAccountID int64
	// WorkspaceOnly is used by anomaly reads for assigned-project members. It
	// keeps workspace-level findings visible while excluding project findings
	// that the caller has not been granted.
	WorkspaceOnly bool
}

type FinOpsUsageSummary struct {
	WorkspaceID int64     `json:"workspace_id"`
	ProjectID   int64     `json:"project_id,omitempty"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Timezone    string    `json:"timezone"`
	Requests    int64     `json:"requests"`
	Spend       float64   `json:"spend"`
	Reserved    float64   `json:"reserved"`
	Members     int64     `json:"members"`
	Projects    int64     `json:"projects"`
	APIKeys     int64     `json:"api_keys"`
}

type FinOpsBreakdown struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Requests int64   `json:"requests"`
	Spend    float64 `json:"spend"`
}

type FinOpsDailySpendPoint struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Spend    float64 `json:"spend"`
}

type FinOpsOverview struct {
	ServiceAccounts []FinOpsServiceAccountBreakdown `json:"service_accounts"`
	Summary         FinOpsUsageSummary              `json:"summary"`
	DailySpend      []FinOpsDailySpendPoint         `json:"daily_spend"`
	Projects        []FinOpsBreakdown               `json:"projects"`
	Platforms       []FinOpsBreakdown               `json:"platforms"`
	Models          []FinOpsBreakdown               `json:"models"`
	APIKeys         []FinOpsBreakdown               `json:"api_keys"`
}
type FinOpsServiceAccountBreakdown struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Requests int64    `json:"requests"`
	Spend    float64  `json:"spend"`
	Tokens   int64    `json:"tokens"`
	Models   []string `json:"models"`
}

type BudgetView struct {
	WorkspaceID int64        `json:"workspace_id"`
	ProjectID   int64        `json:"project_id,omitempty"`
	Policy      BudgetPolicy `json:"policy"`
	PeriodStart time.Time    `json:"period_start"`
	PeriodEnd   time.Time    `json:"period_end"`
	Spent       float64      `json:"spent"`
	Reserved    float64      `json:"reserved"`
	Remaining   float64      `json:"remaining"`
	OverBudget  bool         `json:"over_budget"`
}

type BudgetPolicyInput struct {
	Amount    float64 `json:"amount"`
	HardLimit bool    `json:"hard_limit"`
	Enabled   bool    `json:"enabled"`
	Timezone  string  `json:"timezone"`
}

type FinOpsRepository interface {
	GetUsageSummary(context.Context, FinOpsScope, time.Time, time.Time, string) (*FinOpsUsageSummary, error)
	GetOverview(context.Context, FinOpsScope, time.Time, time.Time, string) (*FinOpsOverview, error)
	GetBudget(context.Context, FinOpsScope) (*BudgetView, error)
	SetBudget(context.Context, int64, FinOpsScope, BudgetPolicyInput) (*BudgetView, error)
}

func (s *WorkspaceService) GetUsageSummary(ctx context.Context, actorID, workspaceID, projectID int64, start, end time.Time, timezone string, serviceAccountIDs ...int64) (*FinOpsUsageSummary, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, ErrWorkspaceInvalid
	}
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "usage.read"); err != nil {
			return nil, err
		}
	} else if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "usage.read"); err != nil {
		return nil, err
	}
	r, ok := s.repo.(FinOpsRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	scope, err := s.serviceAccountFinOpsScope(ctx, actorID, workspaceID, projectID, serviceAccountIDs)
	if err != nil {
		return nil, err
	}
	return r.GetUsageSummary(ctx, scope, start, end, timezone)
}

func (s *WorkspaceService) GetOverview(ctx context.Context, actorID, workspaceID, projectID int64, start, end time.Time, timezone string, serviceAccountIDs ...int64) (*FinOpsOverview, error) {
	if timezone == "" {
		timezone = "UTC"
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, ErrWorkspaceInvalid
	}
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "usage.read"); err != nil {
			return nil, err
		}
	} else if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "usage.read"); err != nil {
		return nil, err
	}
	r, ok := s.repo.(FinOpsRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	scope, err := s.serviceAccountFinOpsScope(ctx, actorID, workspaceID, projectID, serviceAccountIDs)
	if err != nil {
		return nil, err
	}
	return r.GetOverview(ctx, scope, start, end, timezone)
}

func (s *WorkspaceService) serviceAccountFinOpsScope(ctx context.Context, actor, workspace, project int64, ids []int64) (FinOpsScope, error) {
	scope := FinOpsScope{WorkspaceID: workspace, ProjectID: project}
	if len(ids) == 0 {
		return scope, nil
	}
	if len(ids) != 1 || ids[0] <= 0 {
		return scope, ErrWorkspaceNotFound
	}
	r, ok := s.repo.(interface {
		ValidateServiceAccountUsageScope(context.Context, int64, int64, int64, int64) error
	})
	if !ok {
		return scope, ErrWorkspaceForbidden
	}
	if err := r.ValidateServiceAccountUsageScope(ctx, actor, workspace, project, ids[0]); err != nil {
		return scope, err
	}
	scope.ServiceAccountID = ids[0]
	return scope, nil
}

func (s *WorkspaceService) GetBudget(ctx context.Context, actorID, workspaceID, projectID int64) (*BudgetView, error) {
	permission := "budget.read"
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, permission); err != nil {
			return nil, err
		}
	} else if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, permission); err != nil {
		return nil, err
	}
	r, ok := s.repo.(FinOpsRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	return r.GetBudget(ctx, FinOpsScope{WorkspaceID: workspaceID, ProjectID: projectID})
}

func (s *WorkspaceService) SetBudget(ctx context.Context, actorID, workspaceID, projectID int64, in BudgetPolicyInput) (*BudgetView, error) {
	if in.Amount < 0 || len(in.Timezone) > 128 {
		return nil, ErrWorkspaceInvalid
	}
	if in.Timezone == "" {
		in.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		return nil, ErrWorkspaceInvalid
	}
	if projectID > 0 {
		if _, err := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "budget.update"); err != nil {
			return nil, err
		}
	} else if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "budget.update"); err != nil {
		return nil, err
	}
	r, ok := s.repo.(FinOpsRepository)
	if !ok {
		return nil, ErrWorkspaceConflict
	}
	view, err := r.SetBudget(ctx, actorID, FinOpsScope{WorkspaceID: workspaceID, ProjectID: projectID}, in)
	if err == nil {
		s.invalidate(ctx, workspaceID)
	}
	return view, err
}
