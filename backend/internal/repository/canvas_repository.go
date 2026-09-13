package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// canvasRepository keeps all user ownership predicates in SQL. This is a
// deliberate defense-in-depth boundary: handlers never query a project or an
// asset by ID alone.
type canvasRepository struct{ db *sql.DB }

func NewCanvasRepository(db *sql.DB) service.CanvasProjectRepository {
	return &canvasRepository{db: db}
}

const canvasProjectColumns = `id, user_id, title, document::text, schema_version, revision, document_bytes, last_opened_at, created_at, updated_at`
const canvasAssetColumns = `id, project_id, user_id, object_key, status, source, source_task_id, source_image_index, idempotency_key, file_name, content_type, size_bytes, sha256, width, height, delete_attempts, delete_next_attempt_at, deleted_at, created_at, updated_at`
const canvasAssetColumnsAliased = `a.id, a.project_id, a.user_id, a.object_key, a.status, a.source, a.source_task_id, a.source_image_index, a.idempotency_key, a.file_name, a.content_type, a.size_bytes, a.sha256, a.width, a.height, a.delete_attempts, a.delete_next_attempt_at, a.deleted_at, a.created_at, a.updated_at`

func scanCanvasProject(scan func(...any) error) (*service.CanvasProject, error) {
	var project service.CanvasProject
	var raw string
	var opened sql.NullTime
	if err := scan(&project.ID, &project.UserID, &project.Title, &raw, &project.SchemaVersion, &project.Revision, &project.DocumentBytes, &opened, &project.CreatedAt, &project.UpdatedAt); err != nil {
		return nil, err
	}
	project.Document = json.RawMessage(raw)
	if opened.Valid {
		value := opened.Time
		project.LastOpenedAt = &value
	}
	return &project, nil
}

func (r *canvasRepository) ListProjects(ctx context.Context, userID int64, limit int) ([]*service.CanvasProject, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+canvasProjectColumns+` FROM canvas_projects WHERE user_id=$1 AND deleted_at IS NULL ORDER BY updated_at DESC, id DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]*service.CanvasProject, 0)
	for rows.Next() {
		project, err := scanCanvasProject(rows.Scan)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (r *canvasRepository) CreateProject(ctx context.Context, project *service.CanvasProject) (*service.CanvasProject, error) {
	if r == nil || r.db == nil || project == nil {
		return nil, errors.New("invalid canvas project repository request")
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO canvas_projects (user_id,title,document,schema_version,revision,document_bytes,last_opened_at) VALUES ($1,$2,$3::jsonb,$4,$5,$6,NOW()) RETURNING `+canvasProjectColumns, project.UserID, project.Title, string(project.Document), project.SchemaVersion, project.Revision, project.DocumentBytes)
	return scanCanvasProject(row.Scan)
}

func (r *canvasRepository) GetProject(ctx context.Context, userID, projectID int64) (*service.CanvasProject, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+canvasProjectColumns+` FROM canvas_projects WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, projectID, userID)
	project, err := scanCanvasProject(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCanvasNotFound
	}
	return project, err
}

func (r *canvasRepository) UpdateProject(ctx context.Context, userID, projectID int64, expectedRevision int, title string, document json.RawMessage, schemaVersion, documentBytes int) (*service.CanvasProject, bool, error) {
	if r == nil || r.db == nil {
		return nil, false, errors.New("nil canvas repository")
	}
	if expectedRevision <= 0 {
		return nil, false, service.ErrCanvasConflict
	}
	row := r.db.QueryRowContext(ctx, `
UPDATE canvas_projects
SET title=CASE WHEN $4='' THEN title ELSE $4 END,
    document=$5::jsonb,
    schema_version=$6,
    document_bytes=$7,
    revision=revision+1,
    last_opened_at=NOW(),
    updated_at=NOW()
WHERE id=$1 AND user_id=$2 AND revision=$3 AND deleted_at IS NULL
RETURNING `+canvasProjectColumns, projectID, userID, expectedRevision, title, string(document), schemaVersion, documentBytes)
	project, err := scanCanvasProject(row.Scan)
	if !errors.Is(err, sql.ErrNoRows) {
		return project, false, err
	}
	var exists bool
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM canvas_projects WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL)`, projectID, userID).Scan(&exists); err != nil {
		return nil, false, err
	}
	if !exists {
		return nil, false, nil
	}
	return nil, true, nil
}

func (r *canvasRepository) DeleteProject(ctx context.Context, userID, projectID int64) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("nil canvas repository")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE canvas_projects SET deleted_at=NOW(), updated_at=NOW() WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`, projectID, userID)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if changed == 0 {
		return false, tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE canvas_assets SET status='delete_pending', delete_next_attempt_at=NOW(), updated_at=NOW() WHERE project_id=$1 AND user_id=$2 AND status IN ('uploading','ready','failed')`, projectID, userID); err != nil {
		return false, err
	}
	return true, tx.Commit()
}

func (r *canvasRepository) CreateRevision(ctx context.Context, revision *service.CanvasRevision, documentBytes int) (*service.CanvasRevision, error) {
	if r == nil || r.db == nil || revision == nil {
		return nil, errors.New("invalid canvas revision repository request")
	}
	row := r.db.QueryRowContext(ctx, `
INSERT INTO canvas_revisions (project_id,user_id,revision,source,schema_version,document,document_bytes)
VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7)
ON CONFLICT (project_id,revision) DO UPDATE SET source=EXCLUDED.source, schema_version=EXCLUDED.schema_version, document=EXCLUDED.document, document_bytes=EXCLUDED.document_bytes, created_at=NOW()
RETURNING id,project_id,user_id,revision,source,schema_version,document::text,created_at`, revision.ProjectID, revision.UserID, revision.Revision, revision.Source, revision.SchemaVersion, string(revision.Document), documentBytes)
	return scanCanvasRevision(row.Scan)
}

func scanCanvasRevision(scan func(...any) error) (*service.CanvasRevision, error) {
	var revision service.CanvasRevision
	var raw string
	if err := scan(&revision.ID, &revision.ProjectID, &revision.UserID, &revision.Revision, &revision.Source, &revision.SchemaVersion, &raw, &revision.CreatedAt); err != nil {
		return nil, err
	}
	revision.Document = json.RawMessage(raw)
	return &revision, nil
}

func (r *canvasRepository) ListRevisions(ctx context.Context, userID, projectID int64, limit int) ([]*service.CanvasRevision, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT r.id,r.project_id,r.user_id,r.revision,r.source,r.schema_version,r.document::text,r.created_at FROM canvas_revisions r JOIN canvas_projects p ON p.id=r.project_id WHERE r.project_id=$1 AND r.user_id=$2 AND p.deleted_at IS NULL ORDER BY r.created_at DESC,r.id DESC LIMIT $3`, projectID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*service.CanvasRevision, 0)
	for rows.Next() {
		item, err := scanCanvasRevision(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *canvasRepository) GetRevision(ctx context.Context, userID, projectID, revisionID int64) (*service.CanvasRevision, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	row := r.db.QueryRowContext(ctx, `SELECT r.id,r.project_id,r.user_id,r.revision,r.source,r.schema_version,r.document::text,r.created_at FROM canvas_revisions r JOIN canvas_projects p ON p.id=r.project_id WHERE r.id=$1 AND r.project_id=$2 AND r.user_id=$3 AND p.deleted_at IS NULL`, revisionID, projectID, userID)
	revision, err := scanCanvasRevision(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCanvasNotFound
	}
	return revision, err
}

func scanCanvasAsset(scan func(...any) error) (*service.CanvasAsset, error) {
	var asset service.CanvasAsset
	var sourceIndex, width, height sql.NullInt64
	var deleteNext, deleted sql.NullTime
	if err := scan(&asset.ID, &asset.ProjectID, &asset.UserID, &asset.ObjectKey, &asset.Status, &asset.Source, &asset.SourceTaskID, &sourceIndex, &asset.IdempotencyKey, &asset.FileName, &asset.ContentType, &asset.SizeBytes, &asset.SHA256, &width, &height, &asset.DeleteAttempts, &deleteNext, &deleted, &asset.CreatedAt, &asset.UpdatedAt); err != nil {
		return nil, err
	}
	if sourceIndex.Valid {
		value := int(sourceIndex.Int64)
		asset.SourceImageIndex = &value
	}
	if width.Valid {
		value := int(width.Int64)
		asset.Width = &value
	}
	if height.Valid {
		value := int(height.Int64)
		asset.Height = &value
	}
	if deleteNext.Valid {
		value := deleteNext.Time
		asset.DeleteNextAt = &value
	}
	if deleted.Valid {
		value := deleted.Time
		asset.DeletedAt = &value
	}
	return &asset, nil
}

func (r *canvasRepository) CreateAsset(ctx context.Context, asset *service.CanvasAsset) (*service.CanvasAsset, error) {
	if r == nil || r.db == nil || asset == nil {
		return nil, errors.New("invalid canvas asset repository request")
	}
	row := r.db.QueryRowContext(ctx, `INSERT INTO canvas_assets (project_id,user_id,status,source,source_task_id,source_image_index,idempotency_key,file_name,content_type,size_bytes,sha256) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING `+canvasAssetColumns, asset.ProjectID, asset.UserID, asset.Status, asset.Source, asset.SourceTaskID, canvasNullInt(asset.SourceImageIndex), asset.IdempotencyKey, asset.FileName, asset.ContentType, asset.SizeBytes, asset.SHA256)
	return scanCanvasAsset(row.Scan)
}

func (r *canvasRepository) FindAssetByIdempotency(ctx context.Context, userID int64, idempotencyKey string) (*service.CanvasAsset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+canvasAssetColumns+` FROM canvas_assets WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey)
	asset, err := scanCanvasAsset(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return asset, err
}

func (r *canvasRepository) MarkAssetReady(ctx context.Context, userID, assetID int64, objectKey, contentType string, sizeBytes int64, checksum string, width, height *int) (*service.CanvasAsset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	row := r.db.QueryRowContext(ctx, `UPDATE canvas_assets SET object_key=$3,status='ready',content_type=$4,size_bytes=$5,sha256=$6,width=$7,height=$8,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status='uploading' RETURNING `+canvasAssetColumns, assetID, userID, objectKey, contentType, sizeBytes, checksum, canvasNullInt(width), canvasNullInt(height))
	asset, err := scanCanvasAsset(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCanvasAsset
	}
	return asset, err
}

func (r *canvasRepository) MarkAssetFailed(ctx context.Context, userID, assetID int64) error {
	if r == nil || r.db == nil {
		return errors.New("nil canvas repository")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE canvas_assets SET status='failed',updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status='uploading'`, assetID, userID)
	return err
}

func (r *canvasRepository) GetAsset(ctx context.Context, userID, assetID int64) (*service.CanvasAsset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	row := r.db.QueryRowContext(ctx, `SELECT `+canvasAssetColumnsAliased+` FROM canvas_assets a JOIN canvas_projects p ON p.id=a.project_id WHERE a.id=$1 AND a.user_id=$2 AND p.deleted_at IS NULL`, assetID, userID)
	asset, err := scanCanvasAsset(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrCanvasAsset
	}
	return asset, err
}

func (r *canvasRepository) RequestAssetDeletion(ctx context.Context, userID, assetID int64) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("nil canvas repository")
	}
	result, err := r.db.ExecContext(ctx, `UPDATE canvas_assets SET status='delete_pending',delete_next_attempt_at=NOW(),updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('uploading','ready','failed')`, assetID, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

func (r *canvasRepository) ClaimAssetsForDeletion(ctx context.Context, limit int, lease time.Duration) ([]*service.CanvasAsset, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("nil canvas repository")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if lease <= 0 {
		lease = 2 * time.Minute
	}
	rows, err := r.db.QueryContext(ctx, `WITH claimed AS (SELECT id FROM canvas_assets WHERE status='delete_pending' AND (delete_next_attempt_at IS NULL OR delete_next_attempt_at<=NOW()) ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED) UPDATE canvas_assets a SET delete_attempts=a.delete_attempts+1,delete_next_attempt_at=NOW()+($2::bigint * INTERVAL '1 millisecond'),updated_at=NOW() FROM claimed WHERE a.id=claimed.id RETURNING `+canvasAssetColumnsAliased, limit, lease.Milliseconds())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets := make([]*service.CanvasAsset, 0)
	for rows.Next() {
		asset, err := scanCanvasAsset(rows.Scan)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (r *canvasRepository) MarkAssetDeleted(ctx context.Context, assetID int64) error {
	if r == nil || r.db == nil {
		return errors.New("nil canvas repository")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE canvas_assets SET status='deleted',deleted_at=NOW(),delete_next_attempt_at=NULL,updated_at=NOW() WHERE id=$1`, assetID)
	return err
}
func (r *canvasRepository) RetryAssetDeletion(ctx context.Context, assetID int64, nextAt time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("nil canvas repository")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE canvas_assets SET status='delete_pending',delete_next_attempt_at=$2,updated_at=NOW() WHERE id=$1 AND status='delete_pending'`, assetID, nextAt.UTC())
	return err
}

func canvasNullInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

var _ service.CanvasProjectRepository = (*canvasRepository)(nil)
