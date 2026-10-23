package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	opsAlertEvaluatorJobName = "ops_alert_evaluator"

	opsAlertEvaluatorTimeout         = 45 * time.Second
	opsAlertEvaluatorLeaderLockKey   = "ops:alert:evaluator:leader"
	opsAlertEvaluatorLeaderLockTTL   = 90 * time.Second
	opsAlertEvaluatorSkipLogInterval = 1 * time.Minute
)

var opsAlertEvaluatorReleaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

type OpsAlertEvaluatorService struct {
	opsService   *OpsService
	opsRepo      OpsRepository
	emailService *EmailService
	proxyRepo    ProxyRepository

	redisClient redis.UniversalClient
	cfg         *config.Config
	instanceID  string

	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup

	mu         sync.Mutex
	ruleStates map[int64]*opsAlertRuleState

	emailLimiter *slidingWindowLimiter

	// deliverOpsAlertEmail overrides SMTP delivery in unit tests.
	// Production leaves it nil and uses the notification service.
	deliverOpsAlertEmail   func(ctx context.Context, to, subject, body string) error
	monitoringAlertSamples func(context.Context, string, map[string]any, time.Time, time.Time) ([]OpsMonitoringAlertSample, error)

	skipLogMu sync.Mutex
	skipLogAt time.Time

	warnNoRedisOnce sync.Once
}

type opsAlertRuleState struct {
	RuleRevision        int64
	LastEvaluatedAt     time.Time
	ConsecutiveBreaches int
	DataStatus          string
}

type alertMetricStatus int

const (
	alertMetricUnavailable alertMetricStatus = iota
	alertMetricOK
	alertMetricInsufficient
)

func NewOpsAlertEvaluatorService(
	opsService *OpsService,
	opsRepo OpsRepository,
	emailService *EmailService,
	redisClient redis.UniversalClient,
	cfg *config.Config,
	proxyRepo ProxyRepository,
) *OpsAlertEvaluatorService {
	return &OpsAlertEvaluatorService{
		opsService:   opsService,
		opsRepo:      opsRepo,
		emailService: emailService,
		proxyRepo:    proxyRepo,
		redisClient:  redisClient,
		cfg:          cfg,
		instanceID:   uuid.NewString(),
		ruleStates:   map[int64]*opsAlertRuleState{},
		emailLimiter: newSlidingWindowLimiter(0, time.Hour),
	}
}

func (s *OpsAlertEvaluatorService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		if s.stopCh == nil {
			s.stopCh = make(chan struct{})
		}
		s.wg.Add(2)
		go s.run()
		go s.runAlertDeliveryWorker()
	})
}

func (s *OpsAlertEvaluatorService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.stopCh != nil {
			close(s.stopCh)
		}
	})
	s.wg.Wait()
}

func (s *OpsAlertEvaluatorService) run() {
	defer s.wg.Done()

	// Start immediately to produce early feedback in ops dashboard.
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			interval := s.getInterval()
			s.evaluateOnce(interval)
			timer.Reset(interval)
		case <-s.stopCh:
			return
		}
	}
}

func (s *OpsAlertEvaluatorService) getInterval() time.Duration {
	// Default.
	interval := 60 * time.Second

	if s == nil || s.opsService == nil {
		return interval
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cfg, err := s.opsService.GetOpsAlertRuntimeSettings(ctx)
	if err != nil || cfg == nil {
		return interval
	}
	if cfg.EvaluationIntervalSeconds <= 0 {
		return interval
	}
	if cfg.EvaluationIntervalSeconds < 1 {
		return interval
	}
	if cfg.EvaluationIntervalSeconds > int((24 * time.Hour).Seconds()) {
		return interval
	}
	return time.Duration(cfg.EvaluationIntervalSeconds) * time.Second
}

func (s *OpsAlertEvaluatorService) evaluateOnce(interval time.Duration) {
	if s == nil || s.opsRepo == nil {
		return
	}
	if s.cfg != nil && !s.cfg.Ops.Enabled {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), opsAlertEvaluatorTimeout)
	defer cancel()

	if s.opsService != nil && !s.opsService.IsMonitoringEnabled(ctx) {
		return
	}

	runtimeCfg := defaultOpsAlertRuntimeSettings()
	if s.opsService != nil {
		if loaded, err := s.opsService.GetOpsAlertRuntimeSettings(ctx); err == nil && loaded != nil {
			runtimeCfg = loaded
		}
	}

	release, ok := s.tryAcquireLeaderLock(ctx, runtimeCfg.DistributedLock)
	if !ok {
		return
	}
	if release != nil {
		defer release()
	}

	startedAt := time.Now().UTC()
	runAt := startedAt

	rules, err := s.opsRepo.ListAlertRules(ctx)
	if err != nil {
		s.recordHeartbeatError(runAt, time.Since(startedAt), err)
		logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] list rules failed: %v", err)
		return
	}

	rulesTotal := len(rules)
	rulesEnabled := 0
	rulesEvaluated := 0
	eventsCreated := 0
	eventsResolved := 0
	emailsSent := 0

	now := time.Now().UTC()
	safeEnd := now.Truncate(time.Minute)
	if safeEnd.IsZero() {
		safeEnd = now
	}

	systemMetrics, _ := s.opsRepo.GetLatestSystemMetrics(ctx, 1)

	// Reload persisted progress on every leadership cycle; a former leader must
	// not overwrite state advanced by another Pod while it was inactive.
	s.mu.Lock()
	s.ruleStates = map[int64]*opsAlertRuleState{}
	s.mu.Unlock()

	for _, rule := range rules {
		if rule == nil || !rule.Enabled || rule.ID <= 0 {
			continue
		}
		rulesEnabled++
		if !s.prepareRuleEvaluationState(ctx, rule) {
			continue
		}
		if err := ValidateOpsAlertFilters(rule.MetricType, rule.Filters); err != nil {
			s.noteRuleSampleStatus(ctx, rule.ID, now, OpsAlertDataStatusInvalidRule)
			continue
		}

		scopePlatform, scopeGroupID, scopeRegion := parseOpsAlertRuleScope(rule.Filters)

		windowMinutes := rule.WindowMinutes
		if windowMinutes <= 0 {
			windowMinutes = 1
		}
		windowStart := safeEnd.Add(-time.Duration(windowMinutes) * time.Minute)
		windowEnd := safeEnd

		if isMonitoringAlertMetric(rule.MetricType) && !usesLocalDatabaseCapacity(rule) {
			if s.evaluateMonitoringAlertRule(ctx, runtimeCfg, rule, windowStart, windowEnd, now, interval) {
				rulesEvaluated++
			}
			continue
		}
		metricValue, status := s.computeRuleMetric(ctx, rule, systemMetrics, windowStart, windowEnd, scopePlatform, scopeGroupID)
		switch status {
		case alertMetricInsufficient:
			// Insufficient samples break sustained evidence but never resolve an incident.
			s.noteRuleSampleStatus(ctx, rule.ID, now, OpsAlertDataStatusInsufficient)
			continue
		case alertMetricOK:
		default:
			s.resetRuleState(ctx, rule.ID, now)
			continue
		}
		rulesEvaluated++

		breachedNow := compareMetric(metricValue, rule.Operator, rule.Threshold)
		required := requiredSustainedBreaches(rule.SustainedMinutes, interval)
		consecutive, stateOK := s.updateRuleBreaches(ctx, rule.ID, now, interval, breachedNow)
		if !stateOK {
			continue
		}

		activeEvent, err := s.opsRepo.GetActiveAlertEvent(ctx, rule.ID)
		if err != nil {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] get active event failed (rule=%d): %v", rule.ID, err)
			continue
		}
		if activeEvent != nil && alertRuleRevision(activeEvent.RuleRevision) != alertRuleRevision(rule.EvaluationRevision) {
			continue
		}

		if breachedNow && consecutive >= required {
			if activeEvent != nil {
				s.ensureFiringNotification(ctx, runtimeCfg, rule, activeEvent, now)
				s.upgradeAlertEvent(ctx, runtimeCfg, rule, activeEvent, metricValue)
				continue
			}

			// Scoped silencing: if a matching silence exists, skip creating a firing event.
			if s.opsService != nil {
				platform := strings.TrimSpace(scopePlatform)
				region := scopeRegion
				if platform != "" {
					if ok, err := s.opsService.IsAlertSilenced(ctx, rule.ID, platform, scopeGroupID, region, now); err == nil && ok {
						continue
					}
				}
			}

			latestEvent, err := s.opsRepo.GetLatestAlertEvent(ctx, rule.ID)
			if err != nil {
				logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] get latest event failed (rule=%d): %v", rule.ID, err)
				continue
			}
			if latestEvent != nil && alertRuleRevision(latestEvent.RuleRevision) == alertRuleRevision(rule.EvaluationRevision) && rule.CooldownMinutes > 0 {
				cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
				if now.Sub(latestEvent.FiredAt) < cooldown {
					continue
				}
			}

			firedEvent := &OpsAlertEvent{
				RuleRevision:   alertRuleRevision(rule.EvaluationRevision),
				RuleID:         rule.ID,
				Severity:       strings.TrimSpace(rule.Severity),
				Status:         OpsAlertStatusFiring,
				Title:          fmt.Sprintf("%s: %s", strings.TrimSpace(rule.Severity), strings.TrimSpace(rule.Name)),
				Description:    buildOpsAlertDescription(rule, metricValue, windowMinutes, scopePlatform, scopeGroupID),
				MetricValue:    float64Ptr(metricValue),
				ThresholdValue: float64Ptr(rule.Threshold),
				Dimensions:     buildOpsAlertDimensions(scopePlatform, scopeGroupID),
				FiredAt:        now,
				CreatedAt:      now,
			}
			firedEvent.Dimensions["rule_revision"] = alertRuleRevision(rule.EvaluationRevision)
			firedEvent.Dimensions["window_minutes"] = windowMinutes

			if critical, configured := alertCriticalThreshold(rule.Filters); configured && compareMetric(metricValue, rule.Operator, critical) {
				firedEvent.Severity = "P0"
				firedEvent.Title = "P0: " + rule.Name
			}
			firedEvent.Deliveries, err = s.prepareAlertDeliveries(ctx, runtimeCfg, rule, firedEvent, "firing")
			if err != nil {
				continue
			}
			created, err := s.opsRepo.CreateAlertEvent(ctx, firedEvent)
			if err != nil {
				logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] create event failed (rule=%d): %v", rule.ID, err)
				continue
			}

			eventsCreated++
			if _, durable := s.opsRepo.(OpsAlertDeliveryRepository); !durable && created != nil && created.ID > 0 {
				if s.maybeSendAlertEmail(ctx, runtimeCfg, rule, created) {
					emailsSent++
				}
			}
			continue
		}

		// Not breached: resolve active event if present.
		if !breachedNow && activeEvent != nil {
			if _, durable := s.opsRepo.(OpsAlertDeliveryRepository); durable {
				if s.resolveAlertWithDeliveries(ctx, runtimeCfg, rule, activeEvent, now, metricValue) {
					eventsResolved++
				}
				continue
			}
			resolvedAt := now
			if err := s.opsRepo.UpdateAlertEventStatus(ctx, activeEvent.ID, OpsAlertStatusResolved, &resolvedAt); err != nil {
				logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] resolve event failed (event=%d): %v", activeEvent.ID, err)
			} else {
				eventsResolved++
				activeEvent.Status = OpsAlertStatusResolved
				activeEvent.ResolvedAt = &resolvedAt
				if s.maybeSendResolvedAlertEmail(ctx, runtimeCfg, rule, activeEvent) {
					emailsSent++
				}
			}
		}
	}

	result := truncateString(fmt.Sprintf("rules=%d enabled=%d evaluated=%d created=%d resolved=%d emails_sent=%d", rulesTotal, rulesEnabled, rulesEvaluated, eventsCreated, eventsResolved, emailsSent), 2048)
	s.recordHeartbeatSuccess(runAt, time.Since(startedAt), result)
}

func (s *OpsAlertEvaluatorService) resetRuleState(ctx context.Context, ruleID int64, now time.Time) {
	if s == nil || ruleID <= 0 {
		return
	}
	state, ok := s.ruleState(ctx, ruleID)
	if !ok {
		return
	}
	s.mu.Lock()
	state.LastEvaluatedAt = now
	state.ConsecutiveBreaches = 0
	state.DataStatus = OpsAlertDataStatusUnavailable
	snapshot := *state
	s.mu.Unlock()
	s.persistRuleState(ctx, ruleID, snapshot)
}

func (s *OpsAlertEvaluatorService) noteRuleSampleStatus(ctx context.Context, ruleID int64, now time.Time, dataStatus string) {
	if s == nil || ruleID <= 0 {
		return
	}
	state, ok := s.ruleState(ctx, ruleID)
	if !ok {
		return
	}
	s.mu.Lock()
	state.LastEvaluatedAt = now
	state.DataStatus = dataStatus
	if dataStatus != OpsAlertDataStatusOK {
		state.ConsecutiveBreaches = 0
	}
	snapshot := *state
	s.mu.Unlock()
	s.persistRuleState(ctx, ruleID, snapshot)
}

func (s *OpsAlertEvaluatorService) updateRuleBreaches(ctx context.Context, ruleID int64, now time.Time, interval time.Duration, breached bool) (int, bool) {
	if s == nil || ruleID <= 0 {
		return 0, false
	}
	state, ok := s.ruleState(ctx, ruleID)
	if !ok {
		return 0, false
	}

	s.mu.Lock()
	if !state.LastEvaluatedAt.IsZero() && interval > 0 {
		if now.Sub(state.LastEvaluatedAt) < interval {
			count := state.ConsecutiveBreaches
			s.mu.Unlock()
			return count, false
		}
		if now.Sub(state.LastEvaluatedAt) >= interval*2 {
			state.ConsecutiveBreaches = 0
		}
	}
	state.LastEvaluatedAt = now
	state.DataStatus = OpsAlertDataStatusOK
	if breached {
		state.ConsecutiveBreaches++
	} else {
		state.ConsecutiveBreaches = 0
	}
	count := state.ConsecutiveBreaches
	snapshot := *state
	s.mu.Unlock()
	return count, s.persistRuleState(ctx, ruleID, snapshot)
}

// ruleState returns the in-memory progress for a rule. Database reads happen
// outside s.mu so a slow query cannot block other rule-state updates.
func (s *OpsAlertEvaluatorService) ruleState(ctx context.Context, ruleID int64) (*opsAlertRuleState, bool) {
	s.mu.Lock()
	if s.ruleStates == nil {
		s.ruleStates = map[int64]*opsAlertRuleState{}
	}
	if state, ok := s.ruleStates[ruleID]; ok {
		s.mu.Unlock()
		return state, true
	}
	s.mu.Unlock()

	loaded := opsAlertRuleState{}
	if s.opsRepo != nil {
		persisted, err := s.opsRepo.GetAlertRuleState(ctx, ruleID)
		if err != nil {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] load rule state failed (rule=%d): %v", ruleID, err)
			return nil, false
		}
		if persisted != nil {
			loaded.RuleRevision = persisted.RuleRevision
			loaded.ConsecutiveBreaches = persisted.ConsecutiveBreaches
			loaded.LastEvaluatedAt = persisted.LastEvaluatedAt
			loaded.DataStatus = persisted.DataStatus
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ruleStates == nil {
		s.ruleStates = map[int64]*opsAlertRuleState{}
	}
	if state, ok := s.ruleStates[ruleID]; ok {
		return state, true
	}
	state := loaded
	s.ruleStates[ruleID] = &state
	return s.ruleStates[ruleID], true
}

func (s *OpsAlertEvaluatorService) persistRuleState(ctx context.Context, ruleID int64, state opsAlertRuleState) bool {
	if s == nil || s.opsRepo == nil || ruleID <= 0 {
		return s != nil && s.opsRepo == nil
	}
	err := s.opsRepo.UpsertAlertRuleState(ctx, &OpsAlertRuleState{
		RuleRevision:        alertRuleRevision(state.RuleRevision),
		RuleID:              ruleID,
		ConsecutiveBreaches: state.ConsecutiveBreaches,
		LastEvaluatedAt:     state.LastEvaluatedAt,
		DataStatus:          state.DataStatus,
	})
	if err != nil {
		logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] persist rule state failed (rule=%d): %v", ruleID, err)
	}
	return err == nil
}

func alertRuleRevision(revision int64) int64 {
	if revision < 1 {
		return 1
	}
	return revision
}

func (s *OpsAlertEvaluatorService) prepareRuleEvaluationState(ctx context.Context, rule *OpsAlertRule) bool {
	state, ok := s.ruleState(ctx, rule.ID)
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if alertRuleRevision(state.RuleRevision) != alertRuleRevision(rule.EvaluationRevision) {
		*state = opsAlertRuleState{}
	}
	state.RuleRevision = alertRuleRevision(rule.EvaluationRevision)
	return true
}

func requiredSustainedBreaches(sustainedMinutes int, interval time.Duration) int {
	if sustainedMinutes <= 0 {
		return 1
	}
	if interval <= 0 {
		return sustainedMinutes
	}
	// The first sample starts the interval; it does not prove an elapsed interval.
	required := int(math.Ceil(float64(sustainedMinutes*60)/interval.Seconds())) + 1
	if required < 1 {
		return 1
	}
	return required
}

func parseOpsAlertRuleScope(filters map[string]any) (platform string, groupID *int64, region *string) {
	if filters == nil {
		return "", nil, nil
	}
	if v, ok := filters["platform"]; ok {
		if s, ok := v.(string); ok {
			platform = strings.TrimSpace(s)
		}
	}
	if v, ok := filters["group_id"]; ok {
		switch t := v.(type) {
		case float64:
			if t > 0 {
				id := int64(t)
				groupID = &id
			}
		case int64:
			if t > 0 {
				id := t
				groupID = &id
			}
		case int:
			if t > 0 {
				id := int64(t)
				groupID = &id
			}
		case string:
			n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
			if err == nil && n > 0 {
				groupID = &n
			}
		}
	}
	if v, ok := filters["region"]; ok {
		if s, ok := v.(string); ok {
			vv := strings.TrimSpace(s)
			if vv != "" {
				region = &vv
			}
		}
	}
	return platform, groupID, region
}

func (s *OpsAlertEvaluatorService) computeRuleMetric(
	ctx context.Context,
	rule *OpsAlertRule,
	systemMetrics *OpsSystemMetricsSnapshot,
	start time.Time,
	end time.Time,
	platform string,
	groupID *int64,
) (float64, alertMetricStatus) {
	if rule == nil {
		return 0, alertMetricUnavailable
	}
	switch strings.TrimSpace(rule.MetricType) {
	case "billing_failed_settlements", "billing_compensation_backlog", "billing_ledger_mismatches":
		if s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		state, err := s.opsService.GetBillingReconciliation(ctx)
		if err != nil || state == nil || state.OperationalState != "Available" {
			return 0, alertMetricUnavailable
		}
		value := state.FailedSettlements
		switch rule.MetricType {
		case "billing_compensation_backlog":
			value = state.CompensationBacklog
		case "billing_ledger_mismatches":
			value = state.LedgerMismatches
		}
		if value == nil {
			return 0, alertMetricUnavailable
		}
		return float64(*value), alertMetricOK
	case "billing_missing_usage_logs", "billing_amount_mismatches":
		if s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		state, err := s.opsService.GetBillingReconciliation(ctx)
		if err != nil || state == nil || state.State != "Available" {
			return 0, alertMetricUnavailable
		}
		value := state.MissingUsageLogs
		if rule.MetricType == "billing_amount_mismatches" {
			value = state.AmountMismatches
		}
		if value == nil {
			return 0, alertMetricUnavailable
		}
		return float64(*value), alertMetricOK
	case "db_instance_usage_percent", "redis_instance_usage_percent":
		if systemMetrics == nil || systemMetrics.Capacity == nil || time.Since(systemMetrics.CreatedAt) > 3*time.Minute {
			return 0, alertMetricUnavailable
		}
		capacity := systemMetrics.Capacity.PostgreSQL
		if rule.MetricType == "redis_instance_usage_percent" {
			capacity = systemMetrics.Capacity.Redis
		}
		return databaseInstanceUsage(capacity)
	case "cpu_usage_percent":
		if systemMetrics != nil && systemMetrics.CPUUsagePercent != nil {
			return *systemMetrics.CPUUsagePercent, alertMetricOK
		}
		return 0, alertMetricUnavailable
	case "memory_usage_percent":
		if systemMetrics != nil && systemMetrics.MemoryUsagePercent != nil {
			return *systemMetrics.MemoryUsagePercent, alertMetricOK
		}
		return 0, alertMetricUnavailable
	case "db_pool_usage_percent":
		// Historical rows omit pool limits. Do not divide a summed cluster
		// count by this process's configured MaxOpenConns.
		snap := s.applicationPoolSnapshot(ctx)
		if snap == nil {
			return 0, alertMetricUnavailable
		}
		return poolUsagePercent(sumOptionalInts(snap.DBConnActive, snap.DBConnIdle), snap.DBMaxOpenConns)
	case "db_advisory_lock_pool_usage_percent", "db_read_replica_pool_usage_percent":
		snap := s.applicationPoolSnapshot(ctx)
		if snap == nil {
			return 0, alertMetricUnavailable
		}
		role := "advisory_lock"
		if rule.MetricType == "db_read_replica_pool_usage_percent" {
			role = "read_replica"
		}
		return databaseRolePoolUsage(snap.DatabasePools, role)
	case "redis_pool_usage_percent":
		snap := s.applicationPoolSnapshot(ctx)
		if snap == nil {
			return 0, alertMetricUnavailable
		}
		return poolUsagePercent(snap.RedisConnTotal, snap.RedisPoolSize)
	case "concurrency_queue_depth":
		if systemMetrics != nil && systemMetrics.ConcurrencyQueueDepth != nil {
			return float64(*systemMetrics.ConcurrencyQueueDepth), alertMetricOK
		}
		return 0, alertMetricUnavailable
	case "group_available_accounts":
		if groupID == nil || *groupID <= 0 {
			return 0, alertMetricUnavailable
		}
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		if availability.Group == nil {
			return 0, alertMetricOK
		}
		return float64(availability.Group.AvailableCount), alertMetricOK
	case "group_available_ratio":
		if groupID == nil || *groupID <= 0 {
			return 0, alertMetricUnavailable
		}
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		return computeGroupAvailableRatio(availability.Group), alertMetricOK
	case "account_rate_limited_count":
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.IsRateLimited
		})), alertMetricOK
	case "account_error_count":
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})), alertMetricOK
	case "account_temp_unscheduled_count":
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		now := time.Now().UTC()
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.TempUnschedulableUntil != nil && now.Before(*acc.TempUnschedulableUntil)
		})), alertMetricOK
	case "group_rate_limit_ratio":
		if groupID == nil || *groupID <= 0 {
			return 0, alertMetricUnavailable
		}
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		if availability.Group == nil || availability.Group.TotalAccounts <= 0 {
			return 0, alertMetricOK
		}
		return (float64(availability.Group.RateLimitCount) / float64(availability.Group.TotalAccounts)) * 100, alertMetricOK
	case "account_error_ratio":
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		total := int64(len(availability.Accounts))
		if total <= 0 {
			return 0, alertMetricOK
		}
		errorCount := countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.HasError && acc.TempUnschedulableUntil == nil
		})
		return (float64(errorCount) / float64(total)) * 100, alertMetricOK
	case "overload_account_count":
		if s == nil || s.opsService == nil {
			return 0, alertMetricUnavailable
		}
		availability, err := s.opsService.GetAccountAvailability(ctx, platform, groupID)
		if err != nil || availability == nil {
			return 0, alertMetricUnavailable
		}
		return float64(countAccountsByCondition(availability.Accounts, func(acc *AccountAvailability) bool {
			return acc.IsOverloaded
		})), alertMetricOK
	case "proxy_expired_count":
		if s == nil || s.proxyRepo == nil {
			return 0, alertMetricUnavailable
		}
		n, err := s.proxyRepo.CountExpired(ctx)
		if err != nil {
			return 0, alertMetricUnavailable
		}
		return float64(n), alertMetricOK
	case "proxy_expiring_soon_count":
		if s == nil || s.proxyRepo == nil {
			return 0, alertMetricUnavailable
		}
		n, err := s.proxyRepo.CountExpiringSoon(ctx, time.Now())
		if err != nil {
			return 0, alertMetricUnavailable
		}
		return float64(n), alertMetricOK
	}

	overview, err := s.opsRepo.GetDashboardOverview(ctx, &OpsDashboardFilter{
		StartTime: start,
		EndTime:   end,
		Platform:  platform,
		GroupID:   groupID,
		QueryMode: OpsQueryModeRaw,
	})
	if err != nil {
		return 0, alertMetricUnavailable
	}
	if overview == nil {
		return 0, alertMetricUnavailable
	}

	switch strings.TrimSpace(rule.MetricType) {
	case "success_rate":
		if status := requestSampleStatus(rule, overview.RequestCountSLA); status != alertMetricOK {
			return 0, status
		}
		return overview.SLA * 100, alertMetricOK
	case "error_rate":
		if status := requestSampleStatus(rule, overview.RequestCountSLA); status != alertMetricOK {
			return 0, status
		}
		return overview.ErrorRate * 100, alertMetricOK
	case "upstream_error_rate":
		if status := requestSampleStatus(rule, overview.RequestCountSLA); status != alertMetricOK {
			return 0, status
		}
		return overview.UpstreamErrorRate * 100, alertMetricOK
	default:
		return 0, alertMetricUnavailable
	}
}

func (s *OpsAlertEvaluatorService) applicationPoolSnapshot(ctx context.Context) *OpsSystemMetricsSnapshot {
	if s == nil || s.opsService == nil {
		return nil
	}
	snap, err := s.opsService.AlertClusterRuntimeSnapshot(ctx)
	if err != nil || snap == nil {
		return nil
	}
	return snap
}

// sumOptionalInts adds every value. A missing or negative part makes the sum
// unknown so an alert does not treat a partial pool reading as zero.
func sumOptionalInts(parts ...*int) *int {
	total := 0
	for _, part := range parts {
		if part == nil || *part < 0 {
			return nil
		}
		total += *part
	}
	return &total
}

// poolUsagePercent matches the ops dashboard: open database connections
// (active + idle) or current Redis connections, divided by the summed
// application pool limit. A missing or non-positive limit is unavailable.
// Values above 100 are reported as 100, the same cap as the dashboard.
func poolUsagePercent(used, limit *int) (float64, alertMetricStatus) {
	if used == nil || *used < 0 || limit == nil || *limit <= 0 {
		return 0, alertMetricUnavailable
	}
	ratio := 100 * float64(*used) / float64(*limit)
	if ratio > 100 {
		ratio = 100
	}
	return roundTo1DP(ratio), alertMetricOK
}

func databaseRolePoolUsage(pools []OpsDatabasePoolStats, role string) (float64, alertMetricStatus) {
	used, limit, samples := 0, 0, 0
	for _, pool := range pools {
		if pool.Role != role {
			continue
		}
		if pool.Limit <= 0 || pool.Active < 0 || pool.Idle < 0 {
			return 0, alertMetricUnavailable
		}
		samples++
		used += pool.Active + pool.Idle
		limit += pool.Limit
	}
	if samples == 0 {
		return 0, alertMetricUnavailable
	}
	return poolUsagePercent(&used, &limit)
}

func requestSampleStatus(rule *OpsAlertRule, requestCount int64) alertMetricStatus {
	minRequests := 0
	if rule != nil {
		minRequests = rule.MinRequests
	}
	if minRequests > 0 && requestCount < int64(minRequests) {
		return alertMetricInsufficient
	}
	if requestCount <= 0 {
		return alertMetricUnavailable
	}
	return alertMetricOK
}

func compareMetric(value float64, operator string, threshold float64) bool {
	switch strings.TrimSpace(operator) {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

func buildOpsAlertDimensions(platform string, groupID *int64) map[string]any {
	dims := map[string]any{}
	if strings.TrimSpace(platform) != "" {
		dims["platform"] = strings.TrimSpace(platform)
	}
	if groupID != nil && *groupID > 0 {
		dims["group_id"] = *groupID
	}
	if len(dims) == 0 {
		return nil
	}
	return dims
}

func buildOpsAlertDescription(rule *OpsAlertRule, value float64, windowMinutes int, platform string, groupID *int64) string {
	if rule == nil {
		return ""
	}
	scope := "overall"
	if strings.TrimSpace(platform) != "" {
		scope = fmt.Sprintf("platform=%s", strings.TrimSpace(platform))
	}
	if groupID != nil && *groupID > 0 {
		scope = fmt.Sprintf("%s group_id=%d", scope, *groupID)
	}
	if windowMinutes <= 0 {
		windowMinutes = 1
	}
	return fmt.Sprintf("%s %s %.2f (current %.2f) over last %dm (%s)",
		strings.TrimSpace(rule.MetricType),
		strings.TrimSpace(rule.Operator),
		rule.Threshold,
		value,
		windowMinutes,
		strings.TrimSpace(scope),
	)
}

func (s *OpsAlertEvaluatorService) maybeSendAlertEmail(ctx context.Context, runtimeCfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent) bool {
	if s == nil || s.emailService == nil || s.opsService == nil || event == nil || rule == nil {
		return false
	}
	if event.EmailSent {
		return false
	}
	return s.sendOpsAlertEmail(ctx, runtimeCfg, rule, event, false)
}

func (s *OpsAlertEvaluatorService) maybeSendResolvedAlertEmail(ctx context.Context, runtimeCfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent) bool {
	if s == nil || s.emailService == nil || s.opsService == nil || event == nil || rule == nil {
		return false
	}
	// Firing already sets EmailSent. Recovery is a separate send and must not
	// reuse that guard. It still happens only once, at the resolve write.
	return s.sendOpsAlertEmail(ctx, runtimeCfg, rule, event, true)
}

func (s *OpsAlertEvaluatorService) sendOpsAlertEmail(ctx context.Context, runtimeCfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent, resolved bool) bool {
	if !rule.NotifyEmail {
		return false
	}

	emailCfg, err := s.opsService.GetEmailNotificationConfig(ctx)
	if err != nil || emailCfg == nil || !emailCfg.Alert.Enabled {
		return false
	}
	if resolved && !emailCfg.Alert.IncludeResolvedAlerts {
		return false
	}
	if len(emailCfg.Alert.Recipients) == 0 {
		return false
	}
	if !shouldSendOpsAlertEmailByMinSeverity(strings.TrimSpace(emailCfg.Alert.MinSeverity), strings.TrimSpace(rule.Severity)) {
		return false
	}

	if runtimeCfg != nil && runtimeCfg.Silencing.Enabled {
		if isOpsAlertSilenced(time.Now().UTC(), rule, event, runtimeCfg.Silencing) {
			return false
		}
	}

	// Apply/update rate limiter. Recovery shares the firing budget.
	s.emailLimiter.SetLimit(emailCfg.Alert.RateLimitPerHour)

	anySent := false
	for _, to := range emailCfg.Alert.Recipients {
		addr := strings.TrimSpace(to)
		if addr == "" {
			continue
		}
		if !s.emailLimiter.Allow(time.Now().UTC()) {
			continue
		}
		locale := notificationEmailLocaleChinese
		if s.emailService.notificationEmailService != nil {
			locale = s.emailService.notificationEmailService.ResolveOpsEventMailLocale(ctx, addr, NotificationEmailEventOpsAlert)
		}
		subject := opsAlertEmailSubject(rule, event, locale)
		body := buildOpsAlertEmailBody(rule, event, locale)
		if s.deliverOpsAlertEmail != nil {
			if err := s.deliverOpsAlertEmail(ctx, addr, subject, body); err != nil {
				continue
			}
			anySent = true
			continue
		}
		if s.emailService.notificationEmailService != nil {
			if err := s.emailService.notificationEmailService.Send(ctx, NotificationEmailSendInput{
				Event:          NotificationEmailEventOpsAlert,
				Locale:         locale,
				RecipientEmail: addr,
				RecipientName:  emailRecipientName(addr),
				SourceType:     "ops_alert",
				SourceID:       fmt.Sprintf("%d", event.ID),
				Variables:      opsAlertEmailVariables(rule, event, locale),
			}); err == nil {
				anySent = true
				continue
			} else if !shouldFallbackNotificationEmail(err) {
				continue
			}
		}
		if err := s.emailService.SendEmail(ctx, addr, subject, body); err != nil {
			// Ignore per-recipient failures; continue best-effort.
			continue
		}
		anySent = true
	}

	if anySent {
		_ = s.opsRepo.UpdateAlertEventEmailSent(context.Background(), event.ID, true)
	}
	return anySent
}

func opsAlertEmailHeadline(event *OpsAlertEvent, locale string) string {
	resolved := event != nil && strings.TrimSpace(event.Status) == OpsAlertStatusResolved
	if normalizeNotificationLocale(locale) == notificationEmailLocaleChinese {
		if resolved {
			return "运维恢复"
		}
		return "运维告警"
	}
	if resolved {
		return "Ops Resolved"
	}
	return "Ops Alert"
}

func opsAlertEmailSubject(rule *OpsAlertRule, event *OpsAlertEvent, locale string) string {
	severity := "-"
	name := "-"
	if rule != nil {
		severity = strings.TrimSpace(rule.Severity)
		name = strings.TrimSpace(rule.Name)
	}
	if event != nil && strings.TrimSpace(event.Severity) != "" {
		severity = event.Severity
	}
	return fmt.Sprintf("[%s][%s] %s", opsAlertEmailHeadline(event, locale), severity, name)
}

func opsAlertEmailVariables(rule *OpsAlertRule, event *OpsAlertEvent, locale string) map[string]string {
	variables := map[string]string{
		"rule_name":         "-",
		"severity":          "-",
		"alert_status":      "-",
		"alert_headline":    opsAlertEmailHeadline(event, locale),
		"metric_type":       "-",
		"operator":          "-",
		"metric_value":      "-",
		"threshold_value":   "-",
		"triggered_at":      formatOpsAlertTime(time.Now()),
		"alert_description": "-",
	}
	if rule != nil {
		variables["rule_name"] = strings.TrimSpace(rule.Name)
		variables["severity"] = strings.TrimSpace(rule.Severity)
		variables["metric_type"] = strings.TrimSpace(rule.MetricType)
		variables["operator"] = strings.TrimSpace(rule.Operator)
		variables["threshold_value"] = fmt.Sprintf("%.2f", rule.Threshold)
		if strings.TrimSpace(rule.Description) != "" {
			variables["alert_description"] = strings.TrimSpace(rule.Description)
		}
	}
	if event != nil {
		variables["alert_status"] = strings.TrimSpace(event.Status)
		if event.Severity != "" {
			variables["severity"] = event.Severity
		}
		if event.MetricValue != nil {
			variables["metric_value"] = fmt.Sprintf("%.2f", *event.MetricValue)
		}
		if event.ThresholdValue != nil {
			variables["threshold_value"] = fmt.Sprintf("%.2f", *event.ThresholdValue)
		}
		if !event.FiredAt.IsZero() {
			variables["triggered_at"] = formatOpsAlertTime(event.FiredAt)
		} else {
			variables["triggered_at"] = formatOpsAlertTime(time.Now())
		}
		if strings.TrimSpace(event.Description) != "" {
			variables["alert_description"] = strings.TrimSpace(event.Description)
		}
	}
	return variables
}

func formatOpsAlertTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return t.UTC().Format(time.RFC3339) + " UTC"
	}
	return t.In(loc).Format("2006-01-02 15:04:05 -0700") + " Asia/Shanghai"
}

func buildOpsAlertEmailBody(rule *OpsAlertRule, event *OpsAlertEvent, locale string) string {
	if rule == nil || event == nil {
		return ""
	}
	metric := strings.TrimSpace(rule.MetricType)
	value := "-"
	threshold := fmt.Sprintf("%.2f", rule.Threshold)
	if event.MetricValue != nil {
		value = fmt.Sprintf("%.2f", *event.MetricValue)
	}
	if event.ThresholdValue != nil {
		threshold = fmt.Sprintf("%.2f", *event.ThresholdValue)
	}
	severity := event.Severity
	if severity == "" {
		severity = rule.Severity
	}
	firedAt := formatOpsAlertTime(event.FiredAt)
	headline := htmlEscape(opsAlertEmailHeadline(event, locale))
	if normalizeNotificationLocale(locale) == notificationEmailLocaleChinese {
		return fmt.Sprintf(`
<h2>%s</h2>
<p><b>规则</b>：%s</p>
<p><b>严重级别</b>：%s</p>
<p><b>状态</b>：%s</p>
<p><b>指标</b>：%s %s %s</p>
<p><b>触发时间</b>：%s</p>
<p><b>说明</b>：%s</p>
`,
			headline,
			htmlEscape(rule.Name),
			htmlEscape(severity),
			htmlEscape(event.Status),
			htmlEscape(metric),
			htmlEscape(rule.Operator),
			htmlEscape(fmt.Sprintf("%s（阈值 %s）", value, threshold)),
			htmlEscape(firedAt),
			htmlEscape(event.Description),
		)
	}
	return fmt.Sprintf(`
<h2>%s</h2>
<p><b>Rule</b>: %s</p>
<p><b>Severity</b>: %s</p>
<p><b>Status</b>: %s</p>
<p><b>Metric</b>: %s %s %s</p>
<p><b>Fired at</b>: %s</p>
<p><b>Description</b>: %s</p>
`,
		headline,
		htmlEscape(rule.Name),
		htmlEscape(severity),
		htmlEscape(event.Status),
		htmlEscape(metric),
		htmlEscape(rule.Operator),
		htmlEscape(fmt.Sprintf("%s (threshold %s)", value, threshold)),
		htmlEscape(firedAt),
		htmlEscape(event.Description),
	)
}

func shouldSendOpsAlertEmailByMinSeverity(minSeverity string, ruleSeverity string) bool {
	minSeverity = strings.ToLower(strings.TrimSpace(minSeverity))
	if minSeverity == "" {
		return true
	}

	eventLevel := opsEmailSeverityForOps(ruleSeverity)
	minLevel := strings.ToLower(minSeverity)

	rank := func(level string) int {
		switch level {
		case "critical":
			return 3
		case "warning":
			return 2
		case "info":
			return 1
		default:
			return 0
		}
	}
	return rank(eventLevel) >= rank(minLevel)
}

func opsEmailSeverityForOps(severity string) string {
	switch strings.ToUpper(strings.TrimSpace(severity)) {
	case "P0":
		return "critical"
	case "P1":
		return "warning"
	default:
		return "info"
	}
}

func isOpsAlertSilenced(now time.Time, rule *OpsAlertRule, event *OpsAlertEvent, silencing OpsAlertSilencingSettings) bool {
	if !silencing.Enabled {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if strings.TrimSpace(silencing.GlobalUntilRFC3339) != "" {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(silencing.GlobalUntilRFC3339)); err == nil {
			if now.Before(t) {
				return true
			}
		}
	}

	for _, entry := range silencing.Entries {
		untilRaw := strings.TrimSpace(entry.UntilRFC3339)
		if untilRaw == "" {
			continue
		}
		until, err := time.Parse(time.RFC3339, untilRaw)
		if err != nil {
			continue
		}
		if now.After(until) {
			continue
		}
		if entry.RuleID != nil && rule != nil && rule.ID > 0 && *entry.RuleID != rule.ID {
			continue
		}
		if len(entry.Severities) > 0 {
			match := false
			for _, s := range entry.Severities {
				if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(event.Severity)) || strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(rule.Severity)) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		return true
	}

	return false
}

func (s *OpsAlertEvaluatorService) tryAcquireLeaderLock(ctx context.Context, lock OpsDistributedLockSettings) (func(), bool) {
	if !lock.Enabled {
		return nil, true
	}
	if s.redisClient == nil {
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] redis not configured; running without distributed lock")
		})
		return nil, true
	}
	key := strings.TrimSpace(lock.Key)
	if key == "" {
		key = opsAlertEvaluatorLeaderLockKey
	}
	ttl := time.Duration(lock.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = opsAlertEvaluatorLeaderLockTTL
	}

	ok, err := s.redisClient.SetNX(ctx, key, s.instanceID, ttl).Result()
	if err != nil {
		// Prefer fail-closed to avoid duplicate evaluators stampeding the DB when Redis is flaky.
		// Single-node deployments can disable the distributed lock via runtime settings.
		s.warnNoRedisOnce.Do(func() {
			logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] leader lock SetNX failed; skipping this cycle: %v", err)
		})
		return nil, false
	}
	if !ok {
		s.maybeLogSkip(key)
		return nil, false
	}
	return func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		_, _ = opsAlertEvaluatorReleaseScript.Run(releaseCtx, s.redisClient, []string{key}, s.instanceID).Result()
	}, true
}

func (s *OpsAlertEvaluatorService) maybeLogSkip(key string) {
	s.skipLogMu.Lock()
	defer s.skipLogMu.Unlock()

	now := time.Now()
	if !s.skipLogAt.IsZero() && now.Sub(s.skipLogAt) < opsAlertEvaluatorSkipLogInterval {
		return
	}
	s.skipLogAt = now
	logger.LegacyPrintf("service.ops_alert_evaluator", "[OpsAlertEvaluator] leader lock held by another instance; skipping (key=%q)", key)
}

func (s *OpsAlertEvaluatorService) recordHeartbeatSuccess(runAt time.Time, duration time.Duration, result string) {
	if s == nil || s.opsRepo == nil {
		return
	}
	now := time.Now().UTC()
	durMs := duration.Milliseconds()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	msg := strings.TrimSpace(result)
	if msg == "" {
		msg = "ok"
	}
	msg = truncateString(msg, 2048)
	_ = s.opsRepo.UpsertJobHeartbeat(ctx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsAlertEvaluatorJobName,
		LastRunAt:      &runAt,
		LastSuccessAt:  &now,
		LastDurationMs: &durMs,
		LastResult:     &msg,
	})
}

func (s *OpsAlertEvaluatorService) recordHeartbeatError(runAt time.Time, duration time.Duration, err error) {
	if s == nil || s.opsRepo == nil || err == nil {
		return
	}
	now := time.Now().UTC()
	durMs := duration.Milliseconds()
	msg := truncateString(err.Error(), 2048)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.opsRepo.UpsertJobHeartbeat(ctx, &OpsUpsertJobHeartbeatInput{
		JobName:        opsAlertEvaluatorJobName,
		LastRunAt:      &runAt,
		LastErrorAt:    &now,
		LastError:      &msg,
		LastDurationMs: &durMs,
	})
}

func htmlEscape(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(s)
}

type slidingWindowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	sent   []time.Time
}

func newSlidingWindowLimiter(limit int, window time.Duration) *slidingWindowLimiter {
	if window <= 0 {
		window = time.Hour
	}
	return &slidingWindowLimiter{
		limit:  limit,
		window: window,
		sent:   []time.Time{},
	}
}

func (l *slidingWindowLimiter) SetLimit(limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.limit = limit
}

func (l *slidingWindowLimiter) Allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.limit <= 0 {
		return true
	}
	cutoff := now.Add(-l.window)
	keep := l.sent[:0]
	for _, t := range l.sent {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	l.sent = keep
	if len(l.sent) >= l.limit {
		return false
	}
	l.sent = append(l.sent, now)
	return true
}

// computeGroupAvailableRatio returns the available percentage for a group.
// Formula: (AvailableCount / TotalAccounts) * 100.
// Returns 0 when TotalAccounts is 0.
func computeGroupAvailableRatio(group *GroupAvailability) float64 {
	if group == nil || group.TotalAccounts <= 0 {
		return 0
	}
	return (float64(group.AvailableCount) / float64(group.TotalAccounts)) * 100
}

// countAccountsByCondition counts accounts that satisfy the given condition.
func countAccountsByCondition(accounts map[int64]*AccountAvailability, condition func(*AccountAvailability) bool) int64 {
	if len(accounts) == 0 || condition == nil {
		return 0
	}
	var count int64
	for _, account := range accounts {
		if account != nil && condition(account) {
			count++
		}
	}
	return count
}
