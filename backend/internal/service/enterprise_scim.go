package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

const (
	SCIMUserSchema  = "urn:ietf:params:scim:schemas:core:2.0:User"
	SCIMGroupSchema = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SCIMPatchSchema = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SCIMListSchema  = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SCIMErrorSchema = "urn:ietf:params:scim:api:messages:2.0:Error"
)

type SCIMError struct {
	Status int
	Type   string
	Detail string
}

func (e *SCIMError) Error() string { return fmt.Sprintf("SCIM %d: %s", e.Status, e.Detail) }
func NewSCIMError(status int, scimType, detail string) error {
	return &SCIMError{Status: status, Type: scimType, Detail: detail}
}

type SCIMPrincipal struct {
	WorkspaceID, ConnectorID, TokenID, ConnectorRevision int64
	PublicID                                             string
}
type SCIMConnector struct {
	ID            int64      `json:"id"`
	WorkspaceID   int64      `json:"workspace_id"`
	Revision      int64      `json:"revision"`
	PublicID      string     `json:"public_endpoint_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	DefaultRole   string     `json:"default_role"`
	GroupMode     string     `json:"group_mode"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DisabledAt    *time.Time `json:"disabled_at,omitempty"`
	LastSyncAt    *time.Time `json:"last_sync_at,omitempty"`
	LastErrorCode string     `json:"last_error_code,omitempty"`
	FailureCount  int        `json:"failure_count"`
}
type SCIMConnectorInput struct {
	Name        string `json:"name"`
	DefaultRole string `json:"default_role"`
	Revision    int64  `json:"revision"`
}
type SCIMToken struct {
	ID          int64      `json:"id"`
	ConnectorID int64      `json:"connector_id"`
	Prefix      string     `json:"token_prefix"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
}
type SCIMTokenInput struct {
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
type SCIMTokenResult struct {
	Token  *SCIMToken `json:"token"`
	Secret string     `json:"secret"`
}
type SCIMName struct {
	GivenName  string `json:"givenName,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
}
type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type,omitempty"`
	Primary bool   `json:"primary"`
	Display string `json:"display,omitempty"`
}
type SCIMMember struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
}
type SCIMMeta struct {
	ResourceType string    `json:"resourceType"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
	Version      string    `json:"version"`
	Location     string    `json:"location"`
}
type SCIMUserInput struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id,omitempty"`
	ExternalID  string      `json:"externalId,omitempty"`
	UserName    string      `json:"userName"`
	Active      *bool       `json:"active,omitempty"`
	Name        SCIMName    `json:"name"`
	DisplayName string      `json:"displayName,omitempty"`
	Emails      []SCIMEmail `json:"emails"`
}
type SCIMUser struct {
	Schemas     []string    `json:"schemas"`
	ID          string      `json:"id"`
	ExternalID  string      `json:"externalId,omitempty"`
	UserName    string      `json:"userName"`
	Active      bool        `json:"active"`
	Name        SCIMName    `json:"name"`
	DisplayName string      `json:"displayName,omitempty"`
	Emails      []SCIMEmail `json:"emails"`
	Meta        SCIMMeta    `json:"meta"`
	WorkspaceID int64       `json:"-"`
	ConnectorID int64       `json:"-"`
	UserID      int64       `json:"-"`
	MemberID    int64       `json:"-"`
	Revision    int64       `json:"-"`
	Deleted     bool        `json:"-"`
}
type SCIMGroupInput struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id,omitempty"`
	ExternalID  string       `json:"externalId,omitempty"`
	DisplayName string       `json:"displayName"`
	Members     []SCIMMember `json:"members"`
}
type SCIMGroup struct {
	Schemas     []string     `json:"schemas"`
	ID          string       `json:"id"`
	ExternalID  string       `json:"externalId,omitempty"`
	DisplayName string       `json:"displayName"`
	Members     []SCIMMember `json:"members"`
	Meta        SCIMMeta     `json:"meta"`
	WorkspaceID int64        `json:"-"`
	ConnectorID int64        `json:"-"`
	Revision    int64        `json:"-"`
	TeamID      *int64       `json:"-"`
	Deleted     bool         `json:"-"`
}
type SCIMPatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path,omitempty"`
	Value json.RawMessage `json:"value,omitempty"`
}
type SCIMPatchRequest struct {
	Schemas    []string             `json:"schemas"`
	Operations []SCIMPatchOperation `json:"Operations"`
}
type SCIMListQuery struct {
	Attribute, Value  string
	StartIndex, Count int
}
type SCIMUserMutation struct {
	Action  string
	User    *SCIMUserInput
	Patch   *SCIMPatchRequest
	IfMatch int64
}
type SCIMGroupMutation struct {
	Action  string
	Group   *SCIMGroupInput
	Patch   *SCIMPatchRequest
	IfMatch int64
}
type SCIMGroupBinding struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	ExternalID  string `json:"external_id,omitempty"`
	Revision    int64  `json:"revision"`
	TeamID      *int64 `json:"team_id"`
}
type SCIMGroupBindingInput struct {
	Revision int64  `json:"revision"`
	TeamID   *int64 `json:"team_id"`
}

type SCIMRepository interface {
	ListConnectors(context.Context, int64, int64) ([]SCIMConnector, error)
	CreateConnector(context.Context, int64, int64, SCIMConnectorInput) (*SCIMConnector, error)
	UpdateConnector(context.Context, int64, int64, int64, SCIMConnectorInput) (*SCIMConnector, error)
	DisableConnector(context.Context, int64, int64, int64, int64) error
	ListTokens(context.Context, int64, int64, int64) ([]SCIMToken, error)
	CreateToken(context.Context, int64, int64, int64, string, []byte, *time.Time) (*SCIMToken, error)
	RevokeToken(context.Context, int64, int64, int64, int64) error
	ListGroupBindings(context.Context, int64, int64, int64) ([]SCIMGroupBinding, error)
	BindGroup(context.Context, int64, int64, int64, string, SCIMGroupBindingInput) error
	Authenticate(context.Context, string, []byte, time.Time) (*SCIMPrincipal, error)
	GetUser(context.Context, *SCIMPrincipal, string) (*SCIMUser, error)
	ListUsers(context.Context, *SCIMPrincipal, SCIMListQuery) ([]SCIMUser, int, error)
	MutateUser(context.Context, *SCIMPrincipal, string, SCIMUserMutation) (*SCIMUser, error)
	GetGroup(context.Context, *SCIMPrincipal, string) (*SCIMGroup, error)
	ListGroups(context.Context, *SCIMPrincipal, SCIMListQuery) ([]SCIMGroup, int, error)
	MutateGroup(context.Context, *SCIMPrincipal, string, SCIMGroupMutation) (*SCIMGroup, error)
	RecordSyncOutcome(context.Context, *SCIMPrincipal, bool, string) error
}
type EnterpriseSCIMService struct {
	repo SCIMRepository
	now  func() time.Time
}

func NewEnterpriseSCIMService(repo SCIMRepository) *EnterpriseSCIMService {
	return &EnterpriseSCIMService{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}
func (s *EnterpriseSCIMService) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}
func validateSCIMConnector(input SCIMConnectorInput) (SCIMConnectorInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.DefaultRole == "" {
		input.DefaultRole = WorkspaceRoleViewer
	}
	if input.Name == "" || len(input.Name) > 120 || !validOIDCManagedRole(input.DefaultRole) || strings.ContainsAny(input.Name, "\x00\r\n") {
		return input, NewSCIMError(400, "invalidValue", "invalid connector configuration")
	}
	return input, nil
}
func (s *EnterpriseSCIMService) ListConnectors(ctx context.Context, w, a int64) ([]SCIMConnector, error) {
	return s.repo.ListConnectors(ctx, w, a)
}
func (s *EnterpriseSCIMService) CreateConnector(ctx context.Context, w, a int64, in SCIMConnectorInput) (*SCIMConnector, error) {
	in, e := validateSCIMConnector(in)
	if e != nil {
		return nil, e
	}
	return s.repo.CreateConnector(ctx, w, a, in)
}
func (s *EnterpriseSCIMService) UpdateConnector(ctx context.Context, w, a, c int64, in SCIMConnectorInput) (*SCIMConnector, error) {
	in, e := validateSCIMConnector(in)
	if e != nil {
		return nil, e
	}
	if in.Revision <= 0 {
		return nil, NewSCIMError(400, "invalidValue", "revision required")
	}
	return s.repo.UpdateConnector(ctx, w, a, c, in)
}
func (s *EnterpriseSCIMService) DisableConnector(ctx context.Context, w, a, c, r int64) error {
	if r <= 0 {
		return NewSCIMError(400, "invalidValue", "revision required")
	}
	return s.repo.DisableConnector(ctx, w, a, c, r)
}
func (s *EnterpriseSCIMService) ListTokens(ctx context.Context, w, a, c int64) ([]SCIMToken, error) {
	return s.repo.ListTokens(ctx, w, a, c)
}
func (s *EnterpriseSCIMService) CreateToken(ctx context.Context, w, a, c int64, in SCIMTokenInput) (*SCIMTokenResult, error) {
	if in.ExpiresAt != nil && (!in.ExpiresAt.After(s.now()) || in.ExpiresAt.After(s.now().Add(366*24*time.Hour))) {
		return nil, NewSCIMError(400, "invalidValue", "invalid token expiry")
	}
	raw, _, e := NewSecureToken(32)
	if e != nil {
		return nil, e
	}
	secret := "mrc_scim_" + raw
	token, e := s.repo.CreateToken(ctx, w, a, c, secret[:17], HashEnterpriseToken(secret), in.ExpiresAt)
	if e != nil {
		return nil, e
	}
	return &SCIMTokenResult{Token: token, Secret: secret}, nil
}
func (s *EnterpriseSCIMService) RevokeToken(ctx context.Context, w, a, c, t int64) error {
	return s.repo.RevokeToken(ctx, w, a, c, t)
}
func (s *EnterpriseSCIMService) ListGroupBindings(ctx context.Context, w, a, c int64) ([]SCIMGroupBinding, error) {
	return s.repo.ListGroupBindings(ctx, w, a, c)
}
func (s *EnterpriseSCIMService) BindGroup(ctx context.Context, w, a, c int64, id string, in SCIMGroupBindingInput) error {
	if !ValidSCIMResourceID(id) || in.Revision <= 0 || in.TeamID != nil && *in.TeamID <= 0 {
		return NewSCIMError(400, "invalidValue", "invalid group binding")
	}
	return s.repo.BindGroup(ctx, w, a, c, id, in)
}
func (s *EnterpriseSCIMService) Authenticate(ctx context.Context, id, bearer string) (*SCIMPrincipal, error) {
	if !ValidSCIMResourceID(id) || len(bearer) != 52 || !strings.HasPrefix(bearer, "mrc_scim_") || !ValidSCIMResourceID(bearer[9:]) {
		return nil, NewSCIMError(401, "", "invalid provisioning credential")
	}
	return s.repo.Authenticate(ctx, id, HashEnterpriseToken(bearer), s.now())
}
func (s *EnterpriseSCIMService) GetUser(ctx context.Context, p *SCIMPrincipal, id string) (*SCIMUser, error) {
	return s.repo.GetUser(ctx, p, id)
}
func (s *EnterpriseSCIMService) ListUsers(ctx context.Context, p *SCIMPrincipal, q SCIMListQuery) ([]SCIMUser, int, error) {
	return s.repo.ListUsers(ctx, p, q)
}
func (s *EnterpriseSCIMService) MutateUser(ctx context.Context, p *SCIMPrincipal, id string, m SCIMUserMutation) (*SCIMUser, error) {
	return s.repo.MutateUser(ctx, p, id, m)
}
func (s *EnterpriseSCIMService) GetGroup(ctx context.Context, p *SCIMPrincipal, id string) (*SCIMGroup, error) {
	return s.repo.GetGroup(ctx, p, id)
}
func (s *EnterpriseSCIMService) ListGroups(ctx context.Context, p *SCIMPrincipal, q SCIMListQuery) ([]SCIMGroup, int, error) {
	return s.repo.ListGroups(ctx, p, q)
}
func (s *EnterpriseSCIMService) MutateGroup(ctx context.Context, p *SCIMPrincipal, id string, m SCIMGroupMutation) (*SCIMGroup, error) {
	return s.repo.MutateGroup(ctx, p, id, m)
}
func (s *EnterpriseSCIMService) RecordSyncOutcome(ctx context.Context, p *SCIMPrincipal, success bool, code string) error {
	return s.repo.RecordSyncOutcome(ctx, p, success, code)
}
func ValidSCIMResourceID(id string) bool {
	if len(id) != 43 {
		return false
	}
	for _, r := range id {
		if (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
func scimStringValid(value string, max int) bool {
	return len(value) <= max && !strings.ContainsAny(value, "\x00\r\n")
}
func ValidateSCIMUserInput(in *SCIMUserInput) (string, error) {
	if in == nil || len(in.Schemas) != 1 || in.Schemas[0] != SCIMUserSchema || strings.TrimSpace(in.UserName) == "" || !scimStringValid(in.UserName, 255) || !scimStringValid(in.ExternalID, 255) || !scimStringValid(in.Name.GivenName, 255) || !scimStringValid(in.Name.FamilyName, 255) || !scimStringValid(in.DisplayName, 512) || len(in.Emails) == 0 || len(in.Emails) > 20 {
		return "", NewSCIMError(400, "invalidValue", "invalid user attributes")
	}
	primary := ""
	count := 0
	for _, email := range in.Emails {
		value := strings.ToLower(strings.TrimSpace(email.Value))
		parsed, e := mail.ParseAddress(value)
		if e != nil || parsed.Address != value || len(value) > 255 || !scimStringValid(email.Display, 512) || !scimStringValid(email.Type, 80) {
			return "", NewSCIMError(400, "invalidValue", "invalid email attributes")
		}
		if email.Primary {
			primary = value
			count++
		}
	}
	if count == 0 && len(in.Emails) == 1 {
		primary = strings.ToLower(strings.TrimSpace(in.Emails[0].Value))
		count = 1
	}
	if count != 1 {
		return "", NewSCIMError(400, "invalidValue", "one primary email required")
	}
	return primary, nil
}
func ValidateSCIMGroupInput(in *SCIMGroupInput) error {
	if in == nil || len(in.Schemas) != 1 || in.Schemas[0] != SCIMGroupSchema || strings.TrimSpace(in.DisplayName) == "" || !scimStringValid(in.DisplayName, 255) || !scimStringValid(in.ExternalID, 255) || len(in.Members) > 5000 {
		return NewSCIMError(400, "invalidValue", "invalid group attributes")
	}
	seen := map[string]bool{}
	for _, m := range in.Members {
		if !ValidSCIMResourceID(m.Value) || !scimStringValid(m.Display, 512) || !scimStringValid(m.Ref, 2048) || seen[m.Value] {
			return NewSCIMError(400, "invalidValue", "invalid group member")
		}
		seen[m.Value] = true
	}
	return nil
}
