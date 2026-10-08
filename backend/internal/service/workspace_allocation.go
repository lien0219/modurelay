package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Allocation values are deliberately small, immutable admission data. They
// are copied into a reservation before a provider call and must not contain
// arbitrary JSON or secrets.
const MaxAllocationTags = 32

var (
	ErrWorkspaceAllocationInvalid  = infraerrors.BadRequest("WORKSPACE_ALLOCATION_INVALID", "invalid workspace allocation")
	ErrWorkspaceAllocationConflict = infraerrors.Conflict("WORKSPACE_ALLOCATION_CONFLICT", "workspace allocation revision conflict")
)

type AllocationSource string

const (
	AllocationSourceAPIKey         AllocationSource = "api_key_override"
	AllocationSourceServiceAccount AllocationSource = "service_account_override"
	AllocationSourceProject        AllocationSource = "project_default"
	AllocationSourceUnallocated    AllocationSource = "unallocated"
)

const (
	AllocationEnvironmentProduction  = "production"
	AllocationEnvironmentStaging     = "staging"
	AllocationEnvironmentDevelopment = "development"
	AllocationEnvironmentTesting     = "testing"
	AllocationEnvironmentUnallocated = "unallocated"
)

var allocationTagKeyRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,62}$`)
var allocationTagSecretRE = regexp.MustCompile(`(?i)(secret|token|password|credential|api[_-]?key)`)

// AllocationSnapshot is the value frozen at admission. A nil CostCenterID
// means that the request is intentionally unallocated.
type AllocationSnapshot struct {
	CostCenterID   *int64            `json:"cost_center_id,omitempty"`
	CostCenterCode string            `json:"cost_center_code,omitempty"`
	CostCenterName string            `json:"cost_center_name,omitempty"`
	Environment    string            `json:"environment"`
	Tags           map[string]string `json:"tags,omitempty"`
	AllocationTags map[string]string `json:"allocation_tags,omitempty"`
	PolicyRevision int64             `json:"policy_revision"`
	Source         AllocationSource  `json:"source,omitempty"`
}

// AllocationConfig is the mutable project/credential configuration form.
type AllocationConfig struct {
	CostCenterID   *int64            `json:"cost_center_id,omitempty"`
	Environment    string            `json:"environment"`
	Tags           map[string]string `json:"tags,omitempty"`
	AllocationTags map[string]string `json:"allocation_tags,omitempty"`
	PolicyRevision int64             `json:"policy_revision,omitempty"`
}

type WorkspaceCostCenter struct {
	ID          int64  `json:"id"`
	WorkspaceID int64  `json:"workspace_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type WorkspaceAllocationTag struct {
	ID          int64  `json:"id"`
	WorkspaceID int64  `json:"workspace_id"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type ProjectAllocation struct {
	WorkspaceID     int64            `json:"workspace_id"`
	ProjectID       int64            `json:"project_id"`
	Allocation      AllocationConfig `json:"allocation"`
	UpdatedByUserID int64            `json:"updated_by_user_id,omitempty"`
}

type APIKeyAllocationOverride struct {
	WorkspaceID int64            `json:"workspace_id"`
	ProjectID   int64            `json:"project_id"`
	APIKeyID    int64            `json:"api_key_id"`
	Allocation  AllocationConfig `json:"allocation"`
}

type ServiceAccountAllocationOverride struct {
	WorkspaceID      int64            `json:"workspace_id"`
	ProjectID        int64            `json:"project_id"`
	ServiceAccountID int64            `json:"service_account_id"`
	Allocation       AllocationConfig `json:"allocation"`
}

type AllocationCenter = WorkspaceCostCenter
type AllocationTag = WorkspaceAllocationTag
type ProjectCostAllocationConfig = AllocationConfig
type AllocationReportFilter = AllocationFilter
type FinopsAllocationReport = AllocationReport

type AllocationFilter struct {
	WorkspaceID  int64
	ProjectID    int64
	From         string `json:"from,omitempty"`
	To           string `json:"to,omitempty"`
	Timezone     string `json:"timezone,omitempty"`
	CostCenterID *int64 `json:"cost_center_id,omitempty"`
	Environment  string `json:"environment,omitempty"`
	TagKey       string `json:"tag_key,omitempty"`
	TagValue     string `json:"tag_value,omitempty"`
}

type AllocationReport struct {
	WorkspaceID     int64                   `json:"workspace_id"`
	ProjectID       int64                   `json:"project_id,omitempty"`
	WorkspaceTotal  float64                 `json:"workspace_total"`
	Allocated       float64                 `json:"allocated"`
	Unallocated     float64                 `json:"unallocated"`
	OverlappingTags bool                    `json:"overlapping_tags"`
	CostCenters     []AllocationReportGroup `json:"cost_centers,omitempty"`
	Environments    []AllocationReportGroup `json:"environments,omitempty"`
	Tags            []AllocationReportGroup `json:"tags,omitempty"`
}

type AllocationReportGroup struct {
	Key          string  `json:"key"`
	Cost         float64 `json:"cost"`
	RequestCount int64   `json:"request_count"`
}

func NormalizeAllocationEnvironment(value string) (string, error) {
	raw := strings.TrimSpace(value)
	v := strings.ToLower(raw)
	switch v {
	case AllocationEnvironmentProduction, AllocationEnvironmentStaging, AllocationEnvironmentDevelopment, AllocationEnvironmentTesting:
		return v, nil
	default:
		return "", ErrWorkspaceAllocationInvalid
	}
}

func ValidateAllocationEnvironment(value string) error {
	_, err := NormalizeAllocationEnvironment(value)
	return err
}

func NormalizeAllocationTagKey(value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	if !allocationTagKeyRE.MatchString(v) || allocationTagSecretRE.MatchString(v) {
		return "", ErrWorkspaceAllocationInvalid
	}
	return v, nil
}

func NormalizeAllocationTagValue(value string) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" || len([]rune(v)) > 255 {
		return "", ErrWorkspaceAllocationInvalid
	}
	return v, nil
}

// ValidateAllocationTags validates and returns a canonical copy. Keys are
// lower-cased; values are trimmed. This also keeps map mutation by callers
// from changing an already resolved snapshot.
func ValidateAllocationTags(tags map[string]string) (map[string]string, error) {
	if len(tags) > MaxAllocationTags {
		return nil, ErrWorkspaceAllocationInvalid
	}
	out := make(map[string]string, len(tags))
	for key, value := range tags {
		k, err := NormalizeAllocationTagKey(key)
		if err != nil {
			return nil, err
		}
		v, err := NormalizeAllocationTagValue(value)
		if err != nil {
			return nil, err
		}
		if _, exists := out[k]; exists {
			return nil, ErrWorkspaceAllocationInvalid
		}
		out[k] = v
	}
	return out, nil
}

// ValidateAllocationJSONTags enforces the JSON contract used by migration 299:
// an object of at most 32 normalized keys whose values are strings.
func ValidateAllocationJSONTags(raw map[string]any) (map[string]string, error) {
	if len(raw) > MaxAllocationTags {
		return nil, ErrWorkspaceAllocationInvalid
	}
	tags := make(map[string]string, len(raw))
	for key, value := range raw {
		str, ok := value.(string)
		if !ok {
			return nil, ErrWorkspaceAllocationInvalid
		}
		tags[key] = str
	}
	return ValidateAllocationTags(tags)
}

// ValidateAllocationTagsJSON validates the wire representation before it is
// persisted as JSONB. Arrays, null, nested objects, numbers, and booleans are
// rejected to match the database scalar-string contract.
func ValidateAllocationTagsJSON(raw []byte) (map[string]string, error) {
	var value map[string]any
	if len(strings.TrimSpace(string(raw))) == 0 || string(strings.TrimSpace(string(raw))) == "null" {
		return nil, ErrWorkspaceAllocationInvalid
	}
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, ErrWorkspaceAllocationInvalid
	}
	return ValidateAllocationJSONTags(value)
}

func ValidateAllocationPolicyRevision(revision int64) error {
	if revision <= 0 {
		return ErrWorkspaceAllocationInvalid
	}
	return nil
}

// ValidateAllocationRevision is retained as a concise alias for callers that
// do not need to distinguish policy from allocation revisions.
func ValidateAllocationRevision(revision int64) error {
	return ValidateAllocationPolicyRevision(revision)
}

// CheckAllocationPolicyRevision verifies an optimistic-concurrency revision.
// A zero expected revision is treated as an unconditional create by callers;
// otherwise the values must match exactly.
func CheckAllocationPolicyRevision(expected, actual int64) error {
	if expected < 0 || actual <= 0 {
		return ErrWorkspaceAllocationInvalid
	}
	if expected != 0 && expected != actual {
		return ErrWorkspaceAllocationConflict
	}
	return nil
}

func (c AllocationConfig) NormalizeAndValidate() (AllocationConfig, error) {
	env, err := NormalizeAllocationEnvironment(c.Environment)
	if err != nil {
		return AllocationConfig{}, err
	}
	tagValues := c.Tags
	if tagValues == nil {
		tagValues = c.AllocationTags
	}
	tags, err := ValidateAllocationTags(tagValues)
	if err != nil {
		return AllocationConfig{}, err
	}
	if err := ValidateAllocationPolicyRevision(c.PolicyRevision); err != nil {
		return AllocationConfig{}, err
	}
	return AllocationConfig{CostCenterID: allocationCloneInt64Ptr(c.CostCenterID), Environment: env, Tags: tags, AllocationTags: tags, PolicyRevision: c.PolicyRevision}, nil
}

func (s AllocationSnapshot) NormalizeAndValidate() (AllocationSnapshot, error) {
	if s.CostCenterID != nil && *s.CostCenterID <= 0 {
		return AllocationSnapshot{}, ErrWorkspaceAllocationInvalid
	}
	if s.Source != "" {
		switch s.Source {
		case AllocationSourceAPIKey, AllocationSourceServiceAccount, AllocationSourceProject, AllocationSourceUnallocated:
		default:
			return AllocationSnapshot{}, ErrWorkspaceAllocationInvalid
		}
	}
	env, err := NormalizeAllocationEnvironment(s.Environment)
	if s.Environment == AllocationEnvironmentUnallocated {
		env = AllocationEnvironmentUnallocated
		err = nil
	}
	if err != nil {
		return AllocationSnapshot{}, err
	}
	tagValues := s.Tags
	if tagValues == nil {
		tagValues = s.AllocationTags
	}
	tags, err := ValidateAllocationTags(tagValues)
	if err != nil {
		return AllocationSnapshot{}, err
	}
	if env == AllocationEnvironmentUnallocated {
		if s.CostCenterID != nil || s.PolicyRevision != 0 || len(tags) != 0 || (s.Source != "" && s.Source != AllocationSourceUnallocated) {
			return AllocationSnapshot{}, ErrWorkspaceAllocationInvalid
		}
	} else if s.PolicyRevision < 0 || s.Source == AllocationSourceUnallocated {
		return AllocationSnapshot{}, ErrWorkspaceAllocationInvalid
	}
	return AllocationSnapshot{CostCenterID: allocationCloneInt64Ptr(s.CostCenterID), CostCenterCode: s.CostCenterCode, CostCenterName: s.CostCenterName, Environment: env, Tags: tags, AllocationTags: tags, PolicyRevision: s.PolicyRevision, Source: s.Source}, nil
}

func UnallocatedAllocation() AllocationSnapshot {
	empty := map[string]string{}
	return AllocationSnapshot{Environment: AllocationEnvironmentUnallocated, Tags: empty, AllocationTags: empty, PolicyRevision: 0, Source: AllocationSourceUnallocated}
}

// ResolveAllocation applies the server-side precedence rule. The returned
// value is a defensive copy and always has an explicit source/environment.
func ResolveAllocation(apiKey, serviceAccount, project *AllocationSnapshot) AllocationSnapshot {
	selected := project
	source := AllocationSourceProject
	if apiKey != nil {
		selected, source = apiKey, AllocationSourceAPIKey
	} else if serviceAccount != nil {
		selected, source = serviceAccount, AllocationSourceServiceAccount
	}
	if selected == nil {
		return UnallocatedAllocation()
	}
	out := cloneAllocationSnapshot(*selected)
	out.Source = source
	if strings.TrimSpace(out.Environment) == "" {
		out.Environment = AllocationEnvironmentUnallocated
	}
	return out
}

func ResolveAllocationPrecedence(apiKey, serviceAccount, project *AllocationSnapshot) AllocationSnapshot {
	return ResolveAllocation(apiKey, serviceAccount, project)
}

func (s AllocationSnapshot) TagsJSON() ([]byte, error) {
	tagValues := s.Tags
	if tagValues == nil {
		tagValues = s.AllocationTags
	}
	tags, err := ValidateAllocationTags(tagValues)
	if err != nil {
		return nil, err
	}
	return json.Marshal(tags)
}

func allocationCloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneAllocationSnapshot(value AllocationSnapshot) AllocationSnapshot {
	if value.Tags == nil {
		value.Tags = value.AllocationTags
	}
	tags := make(map[string]string, len(value.Tags))
	for k, v := range value.Tags {
		tags[k] = v
	}
	value.CostCenterID = allocationCloneInt64Ptr(value.CostCenterID)
	value.Tags = tags
	value.AllocationTags = tags
	return value
}

// SortedTagKeys is useful to deterministic report encoders and tests.
func SortedTagKeys(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (f AllocationFilter) Validate() error {
	if f.WorkspaceID <= 0 || f.ProjectID < 0 {
		return ErrWorkspaceAllocationInvalid
	}
	if f.Environment != "" && f.Environment != AllocationEnvironmentUnallocated {
		if err := ValidateAllocationEnvironment(f.Environment); err != nil {
			return err
		}
	}
	if f.TagKey != "" {
		if _, err := NormalizeAllocationTagKey(f.TagKey); err != nil {
			return err
		}
	}
	if f.TagValue != "" {
		if _, err := NormalizeAllocationTagValue(f.TagValue); err != nil {
			return err
		}
	}
	if f.CostCenterID != nil && *f.CostCenterID <= 0 {
		return fmt.Errorf("%w: cost center id", ErrWorkspaceAllocationInvalid)
	}
	return nil
}
