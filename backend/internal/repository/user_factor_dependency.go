package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userFactorDependencyRepository struct {
	db *sql.DB
}

func NewUserFactorDependencyRepository(db *sql.DB) service.UserFactorDependencyRepository {
	return &userFactorDependencyRepository{db: db}
}

func (r *userFactorDependencyRepository) DisableTOTP(ctx context.Context, userID int64, expectedPasswordHash string) error {
	if r == nil || r.db == nil {
		return service.ErrServiceUnavailable
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var passwordHash string
	var enabled bool
	err = tx.QueryRowContext(ctx, `SELECT password_hash,totp_enabled FROM users WHERE id=$1 AND deleted_at IS NULL AND status='active' FOR UPDATE`, userID).Scan(&passwordHash, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	if passwordHash != expectedPasswordHash {
		return service.ErrPasswordIncorrect
	}
	if !enabled {
		return service.ErrTotpNotSetup
	}
	// Do not take Workspace locks: policy updates lock USER before WORKSPACE,
	// and this MVCC read observes any update that won the shared user lock.
	var required bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN workspaces w ON w.id=m.workspace_id JOIN workspace_security_policies p ON p.workspace_id=w.id WHERE m.user_id=$1 AND m.status='active' AND w.type='organization' AND w.status='active' AND p.require_mfa)`, userID).Scan(&required)
	if err != nil {
		return err
	}
	if required {
		return service.ErrWorkspaceMFARequired
	}
	result, err := tx.ExecContext(ctx, `UPDATE users SET totp_enabled=false,totp_enabled_at=NULL,totp_secret_encrypted=NULL,updated_at=now() WHERE id=$1`, userID)
	if err != nil {
		return err
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if updated != 1 {
		return service.ErrUserNotFound
	}
	return tx.Commit()
}
