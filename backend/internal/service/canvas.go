package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	SettingKeyCanvasConfig = "canvas_config"

	canvasDefaultMaxDocumentBytes  = 5 << 20
	canvasDefaultMaxNodes          = 500
	canvasDefaultMaxAssetBytes     = 32 << 20
	canvasDocumentMinSchemaVersion = 1
	canvasDocumentSchemaVersion    = 2
)

var (
	ErrCanvasDisabled = infraerrors.New(http.StatusNotFound, "CANVAS_DISABLED", "canvas is not enabled")
	ErrCanvasNotFound = infraerrors.New(http.StatusNotFound, "CANVAS_NOT_FOUND", "canvas project was not found")
	ErrCanvasConflict = infraerrors.New(http.StatusPreconditionFailed, "CANVAS_REVISION_CONFLICT", "canvas changed in another session; reload before saving")
	ErrCanvasInvalid  = infraerrors.New(http.StatusBadRequest, "CANVAS_INVALID_DOCUMENT", "canvas document is invalid")
	ErrCanvasStorage  = infraerrors.New(http.StatusServiceUnavailable, "CANVAS_ASSET_STORAGE_UNAVAILABLE", "private object storage is required for canvas assets")
	ErrCanvasAsset    = infraerrors.New(http.StatusNotFound, "CANVAS_ASSET_NOT_FOUND", "canvas asset was not found")
)

// CanvasRuntime is the administrator-controlled feature policy. Storage is
// evaluated separately so document sync can remain available during storage
// maintenance without exposing an unsafe asset URL.
type CanvasRuntime struct {
	Enabled          bool  `json:"enabled"`
	MaxDocumentBytes int   `json:"max_document_bytes"`
	MaxNodes         int   `json:"max_nodes"`
	MaxAssetBytes    int64 `json:"max_asset_bytes"`
}

func defaultCanvasRuntime() CanvasRuntime {
	return CanvasRuntime{
		Enabled:          false,
		MaxDocumentBytes: canvasDefaultMaxDocumentBytes,
		MaxNodes:         canvasDefaultMaxNodes,
		MaxAssetBytes:    canvasDefaultMaxAssetBytes,
	}
}

func normalizeCanvasRuntime(in CanvasRuntime) CanvasRuntime {
	if in.MaxDocumentBytes <= 0 || in.MaxDocumentBytes > 20<<20 {
		in.MaxDocumentBytes = canvasDefaultMaxDocumentBytes
	}
	if in.MaxNodes <= 0 || in.MaxNodes > 5000 {
		in.MaxNodes = canvasDefaultMaxNodes
	}
	if in.MaxAssetBytes <= 0 || in.MaxAssetBytes > 64<<20 {
		in.MaxAssetBytes = canvasDefaultMaxAssetBytes
	}
	return in
}

type CanvasProject struct {
	ID            int64           `json:"id"`
	UserID        int64           `json:"-"`
	Title         string          `json:"title"`
	Document      json.RawMessage `json:"document"`
	SchemaVersion int             `json:"schema_version"`
	Revision      int             `json:"revision"`
	DocumentBytes int             `json:"document_bytes"`
	LastOpenedAt  *time.Time      `json:"last_opened_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type CanvasRevision struct {
	ID            int64           `json:"id"`
	ProjectID     int64           `json:"project_id"`
	UserID        int64           `json:"-"`
	Revision      int             `json:"revision"`
	Source        string          `json:"source"`
	SchemaVersion int             `json:"schema_version"`
	Document      json.RawMessage `json:"document"`
	CreatedAt     time.Time       `json:"created_at"`
}

type CanvasAsset struct {
	ID               int64      `json:"id"`
	ProjectID        int64      `json:"project_id"`
	UserID           int64      `json:"-"`
	Status           string     `json:"status"`
	Source           string     `json:"source"`
	SourceTaskID     string     `json:"source_task_id,omitempty"`
	SourceImageIndex *int       `json:"source_image_index,omitempty"`
	FileName         string     `json:"file_name"`
	ContentType      string     `json:"content_type"`
	SizeBytes        int64      `json:"size_bytes"`
	SHA256           string     `json:"sha256"`
	Width            *int       `json:"width,omitempty"`
	Height           *int       `json:"height,omitempty"`
	ObjectKey        string     `json:"-"`
	IdempotencyKey   string     `json:"-"`
	DeleteAttempts   int        `json:"-"`
	DeleteNextAt     *time.Time `json:"-"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type CanvasAssetUpload struct {
	FileName    string
	ContentType string
	Body        io.Reader
	Size        int64
}

type CanvasProjectRepository interface {
	ListProjects(ctx context.Context, userID int64, limit int) ([]*CanvasProject, error)
	CreateProject(ctx context.Context, project *CanvasProject) (*CanvasProject, error)
	GetProject(ctx context.Context, userID, projectID int64) (*CanvasProject, error)
	UpdateProject(ctx context.Context, userID, projectID int64, expectedRevision int, title string, document json.RawMessage, schemaVersion, documentBytes int) (*CanvasProject, bool, error)
	DeleteProject(ctx context.Context, userID, projectID int64) (bool, error)
	CreateRevision(ctx context.Context, revision *CanvasRevision, documentBytes int) (*CanvasRevision, error)
	ListRevisions(ctx context.Context, userID, projectID int64, limit int) ([]*CanvasRevision, error)
	GetRevision(ctx context.Context, userID, projectID, revisionID int64) (*CanvasRevision, error)
	CreateAsset(ctx context.Context, asset *CanvasAsset) (*CanvasAsset, error)
	FindAssetByIdempotency(ctx context.Context, userID int64, idempotencyKey string) (*CanvasAsset, error)
	MarkAssetReady(ctx context.Context, userID, assetID int64, objectKey, contentType string, sizeBytes int64, checksum string, width, height *int) (*CanvasAsset, error)
	MarkAssetFailed(ctx context.Context, userID, assetID int64) error
	GetAsset(ctx context.Context, userID, assetID int64) (*CanvasAsset, error)
	RequestAssetDeletion(ctx context.Context, userID, assetID int64) (bool, error)
	ClaimAssetsForDeletion(ctx context.Context, limit int, lease time.Duration) ([]*CanvasAsset, error)
	MarkAssetDeleted(ctx context.Context, assetID int64) error
	RetryAssetDeletion(ctx context.Context, assetID int64, nextAt time.Time) error
}

// CanvasObjectStore is deliberately private. A public_base_url-backed image
// store may serve gateway task results, but cannot be used for user projects.
type CanvasObjectStore interface {
	ImageStorage
	Copy(ctx context.Context, sourceKey, destinationKey string) error
	Delete(ctx context.Context, key string) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	Private() bool
}

type CanvasStoreResolver func() (CanvasObjectStore, bool)

type CanvasService struct {
	repo          CanvasProjectRepository
	settingRepo   SettingRepository
	storeResolver CanvasStoreResolver
	imageTasks    *ImageTaskService
	httpUpstream  HTTPUpstream

	modelsListReadMaxBytes       int64
	providerResponseReadMaxBytes int64
	resolveModelHost             func(context.Context, string) (context.Context, error)

	mu       sync.Mutex
	cachedAt time.Time
	cached   CanvasRuntime

	cleanupMu     sync.Mutex
	cleanupCancel context.CancelFunc
	cleanupDone   chan struct{}
}

func NewCanvasService(repo CanvasProjectRepository, settingRepo SettingRepository, storeResolver CanvasStoreResolver, imageTasks *ImageTaskService) *CanvasService {
	return &CanvasService{repo: repo, settingRepo: settingRepo, storeResolver: storeResolver, imageTasks: imageTasks}
}

func (s *CanvasService) StartCleanup() {
	if s == nil || s.repo == nil {
		return
	}
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()
	if s.cleanupCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cleanupCancel = cancel
	s.cleanupDone = make(chan struct{})
	go s.runCleanupLoop(ctx, s.cleanupDone)
}

func (s *CanvasService) StopCleanup() {
	if s == nil {
		return
	}
	s.cleanupMu.Lock()
	cancel := s.cleanupCancel
	done := s.cleanupDone
	s.cleanupCancel = nil
	s.cleanupDone = nil
	s.cleanupMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (s *CanvasService) runCleanupLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.runCleanupOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *CanvasService) runCleanupOnce(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	count, err := s.CleanupAssets(ctx, 50)
	if err != nil {
		logger.LegacyPrintf("service.canvas_cleanup", "[CanvasCleanup] cleanup failed err=%v", err)
		return
	}
	if count > 0 {
		logger.LegacyPrintf("service.canvas_cleanup", "[CanvasCleanup] processed pending assets count=%d", count)
	}
}

func (s *CanvasService) Runtime(ctx context.Context) (CanvasRuntime, error) {
	if s == nil || s.settingRepo == nil {
		return defaultCanvasRuntime(), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cachedAt.IsZero() && time.Since(s.cachedAt) < time.Minute {
		return s.cached, nil
	}
	runtime := defaultCanvasRuntime()
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyCanvasConfig)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return runtime, err
	}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &runtime); err != nil {
			return defaultCanvasRuntime(), fmt.Errorf("parse canvas settings: %w", err)
		}
	}
	// The public/admin feature flag is the operational enablement switch.
	// CanvasRuntime may carry limits in its private JSON setting, but a missing
	// runtime record must still honor the explicit canvas_enabled setting.
	enabledRaw, enabledErr := s.settingRepo.GetValue(ctx, SettingKeyCanvasEnabled)
	if enabledErr != nil {
		// The feature flag is fail-closed. Older deployments without the key
		// must run the migration before exposing a canvas, even if a stale
		// canvas_config record exists.
		runtime.Enabled = false
	} else {
		runtime.Enabled = strings.EqualFold(strings.TrimSpace(enabledRaw), "true")
	}
	s.cached = normalizeCanvasRuntime(runtime)
	s.cachedAt = time.Now()
	return s.cached, nil
}

func (s *CanvasService) UpdateRuntime(ctx context.Context, runtime CanvasRuntime) (CanvasRuntime, error) {
	if s == nil || s.settingRepo == nil {
		return CanvasRuntime{}, errors.New("canvas settings repository is unavailable")
	}
	runtime = normalizeCanvasRuntime(runtime)
	raw, err := json.Marshal(runtime)
	if err != nil {
		return CanvasRuntime{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyCanvasConfig, string(raw)); err != nil {
		return CanvasRuntime{}, err
	}
	s.mu.Lock()
	s.cached = runtime
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return runtime, nil
}

func (s *CanvasService) requireEnabled(ctx context.Context) (CanvasRuntime, error) {
	runtime, err := s.Runtime(ctx)
	if err != nil {
		return CanvasRuntime{}, err
	}
	if !runtime.Enabled {
		return CanvasRuntime{}, ErrCanvasDisabled
	}
	return runtime, nil
}

func (s *CanvasService) ListProjects(ctx context.Context, userID int64) ([]*CanvasProject, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListProjects(ctx, userID, 100)
}

func (s *CanvasService) CreateProject(ctx context.Context, userID int64, title string) (*CanvasProject, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	title = normalizeCanvasTitle(title)
	document := defaultCanvasDocument()
	return s.repo.CreateProject(ctx, &CanvasProject{UserID: userID, Title: title, Document: document, SchemaVersion: canvasDocumentSchemaVersion, Revision: 1, DocumentBytes: len(document)})
}

func (s *CanvasService) GetProject(ctx context.Context, userID, projectID int64) (*CanvasProject, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	project, err := s.repo.GetProject(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	return project, nil
}

func (s *CanvasService) SaveProject(ctx context.Context, userID, projectID int64, expectedRevision int, title string, document json.RawMessage) (*CanvasProject, error) {
	runtime, err := s.requireEnabled(ctx)
	if err != nil {
		return nil, err
	}
	schemaVersion, bytes, err := validateCanvasDocument(document, runtime)
	if err != nil {
		return nil, err
	}
	project, conflict, err := s.repo.UpdateProject(ctx, userID, projectID, expectedRevision, normalizeCanvasTitle(title), document, schemaVersion, bytes)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrCanvasConflict
	}
	if project == nil {
		return nil, ErrCanvasNotFound
	}
	return project, nil
}

func (s *CanvasService) DeleteProject(ctx context.Context, userID, projectID int64) error {
	if _, err := s.requireEnabled(ctx); err != nil {
		return err
	}
	deleted, err := s.repo.DeleteProject(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrCanvasNotFound
	}
	return nil
}

func (s *CanvasService) CreateCheckpoint(ctx context.Context, userID, projectID int64) (*CanvasRevision, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	project, err := s.repo.GetProject(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateRevision(ctx, &CanvasRevision{ProjectID: project.ID, UserID: userID, Revision: project.Revision, Source: "manual", SchemaVersion: project.SchemaVersion, Document: project.Document}, project.DocumentBytes)
}

func (s *CanvasService) ListRevisions(ctx context.Context, userID, projectID int64) ([]*CanvasRevision, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	return s.repo.ListRevisions(ctx, userID, projectID, 50)
}

func (s *CanvasService) RestoreRevision(ctx context.Context, userID, projectID, revisionID, expectedRevision int64) (*CanvasProject, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	revision, err := s.repo.GetRevision(ctx, userID, projectID, revisionID)
	if err != nil {
		return nil, err
	}
	project, conflict, err := s.repo.UpdateProject(ctx, userID, projectID, int(expectedRevision), "", revision.Document, revision.SchemaVersion, len(revision.Document))
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, ErrCanvasConflict
	}
	if project == nil {
		return nil, ErrCanvasNotFound
	}
	_, _ = s.repo.CreateRevision(ctx, &CanvasRevision{ProjectID: project.ID, UserID: userID, Revision: project.Revision, Source: "restore", SchemaVersion: project.SchemaVersion, Document: project.Document}, project.DocumentBytes)
	return project, nil
}

func (s *CanvasService) UploadAsset(ctx context.Context, userID, projectID int64, idempotencyKey string, upload CanvasAssetUpload) (*CanvasAsset, error) {
	runtime, err := s.requireEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.GetProject(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if existing, err := s.findIdempotentAsset(ctx, userID, idempotencyKey); err != nil || existing != nil {
		if err == nil && (existing.ProjectID != projectID || existing.Source != "upload") {
			return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "idempotency_key"})
		}
		return existing, err
	}
	if upload.Size <= 0 || upload.Size > runtime.MaxAssetBytes || upload.Body == nil {
		return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "asset"})
	}
	store, err := s.privateStore()
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(io.LimitReader(upload.Body, runtime.MaxAssetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read canvas asset: %w", err)
	}
	if int64(len(data)) > runtime.MaxAssetBytes {
		return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "asset"})
	}
	contentType, supported := detectCanvasAssetContentType(data, upload.ContentType)
	if !supported {
		return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "content_type"})
	}
	checksum := sha256.Sum256(data)
	asset, err := s.repo.CreateAsset(ctx, &CanvasAsset{ProjectID: projectID, UserID: userID, Status: "uploading", Source: "upload", IdempotencyKey: idempotencyKey, FileName: normalizeCanvasFileName(upload.FileName), ContentType: contentType, SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(checksum[:])})
	if err != nil {
		return nil, err
	}
	key := canvasAssetObjectKey(userID, projectID, asset.ID, contentType)
	if _, err := store.Save(ctx, key, contentType, data); err != nil {
		deleteCanvasObjectBestEffort(store, key)
		_ = s.repo.MarkAssetFailed(context.Background(), userID, asset.ID)
		return nil, fmt.Errorf("store canvas asset: %w", err)
	}
	ready, err := s.repo.MarkAssetReady(ctx, userID, asset.ID, key, contentType, int64(len(data)), hex.EncodeToString(checksum[:]), nil, nil)
	if err != nil {
		deleteCanvasObjectBestEffort(store, key)
		_ = s.repo.MarkAssetFailed(context.Background(), userID, asset.ID)
		return nil, err
	}
	return ready, nil
}

func (s *CanvasService) PromoteTaskAsset(ctx context.Context, userID, projectID int64, taskID string, imageIndex int, idempotencyKey string) (*CanvasAsset, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	if _, err := s.repo.GetProject(ctx, userID, projectID); err != nil {
		return nil, err
	}
	if existing, err := s.findIdempotentAsset(ctx, userID, idempotencyKey); err != nil || existing != nil {
		matchesIndex := existing != nil && existing.SourceImageIndex != nil && *existing.SourceImageIndex == imageIndex
		if err == nil && (existing.ProjectID != projectID || existing.Source != "generation" || existing.SourceTaskID != taskID || !matchesIndex) {
			return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "idempotency_key"})
		}
		return existing, err
	}
	if s.imageTasks == nil {
		return nil, ErrCanvasStorage
	}
	stored, err := s.imageTasks.StoredAssetsForUser(ctx, userID, taskID)
	if err != nil {
		return nil, err
	}
	if imageIndex < 0 || imageIndex >= len(stored) {
		return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "image_index"})
	}
	source := stored[imageIndex]
	if source.ObjectKey == "" || !isCanvasImageType(source.ContentType) {
		return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "asset"})
	}
	store, err := s.privateStore()
	if err != nil {
		return nil, err
	}
	index := imageIndex
	asset, err := s.repo.CreateAsset(ctx, &CanvasAsset{ProjectID: projectID, UserID: userID, Status: "uploading", Source: "generation", SourceTaskID: taskID, SourceImageIndex: &index, IdempotencyKey: idempotencyKey, FileName: "generated" + canvasAssetExtension(source.ContentType), ContentType: source.ContentType, SizeBytes: source.SizeBytes})
	if err != nil {
		return nil, err
	}
	key := canvasAssetObjectKey(userID, projectID, asset.ID, source.ContentType)
	if err := store.Copy(ctx, source.ObjectKey, key); err != nil {
		deleteCanvasObjectBestEffort(store, key)
		_ = s.repo.MarkAssetFailed(context.Background(), userID, asset.ID)
		return nil, fmt.Errorf("copy generated canvas asset: %w", err)
	}
	ready, err := s.repo.MarkAssetReady(ctx, userID, asset.ID, key, source.ContentType, source.SizeBytes, source.SHA256, nil, nil)
	if err != nil {
		deleteCanvasObjectBestEffort(store, key)
		_ = s.repo.MarkAssetFailed(context.Background(), userID, asset.ID)
		return nil, err
	}
	return ready, nil
}

func (s *CanvasService) AssetAccessURL(ctx context.Context, userID, assetID int64) (string, error) {
	if _, err := s.requireEnabled(ctx); err != nil {
		return "", err
	}
	asset, err := s.repo.GetAsset(ctx, userID, assetID)
	if err != nil {
		return "", err
	}
	if asset.Status != "ready" || asset.ObjectKey == "" {
		return "", ErrCanvasAsset
	}
	store, err := s.privateStore()
	if err != nil {
		return "", err
	}
	return store.PresignGet(ctx, asset.ObjectKey, 15*time.Minute)
}

func (s *CanvasService) DeleteAsset(ctx context.Context, userID, assetID int64) error {
	if _, err := s.requireEnabled(ctx); err != nil {
		return err
	}
	deleted, err := s.repo.RequestAssetDeletion(ctx, userID, assetID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrCanvasAsset
	}
	return nil
}

func (s *CanvasService) CleanupAssets(ctx context.Context, limit int) (int, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	assets, err := s.repo.ClaimAssetsForDeletion(ctx, limit, 2*time.Minute)
	if err != nil {
		return 0, err
	}
	if len(assets) == 0 {
		return 0, nil
	}
	store, resolverErr := s.privateStore()
	for _, asset := range assets {
		deleteErr := resolverErr
		if deleteErr == nil && asset.ObjectKey != "" {
			deleteErr = store.Delete(ctx, asset.ObjectKey)
		}
		if deleteErr == nil {
			_ = s.repo.MarkAssetDeleted(ctx, asset.ID)
			continue
		}
		backoff := time.Duration(minCanvasInt(asset.DeleteAttempts+1, 8)) * time.Minute
		_ = s.repo.RetryAssetDeletion(ctx, asset.ID, time.Now().UTC().Add(backoff))
	}
	return len(assets), nil
}

func deleteCanvasObjectBestEffort(store CanvasObjectStore, key string) {
	if store == nil || key == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = store.Delete(ctx, key)
}

func (s *CanvasService) findIdempotentAsset(ctx context.Context, userID int64, key string) (*CanvasAsset, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	if len(key) > 128 {
		return nil, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "idempotency_key"})
	}
	return s.repo.FindAssetByIdempotency(ctx, userID, key)
}

func (s *CanvasService) privateStore() (CanvasObjectStore, error) {
	if s == nil || s.storeResolver == nil {
		return nil, ErrCanvasStorage
	}
	store, enabled := s.storeResolver()
	if !enabled || store == nil || !store.Private() {
		return nil, ErrCanvasStorage
	}
	return store, nil
}

func defaultCanvasDocument() json.RawMessage {
	return json.RawMessage(`{"schema_version":1,"nodes":[],"edges":[],"viewport":{"x":0,"y":0,"zoom":1}}`)
}

func normalizeCanvasTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled canvas"
	}
	runes := []rune(title)
	if len(runes) > 160 {
		return string(runes[:160])
	}
	return title
}

func normalizeCanvasFileName(name string) string {
	name = path.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == "" {
		return "asset"
	}
	runes := []rune(name)
	if len(runes) > 255 {
		return string(runes[:255])
	}
	return name
}

func validateCanvasDocument(raw json.RawMessage, runtime CanvasRuntime) (int, int, error) {
	if len(raw) == 0 || len(raw) > runtime.MaxDocumentBytes || !json.Valid(raw) {
		return 0, 0, ErrCanvasInvalid
	}
	var doc struct {
		SchemaVersion int `json:"schema_version"`
		Nodes         []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"nodes"`
		Edges []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"edges"`
		Viewport json.RawMessage `json:"viewport"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0, 0, ErrCanvasInvalid
	}
	if doc.SchemaVersion < canvasDocumentMinSchemaVersion || doc.SchemaVersion > canvasDocumentSchemaVersion || doc.Nodes == nil || doc.Edges == nil || len(doc.Nodes) > runtime.MaxNodes || len(doc.Edges) > runtime.MaxNodes*4 || len(doc.Viewport) == 0 {
		return 0, 0, ErrCanvasInvalid
	}
	ids := make(map[string]struct{}, len(doc.Nodes))
	for _, node := range doc.Nodes {
		if len(node.ID) == 0 || len(node.ID) > 128 || !isCanvasNodeType(node.Type) {
			return 0, 0, ErrCanvasInvalid
		}
		if _, found := ids[node.ID]; found {
			return 0, 0, ErrCanvasInvalid
		}
		ids[node.ID] = struct{}{}
	}
	for _, edge := range doc.Edges {
		if len(edge.ID) == 0 || len(edge.ID) > 128 || edge.Source == edge.Target {
			return 0, 0, ErrCanvasInvalid
		}
		if _, source := ids[edge.Source]; !source {
			return 0, 0, ErrCanvasInvalid
		}
		if _, target := ids[edge.Target]; !target {
			return 0, 0, ErrCanvasInvalid
		}
	}
	var anyDocument any
	_ = json.Unmarshal(raw, &anyDocument)
	if canvasContainsSecretKey(anyDocument) {
		return 0, 0, ErrCanvasInvalid.WithMetadata(map[string]string{"field": "credential"})
	}
	return doc.SchemaVersion, len(raw), nil
}

func canvasContainsSecretKey(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if isCanvasSecretKey(key) || canvasContainsSecretKey(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if canvasContainsSecretKey(child) {
				return true
			}
		}
	}
	return false
}

func isCanvasSecretKey(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	return strings.Contains(key, "apikey") || strings.Contains(key, "authorization") || strings.Contains(key, "credential") || strings.Contains(key, "secret") || strings.Contains(key, "accesstoken")
}

func isCanvasNodeType(kind string) bool {
	switch kind {
	case "prompt", "reference", "generation", "image", "text", "video", "audio", "config", "group":
		return true
	default:
		return false
	}
}
func isCanvasImageType(contentType string) bool {
	switch contentType {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
		return true
	default:
		return false
	}
}

func detectCanvasAssetContentType(data []byte, declared string) (string, bool) {
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	if isCanvasImageType(detected) {
		return detected, true
	}

	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		brand := string(data[8:12])
		switch brand {
		case "M4A ", "M4B ", "M4P ", "F4A ", "F4B ":
			return "audio/mp4", true
		case "isom", "iso2", "avc1", "mp41", "mp42", "dash", "M4V ", "MSNV":
			return "video/mp4", true
		}
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x1a, 0x45, 0xdf, 0xa3}) {
		declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
		if strings.HasPrefix(declared, "audio/") {
			return "audio/webm", true
		}
		return "video/webm", true
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return "audio/wav", true
	}
	if len(data) >= 4 && string(data[:4]) == "OggS" {
		return "audio/ogg", true
	}
	if len(data) >= 4 && string(data[:4]) == "fLaC" {
		return "audio/flac", true
	}
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return "audio/mpeg", true
	}
	if len(data) >= 2 && data[0] == 0xff {
		if data[1]&0xf6 == 0xf0 {
			return "audio/aac", true
		}
		if data[1]&0xe0 == 0xe0 && data[1]&0x06 != 0 {
			return "audio/mpeg", true
		}
	}
	if len(data) >= 4 && string(data[:4]) == "ADIF" {
		return "audio/aac", true
	}
	return "", false
}

func canvasAssetExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav":
		return ".wav"
	case "audio/ogg", "audio/opus":
		return ".ogg"
	case "audio/mp4":
		return ".m4a"
	case "audio/aac":
		return ".aac"
	case "audio/flac":
		return ".flac"
	default:
		return ".png"
	}
}
func canvasAssetObjectKey(userID, projectID, assetID int64, contentType string) string {
	return fmt.Sprintf("canvas/%d/%d/%d/%s%s", userID, projectID, assetID, uuid.NewString(), canvasAssetExtension(contentType))
}
func minCanvasInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
