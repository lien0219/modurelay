package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type activitySettingRepoStub struct {
	setMultipleErr error
	updates        map[string]string
}

func (s *activitySettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *activitySettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *activitySettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *activitySettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *activitySettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	s.updates = settings
	return s.setMultipleErr
}

func (s *activitySettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *activitySettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestActivityServiceSetEnabledNotifiesAfterPersistence(t *testing.T) {
	repo := &activitySettingRepoStub{}
	service := &ActivityService{settingRepo: repo}
	notifications := 0
	service.SetOnUpdateCallback(func() { notifications++ })

	require.NoError(t, service.SetEnabled(context.Background(), true))
	require.Equal(t, map[string]string{SettingKeyActivityCenterEnabled: "true"}, repo.updates)
	require.Equal(t, 1, notifications)
}

func TestActivityServiceSetEnabledDoesNotNotifyWhenPersistenceFails(t *testing.T) {
	repo := &activitySettingRepoStub{setMultipleErr: errors.New("settings unavailable")}
	service := &ActivityService{settingRepo: repo}
	notifications := 0
	service.SetOnUpdateCallback(func() { notifications++ })

	require.Error(t, service.SetEnabled(context.Background(), false))
	require.Equal(t, 0, notifications)
}

func validLotteryConfigInput() LotteryConfigInput {
	return LotteryConfigInput{
		Currency:           "CNY",
		RechargeThreshold:  "100.00000000",
		DrawsPerThreshold:  1,
		MaxChancesPerOrder: 10,
		PerUserDrawLimit:   100,
		DailyDrawLimit:     5,
		DailyLimitTimezone: "UTC",
		Prizes: []LotteryPrizeInput{
			{Name: "Balance reward", Amount: "8.88", ProbabilityPPM: 250_000, SortOrder: 10},
			{Name: "Try again", Amount: "0", ProbabilityPPM: 750_000, SortOrder: 20},
		},
	}
}

func TestValidateLotteryConfigAcceptsExactProductionConfiguration(t *testing.T) {
	input := validLotteryConfigInput()
	require.NoError(t, validateLotteryConfig(input))
}

func TestValidateLotteryConfigRejectsInvalidFinancialRules(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		mutate func(*LotteryConfigInput)
	}{
		{name: "probability total", mutate: func(input *LotteryConfigInput) { input.Prizes[1].ProbabilityPPM-- }},
		{name: "single probability above scale", mutate: func(input *LotteryConfigInput) {
			input.Prizes[0].ProbabilityPPM = ActivityProbabilityScale + 1
			input.Prizes[1].ProbabilityPPM = -1
		}},
		{name: "no positive reward", mutate: func(input *LotteryConfigInput) { input.Prizes[0].Amount = "0" }},
		{name: "money precision", mutate: func(input *LotteryConfigInput) { input.Prizes[0].Amount = "0.000000001" }},
		{name: "threshold precision", mutate: func(input *LotteryConfigInput) { input.RechargeThreshold = "1.000000001" }},
		{name: "timezone", mutate: func(input *LotteryConfigInput) { input.DailyLimitTimezone = "Invalid/Timezone" }},
		{name: "window", mutate: func(input *LotteryConfigInput) {
			input.StartsAt = &now
			end := now.Add(-time.Second)
			input.EndsAt = &end
		}},
		{name: "too many prizes", mutate: func(input *LotteryConfigInput) {
			input.Prizes = make([]LotteryPrizeInput, maxActivityPrizeCount+1)
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validLotteryConfigInput()
			test.mutate(&input)
			require.ErrorIs(t, validateLotteryConfig(input), ErrActivityConfigInvalid)
		})
	}
}

func TestValidateBenefitConfigAcceptsFixedAndRandomRewards(t *testing.T) {
	input := BenefitConfigInput{
		Currency:     "CNY",
		RewardAmount: "0.00000001",
		TotalStock:   100,
		PerUserLimit: 1,
	}
	require.NoError(t, validateBenefitConfig(input))

	input.RewardAmount = "0"
	input.RandomMinAmount = "0.01"
	input.RandomMaxAmount = "8.88"
	require.NoError(t, validateBenefitConfig(input))
}

func TestValidateBenefitConfigRejectsInvalidRewardModes(t *testing.T) {
	valid := BenefitConfigInput{
		Currency:     "CNY",
		RewardAmount: "1",
		TotalStock:   100,
		PerUserLimit: 1,
	}
	tests := []struct {
		name   string
		mutate func(*BenefitConfigInput)
	}{
		{name: "fixed with random range", mutate: func(input *BenefitConfigInput) { input.RandomMaxAmount = "1" }},
		{name: "random without range", mutate: func(input *BenefitConfigInput) { input.RewardAmount = "0" }},
		{name: "random zero minimum", mutate: func(input *BenefitConfigInput) {
			input.RewardAmount, input.RandomMinAmount, input.RandomMaxAmount = "0", "0", "1"
		}},
		{name: "random reversed range", mutate: func(input *BenefitConfigInput) {
			input.RewardAmount, input.RandomMinAmount, input.RandomMaxAmount = "0", "2", "1"
		}},
		{name: "random sub-cent precision", mutate: func(input *BenefitConfigInput) {
			input.RewardAmount, input.RandomMinAmount, input.RandomMaxAmount = "0", "0.001", "1"
		}},
		{name: "random above maximum", mutate: func(input *BenefitConfigInput) {
			input.RewardAmount, input.RandomMinAmount, input.RandomMaxAmount = "0", "1", "1000000.01"
		}},
		{name: "fixed money precision", mutate: func(input *BenefitConfigInput) { input.RewardAmount = "0.000000001" }},
		{name: "stock above maximum", mutate: func(input *BenefitConfigInput) { input.TotalStock = maxActivityConfigurationLimit + 1 }},
		{name: "daily limit must remain one", mutate: func(input *BenefitConfigInput) { input.PerUserLimit = 2 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := valid
			test.mutate(&input)
			require.ErrorIs(t, validateBenefitConfig(input), ErrActivityConfigInvalid)
		})
	}
}

func TestActivityProbabilityVisibilityDiffersForUserAndAdminPayloads(t *testing.T) {
	userActivity := Activity{
		Lottery: &LotteryConfig{Prizes: []ActivityPrize{{ID: 1, Name: "Reward", ProbabilityPPM: 250_000}}},
	}
	hideLotteryProbability(&userActivity)
	userPayload, err := json.Marshal(userActivity)
	require.NoError(t, err)
	require.NotContains(t, string(userPayload), "probability_ppm")

	adminView := ActivityCenterAdminView{
		Activities: []Activity{{
			Lottery: &LotteryConfig{Prizes: []ActivityPrize{{ID: 1, Name: "Reward", ProbabilityPPM: 250_000}}},
		}},
	}
	adminPayload, err := json.Marshal(adminView)
	require.NoError(t, err)
	require.Contains(t, string(adminPayload), `"probability_ppm":250000`)
}
