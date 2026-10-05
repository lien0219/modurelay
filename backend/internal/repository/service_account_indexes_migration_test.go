package repository

import (
	"context"
	"database/sql"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestServiceAccountIndexesMigrationRepairsInterruptedBuilds(t *testing.T) {
	for name, indexes := range map[string][]string{
		"286_service_account_indexes_notx.sql":       {"api_keys_service_account_id", "usage_logs_service_account_created", "budget_reservations_service_account_pending", "batch_image_jobs_service_account_created"},
		"289_service_account_audit_indexes_notx.sql": {"idx_content_moderation_logs_service_account_created", "idx_prompt_audit_jobs_service_account_created", "idx_prompt_audit_events_service_account_created", "idx_ops_error_logs_service_account_created", "idx_ops_system_logs_service_account_created"},
	} {
		content, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		for _, invalid := range append([]string{"none"}, indexes...) {
			t.Run(name+"/"+invalid, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()
				prepareMigrationsBootstrapExpectations(mock)
				mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").WithArgs(name).WillReturnError(sql.ErrNoRows)
				for _, index := range indexes {
					mock.ExpectQuery("SELECT EXISTS \\(").WithArgs(index).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(index == invalid))
					if index == invalid {
						mock.ExpectExec("DROP INDEX CONCURRENTLY IF EXISTS " + index).WillReturnResult(sqlmock.NewResult(0, 0))
					}
				}
				for _, index := range indexes {
					mock.ExpectExec("CREATE INDEX CONCURRENTLY IF NOT EXISTS " + index).WillReturnResult(sqlmock.NewResult(0, 0))
				}
				mock.ExpectExec("INSERT INTO schema_migrations").WithArgs(name, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("SELECT pg_advisory_unlock\\(\\$1\\)").WithArgs(migrationsAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))
				err = applyMigrationsFS(context.Background(), db, fstest.MapFS{name: &fstest.MapFile{Data: content}})
				require.NoError(t, err, "failed online index builds must be repaired before recording the migration")
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
