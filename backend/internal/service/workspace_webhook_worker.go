package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/google/uuid"
)

const (
	webhookWorkerPollInterval = 500 * time.Millisecond
	webhookWorkerLease        = 2 * time.Minute
	webhookWorkerConcurrency  = 8
	webhookRequestTimeout     = 10 * time.Second
	webhookMaxBodyBytes       = 64 << 10
	webhookPersistenceTimeout = 5 * time.Second
)

var (
	errWebhookResponseTooLarge  = errors.New("webhook response exceeded size limit")
	errWebhookDNSPinning        = errors.New("webhook DNS pinning failed")
	webhookPreviewSecretPattern = regexp.MustCompile(`(?i)\b(?:whsec_|sk-)[a-z0-9_-]+`)
	webhookPreviewBearerPattern = regexp.MustCompile(`(?i)\bbearer\s+[^\s,;"<>]+`)
	webhookPreviewFieldPattern  = regexp.MustCompile(`(?i)("(?:api[_-]?key|secret|token|access_token|refresh_token|authorization|cookie|password)"\s*:\s*)("(?:[^"\\]|\\.)*"|[^\s,}]+)`)
	webhookPreviewURLPattern    = regexp.MustCompile(`(?i)https?://[^\s"'<>]+`)
)

type webhookHTTPStatusError int

func (e webhookHTTPStatusError) Error() string {
	return fmt.Sprintf("webhook returned HTTP %d", int(e))
}

type WorkspaceWebhookWorker struct {
	repo      WorkspaceWebhookRepository
	encryptor SecretEncryptor
	client    *http.Client
	workerID  string
	ctx       context.Context
	cancel    context.CancelFunc
	start     sync.Once
	stop      sync.Once
	wg        sync.WaitGroup
}

func NewWorkspaceWebhookWorker(repo WorkspaceWebhookRepository, encryptor SecretEncryptor) *WorkspaceWebhookWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkspaceWebhookWorker{
		repo: repo, encryptor: encryptor, workerID: uuid.NewString(), ctx: ctx, cancel: cancel,
		client: newWorkspaceWebhookHTTPClient(),
	}
}

func newWorkspaceWebhookHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 8 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       45 * time.Second,
		TLSHandshakeTimeout:   8 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			return urlvalidator.DialContextWithPinnedIPs(ctx, network, address, dialer.DialContext)
		},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   webhookRequestTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (w *WorkspaceWebhookWorker) Start() {
	if w == nil || w.repo == nil || w.encryptor == nil {
		return
	}
	w.start.Do(func() {
		w.wg.Add(1)
		go w.run()
	})
}

func (w *WorkspaceWebhookWorker) Stop() {
	if w == nil {
		return
	}
	w.stop.Do(func() {
		w.cancel()
		w.wg.Wait()
		if w.client != nil {
			w.client.CloseIdleConnections()
		}
	})
}

func (w *WorkspaceWebhookWorker) run() {
	defer w.wg.Done()
	ticker := time.NewTicker(webhookWorkerPollInterval)
	defer ticker.Stop()
	for {
		if err := w.processBatch(w.ctx); err != nil && w.ctx.Err() == nil {
			slog.Warn("workspace_webhook_claim_failed", "error_code", "repository_claim_failed")
		}
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *WorkspaceWebhookWorker) processBatch(ctx context.Context) error {
	claims, err := w.repo.ClaimDeliveries(ctx, w.workerID, webhookWorkerConcurrency, webhookWorkerLease)
	if err != nil {
		return err
	}
	sem := make(chan struct{}, webhookWorkerConcurrency)
	var wg sync.WaitGroup
	for _, claim := range claims {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(item WebhookDeliveryClaim) {
			defer wg.Done()
			defer func() { <-sem }()
			w.processClaim(ctx, item)
		}(claim)
	}
	wg.Wait()
	return nil
}

func (w *WorkspaceWebhookWorker) processClaim(parent context.Context, claim WebhookDeliveryClaim) {
	secret, err := w.encryptor.Decrypt(claim.SecretCurrent)
	if err != nil || strings.TrimSpace(secret) == "" {
		w.markDead(claim, 0, "", "webhook secret decryption failed")
		return
	}
	previous := ""
	if claim.SecretPrevious != "" && claim.PreviousExpiresAt != nil && claim.PreviousExpiresAt.After(time.Now().UTC()) {
		previous, _ = w.encryptor.Decrypt(claim.SecretPrevious)
	}
	validated, err := ValidateWorkspaceWebhookURL(claim.URL)
	if err != nil {
		w.markDead(claim, 0, "", "webhook URL rejected")
		return
	}
	u, err := parseWebhookURL(validated)
	if err != nil {
		w.markDead(claim, 0, "", "webhook URL rejected")
		return
	}
	requestCtx, cancel := context.WithTimeout(parent, webhookRequestTimeout)
	defer cancel()
	pinnedCtx, err := urlvalidator.ResolveAndPinHost(requestCtx, u.Hostname())
	if err != nil {
		w.retryOrDead(claim, 0, "", err)
		return
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	pinnedAddresses, pinned, pinErr := urlvalidator.PinnedDialAddresses(pinnedCtx, net.JoinHostPort(u.Hostname(), port))
	if pinErr != nil || !pinned || len(pinnedAddresses) == 0 {
		w.retryOrDead(claim, 0, "", errWebhookDNSPinning)
		return
	}
	for _, address := range pinnedAddresses {
		host, _, splitErr := net.SplitHostPort(address)
		if splitErr != nil || !webhookIPAllowed(net.ParseIP(host)) {
			w.markDead(claim, 0, "", "webhook resolved address rejected")
			return
		}
	}
	active, err := w.repo.DeliveryEndpointActive(requestCtx, claim.ID, claim.LockOwner)
	if err != nil {
		w.retryOrDead(claim, 0, "", err)
		return
	}
	if !active {
		w.markRetry(claim, 0, "", "webhook endpoint disabled or claim no longer active")
		return
	}
	req, err := http.NewRequestWithContext(pinnedCtx, http.MethodPost, validated, bytes.NewReader(claim.Payload))
	if err != nil {
		w.markDead(claim, 0, "", "webhook request construction failed")
		return
	}
	now := time.Now().UTC()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ModuRelay-Webhook/1.0")
	req.Header.Set("X-ModuRelay-Event-Type", claim.EventType)
	req.Header.Set("X-ModuRelay-Event-ID", claim.EventID)
	req.Header.Set("X-ModuRelay-Delivery-ID", strconv.FormatInt(claim.ID, 10))
	req.Header.Set("X-ModuRelay-Timestamp", strconv.FormatInt(now.Unix(), 10))
	req.Header.Set("X-ModuRelay-Delivery-Attempt", strconv.Itoa(claim.Attempts))
	previousUntil := time.Time{}
	if claim.PreviousExpiresAt != nil {
		previousUntil = *claim.PreviousExpiresAt
	}
	signature := SignWebhookPayloadWithPrevious(secret, previous, previousUntil, now, claim.Payload)
	req.Header.Set("X-ModuRelay-Signature", strings.TrimPrefix(signature, "t="+strconv.FormatInt(now.Unix(), 10)+","))
	resp, err := w.client.Do(req)
	if err != nil {
		w.retryOrDead(claim, 0, "", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	previewBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, webhookMaxBodyBytes+1))
	previewBody := string(previewBytes)
	previewBody = strings.ReplaceAll(previewBody, secret, "[redacted]")
	if previous != "" {
		previewBody = strings.ReplaceAll(previewBody, previous, "[redacted]")
	}
	preview := sanitizeWebhookPreview([]byte(previewBody), WebhookResponsePreviewBytes)
	if readErr != nil {
		w.retryOrDead(claim, resp.StatusCode, preview, readErr)
		return
	}
	if len(previewBytes) > webhookMaxBodyBytes {
		w.retryOrDead(claim, resp.StatusCode, preview, errWebhookResponseTooLarge)
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		w.persistOutcome(claim, "succeeded", resp.StatusCode, "", func(ctx context.Context) error {
			return w.repo.MarkDeliverySuccess(ctx, claim.ID, claim.LockOwner, resp.StatusCode, preview)
		})
		return
	}
	if ShouldRetryWebhookResponse(resp.StatusCode, nil) {
		w.retryOrDead(claim, resp.StatusCode, preview, webhookHTTPStatusError(resp.StatusCode))
		return
	}
	w.markDead(claim, resp.StatusCode, preview, webhookHTTPStatusError(resp.StatusCode).Error())
}

func (w *WorkspaceWebhookWorker) retryOrDead(claim WebhookDeliveryClaim, status int, preview string, err error) {
	if claim.Attempts >= WebhookMaxAttempts {
		w.markDead(claim, status, preview, boundedWebhookError(err))
		return
	}
	w.markRetry(claim, status, preview, boundedWebhookError(err))
}

func (w *WorkspaceWebhookWorker) markDead(claim WebhookDeliveryClaim, status int, preview, reason string) {
	w.persistOutcome(claim, "dead", status, "delivery_terminal", func(ctx context.Context) error {
		return w.repo.MarkDeliveryDead(ctx, claim.ID, claim.LockOwner, status, preview, reason)
	})
}

func (w *WorkspaceWebhookWorker) markRetry(claim WebhookDeliveryClaim, status int, preview, reason string) {
	next := time.Now().UTC().Add(webhookRetryDelay(claim.Attempts))
	w.persistOutcome(claim, "retrying", status, "delivery_retryable", func(ctx context.Context) error {
		return w.repo.MarkDeliveryRetry(ctx, claim.ID, claim.LockOwner, next, reason, status, preview)
	})
}

func (w *WorkspaceWebhookWorker) persistOutcome(claim WebhookDeliveryClaim, state string, status int, errorCode string, update func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), webhookPersistenceTimeout)
	defer cancel()
	attrs := []any{"event_id", claim.EventID, "delivery_id", claim.ID, "webhook_id", claim.WebhookID, "workspace_id", claim.WorkspaceID, "attempt", claim.Attempts, "state", state, "response_status", status, "error_code", errorCode}
	if err := update(ctx); err != nil {
		code := "repository_update_failed"
		if errors.Is(err, ErrWebhookDeliveryLeaseLost) {
			code = "delivery_lease_lost"
		}
		attrs[len(attrs)-1] = code
		slog.Warn("workspace_webhook_delivery_state_failed", attrs...)
		return
	}
	slog.Info("workspace_webhook_delivery", attrs...)
}

func webhookRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delays := [...]time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour}
	index := attempt - 1
	if index >= len(delays) {
		index = len(delays) - 1
	}
	base := delays[index]
	// Spread retries from concurrent workers so a shared outage does not
	// synchronize every endpoint on the same retry instant.
	jitterMax := base / 4
	if jitterMax <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(int64(jitterMax)+1))
}

func boundedWebhookError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "webhook request timed out"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "webhook request timed out"
	}
	var statusErr webhookHTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.Error()
	}
	if errors.Is(err, errWebhookResponseTooLarge) || errors.Is(err, errWebhookDNSPinning) {
		return err.Error()
	}
	// net/http errors include the raw target URL. Persist only a stable safe
	// classification, never a URL, credential, or provider response body.
	return "webhook network or response read failed"
}

func sanitizeWebhookPreview(body []byte, max int) string {
	value := strings.TrimSpace(string(body))
	value = strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return -1
		}
		return r
	}, value)
	value = webhookPreviewFieldPattern.ReplaceAllString(value, `${1}"[redacted]"`)
	value = webhookPreviewSecretPattern.ReplaceAllString(value, "[redacted]")
	value = webhookPreviewBearerPattern.ReplaceAllString(value, "Bearer [redacted]")
	value = webhookPreviewURLPattern.ReplaceAllString(value, "[url]")
	if max > 0 && len(value) > max {
		suffix := "...(truncated)"
		if max <= len(suffix) {
			return suffix[:max]
		}
		value = value[:max-len(suffix)]
		for !utf8.ValidString(value) {
			value = value[:len(value)-1]
		}
		return value + suffix
	}
	return value
}

func parseWebhookURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}
