package service

import (
	"bytes"
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
	"time"
)

type lifecycleRuntimeRepo struct {
	WorkspaceRepository
	WorkspaceLifecycleRepository
	claim     *LifecycleExportJob
	cancelled bool
	finished  error
}

func (r *lifecycleRuntimeRepo) ClaimLifecycleExport(context.Context) (*LifecycleExportJob, error) {
	j := r.claim
	r.claim = nil
	return j, nil
}
func (r *lifecycleRuntimeRepo) WriteLifecycleExportSnapshot(_ context.Context, _ *LifecycleExportJob, w io.Writer) (*LifecycleExportResult, error) {
	_, e := w.Write([]byte("tenant-snapshot"))
	return &LifecycleExportResult{DataCutoff: time.Now(), Records: 1}, e
}
func (r *lifecycleRuntimeRepo) FinishLifecycleExport(_ context.Context, _ *LifecycleExportJob, _ *LifecycleExportResult, e error) error {
	r.finished = e
	if r.cancelled {
		return ErrLifecycleLeaseLost
	}
	return e
}

type lifecycleRuntimeStore struct {
	BackupObjectStore
	uploaded []byte
	deleted  bool
}

func (s *lifecycleRuntimeStore) UploadFile(_ context.Context, _ string, path string, _ string) (int64, error) {
	data, e := os.ReadFile(path)
	s.uploaded = data
	return int64(len(data)), e
}
func (s *lifecycleRuntimeStore) Delete(context.Context, string) error { s.deleted = true; return nil }

func TestLifecycleRuntimeDisabledByDefault(t *testing.T) {
	s := NewWorkspaceService(&lifecycleRuntimeRepo{})
	_, e := s.CreateLifecycleExport(context.Background(), 1, 1)
	require.ErrorIs(t, e, ErrLifecycleDisabled)
}
func TestLifecycleRuntimeEncryptsBeforeUploadAndDefersLostLeaseCleanupToLedger(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	r := &lifecycleRuntimeRepo{claim: &LifecycleExportJob{ID: "job", WorkspaceID: 1, ObjectKey: "scope", LeaseToken: "token"}, cancelled: true}
	store := &lifecycleRuntimeStore{}
	s := NewWorkspaceService(r)
	s.lifecycle = &LifecycleRuntime{repo: r, cfg: config.DataLifecycleConfig{Enabled: true}, key: key, store: store, owner: s}
	e := s.lifecycle.processExport(context.Background())
	require.ErrorIs(t, e, ErrLifecycleLeaseLost)
	require.False(t, store.deleted, "lease loss cannot authorize object destruction outside the hold/policy-fenced ledger")
	require.NotContains(t, string(store.uploaded), "tenant-snapshot")
	var plain bytes.Buffer
	require.NoError(t, DecryptLifecycleArtifact(&plain, bytes.NewReader(store.uploaded), key, "scope"))
	require.Equal(t, "tenant-snapshot", plain.String())
	require.False(t, errors.Is(r.finished, ErrLifecycleExportLimit))
}

type lifecycleCryptoRuntimeRepo struct {
	lifecycleRuntimeRepo
	ready bool
}

func (r *lifecycleCryptoRuntimeRepo) RegisterLifecycleCryptoReader(context.Context, LifecycleCryptoReader, bool) error {
	if !r.ready {
		return ErrLifecycleKeyRingNotReady
	}
	return nil
}
func (r *lifecycleCryptoRuntimeRepo) ClaimLifecycleExportWithCrypto(ctx context.Context, _ LifecycleCryptoReader, _ bool) (*LifecycleExportJob, error) {
	if !r.ready {
		return nil, ErrLifecycleKeyRingNotReady
	}
	return r.ClaimLifecycleExport(ctx)
}

func TestLifecycleRuntimeV2RequiresConsensusBeforeClaimAndUpload(t *testing.T) {
	ring := fixtureLifecycleKeyRing(t)
	r := &lifecycleCryptoRuntimeRepo{lifecycleRuntimeRepo: lifecycleRuntimeRepo{claim: &LifecycleExportJob{ID: "job", WorkspaceID: 1, ObjectKey: "scope", LeaseToken: "token"}}}
	store := &lifecycleRuntimeStore{}
	runtime := &LifecycleRuntime{repo: r, cryptoRepo: r, keys: ring, store: store, cfg: config.DataLifecycleConfig{Enabled: true, V2WriteEnabled: true}}
	require.ErrorIs(t, runtime.processExport(context.Background()), ErrLifecycleKeyRingNotReady)
	require.Empty(t, store.uploaded)
	require.NotNil(t, r.claim, "failed promotion leaves the queued attempt available")
	r.ready = true
	require.NoError(t, runtime.processExport(context.Background()))
	require.True(t, bytes.HasPrefix(store.uploaded, []byte("MRLEX02\n")))
	var out bytes.Buffer
	require.NoError(t, DecryptLifecycleArtifactWithKeys(context.Background(), &out, bytes.NewReader(store.uploaded), ring, "scope"))
	require.Equal(t, "tenant-snapshot", out.String())
}
