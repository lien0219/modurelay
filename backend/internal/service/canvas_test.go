package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateCanvasDocumentSupportsCurrentAndLegacySchema(t *testing.T) {
	runtime := CanvasRuntime{MaxDocumentBytes: 1 << 20, MaxNodes: 10}
	for _, version := range []int{canvasDocumentMinSchemaVersion, canvasDocumentSchemaVersion} {
		document := json.RawMessage(fmt.Sprintf(`{"schema_version":%d,"nodes":[],"edges":[],"viewport":{"x":0,"y":0,"zoom":1}}`, version))
		actual, size, err := validateCanvasDocument(document, runtime)
		require.NoError(t, err)
		require.Equal(t, version, actual)
		require.Equal(t, len(document), size)
	}

	for _, version := range []int{0, canvasDocumentSchemaVersion + 1} {
		document := json.RawMessage(fmt.Sprintf(`{"schema_version":%d,"nodes":[],"edges":[],"viewport":{"x":0,"y":0,"zoom":1}}`, version))
		_, _, err := validateCanvasDocument(document, runtime)
		require.Error(t, err)
	}
}

func TestDetectCanvasAssetContentTypeUsesFileSignature(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		declared string
		want     string
	}{
		{name: "png", data: []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, declared: "text/plain", want: "image/png"},
		{name: "mp4", data: []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, declared: "application/octet-stream", want: "video/mp4"},
		{name: "m4a", data: []byte{0, 0, 0, 20, 'f', 't', 'y', 'p', 'M', '4', 'A', ' '}, declared: "video/mp4", want: "audio/mp4"},
		{name: "video webm", data: []byte{0x1a, 0x45, 0xdf, 0xa3}, declared: "video/webm", want: "video/webm"},
		{name: "audio webm", data: []byte{0x1a, 0x45, 0xdf, 0xa3}, declared: "audio/webm", want: "audio/webm"},
		{name: "mp3 id3", data: []byte{'I', 'D', '3', 4}, declared: "application/octet-stream", want: "audio/mpeg"},
		{name: "wav", data: []byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'A', 'V', 'E'}, declared: "audio/wav", want: "audio/wav"},
		{name: "ogg", data: []byte{'O', 'g', 'g', 'S'}, declared: "audio/ogg", want: "audio/ogg"},
		{name: "aac", data: []byte{0xff, 0xf1, 0x50, 0x80}, declared: "audio/aac", want: "audio/aac"},
		{name: "flac", data: []byte{'f', 'L', 'a', 'C'}, declared: "audio/flac", want: "audio/flac"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := detectCanvasAssetContentType(tt.data, tt.declared)
			require.True(t, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDetectCanvasAssetContentTypeRejectsDeclaredSpoof(t *testing.T) {
	got, ok := detectCanvasAssetContentType([]byte("not a media file"), "video/mp4")
	require.False(t, ok)
	require.Empty(t, got)
}

func TestCanvasAssetExtension(t *testing.T) {
	require.Equal(t, ".mp4", canvasAssetExtension("video/mp4"))
	require.Equal(t, ".webm", canvasAssetExtension("video/webm"))
	require.Equal(t, ".mp3", canvasAssetExtension("audio/mpeg"))
	require.Equal(t, ".m4a", canvasAssetExtension("audio/mp4"))
	require.Equal(t, ".flac", canvasAssetExtension("audio/flac"))
}

type canvasCleanupRepoStub struct {
	CanvasProjectRepository
	assets      []*CanvasAsset
	claimCalled chan struct{}
	deleted     []int64
	retried     []int64
}

func (r *canvasCleanupRepoStub) ClaimAssetsForDeletion(context.Context, int, time.Duration) ([]*CanvasAsset, error) {
	if r.claimCalled != nil {
		select {
		case r.claimCalled <- struct{}{}:
		default:
		}
	}
	assets := r.assets
	r.assets = nil
	return assets, nil
}

func (r *canvasCleanupRepoStub) MarkAssetDeleted(_ context.Context, assetID int64) error {
	r.deleted = append(r.deleted, assetID)
	return nil
}

func (r *canvasCleanupRepoStub) RetryAssetDeletion(_ context.Context, assetID int64, _ time.Time) error {
	r.retried = append(r.retried, assetID)
	return nil
}

type canvasObjectStoreStub struct {
	deleteErrors map[string]error
	deleted      []string
}

func (s *canvasObjectStoreStub) Save(context.Context, string, string, []byte) (string, error) {
	return "", nil
}
func (s *canvasObjectStoreStub) Copy(context.Context, string, string) error { return nil }
func (s *canvasObjectStoreStub) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.deleteErrors[key]
}
func (s *canvasObjectStoreStub) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}
func (s *canvasObjectStoreStub) Private() bool { return true }

func TestCanvasCleanupIsolatesObjectDeletionFailures(t *testing.T) {
	repo := &canvasCleanupRepoStub{assets: []*CanvasAsset{
		{ID: 1, ObjectKey: "canvas/first.png"},
		{ID: 2, ObjectKey: "canvas/second.png"},
	}}
	store := &canvasObjectStoreStub{deleteErrors: map[string]error{"canvas/first.png": errors.New("storage unavailable")}}
	service := NewCanvasService(repo, nil, func() (CanvasObjectStore, bool) { return store, true }, nil)

	count, err := service.CleanupAssets(context.Background(), 50)

	require.NoError(t, err)
	require.Equal(t, 2, count)
	require.Equal(t, []string{"canvas/first.png", "canvas/second.png"}, store.deleted)
	require.Equal(t, []int64{1}, repo.retried)
	require.Equal(t, []int64{2}, repo.deleted)
}

func TestCanvasCleanupWorkerRunsImmediatelyAndStops(t *testing.T) {
	repo := &canvasCleanupRepoStub{claimCalled: make(chan struct{}, 1)}
	service := NewCanvasService(repo, nil, nil, nil)

	service.StartCleanup()
	select {
	case <-repo.claimCalled:
	case <-time.After(time.Second):
		t.Fatal("canvas cleanup worker did not process the initial backlog")
	}
	require.NotPanics(t, service.StopCleanup)
}
