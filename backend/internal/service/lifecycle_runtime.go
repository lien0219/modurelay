package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
)

// One export build per process and two authenticated downloads keep memory and
// object-store concurrency bounded across all WorkspaceService instances.
var lifecycleExportSlot = make(chan struct{}, 1)
var lifecycleDownloadSlots = make(chan struct{}, 2)

type LifecycleRuntime struct {
	repo             WorkspaceLifecycleRepository
	owner            *WorkspaceService
	cfg              config.DataLifecycleConfig
	store            BackupObjectStore
	key              []byte
	keys             *LifecycleKeyRing
	cryptoRepo       LifecycleCryptoRepository
	reader           LifecycleCryptoReader
	mu               sync.Mutex
	started, stopped bool
	cancel           context.CancelFunc
	wg               sync.WaitGroup
}

func (s *WorkspaceService) ConfigureLifecycle(cfg *config.Config, factory BackupObjectStoreFactory) error {
	if cfg == nil || !cfg.DataLifecycle.Enabled {
		return nil
	}
	if err := cfg.DataLifecycle.Validate(); err != nil {
		return err
	}
	repo, ok := s.repo.(WorkspaceLifecycleRepository)
	if !ok || factory == nil {
		return ErrLifecycleDisabled
	}
	key, err := hex.DecodeString(cfg.DataLifecycle.EncryptionKey)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c := cfg.DataLifecycle
	ring, err := NewLifecycleKeyRing(c)
	if err != nil {
		return err
	}
	cryptoRepo, hasCryptoRepo := s.repo.(LifecycleCryptoRepository)
	if !hasCryptoRepo && (len(c.Keys) > 0 || c.V2WriteEnabled) {
		return ErrLifecycleKeyRingNotReady
	}
	instanceID := c.InstanceID
	if instanceID == "" {
		instanceID = uuid.NewString()
	}
	reader := LifecycleCryptoReader{InstanceID: instanceID, Token: uuid.NewString(), Fingerprint: ring.Fingerprint, ExpectedInstanceIDs: append([]string(nil), c.ExpectedInstanceIDs...), V2Readable: len(c.Keys) > 0, RollbackCompatible: c.V2RollbackCompatible}
	if hasCryptoRepo {
		if err = cryptoRepo.RegisterLifecycleCryptoReader(ctx, reader, c.V2WriteEnabled); err != nil {
			return err
		}
	}
	store, err := factory(ctx, &BackupS3Config{Endpoint: c.Endpoint, Region: c.Region, Bucket: c.Bucket, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, ForcePathStyle: c.ForcePathStyle})
	if err != nil {
		return ErrLifecycleStorage.WithCause(err)
	}
	s.lifecycle = &LifecycleRuntime{repo: repo, owner: s, cfg: c, store: store, key: key, keys: ring, cryptoRepo: cryptoRepo, reader: reader}
	return nil
}
func (s *WorkspaceService) LifecycleCapabilities() map[string]bool {
	return map[string]bool{"export_enabled": s.lifecycle != nil && s.lifecycle.cfg.Enabled, "purge_enabled": s.lifecycle != nil && s.lifecycle.cfg.PurgeEnabled}
}
func (s *WorkspaceService) lifecycleRepo() (WorkspaceLifecycleRepository, error) {
	r, ok := s.repo.(WorkspaceLifecycleRepository)
	if !ok {
		return nil, ErrLifecycleDisabled
	}
	return r, nil
}
func (s *WorkspaceService) StartLifecycleWorker() {
	r := s.lifecycle
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.stopped {
		return
	}
	r.started = true
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			if err := r.processExport(ctx); err != nil && ctx.Err() == nil && !errors.Is(err, ErrLifecycleLeaseLost) {
				slog.Warn("workspace export batch failed", "failure_code", "EXPORT_BATCH_FAILED")
			}
			cleanup, c := context.WithTimeout(ctx, 20*time.Second)
			err := r.repo.CleanupLifecycleExports(cleanup, r.store.Delete)
			c()
			if err != nil && ctx.Err() == nil {
				slog.Warn("workspace artifact cleanup failed", "failure_code", "EXPORT_CLEANUP_FAILED")
			}
			if r.cfg.PurgeEnabled {
				if err = r.processDeletion(ctx); err != nil && ctx.Err() == nil && !errors.Is(err, ErrLifecycleLeaseLost) {
					slog.Warn("workspace purge batch failed", "failure_code", "PURGE_BATCH_FAILED")
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
func (s *WorkspaceService) StopLifecycleWorker() {
	r := s.lifecycle
	if r == nil {
		return
	}
	r.mu.Lock()
	r.stopped = true
	if r.cancel != nil {
		r.cancel()
	}
	r.mu.Unlock()
	r.wg.Wait()
}
func (r *LifecycleRuntime) processExport(ctx context.Context) error {
	select {
	case lifecycleExportSlot <- struct{}{}:
		defer func() { <-lifecycleExportSlot }()
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	var job *LifecycleExportJob
	var err error
	if r.cryptoRepo != nil {
		job, err = r.cryptoRepo.ClaimLifecycleExportWithCrypto(ctx, r.reader, r.cfg.V2WriteEnabled)
	} else if r.cfg.V2WriteEnabled {
		return ErrLifecycleKeyRingNotReady
	} else {
		job, err = r.repo.ClaimLifecycleExport(ctx)
	}
	if err != nil || job == nil {
		return err
	}
	var plain bytes.Buffer
	result, buildErr := r.repo.WriteLifecycleExportSnapshot(ctx, job, &plain)
	if buildErr == nil {
		// Only encrypted bytes ever touch the temporary filesystem.
		file, e := os.CreateTemp("", "modurelay-export-*.enc")
		if e != nil {
			buildErr = e
		} else {
			path := file.Name()
			defer func() { _ = os.Remove(path) }()
			if r.cfg.V2WriteEnabled {
				buildErr = EncryptLifecycleArtifactV2(ctx, file, bytes.NewReader(plain.Bytes()), r.keys, job.ObjectKey)
			} else {
				buildErr = EncryptLifecycleArtifact(file, bytes.NewReader(plain.Bytes()), r.key, job.ObjectKey)
			}
			closeErr := file.Close()
			if buildErr == nil {
				buildErr = closeErr
			}
			if buildErr == nil {
				upload, c := context.WithTimeout(ctx, 30*time.Second)
				_, buildErr = r.store.UploadFile(upload, job.ObjectKey, path, "application/octet-stream")
				c()
			}
		}
	}
	finish, c := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer c()
	err = r.repo.FinishLifecycleExport(finish, job, result, buildErr)
	// Every attempt has a durable object ledger. Lease loss or an ambiguous DB
	// result never authorizes a second destructive path around holds and floors.
	// The bounded cleanup worker checks the current committed state before IO.
	return err
}
func (r *LifecycleRuntime) processDeletion(ctx context.Context) error {
	job, err := r.repo.ClaimLifecycleDeletion(ctx)
	if err != nil || job == nil {
		return err
	}
	err = r.repo.RunLifecyclePurgeBatch(ctx, job, 200)
	if err == nil {
		r.owner.invalidate(context.WithoutCancel(ctx), job.WorkspaceID)
	}
	release, c := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer c()
	e := r.repo.ReleaseLifecycleDeletion(release, job, err)
	if errors.Is(e, ErrLifecycleLeaseLost) && err == nil {
		return nil
	}
	return errors.Join(err, e)
}
func (s *WorkspaceService) CreateLifecycleExport(ctx context.Context, a, w int64) (*LifecycleExportJob, error) {
	if s.lifecycle == nil {
		return nil, ErrLifecycleDisabled
	}
	return s.lifecycle.repo.CreateLifecycleExport(ctx, a, w)
}
func (s *WorkspaceService) ListLifecycleExports(ctx context.Context, a, w int64, page, size int) ([]LifecycleExportJob, int64, error) {
	r, e := s.lifecycleRepo()
	if e != nil {
		return nil, 0, e
	}
	return r.ListLifecycleExports(ctx, a, w, page, size)
}
func (s *WorkspaceService) GetLifecycleExport(ctx context.Context, a, w int64, id string) (*LifecycleExportJob, error) {
	r, e := s.lifecycleRepo()
	if e != nil {
		return nil, e
	}
	return r.GetLifecycleExport(ctx, a, w, id)
}
func (s *WorkspaceService) CancelLifecycleExport(ctx context.Context, a, w int64, id string) error {
	r, e := s.lifecycleRepo()
	if e != nil {
		return e
	}
	return r.CancelLifecycleExport(ctx, a, w, id)
}
func (s *WorkspaceService) AuthorizeLifecycleDownload(ctx context.Context, a, w int64, id string) (*LifecycleDownloadGrant, error) {
	if s.lifecycle == nil {
		return nil, ErrLifecycleDisabled
	}
	return s.lifecycle.repo.AuthorizeLifecycleDownload(ctx, a, w, id)
}
func (s *WorkspaceService) DownloadLifecycleExport(ctx context.Context, a, w int64, id, token string) ([]byte, error) {
	r := s.lifecycle
	if r == nil {
		return nil, ErrLifecycleDisabled
	}
	select {
	case lifecycleDownloadSlots <- struct{}{}:
		defer func() { <-lifecycleDownloadSlots }()
	default:
		return nil, ErrLifecycleRateLimit
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	job, err := r.repo.RedeemLifecycleDownload(ctx, a, w, id, token)
	if err != nil {
		return nil, err
	}
	if !ValidLifecycleObjectKey(job.ObjectKey, w, id) {
		return nil, ErrWorkspaceConflict
	}
	body, err := r.store.Download(ctx, job.ObjectKey)
	if err != nil {
		return nil, ErrLifecycleStorage
	}
	defer func() { _ = body.Close() }()
	var plain bytes.Buffer
	limited := io.LimitReader(body, LifecycleMaxArtifactBytes+(4<<20))
	if r.keys != nil {
		err = DecryptLifecycleArtifactWithKeys(ctx, &plain, limited, r.keys, job.ObjectKey)
	} else {
		err = DecryptLifecycleArtifact(&plain, limited, r.key, job.ObjectKey)
	}
	if err != nil {
		return nil, ErrLifecycleStorage
	}
	sum := sha256.Sum256(plain.Bytes())
	if hex.EncodeToString(sum[:]) != job.ArtifactSHA256 || int64(plain.Len()) != job.SizeBytes {
		return nil, ErrLifecycleStorage
	}
	// Membership, policy and expiry may change while object storage is read.
	live, err := r.repo.GetLifecycleExport(ctx, a, w, id)
	if err != nil {
		return nil, err
	}
	if live.State != "completed" || (live.ExpiresAt != nil && !live.ExpiresAt.After(time.Now())) {
		return nil, ErrWorkspaceNotFound
	}
	return plain.Bytes(), nil
}
func (s *WorkspaceService) LifecyclePreflight(ctx context.Context, a, w int64) (*LifecyclePreflight, error) {
	r, e := s.lifecycleRepo()
	if e != nil {
		return nil, e
	}
	return r.LifecyclePreflight(ctx, a, w)
}
func (s *WorkspaceService) LifecycleDeletionChallenge(ctx context.Context, a, w int64) (*LifecycleChallenge, error) {
	if s.lifecycle == nil || !s.lifecycle.cfg.PurgeEnabled {
		return nil, ErrLifecycleDisabled
	}
	return s.lifecycle.repo.LifecycleDeletionChallenge(ctx, a, w)
}
func (s *WorkspaceService) RequestLifecycleDeletion(ctx context.Context, a, w int64, name, token string) (*LifecycleDeletionJob, error) {
	if s.lifecycle == nil || !s.lifecycle.cfg.PurgeEnabled {
		return nil, ErrLifecycleDisabled
	}
	j, e := s.lifecycle.repo.RequestLifecycleDeletion(ctx, a, w, name, token)
	if e == nil {
		s.invalidate(ctx, w)
	}
	return j, e
}
func (s *WorkspaceService) GetLifecycleDeletion(ctx context.Context, a, w int64) (*LifecycleDeletionJob, error) {
	r, e := s.lifecycleRepo()
	if e != nil {
		return nil, e
	}
	return r.GetLifecycleDeletion(ctx, a, w)
}
func (s *WorkspaceService) CancelLifecycleDeletion(ctx context.Context, a, w int64, id string) error {
	r, e := s.lifecycleRepo()
	if e != nil {
		return e
	}
	e = r.CancelLifecycleDeletion(ctx, a, w, id)
	if e == nil {
		s.invalidate(ctx, w)
	}
	return e
}
func (s *WorkspaceService) RetryLifecycleDeletion(ctx context.Context, a, w int64, id string) (*LifecycleDeletionJob, error) {
	if s.lifecycle == nil || !s.lifecycle.cfg.PurgeEnabled {
		return nil, ErrLifecycleDisabled
	}
	j, e := s.lifecycle.repo.RetryLifecycleDeletion(ctx, a, w, id)
	if e == nil {
		s.invalidate(ctx, w)
	}
	return j, e
}
