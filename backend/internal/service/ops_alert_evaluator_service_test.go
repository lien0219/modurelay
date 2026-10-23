//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var _ OpsRepository = (*stubOpsRepo)(nil)

type stubOpsRepo struct {
	OpsRepository
	overview    *OpsDashboardOverview
	err         error
	rules       []*OpsAlertRule
	states      map[int64]*OpsAlertRuleState
	upserts     []*OpsAlertRuleState
	activeCalls int
	statusCalls int
	emailWrites int
	activeEvent *OpsAlertEvent
	statusErr   error
}

func (s *stubOpsRepo) GetDashboardOverview(ctx context.Context, filter *OpsDashboardFilter) (*OpsDashboardOverview, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.overview != nil {
		return s.overview, nil
	}
	return &OpsDashboardOverview{}, nil
}

func (s *stubOpsRepo) ListAlertRules(ctx context.Context) ([]*OpsAlertRule, error) {
	return s.rules, nil
}

func (s *stubOpsRepo) GetLatestSystemMetrics(ctx context.Context, windowMinutes int) (*OpsSystemMetricsSnapshot, error) {
	return nil, nil
}

func (s *stubOpsRepo) UpsertJobHeartbeat(ctx context.Context, input *OpsUpsertJobHeartbeatInput) error {
	return nil
}

func (s *stubOpsRepo) GetAlertRuleState(ctx context.Context, ruleID int64) (*OpsAlertRuleState, error) {
	if s.states == nil {
		return nil, nil
	}
	return s.states[ruleID], nil
}

func (s *stubOpsRepo) UpsertAlertRuleState(ctx context.Context, state *OpsAlertRuleState) error {
	if state == nil {
		return nil
	}
	copied := *state
	s.upserts = append(s.upserts, &copied)
	if s.states == nil {
		s.states = map[int64]*OpsAlertRuleState{}
	}
	s.states[state.RuleID] = &copied
	return nil
}

func (s *stubOpsRepo) GetActiveAlertEvent(ctx context.Context, ruleID int64) (*OpsAlertEvent, error) {
	s.activeCalls++
	return s.activeEvent, nil
}

func (s *stubOpsRepo) UpdateAlertEventStatus(ctx context.Context, eventID int64, status string, resolvedAt *time.Time) error {
	s.statusCalls++
	if s.statusErr != nil {
		return s.statusErr
	}
	if s.activeEvent != nil && s.activeEvent.ID == eventID {
		s.activeEvent = nil
	}
	return nil
}

func (s *stubOpsRepo) UpdateAlertEventEmailSent(ctx context.Context, eventID int64, emailSent bool) error {
	s.emailWrites++
	return nil
}

type alertSettingsRepo struct {
	SettingRepository
	values map[string]string
}

func (r *alertSettingsRepo) GetValue(ctx context.Context, key string) (string, error) {
	if r.values == nil {
		return "", ErrSettingNotFound
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *alertSettingsRepo) Set(ctx context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func TestComputeGroupAvailableRatio(t *testing.T) {
	t.Parallel()

	t.Run("正常情况: 10个账号, 8个可用 = 80%", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  10,
			AvailableCount: 8,
		})
		require.InDelta(t, 80.0, got, 0.0001)
	})

	t.Run("边界情况: TotalAccounts = 0 应返回 0", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  0,
			AvailableCount: 8,
		})
		require.Equal(t, 0.0, got)
	})

	t.Run("边界情况: AvailableCount = 0 应返回 0%", func(t *testing.T) {
		t.Parallel()

		got := computeGroupAvailableRatio(&GroupAvailability{
			TotalAccounts:  10,
			AvailableCount: 0,
		})
		require.Equal(t, 0.0, got)
	})
}

func TestCountAccountsByCondition(t *testing.T) {
	t.Parallel()

	t.Run("测试限流账号统计: acc.IsRateLimited", func(t *testing.T) {
		t.Parallel()

		accounts := map[int64]*AccountAvailability{
			1: {IsRateLimited: true},
			2: {IsRateLimited: false},
			3: {IsRateLimited: true},
		}

		got := countAccountsByCondition(accounts, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})
		require.Equal(t, int64(2), got)
	})

	t.Run("测试错误账号统计（排除临时不可调度）: acc.HasError && acc.TempUnschedulableUntil == nil", func(t *testing.T) {
		t.Parallel()

		until := time.Now().UTC().Add(5 * time.Minute)
		accounts := map[int64]*AccountAvailability{
			1: {HasError: true},
			2: {HasError: true, TempUnschedulableUntil: &until},
			3: {HasError: false},
		}

		got := countAccountsByCondition(accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})
		require.Equal(t, int64(1), got)
	})

	t.Run("边界情况: 空 map 应返回 0", func(t *testing.T) {
		t.Parallel()

		got := countAccountsByCondition(map[int64]*AccountAvailability{}, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})
		require.Equal(t, int64(0), got)
	})
}

// TestComputeRuleMetric_AccountTempUnscheduledCount verifies the new
// account_temp_unscheduled_count metric counts accounts currently in the
// temp-unscheduled window and ignores those whose window has expired or
// were never temp-unscheduled.
func TestComputeRuleMetric_AccountTempUnscheduledCount(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	futureUntil := now.Add(5 * time.Minute)
	pastUntil := now.Add(-1 * time.Minute)

	availability := &OpsAccountAvailability{
		Accounts: map[int64]*AccountAvailability{
			// currently temp-unscheduled (window active)
			1: {TempUnschedulableUntil: &futureUntil},
			2: {TempUnschedulableUntil: &futureUntil},
			// temp-unsched window already expired → should NOT count
			3: {TempUnschedulableUntil: &pastUntil},
			// never temp-unscheduled
			4: {HasError: true},
			5: {IsRateLimited: true},
		},
	}

	opsService := &OpsService{
		getAccountAvailability: func(_ context.Context, _ string, _ *int64) (*OpsAccountAvailability, error) {
			return availability, nil
		},
	}
	svc := &OpsAlertEvaluatorService{
		opsService: opsService,
		opsRepo:    &stubOpsRepo{},
	}

	rule := &OpsAlertRule{MetricType: "account_temp_unscheduled_count"}
	val, status := svc.computeRuleMetric(context.Background(), rule, nil,
		now.Add(-5*time.Minute), now, "", nil)

	require.Equal(t, alertMetricOK, status)
	require.InDelta(t, 2.0, val, 0.0001, "only 2 accounts have an active temp-unsched window")
}

func TestComputeRuleMetricNewIndicators(t *testing.T) {
	t.Parallel()

	groupID := int64(101)
	platform := "openai"

	availability := &OpsAccountAvailability{
		Group: &GroupAvailability{
			GroupID:        groupID,
			TotalAccounts:  10,
			AvailableCount: 8,
		},
		Accounts: map[int64]*AccountAvailability{
			1: {IsRateLimited: true},
			2: {IsRateLimited: true},
			3: {HasError: true},
			4: {HasError: true, TempUnschedulableUntil: timePtr(time.Now().UTC().Add(2 * time.Minute))},
			5: {HasError: false, IsRateLimited: false},
		},
	}

	opsService := &OpsService{
		getAccountAvailability: func(_ context.Context, _ string, _ *int64) (*OpsAccountAvailability, error) {
			return availability, nil
		},
	}

	svc := &OpsAlertEvaluatorService{
		opsService: opsService,
		opsRepo:    &stubOpsRepo{overview: &OpsDashboardOverview{}},
	}

	start := time.Now().UTC().Add(-5 * time.Minute)
	end := time.Now().UTC()
	ctx := context.Background()

	tests := []struct {
		name       string
		metricType string
		groupID    *int64
		wantValue  float64
		wantOK     bool
	}{
		{
			name:       "group_available_accounts",
			metricType: "group_available_accounts",
			groupID:    &groupID,
			wantValue:  8,
			wantOK:     true,
		},
		{
			name:       "group_available_ratio",
			metricType: "group_available_ratio",
			groupID:    &groupID,
			wantValue:  80.0,
			wantOK:     true,
		},
		{
			name:       "account_rate_limited_count",
			metricType: "account_rate_limited_count",
			groupID:    nil,
			wantValue:  2,
			wantOK:     true,
		},
		{
			name:       "account_error_count",
			metricType: "account_error_count",
			groupID:    nil,
			wantValue:  1,
			wantOK:     true,
		},
		{
			name:       "group_available_accounts without group_id returns false",
			metricType: "group_available_accounts",
			groupID:    nil,
			wantValue:  0,
			wantOK:     false,
		},
		{
			name:       "group_available_ratio without group_id returns false",
			metricType: "group_available_ratio",
			groupID:    nil,
			wantValue:  0,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rule := &OpsAlertRule{
				MetricType: tt.metricType,
			}
			gotValue, gotStatus := svc.computeRuleMetric(ctx, rule, nil, start, end, platform, tt.groupID)
			if tt.wantOK {
				require.Equal(t, alertMetricOK, gotStatus)
			} else {
				require.Equal(t, alertMetricUnavailable, gotStatus)
				return
			}
			require.InDelta(t, tt.wantValue, gotValue, 0.0001)
		})
	}
}

func TestComputeRuleMetricRateSampleGate(t *testing.T) {
	t.Parallel()

	svc := &OpsAlertEvaluatorService{opsRepo: &stubOpsRepo{overview: &OpsDashboardOverview{
		RequestCountSLA:   4,
		SLA:               0.9,
		ErrorRate:         0.1,
		UpstreamErrorRate: 0.05,
	}}}
	start := time.Now().UTC().Add(-time.Minute)
	end := time.Now().UTC()
	rule := &OpsAlertRule{MetricType: "error_rate", MinRequests: 5}
	_, status := svc.computeRuleMetric(context.Background(), rule, nil, start, end, "", nil)
	require.Equal(t, alertMetricInsufficient, status)

	rule.MinRequests = 0
	svc.opsRepo = &stubOpsRepo{overview: &OpsDashboardOverview{RequestCountSLA: 0}}
	_, status = svc.computeRuleMetric(context.Background(), rule, nil, start, end, "", nil)
	require.Equal(t, alertMetricUnavailable, status)

	svc.opsRepo = &stubOpsRepo{overview: &OpsDashboardOverview{RequestCountSLA: 5, ErrorRate: 0.2}}
	rule.MinRequests = 5
	value, status := svc.computeRuleMetric(context.Background(), rule, nil, start, end, "", nil)
	require.Equal(t, alertMetricOK, status)
	require.InDelta(t, 20, value, 0.0001)
}

func TestEvaluateOnceInsufficientResetsBreachesAndDoesNotResolve(t *testing.T) {
	t.Parallel()

	evaluatedAt := time.Now().UTC().Add(-30 * time.Second)
	repo := &stubOpsRepo{
		overview: &OpsDashboardOverview{RequestCountSLA: 2, ErrorRate: 1},
		rules: []*OpsAlertRule{{
			ID:               7,
			Name:             "error rate",
			Enabled:          true,
			MetricType:       "error_rate",
			Operator:         ">",
			Threshold:        1,
			WindowMinutes:    1,
			SustainedMinutes: 5,
			MinRequests:      5,
		}},
		states: map[int64]*OpsAlertRuleState{
			7: {RuleID: 7, ConsecutiveBreaches: 4, LastEvaluatedAt: evaluatedAt, DataStatus: OpsAlertDataStatusOK},
		},
	}
	svc := &OpsAlertEvaluatorService{opsRepo: repo, ruleStates: map[int64]*opsAlertRuleState{}}
	svc.evaluateOnce(time.Minute)

	require.Zero(t, repo.activeCalls)
	require.Zero(t, repo.statusCalls)
	require.NotEmpty(t, repo.upserts)
	last := repo.upserts[len(repo.upserts)-1]
	require.Equal(t, 0, last.ConsecutiveBreaches)
	require.Equal(t, OpsAlertDataStatusInsufficient, last.DataStatus)
	require.False(t, last.LastEvaluatedAt.Equal(evaluatedAt))
}

func TestEvaluateOnceUnavailableResetsBreachesWithoutResolving(t *testing.T) {
	t.Parallel()

	repo := &stubOpsRepo{
		overview: &OpsDashboardOverview{RequestCountSLA: 0},
		rules: []*OpsAlertRule{{
			ID:            8,
			Enabled:       true,
			MetricType:    "success_rate",
			Operator:      "<",
			Threshold:     99,
			WindowMinutes: 1,
			MinRequests:   0,
		}},
		states: map[int64]*OpsAlertRuleState{
			8: {RuleID: 8, ConsecutiveBreaches: 3, LastEvaluatedAt: time.Now().UTC(), DataStatus: OpsAlertDataStatusOK},
		},
	}
	svc := &OpsAlertEvaluatorService{opsRepo: repo, ruleStates: map[int64]*opsAlertRuleState{}}
	svc.evaluateOnce(time.Minute)

	require.Zero(t, repo.activeCalls)
	require.Zero(t, repo.statusCalls)
	require.NotEmpty(t, repo.upserts)
	last := repo.upserts[len(repo.upserts)-1]
	require.Equal(t, 0, last.ConsecutiveBreaches)
	require.Equal(t, OpsAlertDataStatusUnavailable, last.DataStatus)
}

func TestUpdateRuleBreachesContinuesPersistedCount(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	repo := &stubOpsRepo{states: map[int64]*OpsAlertRuleState{
		9: {RuleID: 9, ConsecutiveBreaches: 2, LastEvaluatedAt: now.Add(-time.Minute), DataStatus: OpsAlertDataStatusOK},
	}}
	svc := &OpsAlertEvaluatorService{opsRepo: repo, ruleStates: map[int64]*opsAlertRuleState{}}
	count, ok := svc.updateRuleBreaches(context.Background(), 9, now, time.Minute, true)
	require.True(t, ok)
	require.Equal(t, 3, count)
	require.Equal(t, OpsAlertDataStatusOK, repo.upserts[0].DataStatus)

	gapNow := now.Add(5 * time.Minute)
	count, ok = svc.updateRuleBreaches(context.Background(), 9, gapNow, time.Minute, true)
	require.True(t, ok)
	require.Equal(t, 1, count)
}

func TestBuildOpsAlertEmailBodyUsesLocaleAndShanghaiTime(t *testing.T) {
	t.Parallel()

	fired := time.Date(2026, 10, 22, 0, 30, 0, 0, time.UTC)
	rule := &OpsAlertRule{Name: "错误率", Severity: "P1", MetricType: "error_rate", Operator: ">", Threshold: 5}
	event := &OpsAlertEvent{Status: OpsAlertStatusFiring, FiredAt: fired, Description: "over"}
	chinese := buildOpsAlertEmailBody(rule, event, "zh")
	require.Contains(t, chinese, "运维告警")
	require.Contains(t, chinese, "Asia/Shanghai")
	require.Contains(t, chinese, "2026-10-22 08:30:00")
	english := buildOpsAlertEmailBody(rule, event, "en")
	require.Contains(t, english, "Ops Alert")
	require.NotContains(t, english, "运维告警")
	require.Contains(t, english, "Asia/Shanghai")

	resolved := &OpsAlertEvent{Status: OpsAlertStatusResolved, FiredAt: fired, Description: "clear"}
	require.Contains(t, buildOpsAlertEmailBody(rule, resolved, "zh"), "运维恢复")
	require.Contains(t, buildOpsAlertEmailBody(rule, resolved, "en"), "Ops Resolved")
	require.Equal(t, "[运维恢复][P1] 错误率", opsAlertEmailSubject(rule, resolved, "zh"))
	require.Equal(t, "[Ops Alert][P1] 错误率", opsAlertEmailSubject(rule, event, "en"))
}

func TestComputeRuleMetricApplicationPoolUsage(t *testing.T) {
	t.Parallel()

	active, idle, limit := 8, 2, 20
	redisTotal, redisLimit := 9, 10
	over, overLimit := 30, 10
	zero := 0
	negative := -1

	live := &OpsSystemMetricsSnapshot{
		DBConnActive:   &active,
		DBConnIdle:     &idle,
		DBMaxOpenConns: &limit,
		RedisConnTotal: &redisTotal,
		RedisPoolSize:  &redisLimit,
	}
	svc := &OpsAlertEvaluatorService{opsService: &OpsService{
		alertClusterRuntimeSnapshot: func(context.Context) (*OpsSystemMetricsSnapshot, error) {
			return live, nil
		},
	}}
	start := time.Now().UTC().Add(-time.Minute)
	end := time.Now().UTC()

	dbValue, dbStatus := svc.computeRuleMetric(context.Background(), &OpsAlertRule{MetricType: "db_pool_usage_percent"}, nil, start, end, "", nil)
	require.Equal(t, alertMetricOK, dbStatus)
	require.InDelta(t, 50, dbValue, 0.0001)

	redisValue, redisStatus := svc.computeRuleMetric(context.Background(), &OpsAlertRule{MetricType: "redis_pool_usage_percent"}, nil, start, end, "", nil)
	require.Equal(t, alertMetricOK, redisStatus)
	require.InDelta(t, 90, redisValue, 0.0001)

	historical := &OpsSystemMetricsSnapshot{DBConnActive: &active, DBConnIdle: &idle, DBMaxOpenConns: &limit}
	_, missingStatus := (&OpsAlertEvaluatorService{}).computeRuleMetric(context.Background(), &OpsAlertRule{MetricType: "db_pool_usage_percent"}, historical, start, end, "", nil)
	require.Equal(t, alertMetricUnavailable, missingStatus)

	cases := []struct {
		name  string
		snap  *OpsSystemMetricsSnapshot
		kind  string
		ok    bool
		value float64
	}{
		{name: "zero open connections is a real zero", snap: &OpsSystemMetricsSnapshot{DBConnActive: &zero, DBConnIdle: &zero, DBMaxOpenConns: &limit}, kind: "db_pool_usage_percent", ok: true, value: 0},
		{name: "missing idle is unknown", snap: &OpsSystemMetricsSnapshot{DBConnActive: &active, DBMaxOpenConns: &limit}, kind: "db_pool_usage_percent"},
		{name: "zero database limit is unknown", snap: &OpsSystemMetricsSnapshot{DBConnActive: &active, DBConnIdle: &idle, DBMaxOpenConns: &zero}, kind: "db_pool_usage_percent"},
		{name: "overfull pool reports 100", snap: &OpsSystemMetricsSnapshot{RedisConnTotal: &over, RedisPoolSize: &overLimit}, kind: "redis_pool_usage_percent", ok: true, value: 100},
		{name: "missing redis limit is unknown", snap: &OpsSystemMetricsSnapshot{RedisConnTotal: &redisTotal}, kind: "redis_pool_usage_percent"},
		{name: "negative redis count is unknown", snap: &OpsSystemMetricsSnapshot{RedisConnTotal: &negative, RedisPoolSize: &redisLimit}, kind: "redis_pool_usage_percent"},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			local := &OpsAlertEvaluatorService{opsService: &OpsService{
				alertClusterRuntimeSnapshot: func(context.Context) (*OpsSystemMetricsSnapshot, error) {
					return tt.snap, nil
				},
			}}
			value, status := local.computeRuleMetric(context.Background(), &OpsAlertRule{MetricType: tt.kind}, nil, start, end, "", nil)
			if !tt.ok {
				require.Equal(t, alertMetricUnavailable, status)
				return
			}
			require.Equal(t, alertMetricOK, status)
			require.InDelta(t, tt.value, value, 0.0001)
		})
	}
}

func TestEvaluateOnceResolvedEmailFollowsIncludeResolved(t *testing.T) {
	t.Parallel()

	newSvc := func(t *testing.T, include bool, notify bool) (*OpsAlertEvaluatorService, *stubOpsRepo, *[]string) {
		cfg := defaultOpsEmailNotificationConfig()
		cfg.Alert.Enabled = true
		cfg.Alert.IncludeResolvedAlerts = include
		cfg.Alert.Recipients = []string{"ops@example.com"}
		raw, err := json.Marshal(cfg)
		require.NoError(t, err)
		repo := &stubOpsRepo{
			overview: &OpsDashboardOverview{RequestCountSLA: 10, ErrorRate: 0},
			rules: []*OpsAlertRule{{
				ID:            11,
				Name:          "错误率",
				Enabled:       true,
				NotifyEmail:   notify,
				MetricType:    "error_rate",
				Operator:      ">",
				Threshold:     50,
				WindowMinutes: 1,
				Severity:      "P1",
			}},
			activeEvent: &OpsAlertEvent{ID: 99, RuleID: 11, Status: OpsAlertStatusFiring, EmailSent: true, FiredAt: time.Now().UTC()},
		}
		var subjects []string
		svc := &OpsAlertEvaluatorService{
			opsRepo: repo,
			opsService: &OpsService{settingRepo: &alertSettingsRepo{values: map[string]string{
				SettingKeyOpsEmailNotificationConfig: string(raw),
			}}},
			emailService: &EmailService{},
			emailLimiter: newSlidingWindowLimiter(0, time.Hour),
			ruleStates:   map[int64]*opsAlertRuleState{},
			deliverOpsAlertEmail: func(ctx context.Context, to, subject, body string) error {
				subjects = append(subjects, subject)
				return nil
			},
		}
		return svc, repo, &subjects
	}

	t.Run("disabled by default does not send", func(t *testing.T) {
		t.Parallel()
		svc, repo, subjects := newSvc(t, false, true)
		svc.evaluateOnce(time.Minute)
		require.Equal(t, 1, repo.statusCalls)
		require.Empty(t, *subjects)
		require.Zero(t, repo.emailWrites)
	})

	t.Run("enabled sends one recovery mail even when firing was already emailed", func(t *testing.T) {
		t.Parallel()
		svc, repo, subjects := newSvc(t, true, true)
		svc.evaluateOnce(time.Minute)
		require.Equal(t, []string{"[运维恢复][P1] 错误率"}, *subjects)
		require.Equal(t, 1, repo.emailWrites)
		svc.evaluateOnce(time.Minute)
		require.Equal(t, []string{"[运维恢复][P1] 错误率"}, *subjects)
	})

	t.Run("rule email switch still gates recovery", func(t *testing.T) {
		t.Parallel()
		svc, repo, subjects := newSvc(t, true, false)
		svc.evaluateOnce(time.Minute)
		require.Equal(t, 1, repo.statusCalls)
		require.Empty(t, *subjects)
	})

	t.Run("failed resolve does not send", func(t *testing.T) {
		t.Parallel()
		svc, repo, subjects := newSvc(t, true, true)
		repo.statusErr = context.DeadlineExceeded
		svc.evaluateOnce(time.Minute)
		require.Empty(t, *subjects)
		require.Zero(t, repo.emailWrites)
	})
}
