//go:build integration

package repository

import (
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLifecyclePostgresCryptoConsensusPausesClaimWithoutLosingJobs(t *testing.T) {
	ctx, repo, owner, w := lifecycleFixture(t)
	job, err := repo.CreateLifecycleExport(ctx, owner.ID, w.ID)
	require.NoError(t, err)
	a := service.LifecycleCryptoReader{InstanceID: "one", Token: uuid.NewString(), Fingerprint: strings.Repeat("a", 64), ExpectedInstanceIDs: []string{"one", "two"}, V2Readable: true, RollbackCompatible: true}
	b := a
	b.InstanceID = "two"
	b.Token = uuid.NewString()
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, a, false))
	claim, err := repo.ClaimLifecycleExportWithCrypto(ctx, a, true)
	require.ErrorIs(t, err, service.ErrLifecycleKeyRingNotReady)
	require.Nil(t, claim)
	var state string
	var attempts int
	require.NoError(t, integrationDB.QueryRow(`SELECT state,attempts FROM workspace_export_jobs WHERE id=$1`, job.ID).Scan(&state, &attempts))
	require.Equal(t, "pending", state)
	require.Zero(t, attempts)
	b.Fingerprint = strings.Repeat("b", 64)
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, b, false))
	_, err = repo.ClaimLifecycleExportWithCrypto(ctx, a, true)
	require.ErrorIs(t, err, service.ErrLifecycleKeyRingNotReady)
	b.Fingerprint = a.Fingerprint
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, b, false))
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, a, true))
	_, err = integrationDB.Exec(`UPDATE workspace_export_crypto_readers SET live_until=clock_timestamp()-interval '1 second' WHERE instance_id='two'`)
	require.NoError(t, err)
	_, err = repo.ClaimLifecycleExportWithCrypto(ctx, a, true)
	require.ErrorIs(t, err, service.ErrLifecycleKeyRingNotReady)
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, b, false))
	b.RollbackCompatible = false
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, b, false))
	require.ErrorIs(t, repo.RegisterLifecycleCryptoReader(ctx, a, true), service.ErrLifecycleKeyRingNotReady)
	b.RollbackCompatible = true
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, b, false))
	other := a
	other.InstanceID = "unapproved"
	other.Token = uuid.NewString()
	other.ExpectedInstanceIDs = nil
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, other, false))
	require.ErrorIs(t, repo.RegisterLifecycleCryptoReader(ctx, a, true), service.ErrLifecycleKeyRingNotReady)
	_, err = integrationDB.Exec(`UPDATE workspace_export_crypto_readers SET live_until=clock_timestamp()-interval '1 second' WHERE instance_id='unapproved'`)
	require.NoError(t, err)
	claim, err = repo.ClaimLifecycleExportWithCrypto(ctx, a, true)
	require.NoError(t, err)
	require.NotNil(t, claim)
	require.Equal(t, job.ID, claim.ID)
	require.Equal(t, 1, claim.Attempts)
}

func TestLifecyclePostgresCryptoReaderRejectsDuplicateInstanceIdentity(t *testing.T) {
	ctx, repo, _, _ := lifecycleFixture(t)
	a := service.LifecycleCryptoReader{InstanceID: "one", Token: uuid.NewString(), Fingerprint: strings.Repeat("a", 64)}
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, a, false))
	b := a
	b.Token = uuid.NewString()
	require.ErrorIs(t, repo.RegisterLifecycleCryptoReader(ctx, b, false), service.ErrLifecycleKeyRingNotReady)
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, a, false))
	_, err := integrationDB.Exec(`UPDATE workspace_export_crypto_readers SET live_until=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	require.NoError(t, repo.RegisterLifecycleCryptoReader(ctx, b, false), "restart takes over expired instance registration")
}
