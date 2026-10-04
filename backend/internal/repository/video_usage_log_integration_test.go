//go:build integration

package repository

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
)

func TestVideoUsageLongRequestID(t *testing.T) {
	for _, taskID := range []string{
		"seedance:kz-cgt-" + strings.Repeat("a", 50),
		"seedance:" + strings.Repeat("b", 235),
	} {
		for _, writer := range []string{"sync", "batch", "ent"} {
			t.Run(writer+"-"+strconv.Itoa(len(service.StableGrokVideoBillingRequestID(taskID))), func(t *testing.T) {
				ctx := context.Background()
				client := testEntClient(t)
				usageRepo := newUsageLogRepositoryWithSQL(client, integrationDB)
				billingRepo := NewUsageBillingRepository(client, integrationDB)
				user := mustCreateUser(t, client, &service.User{Email: uuid.NewString() + "@video-usage.test", Balance: 100})
				apiKey := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-video-usage-" + uuid.NewString(), Name: "video"})
				account := mustCreateAccount(t, client, &service.Account{Name: "video-usage-" + uuid.NewString(), Type: service.AccountTypeAPIKey, Platform: service.PlatformSeedance})
				requestID := service.StableGrokVideoBillingRequestID(taskID)
				cmd := &service.UsageBillingCommand{
					RequestID: requestID, APIKeyID: apiKey.ID, UserID: user.ID,
					AccountID: account.ID, AccountType: service.AccountTypeAPIKey, BalanceCost: 1.25,
				}
				billed, err := billingRepo.Apply(ctx, cmd)
				require.NoError(t, err)
				require.True(t, billed.Applied)

				mode, resolution, duration := string(service.BillingModeVideo), "720p", 5
				usageLog := &service.UsageLog{
					UserID: user.ID, APIKeyID: apiKey.ID, AccountID: account.ID,
					RequestID: requestID, Model: "seedance-2.0", VideoCount: 1,
					VideoResolution: &resolution, VideoDurationSeconds: &duration,
					BillingMode: &mode, TotalCost: 1.25, ActualCost: 1.25, RateMultiplier: 1, CreatedAt: time.Now().UTC(),
				}
				switch writer {
				case "sync":
					inserted, err := usageRepo.Create(ctx, usageLog)
					require.NoError(t, err)
					require.True(t, inserted)
				case "batch":
					require.NoError(t, usageRepo.CreateBestEffort(ctx, usageLog))
				case "ent":
					row, err := client.UsageLog.Create().
						SetUserID(user.ID).SetAPIKeyID(apiKey.ID).SetAccountID(account.ID).
						SetRequestID(requestID).SetModel(usageLog.Model).SetVideoCount(1).
						SetVideoResolution(resolution).SetVideoDurationSeconds(duration).
						SetBillingMode(mode).SetTotalCost(1.25).SetActualCost(1.25).Save(ctx)
					require.NoError(t, err)
					usageLog.ID = row.ID
				}
				// Best-effort inserts deliberately do not populate the caller's ID.
				require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&usageLog.ID))
				saved, err := usageRepo.GetByID(ctx, usageLog.ID)
				require.NoError(t, err)
				require.Equal(t, requestID, saved.RequestID)
				require.Equal(t, 1, saved.VideoCount)
				require.Equal(t, &resolution, saved.VideoResolution)
				require.Equal(t, &duration, saved.VideoDurationSeconds)
				require.InDelta(t, 1.25, saved.ActualCost, 1e-12)

				// Replayed status/content requests preserve both usage and billing dedup.
				replayed, err := billingRepo.Apply(ctx, cmd)
				require.NoError(t, err)
				require.False(t, replayed.Applied)
				inserted, err := usageRepo.Create(ctx, usageLog)
				require.NoError(t, err)
				require.False(t, inserted)
				var balance float64
				require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
				require.InDelta(t, 98.75, balance, 1e-8)
				var count int
				require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE request_id = $1 AND api_key_id = $2", requestID, apiKey.ID).Scan(&count))
				require.Equal(t, 1, count)
			})
		}
	}
}

func TestMigration272PreservesUsageRequestIDs(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	// Shadow the public table to exercise the old schema without modifying other tests.
	_, err := tx.ExecContext(ctx, `CREATE TEMP TABLE usage_logs (
		request_id VARCHAR(64) NOT NULL, api_key_id BIGINT NOT NULL,
		UNIQUE (request_id, api_key_id)
	) ON COMMIT DROP;
	INSERT INTO usage_logs (request_id, api_key_id) VALUES ('grok-video:existing', 1);`)
	require.NoError(t, err)
	migrationSQL, err := dbmigrations.FS.ReadFile("272_expand_usage_log_request_id.sql")
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, err = tx.ExecContext(ctx, string(migrationSQL))
		require.NoError(t, err)
	}
	longID := service.StableGrokVideoBillingRequestID("seedance:" + strings.Repeat("b", 235))
	_, err = tx.ExecContext(ctx, "INSERT INTO usage_logs (request_id, api_key_id) VALUES ($1, 1)", longID)
	require.NoError(t, err)
	result, err := tx.ExecContext(ctx, "INSERT INTO usage_logs (request_id, api_key_id) VALUES ($1, 1) ON CONFLICT (request_id, api_key_id) DO NOTHING", longID)
	require.NoError(t, err)
	inserted, err := result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, inserted)
	var count int
	require.NoError(t, tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs WHERE request_id IN ($1, $2)", "grok-video:existing", longID).Scan(&count))
	require.Equal(t, 2, count)
}
