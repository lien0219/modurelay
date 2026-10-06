package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// PolicyScope identifies a layer in the admission hierarchy. A lower layer
// can only narrow the set and numeric limits selected by a parent layer.
type PolicyScope string

const (
	PolicyScopeGroup          PolicyScope = "group"
	PolicyScopeWorkspace      PolicyScope = "workspace"
	PolicyScopeProject        PolicyScope = "project"
	PolicyScopeServiceAccount PolicyScope = "service_account"
	PolicyScopeCredential     PolicyScope = "credential"
)

var (
	ErrInvalidPolicy          = errors.New("invalid policy")
	ErrPolicyNotFound         = errors.New("policy not found")
	ErrPolicyRevisionConflict = errors.New("policy revision conflict")
)

// PolicyRef is the stable storage key for one policy scope. Scope IDs are
// tenant-owned IDs; callers must resolve and authorize them before storage.
type PolicyRef struct {
	Scope   PolicyScope `json:"scope"`
	ScopeID int64       `json:"scope_id"`
}

// Policy stores restrictions for one scope. A nil allowlist means this scope
// inherits its parent's value. A non-nil empty allowlist denies every value.
// Numeric limits use nil for inheritance and positive values for a limit; zero
// is invalid so it cannot be confused with either unlimited or deny-all.
type Policy struct {
	Scope               PolicyScope `json:"scope"`
	ScopeID             int64       `json:"scope_id"`
	Revision            int64       `json:"revision"`
	AllowedModels       []string    `json:"allowed_models"`
	AllowedPlatforms    []string    `json:"allowed_platforms"`
	RPMLimit            *int64      `json:"rpm_limit"`
	DailyRequestLimit   *int64      `json:"daily_request_limit"`
	MonthlyRequestLimit *int64      `json:"monthly_request_limit"`
	DailyTokenLimit     *int64      `json:"daily_token_limit"`
	MonthlyTokenLimit   *int64      `json:"monthly_token_limit"`
}

// PolicyLayers preserves each source instead of materializing set
// intersections. This keeps aliases/patterns and future group constraints at
// their owning boundary while EffectivePolicy applies all predicates at
// admission time.
type PolicyLayers struct {
	Group          *Policy `json:"group,omitempty"`
	Workspace      *Policy `json:"workspace,omitempty"`
	Project        *Policy `json:"project,omitempty"`
	ServiceAccount *Policy `json:"service_account,omitempty"`
	Credential     *Policy `json:"credential,omitempty"`
}

// PolicyContext is the server-resolved identity used to load policy layers.
// Client-provided tenant IDs must never be copied into this value.
type PolicyContext struct {
	GroupID          int64 `json:"group_id,omitempty"`
	WorkspaceID      int64 `json:"workspace_id,omitempty"`
	ProjectID        int64 `json:"project_id,omitempty"`
	ServiceAccountID int64 `json:"service_account_id,omitempty"`
	CredentialID     int64 `json:"credential_id,omitempty"`
	// APIKeyID is an explicit compatibility alias for callers whose identity
	// model still calls credentials API keys. CredentialID takes precedence.
	APIKeyID int64 `json:"api_key_id,omitempty"`
}

// PolicyRepository is deliberately small so SQL, test, and cache-backed
// implementations can share the resolver. GetPolicy returns (nil, nil) when
// a scope has no policy and therefore inherits without adding constraints.
type PolicyRepository interface {
	GetPolicy(context.Context, PolicyRef) (*Policy, error)
	// UpdatePolicy creates a missing policy when expectedRevision is zero. For
	// an existing policy, expectedRevision must equal its current revision.
	// Successful writes always increment the stored revision.
	UpdatePolicy(context.Context, PolicyRef, int64, Policy) (*Policy, error)
}

// Validate checks storage/API invariants without changing the nil-versus-empty
// state of allowlists.
func (p Policy) Validate() error {
	if p.Scope != "" && !validPolicyScope(p.Scope) {
		return fmt.Errorf("%w: unknown scope %q", ErrInvalidPolicy, p.Scope)
	}
	if p.ScopeID < 0 || p.Revision < 0 {
		return fmt.Errorf("%w: scope_id and revision cannot be negative", ErrInvalidPolicy)
	}
	if err := validatePolicyList(p.AllowedModels, 1000); err != nil {
		return fmt.Errorf("%w: allowed_models: %v", ErrInvalidPolicy, err)
	}
	if err := validatePolicyList(p.AllowedPlatforms, 256); err != nil {
		return fmt.Errorf("%w: allowed_platforms: %v", ErrInvalidPolicy, err)
	}
	for name, value := range map[string]*int64{
		"rpm_limit":             p.RPMLimit,
		"daily_request_limit":   p.DailyRequestLimit,
		"monthly_request_limit": p.MonthlyRequestLimit,
		"daily_token_limit":     p.DailyTokenLimit,
		"monthly_token_limit":   p.MonthlyTokenLimit,
	} {
		if value != nil && *value <= 0 {
			return fmt.Errorf("%w: %s must be positive", ErrInvalidPolicy, name)
		}
	}
	return nil
}

func validPolicyScope(scope PolicyScope) bool {
	switch scope {
	case PolicyScopeGroup, PolicyScopeWorkspace, PolicyScopeProject, PolicyScopeServiceAccount, PolicyScopeCredential:
		return true
	default:
		return false
	}
}

func validatePolicyList(values []string, maxEntries int) error {
	if len(values) > maxEntries {
		return fmt.Errorf("must contain at most %d entries", maxEntries)
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return errors.New("entries must not be blank")
		}
		if len(value) > 255 {
			return errors.New("entries must be at most 255 bytes")
		}
	}
	return nil
}

// CanonicalModel and CanonicalPlatform keep policy matching independent of
// cosmetic case/whitespace differences. The models/ prefix is accepted for
// compatibility with Gemini model identifiers already used in this repository.
func CanonicalModel(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	model = strings.TrimPrefix(model, "models/")
	return model
}

func CanonicalPlatform(platform string) string {
	return strings.ToLower(strings.TrimSpace(platform))
}

func allowsValue(allowlist []string, value string, canonical func(string) string) bool {
	if allowlist == nil {
		return true
	}
	want := canonical(value)
	for _, allowed := range allowlist {
		if canonical(allowed) == want {
			return true
		}
	}
	return false
}

// allowsModelValue preserves the wildcard model semantics used by the legacy
// Group allowlist while keeping platform matching exact. A policy model entry
// may contain '*' anywhere; matching remains anchored at both ends.
func allowsModelValue(allowlist []string, value string) bool {
	if allowlist == nil {
		return true
	}
	want := CanonicalModel(value)
	for _, allowed := range allowlist {
		if policyModelPatternMatches(CanonicalModel(allowed), want) {
			return true
		}
	}
	return false
}

func policyModelPatternMatches(pattern, value string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	if !strings.HasPrefix(value, parts[0]) {
		return false
	}
	remainder := value[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		idx := strings.Index(remainder, part)
		if idx < 0 {
			return false
		}
		remainder = remainder[idx+len(part):]
	}
	return strings.HasSuffix(remainder, parts[len(parts)-1])
}

// AllowsModel applies this single policy's model predicate. It is intentionally
// exported so existing group/key restrictions can be composed without first
// calculating a brittle pattern intersection.
func (p *Policy) AllowsModel(model string) bool {
	if p == nil {
		return true
	}
	return allowsModelValue(p.AllowedModels, model)
}

// AllowsPlatform applies this single policy's platform predicate.
func (p *Policy) AllowsPlatform(platform string) bool {
	if p == nil {
		return true
	}
	return allowsValue(p.AllowedPlatforms, platform, CanonicalPlatform)
}

// EffectivePolicy retains source layers and exposes the minimum configured
// numeric limits. A nil effective limit means no layer configured a limit.
type EffectivePolicy struct {
	Layers              PolicyLayers          `json:"layers"`
	Revisions           map[PolicyScope]int64 `json:"revisions"`
	RPMLimit            *int64                `json:"rpm_limit"`
	DailyRequestLimit   *int64                `json:"daily_request_limit"`
	MonthlyRequestLimit *int64                `json:"monthly_request_limit"`
	DailyTokenLimit     *int64                `json:"daily_token_limit"`
	MonthlyTokenLimit   *int64                `json:"monthly_token_limit"`
}

// ResolveEffectivePolicy validates all supplied layers and composes them with
// AND semantics. Missing layers inherit. A child can never broaden a parent.
func ResolveEffectivePolicy(layers PolicyLayers) (EffectivePolicy, error) {
	entries := []struct {
		scope  PolicyScope
		policy *Policy
	}{
		{PolicyScopeGroup, layers.Group},
		{PolicyScopeWorkspace, layers.Workspace},
		{PolicyScopeProject, layers.Project},
		{PolicyScopeServiceAccount, layers.ServiceAccount},
		{PolicyScopeCredential, layers.Credential},
	}
	ordered := make([]*Policy, 0, len(entries))
	for _, entry := range entries {
		policy := entry.policy
		if policy == nil {
			continue
		}
		if policy.Scope != "" && policy.Scope != entry.scope {
			return EffectivePolicy{}, fmt.Errorf("%w: %s layer contains %s policy", ErrInvalidPolicy, entry.scope, policy.Scope)
		}
		if err := policy.Validate(); err != nil {
			return EffectivePolicy{}, err
		}
		ordered = append(ordered, policy)
	}
	effective := EffectivePolicy{
		Layers:    layers,
		Revisions: make(map[PolicyScope]int64, len(ordered)),
	}
	effective.RPMLimit = minimumLimit(ordered, func(p *Policy) *int64 { return p.RPMLimit })
	effective.DailyRequestLimit = minimumLimit(ordered, func(p *Policy) *int64 { return p.DailyRequestLimit })
	effective.MonthlyRequestLimit = minimumLimit(ordered, func(p *Policy) *int64 { return p.MonthlyRequestLimit })
	effective.DailyTokenLimit = minimumLimit(ordered, func(p *Policy) *int64 { return p.DailyTokenLimit })
	effective.MonthlyTokenLimit = minimumLimit(ordered, func(p *Policy) *int64 { return p.MonthlyTokenLimit })
	for _, policy := range ordered {
		if policy != nil && policy.Scope != "" {
			effective.Revisions[policy.Scope] = policy.Revision
		}
	}
	return effective, nil
}

func minimumLimit(policies []*Policy, get func(*Policy) *int64) *int64 {
	var minimum *int64
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		value := get(policy)
		if value == nil || (minimum != nil && *minimum <= *value) {
			continue
		}
		copyValue := *value
		minimum = &copyValue
	}
	return minimum
}

func (e EffectivePolicy) AllowsModel(model string) bool {
	return e.Layers.Group.AllowsModel(model) && e.Layers.Workspace.AllowsModel(model) && e.Layers.Project.AllowsModel(model) && e.Layers.ServiceAccount.AllowsModel(model) && e.Layers.Credential.AllowsModel(model)
}

func (e EffectivePolicy) AllowsPlatform(platform string) bool {
	return e.Layers.Group.AllowsPlatform(platform) && e.Layers.Workspace.AllowsPlatform(platform) && e.Layers.Project.AllowsPlatform(platform) && e.Layers.ServiceAccount.AllowsPlatform(platform) && e.Layers.Credential.AllowsPlatform(platform)
}

// The Value helpers use zero only as the representation of an unrestricted
// effective value; stored policy values never accept zero.
func (e EffectivePolicy) RPMLimitValue() int64            { return limitValue(e.RPMLimit) }
func (e EffectivePolicy) DailyRequestLimitValue() int64   { return limitValue(e.DailyRequestLimit) }
func (e EffectivePolicy) MonthlyRequestLimitValue() int64 { return limitValue(e.MonthlyRequestLimit) }
func (e EffectivePolicy) DailyTokenLimitValue() int64     { return limitValue(e.DailyTokenLimit) }
func (e EffectivePolicy) MonthlyTokenLimitValue() int64   { return limitValue(e.MonthlyTokenLimit) }
func limitValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func (e EffectivePolicy) Revision(scope PolicyScope) int64 {
	return e.Revisions[scope]
}

// EffectivePolicyResolver loads server-resolved hierarchy IDs through the
// repository. Missing scope policies are ordinary inheritance and not errors.
type EffectivePolicyResolver struct {
	repo PolicyRepository
}

func NewEffectivePolicyResolver(repo PolicyRepository) *EffectivePolicyResolver {
	return &EffectivePolicyResolver{repo: repo}
}

func (r *EffectivePolicyResolver) Resolve(ctx context.Context, identity PolicyContext) (EffectivePolicy, error) {
	if r == nil || r.repo == nil {
		return EffectivePolicy{}, fmt.Errorf("%w: policy repository is required", ErrInvalidPolicy)
	}
	layers := PolicyLayers{}
	var err error
	if identity.GroupID > 0 {
		layers.Group, err = r.load(ctx, PolicyRef{Scope: PolicyScopeGroup, ScopeID: identity.GroupID})
		if err != nil {
			return EffectivePolicy{}, err
		}
	}
	if identity.WorkspaceID > 0 {
		layers.Workspace, err = r.load(ctx, PolicyRef{Scope: PolicyScopeWorkspace, ScopeID: identity.WorkspaceID})
		if err != nil {
			return EffectivePolicy{}, err
		}
	}
	if identity.ProjectID > 0 {
		layers.Project, err = r.load(ctx, PolicyRef{Scope: PolicyScopeProject, ScopeID: identity.ProjectID})
		if err != nil {
			return EffectivePolicy{}, err
		}
	}
	if identity.ServiceAccountID > 0 {
		layers.ServiceAccount, err = r.load(ctx, PolicyRef{Scope: PolicyScopeServiceAccount, ScopeID: identity.ServiceAccountID})
		if err != nil {
			return EffectivePolicy{}, err
		}
	}
	credentialID := identity.CredentialID
	if credentialID == 0 {
		credentialID = identity.APIKeyID
	}
	if credentialID > 0 {
		layers.Credential, err = r.load(ctx, PolicyRef{Scope: PolicyScopeCredential, ScopeID: credentialID})
		if err != nil {
			return EffectivePolicy{}, err
		}
	}
	return ResolveEffectivePolicy(layers)
}

func (r *EffectivePolicyResolver) load(ctx context.Context, ref PolicyRef) (*Policy, error) {
	policy, err := r.repo.GetPolicy(ctx, ref)
	if err != nil {
		if errors.Is(err, ErrPolicyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if policy == nil {
		return nil, nil
	}
	if policy.Scope == "" {
		policy.Scope, policy.ScopeID = ref.Scope, ref.ScopeID
	}
	if policy.Scope != ref.Scope || policy.ScopeID != ref.ScopeID {
		return nil, fmt.Errorf("%w: policy reference mismatch", ErrInvalidPolicy)
	}
	return policy, nil
}

// MemoryPolicyStore is a concurrency-safe reference implementation useful for
// unit tests and as an adapter contract while durable SQL persistence lands.
type MemoryPolicyStore struct {
	mu       sync.RWMutex
	policies map[PolicyRef]Policy
}

func NewMemoryPolicyStore() *MemoryPolicyStore {
	return &MemoryPolicyStore{policies: make(map[PolicyRef]Policy)}
}

func (s *MemoryPolicyStore) GetPolicy(_ context.Context, ref PolicyRef) (*Policy, error) {
	if err := validatePolicyRef(ref); err != nil {
		return nil, err
	}
	s.mu.RLock()
	policy, ok := s.policies[ref]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	return clonePolicy(&policy), nil
}

func (s *MemoryPolicyStore) UpdatePolicy(_ context.Context, ref PolicyRef, expectedRevision int64, value Policy) (*Policy, error) {
	if err := validatePolicyRef(ref); err != nil {
		return nil, err
	}
	if expectedRevision < 0 {
		return nil, fmt.Errorf("%w: expected revision cannot be negative", ErrInvalidPolicy)
	}
	value.Scope, value.ScopeID = ref.Scope, ref.ScopeID
	value.Revision = 0
	if err := value.Validate(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.policies[ref]
	if !exists {
		if expectedRevision != 0 {
			return nil, ErrPolicyRevisionConflict
		}
		value.Revision = 1
	} else {
		if expectedRevision == 0 || current.Revision != expectedRevision {
			return nil, ErrPolicyRevisionConflict
		}
		value.Revision = current.Revision + 1
	}
	s.policies[ref] = *clonePolicy(&value)
	return clonePolicy(&value), nil
}

func validatePolicyRef(ref PolicyRef) error {
	if !validPolicyScope(ref.Scope) || ref.ScopeID <= 0 {
		return fmt.Errorf("%w: invalid policy reference", ErrInvalidPolicy)
	}
	return nil
}

func clonePolicy(policy *Policy) *Policy {
	if policy == nil {
		return nil
	}
	clone := *policy
	if policy.AllowedModels != nil {
		clone.AllowedModels = make([]string, len(policy.AllowedModels))
		copy(clone.AllowedModels, policy.AllowedModels)
	}
	if policy.AllowedPlatforms != nil {
		clone.AllowedPlatforms = make([]string, len(policy.AllowedPlatforms))
		copy(clone.AllowedPlatforms, policy.AllowedPlatforms)
	}
	clone.RPMLimit = cloneInt64(policy.RPMLimit)
	clone.DailyRequestLimit = cloneInt64(policy.DailyRequestLimit)
	clone.MonthlyRequestLimit = cloneInt64(policy.MonthlyRequestLimit)
	clone.DailyTokenLimit = cloneInt64(policy.DailyTokenLimit)
	clone.MonthlyTokenLimit = cloneInt64(policy.MonthlyTokenLimit)
	return &clone
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
