//go:build integration

package repository

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestLifecyclePostgresS3MixedKeyRecoveryAndProtectedObjects(t *testing.T) {
	ctx, repo, owner, w := lifecycleFixture(t)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{
		Image: "quay.io/minio/minio:latest", ExposedPorts: []string{"9000/tcp"}, Cmd: []string{"server", "/data"},
		Env:        map[string]string{"MINIO_ROOT_USER": "phase-i-fixture", "MINIO_ROOT_PASSWORD": "phase-i-disposable-fixture"},
		WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp").WithStartupTimeout(time.Minute),
	}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	endpoint, err := container.Endpoint(ctx, "http")
	require.NoError(t, err)
	c := config.DataLifecycleConfig{Enabled: true, Endpoint: endpoint, Region: "us-east-1", Bucket: "phase-i-exports", AccessKeyID: "phase-i-fixture", SecretAccessKey: "phase-i-disposable-fixture", ForcePathStyle: true, EncryptionKey: strings.Repeat("07", 32), ActiveKeyID: "key-new", Keys: []config.LifecycleEncryptionKey{{ID: "key-old", Version: 1, Status: "decrypt_only", Key: strings.Repeat("08", 32)}, {ID: "key-new", Version: 2, Status: "active", Key: strings.Repeat("09", 32)}}}
	client, err := newS3Client(ctx, s3ClientParams{Endpoint: endpoint, Region: c.Region, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, ForcePathStyle: true})
	require.NoError(t, err)
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(c.Bucket)})
	require.NoError(t, err)
	factory := NewS3BackupStoreFactory()
	store, err := factory(ctx, &service.BackupS3Config{Endpoint: endpoint, Region: c.Region, Bucket: c.Bucket, AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey, ForcePathStyle: true})
	require.NoError(t, err)
	ring, err := service.NewLifecycleKeyRing(c)
	require.NoError(t, err)
	svc := service.NewWorkspaceService(repo)
	require.NoError(t, svc.ConfigureLifecycle(&config.Config{DataLifecycle: c}, factory))
	var objects []string
	for _, version := range []int{1, 2} {
		job, err := repo.CreateLifecycleExport(ctx, owner.ID, w.ID)
		require.NoError(t, err)
		claim, err := repo.ClaimLifecycleExport(ctx)
		require.NoError(t, err)
		require.Equal(t, job.ID, claim.ID)
		var plain, encrypted bytes.Buffer
		result, err := repo.WriteLifecycleExportSnapshot(ctx, claim, &plain)
		require.NoError(t, err)
		if version == 1 {
			err = service.EncryptLifecycleArtifact(&encrypted, bytes.NewReader(plain.Bytes()), bytes.Repeat([]byte{7}, 32), claim.ObjectKey)
		} else {
			err = service.EncryptLifecycleArtifactV2(ctx, &encrypted, bytes.NewReader(plain.Bytes()), ring, claim.ObjectKey)
		}
		require.NoError(t, err)
		path := filepath.Join(t.TempDir(), "artifact.enc")
		require.NoError(t, os.WriteFile(path, encrypted.Bytes(), 0600))
		_, err = store.UploadFile(ctx, claim.ObjectKey, path, "application/octet-stream")
		require.NoError(t, err)
		require.NoError(t, repo.FinishLifecycleExport(ctx, claim, result, nil))
		grant, err := svc.AuthorizeLifecycleDownload(ctx, owner.ID, w.ID, job.ID)
		require.NoError(t, err)
		read, err := svc.DownloadLifecycleExport(ctx, owner.ID, w.ID, job.ID, grant.Token)
		require.NoError(t, err)
		require.Equal(t, plain.Bytes(), read)
		objects = append(objects, claim.ObjectKey)
		// Lost upload acknowledgement leaves a real orphan, with no authority to
		// destroy it or retire its key merely because the queue is empty.
		orphan, err := service.LifecycleObjectKey(w.ID, job.ID, uuid.NewString())
		require.NoError(t, err)
		var orphanCipher bytes.Buffer
		require.NoError(t, service.EncryptLifecycleArtifactV2(ctx, &orphanCipher, strings.NewReader("orphan recovery"), ring, orphan))
		_, err = store.Upload(ctx, orphan, bytes.NewReader(orphanCipher.Bytes()), "application/octet-stream")
		require.NoError(t, err)
		// A truncated object simulates interrupted encrypted upload. The actual
		// authenticated Owner download returns zero bytes, including for V1.
		_, err = store.Upload(ctx, claim.ObjectKey, bytes.NewReader(encrypted.Bytes()[:encrypted.Len()-1]), "application/octet-stream")
		require.NoError(t, err)
		grant, err = svc.AuthorizeLifecycleDownload(ctx, owner.ID, w.ID, job.ID)
		require.NoError(t, err)
		read, err = svc.DownloadLifecycleExport(ctx, owner.ID, w.ID, job.ID, grant.Token)
		require.ErrorIs(t, err, service.ErrLifecycleStorage)
		require.Empty(t, read)
		_, err = store.Upload(ctx, claim.ObjectKey, bytes.NewReader(encrypted.Bytes()), "application/octet-stream")
		require.NoError(t, err)
		body, err := store.Download(ctx, orphan)
		require.NoError(t, err)
		var restored bytes.Buffer
		require.NoError(t, service.DecryptLifecycleArtifactWithKeys(ctx, &restored, body, ring, orphan))
		require.NoError(t, body.Close())
		require.Equal(t, "orphan recovery", restored.String())
	}
	_, err = integrationDB.Exec(`UPDATE workspace_export_objects SET cleanup_after=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	require.NoError(t, repo.CleanupLifecycleExports(ctx, store.Delete))
	for _, key := range objects {
		body, err := store.Download(ctx, key)
		require.NoError(t, err)
		require.NoError(t, body.Close())
	}
	_, err = integrationDB.Exec(`INSERT INTO workspace_lifecycle_holds(workspace_id,code,reason) VALUES($1,'BUSINESS_HOLD','Isolated recovery rehearsal')`, w.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspace_export_jobs SET completed_at=clock_timestamp()-interval '8 days'; UPDATE workspace_export_objects SET cleanup_after=clock_timestamp()-interval '1 second'`)
	require.NoError(t, err)
	for range objects {
		require.NoError(t, repo.CleanupLifecycleExports(ctx, store.Delete))
	}
	for _, key := range objects {
		body, err := store.Download(ctx, key)
		require.NoError(t, err)
		data, err := io.ReadAll(io.LimitReader(body, service.LifecycleMaxArtifactBytes+(4<<20)))
		require.NoError(t, err)
		require.NoError(t, body.Close())
		var read bytes.Buffer
		require.NoError(t, service.DecryptLifecycleArtifactWithKeys(ctx, &read, bytes.NewReader(data), ring, key))
		require.NotEmpty(t, read.Bytes())
	}
}
