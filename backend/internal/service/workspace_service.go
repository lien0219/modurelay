package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"log/slog"
	"net/mail"
	"strings"
	"sync"
	"time"
)

type WorkspaceService struct {
	repo            WorkspaceRepository
	access          *WorkspaceAccessService
	keyInvalidator  interface{ InvalidateWorkspaceAuth(context.Context, int64) }
	bootstrapCancel context.CancelFunc
	bootstrapWG     sync.WaitGroup
}

func NewWorkspaceService(repo WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{repo: repo, access: NewWorkspaceAccessService(repo)}
}

// Production DI owns the worker; direct service construction remains free of
// background activity for tools and existing tests.
func ProvideWorkspaceService(repo WorkspaceRepository) *WorkspaceService {
	s := NewWorkspaceService(repo)
	s.StartBootstrapWorker()
	return s
}
func (s *WorkspaceService) SetKeyInvalidator(k interface{ InvalidateWorkspaceAuth(context.Context, int64) }) {
	s.keyInvalidator = k
}
func (s *WorkspaceService) invalidate(ctx context.Context, w int64) {
	if s.keyInvalidator != nil {
		s.keyInvalidator.InvalidateWorkspaceAuth(ctx, w)
	}
}
func (s *WorkspaceService) StartBootstrapWorker() {
	ctx, cancel := context.WithCancel(context.Background())
	s.bootstrapCancel = cancel
	s.bootstrapWG.Add(1)
	go func() {
		defer s.bootstrapWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			batchCtx, batchCancel := context.WithTimeout(ctx, 20*time.Second)
			err := s.Bootstrap(batchCtx, 100)
			batchCancel()
			if err != nil && ctx.Err() == nil {
				slog.Warn("workspace bootstrap batch failed", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (s *WorkspaceService) StopBootstrapWorker() {
	if s.bootstrapCancel != nil {
		s.bootstrapCancel()
	}
	s.bootstrapWG.Wait()
}
func (s *WorkspaceService) EnsurePersonalWorkspace(ctx context.Context, userID int64) (*Workspace, error) {
	return s.repo.EnsurePersonalWorkspace(ctx, userID)
}

// Bootstrap processes at most batchSize old users (at most 100 keys per user)
// and one additional batchSize NULL-key page. Each transaction is bounded; callers
// may repeat it at startup or on an operational schedule until migration finishes.
func (s *WorkspaceService) Bootstrap(ctx context.Context, batchSize int) error {
	return s.repo.Bootstrap(ctx, batchSize)
}
func (s *WorkspaceService) ListWorkspaces(ctx context.Context, actorID int64, p pagination.PaginationParams) ([]Workspace, int64, error) {
	if _, e := s.EnsurePersonalWorkspace(ctx, actorID); e != nil {
		return nil, 0, e
	}
	return s.repo.ListWorkspaces(ctx, actorID, p)
}
func (s *WorkspaceService) GetWorkspace(ctx context.Context, actorID, workspaceID int64) (*Workspace, error) {
	a, e := s.access.RequireWorkspace(ctx, actorID, workspaceID, "workspace.read")
	if e != nil {
		return nil, e
	}
	return a.Workspace, nil
}
func (s *WorkspaceService) CreateOrganization(ctx context.Context, actorID int64, name, slug string) (*Workspace, error) {
	if e := ValidateOrganizationNameSlug(name, slug); e != nil {
		return nil, e
	}
	return s.repo.CreateOrganization(ctx, actorID, name, slug)
}
func (s *WorkspaceService) mutate(ctx context.Context, actorID, workspaceID int64, m WorkspaceMutation) (*WorkspaceMutationResult, error) {
	permission := WorkspaceMutationPermission(m)
	var e error
	if (m.Action == "project.update" || m.Action == "project.archive") && m.TargetID > 0 {
		_, e = s.access.RequireProject(ctx, actorID, workspaceID, m.TargetID, permission)
	} else if _, e = s.access.RequireWorkspace(ctx, actorID, workspaceID, permission); e != nil {
		return nil, e
	}
	if e != nil {
		return nil, e
	}
	r, e := s.repo.Mutate(ctx, actorID, workspaceID, m)
	if e == nil {
		s.invalidate(ctx, workspaceID)
	}
	return r, e
}
func (s *WorkspaceService) UpdateWorkspace(ctx context.Context, actorID, workspaceID int64, name, slug string) (*Workspace, error) {
	if e := ValidateWorkspaceNameSlug(name, slug); e != nil {
		return nil, e
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "workspace.update", Name: name, Slug: slug})
	if e != nil {
		return nil, e
	}
	return r.Workspace, nil
}
func (s *WorkspaceService) ArchiveWorkspace(ctx context.Context, actorID, workspaceID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "workspace.archive"})
	return e
}
func (s *WorkspaceService) CreateProject(ctx context.Context, actorID, workspaceID int64, p ProjectInput) (*Project, error) {
	if e := ValidateProjectInput(p); e != nil {
		return nil, e
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "project.create", Project: p})
	if e != nil {
		return nil, e
	}
	return r.Project, nil
}
func (s *WorkspaceService) UpdateProject(ctx context.Context, actorID, workspaceID, projectID int64, p ProjectInput) (*Project, error) {
	if e := ValidateProjectInput(p); e != nil {
		return nil, e
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "project.update", TargetID: projectID, Project: p})
	if e != nil {
		return nil, e
	}
	return r.Project, nil
}
func (s *WorkspaceService) GetProject(ctx context.Context, actorID, workspaceID, projectID int64) (*Project, error) {
	a, e := s.access.RequireProject(ctx, actorID, workspaceID, projectID, "project.read")
	if e != nil {
		return nil, e
	}
	return a.Project, nil
}
func (s *WorkspaceService) ArchiveProject(ctx context.Context, actorID, workspaceID, projectID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "project.archive", TargetID: projectID})
	return e
}

func (s *WorkspaceService) SetProjectAccessMode(ctx context.Context, actorID, workspaceID int64, mode string) (*Workspace, error) {
	if !ValidProjectAccessMode(mode) {
		return nil, ErrWorkspaceInvalid
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "workspace.project_access_mode.update", ProjectAccessMode: mode})
	if e != nil {
		return nil, e
	}
	return r.Workspace, nil
}

func (s *WorkspaceService) governanceRepo() (WorkspaceGovernanceRepository, error) {
	r, ok := s.repo.(WorkspaceGovernanceRepository)
	if !ok {
		return nil, ErrWorkspaceNotFound
	}
	return r, nil
}

func (s *WorkspaceService) ListTeams(ctx context.Context, actorID, workspaceID int64, p pagination.PaginationParams) ([]WorkspaceTeam, int64, error) {
	r, e := s.governanceRepo()
	if e != nil {
		return nil, 0, e
	}
	return r.ListTeams(ctx, actorID, workspaceID, p)
}

func (s *WorkspaceService) CreateTeam(ctx context.Context, actorID, workspaceID int64, input WorkspaceTeamInput) (*WorkspaceTeam, error) {
	if e := ValidateWorkspaceTeamInput(input); e != nil {
		return nil, e
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "team.create", Team: input})
	if e != nil {
		return nil, e
	}
	return r.Team, nil
}

func (s *WorkspaceService) UpdateTeam(ctx context.Context, actorID, workspaceID, teamID int64, input WorkspaceTeamInput) (*WorkspaceTeam, error) {
	if e := ValidateWorkspaceTeamInput(input); e != nil {
		return nil, e
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "team.update", TargetID: teamID, Team: input})
	if e != nil {
		return nil, e
	}
	return r.Team, nil
}

func (s *WorkspaceService) ArchiveTeam(ctx context.Context, actorID, workspaceID, teamID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "team.archive", TargetID: teamID})
	return e
}

func (s *WorkspaceService) ListTeamMembers(ctx context.Context, actorID, workspaceID, teamID int64, p pagination.PaginationParams) ([]WorkspaceTeamMember, int64, error) {
	r, e := s.governanceRepo()
	if e != nil {
		return nil, 0, e
	}
	return r.ListTeamMembers(ctx, actorID, workspaceID, teamID, p)
}

func (s *WorkspaceService) AddTeamMember(ctx context.Context, actorID, workspaceID, teamID, memberID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "team.member.add", TargetID: teamID, SubjectID: memberID})
	return e
}

func (s *WorkspaceService) RemoveTeamMember(ctx context.Context, actorID, workspaceID, teamID, memberID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "team.member.remove", TargetID: teamID, SubjectID: memberID})
	return e
}

func (s *WorkspaceService) ListProjectAccessGrants(ctx context.Context, actorID, workspaceID, projectID int64, p pagination.PaginationParams) ([]ProjectAccessGrant, int64, error) {
	r, e := s.governanceRepo()
	if e != nil {
		return nil, 0, e
	}
	return r.ListProjectAccessGrants(ctx, actorID, workspaceID, projectID, p)
}

func (s *WorkspaceService) CreateProjectAccessGrant(ctx context.Context, actorID, workspaceID, projectID int64, input ProjectAccessGrantInput) (*ProjectAccessGrant, error) {
	if projectID <= 0 || ValidateProjectAccessGrantInput(input) != nil {
		return nil, ErrWorkspaceInvalid
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "project_access.grant.create", ProjectID: projectID, Grant: input})
	if e != nil {
		return nil, e
	}
	return r.Grant, nil
}

func (s *WorkspaceService) UpdateProjectAccessGrant(ctx context.Context, actorID, workspaceID, projectID, grantID int64, input ProjectAccessGrantInput) (*ProjectAccessGrant, error) {
	if projectID <= 0 || grantID <= 0 || ValidateProjectAccessGrantInput(input) != nil {
		return nil, ErrWorkspaceInvalid
	}
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "project_access.grant.update", TargetID: grantID, ProjectID: projectID, Grant: input})
	if e != nil {
		return nil, e
	}
	return r.Grant, nil
}

func (s *WorkspaceService) DeleteProjectAccessGrant(ctx context.Context, actorID, workspaceID, projectID, grantID int64) error {
	if projectID <= 0 || grantID <= 0 {
		return ErrWorkspaceInvalid
	}
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "project_access.grant.delete", TargetID: grantID, ProjectID: projectID})
	return e
}
func (s *WorkspaceService) UpdateMember(ctx context.Context, actorID, workspaceID, userID int64, role, status string) error {
	if !ValidWorkspaceRole(role) || (status != "active" && status != "suspended") {
		return ErrWorkspaceInvalid
	}
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "member.update", TargetID: userID, Role: role, Status: status})
	return e
}
func (s *WorkspaceService) RemoveMember(ctx context.Context, actorID, workspaceID, userID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "member.remove", TargetID: userID})
	return e
}
func (s *WorkspaceService) ChangeBillingOwner(ctx context.Context, actorID, workspaceID, userID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "billing.owner.update", TargetID: userID})
	return e
}
func NormalizeWorkspaceInvitationEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	a, e := mail.ParseAddress(email)
	if e != nil || a.Address != email || len(email) > 320 {
		return "", ErrWorkspaceInvalid
	}
	return email, nil
}
func (s *WorkspaceService) CreateInvitation(ctx context.Context, actorID, workspaceID int64, email, role string, ttl time.Duration) (*WorkspaceInvitation, string, error) {
	email, e := NormalizeWorkspaceInvitationEmail(email)
	if e != nil {
		return nil, "", e
	}
	if !ValidWorkspaceRole(role) || ttl <= 0 || ttl > 30*24*time.Hour {
		return nil, "", ErrWorkspaceInvalid
	}
	raw := make([]byte, 32)
	if _, e = rand.Read(raw); e != nil {
		return nil, "", e
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	r, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "member.invite", Email: email, Role: role, TokenHash: hash[:], ExpiresAt: time.Now().UTC().Add(ttl)})
	if e != nil {
		return nil, "", e
	}
	return r.Invitation, token, nil
}
func (s *WorkspaceService) RevokeInvitation(ctx context.Context, actorID, workspaceID, invitationID int64) error {
	_, e := s.mutate(ctx, actorID, workspaceID, WorkspaceMutation{Action: "member.invite", TargetID: invitationID})
	return e
}
func (s *WorkspaceService) AcceptInvitation(ctx context.Context, actorID int64, token string) (*Workspace, error) {
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(raw) != 32 {
		return nil, ErrWorkspaceNotFound
	}
	h := sha256.Sum256([]byte(token))
	w, e := s.repo.AcceptInvitation(ctx, actorID, h[:])
	if e == nil {
		s.invalidate(ctx, w.ID)
	}
	return w, e
}
func (s *WorkspaceService) ListProjects(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]Project, int64, error) {
	return s.repo.ListProjects(ctx, a, w, p)
}
func (s *WorkspaceService) ListMembers(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]WorkspaceMember, int64, error) {
	return s.repo.ListMembers(ctx, a, w, p)
}
func (s *WorkspaceService) ListInvitations(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]WorkspaceInvitation, int64, error) {
	return s.repo.ListInvitations(ctx, a, w, p)
}
func (s *WorkspaceService) ListAudit(ctx context.Context, a, w int64, p pagination.PaginationParams) ([]WorkspaceAudit, int64, error) {
	return s.repo.ListAudit(ctx, a, w, p)
}
func (s *WorkspaceService) AdminList(ctx context.Context, a int64, p pagination.PaginationParams) ([]Workspace, int64, error) {
	return s.repo.AdminList(ctx, a, p)
}
func (s *WorkspaceService) AdminInspect(ctx context.Context, a, w int64) (*Workspace, error) {
	return s.repo.AdminInspect(ctx, a, w)
}
func (s *WorkspaceService) AdminSetStatus(ctx context.Context, a, w int64, status string) error {
	e := s.repo.AdminSetStatus(ctx, a, w, status)
	if e == nil {
		s.invalidate(ctx, w)
	}
	return e
}
