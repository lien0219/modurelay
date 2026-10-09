package service

import (
	"context"
	"time"
)

func (s *WorkspaceService) enterpriseAdminRepo() (EnterpriseAdminRepository, error) {
	r, ok := s.repo.(EnterpriseAdminRepository)
	if !ok {
		return nil, ErrWorkspaceInvalid
	}
	return r, nil
}
func (s *WorkspaceService) AdminSearchWorkspaces(ctx context.Context, a int64, f AdminWorkspaceFilter) (*AdminWorkspacePage, error) {
	start := time.Now()
	defer observeAdminDuration("search", start)
	if a <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	if e := f.Validate(); e != nil {
		return nil, e
	}
	r, e := s.enterpriseAdminRepo()
	if e != nil {
		return nil, e
	}
	return r.AdminSearchWorkspaces(ctx, a, f.Normalized())
}
func (s *WorkspaceService) AdminDiagnosticsOverview(ctx context.Context, a int64) (*AdminDiagnosticsOverview, error) {
	start := time.Now()
	defer observeAdminDuration("overview", start)
	if a <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	r, e := s.enterpriseAdminRepo()
	if e != nil {
		return nil, e
	}
	out, e := r.AdminDiagnosticsOverview(ctx, a)
	if e == nil && out != nil && out.Jobs != nil {
		s.appendAdminRotationCapabilities(&out.Jobs.ExportRotation)
		observeAdminWorkers(out.Jobs)
	}
	return out, e
}
func (s *WorkspaceService) AdminWorkspaceDiagnostics(ctx context.Context, a, w int64) (*AdminWorkspaceDiagnostics, error) {
	start := time.Now()
	defer observeAdminDuration("workspace", start)
	if a <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	if w <= 0 {
		return nil, ErrWorkspaceInvalid
	}
	r, e := s.enterpriseAdminRepo()
	if e != nil {
		return nil, e
	}
	out, e := r.AdminWorkspaceDiagnostics(ctx, a, w)
	if e == nil && out != nil && out.Jobs != nil {
		s.appendAdminRotationCapabilities(&out.Jobs.ExportRotation)
	}
	return out, e
}
func (s *WorkspaceService) AdminOperationsJobs(ctx context.Context, a int64) (*AdminJobsDiagnostics, error) {
	start := time.Now()
	defer observeAdminDuration("jobs", start)
	if a <= 0 {
		return nil, ErrWorkspaceForbidden
	}
	r, e := s.enterpriseAdminRepo()
	if e != nil {
		return nil, e
	}
	out, e := r.AdminOperationsJobs(ctx, a)
	if e == nil && out != nil {
		s.appendAdminRotationCapabilities(&out.ExportRotation)
		observeAdminWorkers(out)
	}
	return out, e
}
func (s *WorkspaceService) appendAdminRotationCapabilities(v *AdminExportRotation) {
	v.State = "blocked"
	v.RotationCertified = false
	v.Capabilities = AdminInstanceCapabilities{Available: true, Scope: "current_instance", ExportEnabled: s.lifecycle != nil && s.lifecycle.cfg.Enabled, PurgeEnabled: s.lifecycle != nil && s.lifecycle.cfg.PurgeEnabled, KeyAvailable: s.lifecycle != nil && len(s.lifecycle.key) == 32}
}
