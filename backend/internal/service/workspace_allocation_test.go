package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAllocationEnvironment(t *testing.T) {
	for _, value := range []string{"production", " staging ", "development", "testing", "custom:eu-west-1"} {
		got, err := NormalizeAllocationEnvironment(value)
		require.NoError(t, err)
		require.NotEmpty(t, got)
	}
	for _, value := range []string{"", "prod", "custom:EU", "custom:-bad", "custom:bad_"} {
		require.Error(t, ValidateAllocationEnvironment(value), value)
	}
}

func TestValidateAllocationTagsBoundsAndScalars(t *testing.T) {
	valid, err := ValidateAllocationTags(map[string]string{"team.name": "Platform", "env": "prod"})
	require.NoError(t, err)
	require.Equal(t, "Platform", valid["team.name"])
	_, err = ValidateAllocationTags(map[string]string{"api_key": "secret"})
	require.Error(t, err)
	_, err = ValidateAllocationJSONTags(map[string]any{"ok": true})
	require.Error(t, err)
	gotJSON, err := ValidateAllocationTagsJSON([]byte(`{"team":"core"}`))
	require.NoError(t, err)
	require.Equal(t, "core", gotJSON["team"])
	_, err = ValidateAllocationTagsJSON([]byte(`{"ok":true}`))
	require.Error(t, err)
	tooMany := make(map[string]string, MaxAllocationTags+1)
	for i := 0; i < MaxAllocationTags+1; i++ {
		tooMany[string(rune('a'+i/26))+string(rune('a'+i%26))] = "v"
	}
	_, err = ValidateAllocationTags(tooMany)
	require.Error(t, err)
}

func TestResolveAllocationPrecedence(t *testing.T) {
	center := int64(7)
	project := &AllocationSnapshot{CostCenterID: &center, Environment: "production", PolicyRevision: 1, Tags: map[string]string{"team": "core"}}
	serviceAccount := &AllocationSnapshot{Environment: "staging", PolicyRevision: 2}
	apiKey := &AllocationSnapshot{Environment: "testing", PolicyRevision: 3}

	got := ResolveAllocation(nil, nil, project)
	require.Equal(t, AllocationSourceProject, got.Source)
	got = ResolveAllocation(nil, serviceAccount, project)
	require.Equal(t, AllocationSourceServiceAccount, got.Source)
	got = ResolveAllocation(apiKey, serviceAccount, project)
	require.Equal(t, AllocationSourceAPIKey, got.Source)
	got.Tags["mutated"] = "caller"
	require.NotContains(t, apiKey.Tags, "mutated")
	got = ResolveAllocation(nil, nil, nil)
	require.Equal(t, AllocationEnvironmentUnallocated, got.Environment)
	require.Equal(t, AllocationSourceUnallocated, got.Source)
}

func TestAllocationConfigAndSnapshotValidation(t *testing.T) {
	_, err := (AllocationConfig{Environment: "production", PolicyRevision: 0}).NormalizeAndValidate()
	require.Error(t, err)
	valid, err := (AllocationConfig{Environment: " Production ", PolicyRevision: 4, Tags: map[string]string{"team": "core"}}).NormalizeAndValidate()
	require.NoError(t, err)
	require.Equal(t, "production", valid.Environment)
	_, err = (AllocationSnapshot{Environment: "unallocated", PolicyRevision: 1}).NormalizeAndValidate()
	require.Error(t, err)
	require.NoError(t, CheckAllocationPolicyRevision(0, 1))
	require.NoError(t, CheckAllocationPolicyRevision(2, 2))
	require.ErrorIs(t, CheckAllocationPolicyRevision(1, 2), ErrWorkspaceAllocationConflict)
}
