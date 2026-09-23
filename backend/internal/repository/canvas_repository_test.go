package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCanvasClaimAssetsForDeletionReturnsQualifiedColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &canvasRepository{db: db}
	query := regexp.QuoteMeta("UPDATE canvas_assets a") + `(?s).*` + regexp.QuoteMeta("RETURNING "+canvasAssetColumnsAliased)
	mock.ExpectQuery(query).
		WithArgs(50, int64((2 * time.Minute).Milliseconds())).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "project_id", "user_id", "object_key", "status", "source",
			"source_task_id", "source_image_index", "idempotency_key", "file_name",
			"content_type", "size_bytes", "sha256", "width", "height", "delete_attempts",
			"delete_next_attempt_at", "deleted_at", "created_at", "updated_at",
		}))

	assets, err := repo.ClaimAssetsForDeletion(context.Background(), 50, 2*time.Minute)
	require.NoError(t, err)
	require.Empty(t, assets)
	require.NoError(t, mock.ExpectationsWereMet())
}
