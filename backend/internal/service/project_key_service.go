package service

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"strings"
)

// TenantKeyResolver always consults live storage; authentication snapshots are
// hints only and never authorize tenant admission.
type TenantKeyResolver interface {
	ResolveTenant(context.Context, *APIKey) (*TenantContext, error)
}
type ProjectKeyRepository interface {
	TenantKeyResolver
	PersonalProject(context.Context, int64) (int64, int64, error)
	ProjectForKey(context.Context, int64) (int64, int64, error)
	ListProjectKeys(context.Context, int64, int64, int64, pagination.PaginationParams) ([]APIKey, int64, error)
	GetProjectKey(context.Context, int64, int64, int64, int64) (*APIKey, error)
	WithProjectKeyMutation(context.Context, int64, int64, int64, string, func(context.Context) error) error
	ListWorkspaceKeySecrets(context.Context, int64) ([]string, error)
}

type projectKeyScope struct {
	ActorID, WorkspaceID, ProjectID, PayerID int64
	Permission                               string
	TargetID                                 int64
}
type projectKeyScopeContextKey struct{}

// ProjectKeyScopeFromContext is consumed by persistence to constrain writes.
// Only service-authorized operations construct this context value.
func ProjectKeyScopeFromContext(ctx context.Context) (actor, workspace, project int64, permission string, ok bool) {
	s, ok := ctx.Value(projectKeyScopeContextKey{}).(*projectKeyScope)
	if !ok {
		return 0, 0, 0, "", false
	}
	return s.ActorID, s.WorkspaceID, s.ProjectID, s.Permission, true
}
func (s *APIKeyService) SetTenantResolver(r TenantKeyResolver) { s.tenantResolver = r }
func (s *APIKeyService) SetEnterpriseIdentityService(identity *EnterpriseIdentityService) {
	s.enterpriseIdentity = identity
}
func (s *APIKeyService) SetBudgetService(b *BudgetService) { s.budgetService = b }
func (s *APIKeyService) BudgetService() *BudgetService     { return s.budgetService }
func (s *APIKeyService) ConfigureWorkspaces(r WorkspaceRepository) {
	s.workspaceRepo = r
	s.workspaceAccess = NewWorkspaceAccessService(r)
	if resolver, ok := s.apiKeyRepo.(TenantKeyResolver); ok {
		s.SetTenantResolver(resolver)
	}
}
func (s *APIKeyService) RevalidateTenant(ctx context.Context, k *APIKey) error {
	if s.tenantResolver == nil {
		return nil
	}
	if k == nil {
		return ErrAPIKeyNotFound
	}
	tenant, e := s.tenantResolver.ResolveTenant(ctx, k)
	if e != nil {
		return e
	}
	if tenant == nil {
		return ErrWorkspaceForbidden
	}
	k.Tenant = tenant
	s.compileAPIKeyIPRules(k)
	// The principal is fresh even when the key and actor came from L1/Redis.
	if s.userRepo != nil {
		principal, e := s.userRepo.GetByID(ctx, tenant.BillingPrincipalUserID)
		if e != nil {
			return e
		}
		if !principal.IsActive() {
			return ErrWorkspaceForbidden
		}
		k.BillingPrincipal = principal
		if k.GroupID != nil {
			group := k.Group
			if group == nil || group.ID != *k.GroupID || !group.IsActive() {
				return ErrGroupNotAllowed
			}
			if !s.canUserBindGroup(ctx, principal, group) || !tenant.AllowsGroup(group.ID) {
				return ErrGroupNotAllowed
			}
		} else {
			k.Group = nil
		}
	}
	return nil
}

// TenantAdmissionSnapshot never mutates a key retained by a previous WS turn or
// detached usage callback. Consumers must retain the returned value for their
// own turn; client tenant fields never participate in this resolution.
func (s *APIKeyService) TenantAdmissionSnapshot(ctx context.Context, k *APIKey) (*APIKey, error) {
	if k == nil {
		return nil, ErrAPIKeyNotFound
	}
	copyKey := *k
	if k.Tenant != nil {
		tenant := *k.Tenant
		tenant.AllowedModels = appendNilStrings(k.Tenant.AllowedModels)
		if k.Tenant.AllowedGroupIDs != nil {
			tenant.AllowedGroupIDs = append([]int64{}, k.Tenant.AllowedGroupIDs...)
		}
		if k.Tenant.Allocation != nil {
			allocation := cloneAllocationSnapshot(*k.Tenant.Allocation)
			tenant.Allocation = &allocation
		}
		copyKey.Tenant = &tenant
	}
	if e := s.RevalidateTenant(ctx, &copyKey); e != nil {
		return nil, e
	}
	return &copyKey, nil
}

// VideoPendingTenantSnapshot restores the immutable tenant and payer captured
// at async task creation. It intentionally does not call live tenant resolution
// because a billing-owner change after creation must not reassign the task.
func (s *APIKeyService) VideoPendingTenantSnapshot(ctx context.Context, pending *GrokVideoPendingBilling, k *APIKey) (*APIKey, error) {
	if pending == nil || k == nil {
		return nil, ErrAPIKeyNotFound
	}
	if pending.WorkspaceID <= 0 || pending.ProjectID <= 0 || pending.BillingPrincipalUserID <= 0 {
		if k.ProjectID != nil || k.Tenant != nil {
			return nil, ErrWorkspaceForbidden
		}
		copyKey := *k
		return &copyKey, nil
	}
	copyKey := pending.ApplyTenantSnapshot(k)
	if copyKey == nil || copyKey.Tenant == nil {
		return nil, ErrWorkspaceForbidden
	}
	if s == nil || s.userRepo == nil {
		return nil, ErrWorkspaceForbidden
	}
	payer, err := s.userRepo.GetByID(ctx, pending.BillingPrincipalUserID)
	// This is settlement of previously admitted work, not new admission. The
	// original user row funds its frozen reservation after a safe owner transfer,
	// even if that user subsequently becomes inactive. Pending-work lifecycle
	// guards block deletion until settlement. SQL billing still verifies the
	// complete immutable reservation and usage identity.
	if err != nil || payer == nil || payer.ID != pending.BillingPrincipalUserID {
		if err != nil {
			return nil, err
		}
		return nil, ErrWorkspaceForbidden
	}
	copyKey.BillingPrincipal = payer
	if copyKey.Tenant.Allocation != nil {
		allocation := cloneAllocationSnapshot(*copyKey.Tenant.Allocation)
		copyKey.Tenant.Allocation = &allocation
	}
	return copyKey, nil
}
func appendNilStrings(in []string) []string {
	if in == nil {
		return nil
	}
	return append([]string{}, in...)
}
func (t *TenantContext) AllowsGroup(id int64) bool {
	if t == nil || t.AllowedGroupIDs == nil {
		return true
	}
	for _, x := range t.AllowedGroupIDs {
		if x == id {
			return true
		}
	}
	return false
}
func (k *APIKey) AllowsModel(model string) bool {
	if k == nil {
		return true
	}
	if k.Group != nil && k.Group.ModelAllowlistEnabled() && !k.Group.ModelAllowlist.Allows(model) {
		return false
	}
	if k.Tenant == nil || k.Tenant.AllowedModels == nil {
		return true
	}
	for _, m := range k.Tenant.AllowedModels {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(m), "models/"), strings.TrimPrefix(strings.TrimSpace(model), "models/")) {
			return true
		}
	}
	return false
}
func (k *APIKey) HasModelRestrictions() bool {
	return k != nil && ((k.Group != nil && k.Group.ModelAllowlistEnabled()) || (k.Tenant != nil && k.Tenant.AllowedModels != nil))
}
func (k *APIKey) FilterModels(models []string) []string {
	out := []string{}
	for _, m := range models {
		if k.AllowsModel(m) {
			out = append(out, m)
		}
	}
	return out
}
func maskProjectKey(k *APIKey) {
	if k == nil {
		return
	}
	if k.ServiceAccountID != nil {
		k.Key = "****"
		if k.KeySuffix != nil {
			k.Key += *k.KeySuffix
		}
		return
	}
	secret := k.Key
	k.Key = "****"
	if len(secret) > 4 {
		k.Key = "****" + secret[len(secret)-4:]
	}
}
func (s *APIKeyService) projectRepository() (ProjectKeyRepository, error) {
	r, ok := s.apiKeyRepo.(ProjectKeyRepository)
	if !ok || s.workspaceAccess == nil {
		return nil, fmt.Errorf("project keys are not configured")
	}
	return r, nil
}
func (s *APIKeyService) projectScope(ctx context.Context, a, w, p int64, permission string) (context.Context, *WorkspaceAccess, error) {
	if s.workspaceAccess == nil {
		return ctx, nil, ErrWorkspaceForbidden
	}
	ac, e := s.workspaceAccess.RequireProject(ctx, a, w, p, permission)
	if e != nil {
		return ctx, nil, e
	}
	// This is a human management scope, including the legacy /keys routes.
	// Gateway key authentication uses RevalidateTenant and never enters it.
	if s.enterpriseIdentity != nil {
		assurance, _ := AuthenticationAssuranceFromContext(ctx)
		if e = s.enterpriseIdentity.CheckWorkspaceAccess(ctx, w, ac.Workspace.Type, PrincipalHuman, assurance); e != nil {
			return ctx, nil, e
		}
	}
	return context.WithValue(ctx, projectKeyScopeContextKey{}, &projectKeyScope{ActorID: a, WorkspaceID: w, ProjectID: p, PayerID: ac.Workspace.BillingOwnerUserID, Permission: permission}), ac, nil
}
func (s *APIKeyService) CreateForProject(ctx context.Context, a, w, p int64, req CreateAPIKeyRequest) (*APIKey, error) {
	r, e := s.projectRepository()
	if e != nil {
		return nil, e
	}
	ctx, _, e = s.projectScope(ctx, a, w, p, "key.create")
	if e != nil {
		return nil, e
	}
	var key *APIKey
	e = r.WithProjectKeyMutation(ctx, a, w, p, "key.create", func(txctx context.Context) error { var err error; key, err = s.create(txctx, a, req); return err })
	if e == nil && key != nil {
		s.InvalidateAuthCacheByKey(ctx, key.Key)
	}
	// Keep the generated secret for the one-time creation response. List/get/
	// update paths mask their returned keys below.
	return key, e
}
func (s *APIKeyService) ListForProject(ctx context.Context, a, w, p int64, params pagination.PaginationParams) ([]APIKey, int64, error) {
	r, e := s.projectRepository()
	if e != nil {
		return nil, 0, e
	}
	if _, _, e = s.projectScope(ctx, a, w, p, "key.read"); e != nil {
		return nil, 0, e
	}
	keys, total, e := r.ListProjectKeys(ctx, a, w, p, params)
	for i := range keys {
		maskProjectKey(&keys[i])
	}
	return keys, total, e
}
func (s *APIKeyService) GetForProject(ctx context.Context, a, w, p, id int64) (*APIKey, error) {
	r, e := s.projectRepository()
	if e != nil {
		return nil, e
	}
	if _, _, e = s.projectScope(ctx, a, w, p, "key.read"); e != nil {
		return nil, e
	}
	k, e := r.GetProjectKey(ctx, a, w, p, id)
	maskProjectKey(k)
	return k, e
}
func (s *APIKeyService) UpdateForProject(ctx context.Context, a, w, p, id int64, req UpdateAPIKeyRequest) (*APIKey, error) {
	r, e := s.projectRepository()
	if e != nil {
		return nil, e
	}
	ctx, _, e = s.projectScope(ctx, a, w, p, "key.update")
	if e != nil {
		return nil, e
	}
	var k *APIKey
	e = r.WithProjectKeyMutation(ctx, a, w, p, "key.update", func(txctx context.Context) error {
		if _, err := r.GetProjectKey(txctx, a, w, p, id); err != nil {
			return err
		}
		var err error
		k, err = s.update(txctx, id, a, req)
		return err
	})
	if e == nil && k != nil {
		s.InvalidateAuthCacheByKey(ctx, k.Key)
	}
	maskProjectKey(k)
	return k, e
}
func (s *APIKeyService) DeleteForProject(ctx context.Context, a, w, p, id int64) error {
	r, e := s.projectRepository()
	if e != nil {
		return e
	}
	ctx, _, e = s.projectScope(ctx, a, w, p, "key.revoke")
	if e != nil {
		return e
	}
	var secret string
	e = r.WithProjectKeyMutation(ctx, a, w, p, "key.revoke", func(txctx context.Context) error {
		key, err := r.GetProjectKey(txctx, a, w, p, id)
		if err != nil {
			return err
		}
		secret = key.Key
		return s.delete(txctx, id, a)
	})
	if e == nil {
		s.InvalidateAuthCacheByKey(ctx, secret)
	}
	return e
}
func (s *APIKeyService) GetAvailableGroupsForProject(ctx context.Context, a, w, p int64) ([]Group, error) {
	_, ac, e := s.projectScope(ctx, a, w, p, "key.create")
	if e != nil {
		return nil, e
	}
	groups, e := s.GetAvailableGroups(ctx, ac.Workspace.BillingOwnerUserID)
	if e != nil {
		return nil, e
	}
	out := []Group{}
	tenant := &TenantContext{AllowedGroupIDs: ac.Project.AllowedGroupIDs}
	for _, g := range groups {
		if tenant.AllowsGroup(g.ID) {
			out = append(out, g)
		}
	}
	return out, nil
}
func (s *APIKeyService) legacyKeyScope(ctx context.Context, a, id int64, permission string) (context.Context, bool, error) {
	if s.workspaceRepo == nil {
		return ctx, false, nil
	}
	r, e := s.projectRepository()
	if e != nil {
		return ctx, false, e
	}
	w, p, e := r.ProjectForKey(ctx, id)
	if e != nil {
		return ctx, false, e
	}
	if p == 0 {
		return ctx, false, nil
	}
	ctx, ac, e := s.projectScope(ctx, a, w, p, permission)
	if e != nil {
		return ctx, false, e
	}
	return ctx, ac.Workspace.Type == "organization", nil
}
func (s *APIKeyService) GetForUser(ctx context.Context, a, id int64) (*APIKey, error) {
	k, e := s.GetByID(ctx, id)
	if e != nil {
		return nil, e
	}
	if k.ServiceAccountID != nil || k.UserID != a {
		return nil, ErrAPIKeyNotFound
	}
	_, org, e := s.legacyKeyScope(ctx, a, id, "key.read")
	if e != nil {
		return nil, e
	}
	if org {
		maskProjectKey(k)
	}
	return k, nil
}
func (s *APIKeyService) InvalidateWorkspaceAuth(ctx context.Context, w int64) {
	if r, ok := s.apiKeyRepo.(ProjectKeyRepository); ok {
		keys, e := r.ListWorkspaceKeySecrets(ctx, w)
		if e == nil {
			for _, key := range keys {
				s.InvalidateAuthCacheByKey(ctx, key)
			}
		}
	}
}

type tenantKeyReadContextKey struct{}

func TenantKeyReadsEnabled(ctx context.Context) bool {
	enabled, _ := ctx.Value(tenantKeyReadContextKey{}).(bool)
	return enabled
}
func (s *APIKeyService) tenantKeyReadContext(ctx context.Context) context.Context {
	if s.workspaceRepo == nil {
		return ctx
	}
	return context.WithValue(ctx, tenantKeyReadContextKey{}, true)
}

func ProjectKeyMutationTargetFromContext(ctx context.Context) int64 {
	scope, ok := ctx.Value(projectKeyScopeContextKey{}).(*projectKeyScope)
	if !ok {
		return 0
	}
	return scope.TargetID
}
