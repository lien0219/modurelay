package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type opsAlertEmailPayload struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type opsAlertWebhookPayload struct {
	Version    int            `json:"version"`
	Transition string         `json:"transition"`
	RuleID     int64          `json:"rule_id"`
	RuleName   string         `json:"rule_name"`
	Event      *OpsAlertEvent `json:"event"`
}

func (s *OpsAlertEvaluatorService) prepareAlertDeliveries(ctx context.Context, cfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent, transition string) ([]*OpsAlertDelivery, error) {
	return s.prepareAlertDeliveriesAt(ctx, cfg, rule, event, transition, time.Now().UTC())
}

func (s *OpsAlertEvaluatorService) prepareAlertDeliveriesAt(ctx context.Context, cfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent, transition string, now time.Time) ([]*OpsAlertDelivery, error) {
	if _, ok := s.opsRepo.(OpsAlertDeliveryRepository); !ok {
		return nil, nil
	}
	if raw, ok := rule.Filters["silenced_until"].(string); ok {
		if until, err := time.Parse(time.RFC3339, raw); err == nil && now.Before(until) {
			return nil, nil
		}
	}
	if cfg != nil && isOpsAlertSilenced(now, rule, event, cfg.Silencing) {
		return nil, nil
	}
	if key, ok := event.Dimensions["resource_key"].(string); ok && key != "" {
		silenced, err := s.opsRepo.(OpsAlertDeliveryRepository).IsAlertResourceSilenced(ctx, rule.ID, key, now)
		if err != nil {
			return nil, err
		}
		if silenced {
			return nil, nil
		}
	} else if platform, groupID, region := parseOpsAlertRuleScope(rule.Filters); platform != "" {
		silenced, err := s.opsRepo.IsAlertSilenced(ctx, rule.ID, platform, groupID, region, now)
		if err != nil {
			return nil, err
		}
		if silenced {
			return nil, nil
		}
	}
	deliveries := []*OpsAlertDelivery{}
	if rule.NotifyEmail && s.opsService != nil {
		emailCfg, err := s.opsService.GetEmailNotificationConfig(ctx)
		if err != nil {
			return nil, err
		}
		if emailCfg != nil && emailCfg.Alert.Enabled && (transition != "resolved" || emailCfg.Alert.IncludeResolvedAlerts) && shouldSendOpsAlertEmailByMinSeverity(emailCfg.Alert.MinSeverity, event.Severity) {
			for _, to := range emailCfg.Alert.Recipients {
				to = strings.TrimSpace(to)
				if to == "" {
					continue
				}
				locale := notificationEmailLocaleChinese
				if s.emailService != nil && s.emailService.notificationEmailService != nil {
					locale = s.emailService.notificationEmailService.ResolveOpsEventMailLocale(ctx, to, NotificationEmailEventOpsAlert)
				}
				snapshot := opsAlertEmailPayload{Subject: opsAlertEmailSubject(rule, event, locale), Body: buildOpsAlertEmailBody(rule, event, locale)}
				if s.emailService != nil && s.emailService.notificationEmailService != nil {
					notifications := s.emailService.notificationEmailService
					tmpl, err := notifications.GetTemplate(ctx, NotificationEmailEventOpsAlert, locale)
					if err == nil {
						vars := notifications.runtimeVariables(ctx, NotificationEmailEventOpsAlert, locale, NotificationEmailSendInput{RecipientEmail: to, RecipientName: emailRecipientName(to), Variables: opsAlertEmailVariables(rule, event, locale)})
						if rendered, err := renderNotificationEmail(NotificationEmailEventOpsAlert, tmpl.Subject, tmpl.HTML, vars, nil); err == nil {
							snapshot.Subject = rendered.Subject
							snapshot.Body = rendered.HTML
						}
					}
				}
				payload, err := json.Marshal(snapshot)
				if err != nil {
					return nil, err
				}
				deliveries = append(deliveries, &OpsAlertDelivery{Channel: "email", Transition: transition, Target: to, Payload: payload})
			}
		}
	}
	for _, target := range opsAlertWebhookURLs(rule.Filters) {
		if err := ValidateOpsAlertWebhookURL(target); err != nil {
			return nil, err
		}
		payload, err := json.Marshal(opsAlertWebhookPayload{Version: 1, Transition: transition, RuleID: rule.ID, RuleName: rule.Name, Event: event})
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, &OpsAlertDelivery{Channel: "webhook", Transition: transition, Target: target, Payload: payload})
	}
	return deliveries, nil
}

// An event can open while notifications are silenced. Retry creation of its
// first firing intent on every valid, still-breaching sample after silence.
// The outbox unique key is the durable acknowledgement: failed/sent tasks are
// retained, so reevaluation or a new leader cannot enqueue them a second time.
func (s *OpsAlertEvaluatorService) ensureFiringNotification(ctx context.Context, cfg *OpsAlertRuntimeSettings, rule *OpsAlertRule, event *OpsAlertEvent, now time.Time) {
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok {
		return
	}
	deliveries, err := s.prepareAlertDeliveriesAt(ctx, cfg, rule, event, "firing", now)
	if err != nil || len(deliveries) == 0 {
		return
	}
	_ = repo.EnsureFiringAlertDeliveries(ctx, event.ID, deliveries)
}

func opsAlertWebhookURLs(filters map[string]any) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v != "" && !seen[v] {
			out = append(out, v)
			seen[v] = true
		}
	}
	switch urls := filters["webhook_urls"].(type) {
	case []string:
		for _, v := range urls {
			add(v)
		}
	case []any:
		for _, v := range urls {
			if str, ok := v.(string); ok {
				add(str)
			}
		}
	}
	return out
}

// Delivery is independent of rule evaluation and its leader lock. A durable
// lease protects each task, including when another Pod takes over after a crash.
func (s *OpsAlertEvaluatorService) runAlertDeliveryWorker() {
	defer s.wg.Done()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-timer.C:
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
			s.processAlertDeliveries(ctx)
			cancel()
			timer.Reset(5 * time.Second)
		}
	}
}

func (s *OpsAlertEvaluatorService) processAlertDeliveries(ctx context.Context) {
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok {
		return
	}
	deliveries, err := repo.ClaimAlertDeliveries(ctx, time.Now().UTC(), 4)
	if err != nil {
		return
	}
	if s.emailLimiter == nil {
		s.emailLimiter = newSlidingWindowLimiter(0, time.Hour)
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for _, d := range deliveries {
		wg.Add(1)
		go func(d *OpsAlertDelivery) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			err := s.deliverAlertNotification(sendCtx, d)
			// Persist even when the evaluator timeout expired after the remote response.
			ackCtx, ackCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer ackCancel()
			_ = repo.CompleteAlertDelivery(ackCtx, d, err, time.Now().UTC().Add(alertDeliveryBackoff(d.Attempts)))
		}(d)
	}
	wg.Wait()
}
func alertDeliveryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 7 {
		attempt = 7
	}
	return time.Duration(1<<uint(attempt-1)) * time.Minute
}
func (s *OpsAlertEvaluatorService) deliverAlertNotification(ctx context.Context, d *OpsAlertDelivery) error {
	if d.Channel == "webhook" {
		var payload opsAlertWebhookPayload
		if err := json.Unmarshal(d.Payload, &payload); err != nil {
			return fmt.Errorf("invalid webhook payload")
		}
		if payload.Event != nil {
			payload.Event.ID = d.EventID
			if stored, err := s.opsRepo.GetAlertEventByID(ctx, d.EventID); err == nil && stored != nil {
				if payload.Event.Dimensions == nil {
					payload.Event.Dimensions = map[string]any{}
				}
				for _, key := range []string{"incident_id", "incident_key"} {
					if v, ok := stored.Dimensions[key]; ok {
						payload.Event.Dimensions[key] = v
					}
				}
			}
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("invalid webhook payload")
		}
		return sendOpsAlertWebhook(ctx, d.Target, raw, strconv.FormatInt(d.ID, 10))
	}
	if d.Channel != "email" {
		return fmt.Errorf("unsupported notification channel")
	}
	if s.opsService != nil {
		cfg, err := s.opsService.GetEmailNotificationConfig(ctx)
		if err != nil {
			return fmt.Errorf("email settings unavailable")
		}
		if cfg != nil && cfg.Alert.RateLimitPerHour > 0 {
			s.emailLimiter.SetLimit(cfg.Alert.RateLimitPerHour)
			if !s.emailLimiter.Allow(time.Now().UTC()) {
				return fmt.Errorf("email rate limit exceeded")
			}
		}
	}
	var payload opsAlertEmailPayload
	if err := json.Unmarshal(d.Payload, &payload); err != nil {
		return fmt.Errorf("invalid email payload")
	}
	if s.deliverOpsAlertEmail != nil {
		if err := s.deliverOpsAlertEmail(ctx, d.Target, payload.Subject, payload.Body); err != nil {
			return redactedSMTPFailure(err)
		}
		return nil
	}
	if s.emailService == nil {
		return fmt.Errorf("email service unavailable")
	}
	if err := s.emailService.SendEmail(ctx, d.Target, payload.Subject, payload.Body); err != nil {
		return redactedSMTPFailure(err)
	}
	return nil
}

func redactedSMTPFailure(err error) error {
	var response *textproto.Error
	if errors.As(err, &response) {
		return fmt.Errorf("SMTP response %d", response.Code)
	}
	var network net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &network) && network.Timeout() {
		return fmt.Errorf("SMTP delivery timeout")
	}
	return fmt.Errorf("SMTP delivery failed")
}

// ValidateOpsAlertWebhookURL validates configuration without a network request.
// Actual connections resolve and pin a public IP in DialContext (DNS rebinding safe).
func ValidateOpsAlertWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || len(raw) > 2048 {
		return fmt.Errorf("webhook URL must be a public HTTPS URL without credentials or fragment")
	}
	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("webhook URL must use port 443")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || !strings.Contains(host, ".") {
		return fmt.Errorf("webhook host must be public")
	}
	if ip := net.ParseIP(host); ip != nil && !publicWebhookIP(ip) {
		return fmt.Errorf("webhook host must be public")
	}
	return nil
}
func publicWebhookIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	// Include metadata, carrier-grade NAT, documentation and reserved ranges.
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16"} {
		_, block, _ := net.ParseCIDR(raw)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
func opsWebhookHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("webhook redirects are disabled") }, Transport: &http.Transport{
		Proxy: nil, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 8 * time.Second,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid webhook address")
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("webhook DNS lookup failed")
			}
			for _, ip := range ips {
				if !publicWebhookIP(ip.IP) {
					return nil, fmt.Errorf("webhook DNS resolved to a forbidden address")
				}
			}
			dialer := net.Dialer{Timeout: 5 * time.Second}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}}
}
func opsWebhookSignature(secret, timestamp string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(timestamp + "."))
	_, _ = h.Write(payload)
	return "sha256=" + hex.EncodeToString(h.Sum(nil))
}
func sendOpsAlertWebhook(ctx context.Context, target string, payload []byte, idempotency string) error {
	client := opsWebhookHTTPClient()
	defer client.CloseIdleConnections()
	return sendOpsAlertWebhookWithClient(ctx, target, payload, idempotency, client)
}
func sendOpsAlertWebhookWithClient(ctx context.Context, target string, payload []byte, idempotency string, client *http.Client) error {
	if err := ValidateOpsAlertWebhookURL(target); err != nil {
		return err
	}
	secret := os.Getenv("OPS_ALERT_WEBHOOK_SECRET")
	if len(secret) < 32 {
		return fmt.Errorf("OPS_ALERT_WEBHOOK_SECRET must contain at least 32 bytes")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("invalid webhook request")
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ModuRelay-Timestamp", timestamp)
	req.Header.Set("X-ModuRelay-Signature", opsWebhookSignature(secret, timestamp, payload))
	req.Header.Set("Idempotency-Key", "ops-alert-"+idempotency)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook delivery failed")
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook HTTP %d", resp.StatusCode)
	}
	return nil
}

func (s *OpsService) ListAlertDeliveries(ctx context.Context, eventID int64) ([]*OpsAlertDelivery, error) {
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok {
		return nil, fmt.Errorf("alert outbox unavailable")
	}
	return repo.ListAlertDeliveries(ctx, eventID)
}
func (s *OpsService) RetryAlertDelivery(ctx context.Context, id int64) error {
	repo, ok := s.opsRepo.(OpsAlertDeliveryRepository)
	if !ok {
		return fmt.Errorf("alert outbox unavailable")
	}
	return repo.RetryAlertDelivery(ctx, id)
}
func (s *OpsService) TestAlertWebhook(ctx context.Context, target string) error {
	return sendOpsAlertWebhook(ctx, target, []byte(`{"version":1,"transition":"test","message":"ModuRelay 运维告警测试"}`), "test-"+strconv.FormatInt(time.Now().UnixNano(), 10))
}
