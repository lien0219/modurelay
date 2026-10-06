package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrWorkspaceNotFound  = infraerrors.NotFound("WORKSPACE_NOT_FOUND", "workspace resource not found")
	ErrWorkspaceForbidden = infraerrors.Forbidden("WORKSPACE_FORBIDDEN", "workspace permission denied")
	ErrWorkspaceConflict  = infraerrors.Conflict("WORKSPACE_CONFLICT", "workspace resource conflicts with existing state or ownership obligations")
	ErrWorkspaceInvalid   = infraerrors.BadRequest("WORKSPACE_INVALID", "invalid workspace input")
)

type Workspace struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	ProjectAccessMode  string    `json:"project_access_mode"`
	OwnerUserID        int64     `json:"owner_user_id"`
	BillingOwnerUserID int64     `json:"billing_owner_user_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
	Permissions        []string  `json:"permissions"`
}
type WorkspaceMember struct {
	ID              int64     `json:"id"`
	WorkspaceID     int64     `json:"workspace_id"`
	UserID          int64     `json:"user_id"`
	Role            string    `json:"role"`
	Status          string    `json:"status"`
	InvitedByUserID *int64    `json:"invited_by_user_id,omitempty"`
	JoinedAt        time.Time `json:"joined_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type WorkspaceInvitation struct {
	ID              int64      `json:"id"`
	WorkspaceID     int64      `json:"workspace_id"`
	Email           string     `json:"email"`
	Role            string     `json:"role"`
	InvitedByUserID int64      `json:"invited_by_user_id"`
	ExpiresAt       time.Time  `json:"expires_at"`
	AcceptedAt      *time.Time `json:"accepted_at"`
	RevokedAt       *time.Time `json:"revoked_at"`
	CreatedAt       time.Time  `json:"created_at"`
}
type Project struct {
	ID              int64  `json:"id"`
	WorkspaceID     int64  `json:"workspace_id"`
	Name            string `json:"name"`
	Slug            string `json:"slug"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	IsDefault       bool   `json:"is_default"`
	CreatedByUserID int64  `json:"created_by_user_id"`
	// nil means unrestricted; a non-nil empty slice denies everything.
	AllowedGroupIDs []int64   `json:"allowed_group_ids"`
	AllowedModels   []string  `json:"allowed_models"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type TenantContext struct {
	WorkspaceID            int64    `json:"workspace_id"`
	ProjectID              int64    `json:"project_id"`
	BillingPrincipalUserID int64    `json:"billing_principal_user_id"`
	BudgetReservationID    string   `json:"budget_reservation_id,omitempty"`
	AllowedGroupIDs        []int64  `json:"allowed_group_ids"`
	AllowedModels          []string `json:"allowed_models"`
}
type WorkspaceAccess struct {
	Workspace   *Workspace       `json:"workspace"`
	Member      *WorkspaceMember `json:"member"`
	Project     *Project         `json:"project,omitempty"`
	ProjectRole string           `json:"project_role,omitempty"`
	// ProjectPermissions is derived from the member's direct or team grant in
	// the same tenant-locked access snapshot. It is never accepted from a
	// caller and is intentionally omitted for workspace-only access.
	ProjectPermissions []string `json:"project_permissions,omitempty"`
	Permissions        []string `json:"permissions"`
}

const (
	ProjectAccessModeAllProjects = "all_projects"
	ProjectAccessModeAssigned    = "assigned_projects"
	ProjectAccessRoleViewer      = "viewer"
	ProjectAccessRoleDeveloper   = "developer"
	ProjectAccessRoleAdmin       = "admin"
	ProjectAccessSubjectMember   = "member"
	ProjectAccessSubjectTeam     = "team"
)

type WorkspaceTeam struct {
	ID          int64     `json:"id"`
	WorkspaceID int64     `json:"workspace_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkspaceTeamMember struct {
	TeamID            int64     `json:"team_id"`
	WorkspaceMemberID int64     `json:"workspace_member_id"`
	UserID            int64     `json:"user_id"`
	Role              string    `json:"role"`
	Status            string    `json:"status"`
	Email             string    `json:"email,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type ProjectAccessGrant struct {
	ID              int64     `json:"id"`
	WorkspaceID     int64     `json:"workspace_id"`
	ProjectID       int64     `json:"project_id"`
	SubjectType     string    `json:"subject_type"`
	SubjectID       int64     `json:"subject_id"`
	Role            string    `json:"role"`
	CreatedByUserID int64     `json:"created_by_user_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type WorkspaceTeamInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

type ProjectAccessGrantInput struct {
	SubjectType string `json:"subject_type"`
	SubjectID   int64  `json:"subject_id"`
	Role        string `json:"role"`
}
type WorkspaceAudit struct {
	ID          int64          `json:"id"`
	WorkspaceID int64          `json:"workspace_id"`
	ProjectID   *int64         `json:"project_id"`
	ActorUserID int64          `json:"actor_user_id"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type"`
	TargetID    int64          `json:"target_id"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}
type ProjectInput struct {
	Name            string   `json:"name"`
	Slug            string   `json:"slug"`
	Description     string   `json:"description"`
	AllowedGroupIDs []int64  `json:"allowed_group_ids"`
	AllowedModels   []string `json:"allowed_models"`
}

// WorkspaceMutation is a closed command set. Repositories validate and authorize
// it again while holding the workspace row lock, before making any change.
// TokenHash is SHA256, never the invitation credential itself.
type WorkspaceMutation struct {
	Action                          string
	TargetID                        int64
	Name, Slug, Role, Status, Email string
	ProjectAccessMode               string
	SubjectType                     string
	SubjectID                       int64
	ProjectID                       int64
	Project                         ProjectInput
	Team                            WorkspaceTeamInput
	Grant                           ProjectAccessGrantInput
	TokenHash                       []byte
	ExpiresAt                       time.Time
}
type WorkspaceMutationResult struct {
	Workspace  *Workspace
	Project    *Project
	Invitation *WorkspaceInvitation
	Team       *WorkspaceTeam
	Grant      *ProjectAccessGrant
}
type WorkspaceRepository interface {
	EnsurePersonalWorkspace(context.Context, int64) (*Workspace, error)
	Bootstrap(context.Context, int) error
	GetAccess(context.Context, int64, int64, int64) (*WorkspaceAccess, error)
	ListWorkspaces(context.Context, int64, pagination.PaginationParams) ([]Workspace, int64, error)
	CreateOrganization(context.Context, int64, string, string) (*Workspace, error)
	Mutate(context.Context, int64, int64, WorkspaceMutation) (*WorkspaceMutationResult, error)
	AcceptInvitation(context.Context, int64, []byte) (*Workspace, error)
	ListProjects(context.Context, int64, int64, pagination.PaginationParams) ([]Project, int64, error)
	ListMembers(context.Context, int64, int64, pagination.PaginationParams) ([]WorkspaceMember, int64, error)
	ListInvitations(context.Context, int64, int64, pagination.PaginationParams) ([]WorkspaceInvitation, int64, error)
	ListAudit(context.Context, int64, int64, pagination.PaginationParams) ([]WorkspaceAudit, int64, error)
	AdminList(context.Context, int64, pagination.PaginationParams) ([]Workspace, int64, error)
	AdminInspect(context.Context, int64, int64) (*Workspace, error)
	AdminSetStatus(context.Context, int64, int64, string) error
}

// WorkspaceGovernanceRepository is optional so existing focused test doubles
// and legacy integrations remain source-compatible while the production
// repository exposes Phase A control-plane reads.
type WorkspaceGovernanceRepository interface {
	ListTeams(context.Context, int64, int64, pagination.PaginationParams) ([]WorkspaceTeam, int64, error)
	ListTeamMembers(context.Context, int64, int64, int64, pagination.PaginationParams) ([]WorkspaceTeamMember, int64, error)
	ListProjectAccessGrants(context.Context, int64, int64, int64, pagination.PaginationParams) ([]ProjectAccessGrant, int64, error)
}

// WorkspaceAdminLifecycleGuard is optionally implemented by UserRepository so
// admin deletion can fail before enumerating/tombstoning API keys.
type WorkspaceAdminLifecycleGuard interface {
	GuardWorkspaceUserDeletion(context.Context, int64) error
}

var workspaceSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

func ValidateWorkspaceNameSlug(name, slug string) error {
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 100 || !workspaceSlug.MatchString(slug) {
		return ErrWorkspaceInvalid
	}
	return nil
}

func ValidateOrganizationNameSlug(name, slug string) error {
	if strings.HasPrefix(slug, "personal-") {
		return ErrWorkspaceInvalid
	}
	return ValidateWorkspaceNameSlug(name, slug)
}
func ValidateProjectInput(p ProjectInput) error {
	if err := ValidateWorkspaceNameSlug(p.Name, p.Slug); err != nil {
		return err
	}
	if utf8.RuneCountInString(p.Description) > 2000 || len(p.AllowedGroupIDs) > 1000 || len(p.AllowedModels) > 1000 {
		return ErrWorkspaceInvalid
	}
	for _, id := range p.AllowedGroupIDs {
		if id <= 0 {
			return ErrWorkspaceInvalid
		}
	}
	for _, m := range p.AllowedModels {
		if strings.TrimSpace(m) == "" || len(m) > 255 {
			return ErrWorkspaceInvalid
		}
	}
	return nil
}

func ValidateWorkspaceTeamInput(t WorkspaceTeamInput) error {
	if err := ValidateWorkspaceNameSlug(t.Name, t.Slug); err != nil {
		return err
	}
	if utf8.RuneCountInString(t.Description) > 2000 {
		return ErrWorkspaceInvalid
	}
	return nil
}

func ValidateProjectAccessGrantInput(g ProjectAccessGrantInput) error {
	if !ValidProjectAccessRole(g.Role) || (g.SubjectType != ProjectAccessSubjectMember && g.SubjectType != ProjectAccessSubjectTeam) || g.SubjectID <= 0 {
		return ErrWorkspaceInvalid
	}
	return nil
}
