package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestActivityCenterMigrationUsesSafeDefaultsAndFinancialConstraints(t *testing.T) {
	content, err := FS.ReadFile("231_activity_center.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	for _, table := range []string{
		"activities",
		"activity_lottery_configs",
		"activity_lottery_prizes",
		"activity_lottery_qualifications",
		"activity_lottery_draw_accounts",
		"activity_lottery_draws",
		"activity_benefit_configs",
		"activity_benefit_claims",
		"activity_reward_ledger",
	} {
		require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS "+table)
	}
	require.Contains(t, sql, "VALUES ('activity_center_enabled', 'false')")
	require.Contains(t, sql, "'published', FALSE, 10, 1")
	require.Contains(t, sql, "'published', FALSE, 20, 1")
	require.Contains(t, sql, "CONSTRAINT activity_lottery_prizes_probability_check CHECK (probability_ppm > 0 AND probability_ppm <= 1000000)")
	require.Contains(t, sql, "CONSTRAINT activity_lottery_draw_accounts_check CHECK ( granted_draws >= 0 AND used_draws >= 0 AND used_draws <= granted_draws )")
	require.Contains(t, sql, "CONSTRAINT activity_reward_ledger_source_unique UNIQUE (source_type, source_id)")
	require.Contains(t, sql, "CONSTRAINT activity_lottery_draws_request_unique UNIQUE (user_id, activity_id, request_id)")
	require.Contains(t, sql, "CONSTRAINT activity_benefit_claims_request_unique UNIQUE (user_id, activity_id, request_id)")
	require.Contains(t, sql, "recharge_threshold > 0 AND recharge_threshold <= 1000000")
	require.Contains(t, sql, "total_stock BETWEEN 0 AND 1000000")
	require.Contains(t, sql, "CREATE OR REPLACE FUNCTION reject_activity_config_mutation()")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON activity_lottery_configs")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON activity_lottery_prizes")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON activity_benefit_configs")
	require.Contains(t, sql, "SELECT c.id, '谢谢参与', 0, 1000000, 10")
}

func TestActivityLotteryCumulativeRechargeMigrationTracksNetPayments(t *testing.T) {
	content, err := FS.ReadFile("232_activity_lottery_cumulative_recharge.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS refunded_amount NUMERIC(20, 8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "granted_draws >= 0 AND revoked_draws >= 0 AND revoked_draws <= granted_draws")
	require.Contains(t, sql, "refunded_amount >= 0 AND refunded_amount <= recharge_amount")
	require.Contains(t, sql, "activity_lottery_qualifications_progress_idx")
}

func TestActivityLotteryBalanceRedeemMigrationAddsExclusiveSource(t *testing.T) {
	content, err := FS.ReadFile("233_activity_lottery_balance_redeem_qualification.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ALTER COLUMN payment_order_id DROP NOT NULL")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS redeem_code_id BIGINT NULL REFERENCES redeem_codes(id) ON DELETE RESTRICT")
	require.Contains(t, sql, "num_nonnulls(payment_order_id, redeem_code_id) = 1")
	require.Contains(t, sql, "UNIQUE (activity_id, redeem_code_id)")
}

func TestActivityBenefitRandomRewardMigrationAddsBoundedCentRange(t *testing.T) {
	content, err := FS.ReadFile("234_activity_benefit_random_reward.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS random_min_amount NUMERIC(20, 8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS random_max_amount NUMERIC(20, 8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CONSTRAINT activity_benefit_configs_reward_mode_check CHECK")
	require.Contains(t, sql, "reward_amount > 0 AND random_min_amount = 0 AND random_max_amount = 0")
	require.Contains(t, sql, "reward_amount = 0 AND random_min_amount > 0 AND random_max_amount >= random_min_amount")
	require.Contains(t, sql, "random_min_amount = ROUND(random_min_amount, 2)")
	require.Contains(t, sql, "reward_amount = 0 AND total_stock = 0")
}

func TestActivityBenefitDailyClaimMigrationPreservesHistoryAndAddsDatabaseGuard(t *testing.T) {
	content, err := FS.ReadFile("235_activity_benefit_daily_claim.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS claim_day DATE")
	require.Contains(t, sql, "AT TIME ZONE 'Asia/Shanghai'")
	require.Contains(t, sql, "ROW_NUMBER() OVER")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS activity_benefit_claims_user_day_unique")
	require.Contains(t, sql, "ON activity_benefit_claims(activity_id, user_id, claim_day)")
	require.Contains(t, sql, "WHERE claim_day IS NOT NULL")
}
