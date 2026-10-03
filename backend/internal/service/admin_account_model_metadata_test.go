package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdateAccountPreservesSyncedModelMetadata(t *testing.T) {
	for _, staleExtra := range []map[string]any{
		{},
		{UpstreamModelMetadataExtraKey: UpstreamModelMetadataSnapshot{
			Source: "upstream", SyncedAt: "2026-10-01T00:00:00Z",
			Models: map[string]UpstreamModelMetadata{"video-v1": {ID: "video-v1", DisplayName: "Old Model Name"}},
		}},
	} {
		fresh := UpstreamModelMetadataSnapshot{
			Source: "upstream", SyncedAt: "2026-10-03T00:00:00Z",
			Models: map[string]UpstreamModelMetadata{
				"video-v1": {ID: "video-v1", DisplayName: "Fresh Video Model", OutputModalities: []string{"video"}},
			},
		}
		account := &Account{ID: 73, Name: "before", Platform: PlatformSeedance, Type: AccountTypeAPIKey, Status: StatusActive}
		account.SetUpstreamModelMetadataSnapshot(fresh)
		repo := &upstreamBillingProbeAdminRepo{upstreamBillingProbeAccountRepo: &upstreamBillingProbeAccountRepo{
			accounts: map[int64]*Account{account.ID: account},
		}}
		svc := &adminServiceImpl{accountRepo: repo}
		updated, err := svc.UpdateAccount(context.Background(), account.ID, &UpdateAccountInput{Name: "after", Extra: staleExtra})
		require.NoError(t, err)
		require.Equal(t, "after", updated.Name)
		snapshot := updated.GetUpstreamModelMetadataSnapshot()
		require.NotNil(t, snapshot)
		require.Equal(t, fresh, *snapshot, "an old edit form must not clear or overwrite synced names and output capabilities")
	}
}
