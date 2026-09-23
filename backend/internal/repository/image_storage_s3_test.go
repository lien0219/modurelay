package repository

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestS3ImageStoragePresignsWithPublicEndpoint(t *testing.T) {
	storage, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{
		Endpoint:        "http://minio.internal:9000",
		PublicEndpoint:  "https://assets.example.com",
		Region:          "us-east-1",
		Bucket:          "private-images",
		AccessKeyID:     "test-ak",
		SecretAccessKey: "test-sk",
		ForcePathStyle:  true,
	})
	require.NoError(t, err)

	rawURL, err := storage.PresignGet(context.Background(), "canvas/user/project/image.png", 15*time.Minute)
	require.NoError(t, err)

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	require.Equal(t, "assets.example.com", parsed.Host)
	require.Equal(t, "/private-images/canvas/user/project/image.png", parsed.Path)
	require.NotEmpty(t, parsed.Query().Get("X-Amz-Signature"))
	require.NotEmpty(t, parsed.Query().Get("X-Amz-Expires"))
}
