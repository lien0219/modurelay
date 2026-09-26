package service

// Email verification service. Emailnator/Gmailnator is intentionally an
// adapter behind a public channel boundary. Provider credentials and IDs never
// cross that boundary.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	mathrand "math/rand"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	xhtml "golang.org/x/net/html"
)

var (
	ErrEmailFeatureDisabled           = errors.New("email service is disabled")
	ErrEmailInsufficientBalance       = errors.New("insufficient balance")
	ErrEmailChannelUnavailable        = errors.New("email channel is unavailable")
	ErrEmailPriceChanged              = errors.New("email quote changed")
	ErrEmailQuoteExpired              = errors.New("email quote expired")
	ErrEmailQuoteInvalid              = errors.New("email quote invalid")
	ErrEmailProviderUnknown           = errors.New("email provider result is unknown")
	ErrEmailNotFound                  = errors.New("email order not found")
	ErrEmailProviderCredentialMissing = errors.New("email provider credential is not configured")
	ErrEmailProviderTestCooldown      = errors.New("email provider test connection is cooling down")
	ErrEmailFreeDailyLimit            = errors.New("free email daily limit reached")
	ErrEmailFreeActiveLimit           = errors.New("too many active free inboxes")
	ErrEmailFreeGenerationCooldown    = errors.New("please wait before creating another free inbox")
)

const (
	EmailRefundIfNoMessage       = "refund_if_no_message"
	EmailNoRefundAfterDelivery   = "no_refund_after_inbox_delivery"
	EmailManualReview            = "manual_review"
	EmailCaptureOnTargetReceived = "on_target_email_received"
	EmailCaptureOnExtracted      = "on_verification_extracted"
)

func enforceEmailRefundSafety(policy string, salePrice float64) string {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		policy = EmailRefundIfNoMessage
	}
	if salePrice > 0 && policy == EmailRefundIfNoMessage {
		return EmailNoRefundAfterDelivery
	}
	return policy
}

type EmailProviderCapabilities struct {
	TemporaryInbox   bool `json:"supports_temporary_inbox"`
	GenerateSingle   bool `json:"supports_generate_single"`
	GenerateBulk     bool `json:"supports_generate_bulk"`
	InboxList        bool `json:"supports_inbox_list"`
	MessageRead      bool `json:"supports_message_read"`
	MessageDelete    bool `json:"supports_message_delete"`
	Polling          bool `json:"supports_polling"`
	VerificationCode bool `json:"supports_verification_code"`
	VerificationURL  bool `json:"supports_verification_url"`
	HTMLMessage      bool `json:"supports_html_message"`
	Webhook          bool `json:"supports_webhook"`
	SSE              bool `json:"supports_sse"`
	IMAP             bool `json:"supports_imap"`
	SMTP             bool `json:"supports_smtp"`
	CustomDomain     bool `json:"supports_custom_domain"`
	PrivateInbox     bool `json:"supports_private_inbox"`
	PersistentInbox  bool `json:"supports_persistent_inbox"`
	ProviderRefund   bool `json:"supports_provider_refund"`
}

type GenerateInboxRequest struct{ AddressType string }
type GeneratedInbox struct {
	ProviderInboxID string
	EmailAddress    string
	AddressType     string
	ExpiresAt       time.Time
}
type ListMessagesRequest struct{ ProviderInboxID, EmailAddress string }
type ProviderEmailSummary struct {
	ProviderMessageID string
	FromAddress       string
	FromName          string
	ToAddress         string
	Subject           string
	ReceivedAt        time.Time
}
type MessageListResult struct{ Messages []ProviderEmailSummary }
type GetMessageRequest struct{ ProviderInboxID, ProviderMessageID, EmailAddress string }
type ProviderEmailMessage struct {
	ProviderMessageID string
	FromAddress       string
	FromName          string
	ToAddress         string
	Subject           string
	TextBody          string
	HTMLBody          string
	ReceivedAt        time.Time
	RawPayload        json.RawMessage
}
type DeleteMessageRequest struct{ ProviderInboxID, ProviderMessageID, EmailAddress string }

type EmailProvider interface {
	Code() string
	Capabilities(context.Context) EmailProviderCapabilities
	Health(context.Context) error
	GenerateInbox(context.Context, GenerateInboxRequest) (*GeneratedInbox, error)
	ListMessages(context.Context, ListMessagesRequest) (*MessageListResult, error)
	GetMessage(context.Context, GetMessageRequest) (*ProviderEmailMessage, error)
	DeleteMessage(context.Context, DeleteMessageRequest) error
}

type EmailProviderHTTPError struct {
	StatusCode int
	RetryAfter time.Duration
}

func (e *EmailProviderHTTPError) Error() string {
	return fmt.Sprintf("email provider returned HTTP %d", e.StatusCode)
}

type emailnatorProvider struct {
	code, baseURL, rapidAPIKey, rapidAPIHost string
	client                                   *http.Client
	cap                                      EmailProviderCapabilities
	endpoints                                map[string]string
	limiter                                  *emailProviderLimiter
}

// Limiter instances are shared by provider configuration, rather than being
// recreated for every order/poll. This enforces a provider-level request rate
// across all callers in a process.
var emailProviderLimiters sync.Map // map[string]*emailProviderLimiter
var emailProviderTestMu sync.Mutex
var emailProviderLastTests = map[int64]time.Time{}

func emailnatorCapabilities() EmailProviderCapabilities {
	return EmailProviderCapabilities{TemporaryInbox: true, GenerateSingle: true, GenerateBulk: true, InboxList: true, MessageRead: true, MessageDelete: true, Polling: true, VerificationCode: true, VerificationURL: true, HTMLMessage: true}
}
func (p *emailnatorProvider) Code() string                                           { return p.code }
func (p *emailnatorProvider) Capabilities(context.Context) EmailProviderCapabilities { return p.cap }
func (p *emailnatorProvider) Health(ctx context.Context) error {
	if healthPath := strings.TrimSpace(p.endpoints["health"]); healthPath != "" {
		_, err := p.request(ctx, "health_check", http.MethodGet, healthPath, nil, nil)
		return err
	}
	// Gmailnator does not expose a stable no-op health endpoint. Generating a
	// disposable address is the smallest authenticated contract check and keeps
	// the admin test aligned with the API that production orders actually use.
	inbox, err := p.GenerateInbox(ctx, GenerateInboxRequest{AddressType: "gmail"})
	if err != nil {
		return err
	}
	if inbox == nil || strings.TrimSpace(inbox.EmailAddress) == "" {
		return errors.New("email provider returned no address")
	}
	return nil
}
func (p *emailnatorProvider) endpoint(name, fallback string) string {
	if v := strings.TrimSpace(p.endpoints[name]); v != "" {
		return v
	}
	return fallback
}
func (p *emailnatorProvider) request(ctx context.Context, operation, method, path string, body any, out any) ([]byte, error) {
	if strings.TrimSpace(p.rapidAPIKey) == "" {
		return nil, errors.New("email provider credential is not configured")
	}
	if p.limiter == nil {
		p.limiter = newEmailProviderLimiter(1, time.Second)
	}
	if err := p.limiter.acquire(ctx); err != nil {
		return nil, err
	}
	defer p.limiter.release()
	target := strings.TrimRight(p.baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-RapidAPI-Key", p.rapidAPIKey)
	host := p.rapidAPIHost
	if host == "" {
		if u, e := url.Parse(p.baseURL); e == nil {
			host = u.Host
		}
	}
	if host != "" {
		req.Header.Set("X-RapidAPI-Host", host)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("email provider request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		e := &EmailProviderHTTPError{StatusCode: resp.StatusCode}
		if v := strings.TrimSpace(resp.Header.Get("Retry-After")); v != "" {
			e.RetryAfter = parseRetryAfter(v, time.Now())
		}
		return nil, e
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, errors.New("email provider returned malformed response")
		}
	}
	return data, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(strings.TrimSpace(value)); err == nil {
		if d := at.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}
func (p *emailnatorProvider) GenerateInbox(ctx context.Context, req GenerateInboxRequest) (*GeneratedInbox, error) {
	type response struct {
		Email   []string `json:"email"`
		Address string   `json:"address"`
		ID      string   `json:"id"`
		InboxID string   `json:"inbox_id"`
		Data    struct {
			Email   []string `json:"email"`
			Address string   `json:"address"`
			ID      string   `json:"id"`
			InboxID string   `json:"inbox_id"`
		} `json:"data"`
	}
	var out response
	addressType := strings.ToLower(strings.TrimSpace(req.AddressType))
	if addressType == "" {
		addressType = "gmail"
	}
	_, err := p.request(ctx, "generate_inbox", http.MethodPost, p.endpoint("generate", "/generate-email"), map[string]any{
		// options is Gmailnator's production contract. The descriptive fields
		// keep compatibility with API gateways that expose the named variants.
		"options":      []int{1, 2, 3},
		"email":        []string{"plusGmail", "dotGmail"},
		"address_type": addressType,
	}, &out)
	if err != nil {
		return nil, err
	}
	address := out.Address
	if len(out.Email) > 0 {
		address = out.Email[0]
	}
	if address == "" {
		address = out.Data.Address
	}
	if address == "" && len(out.Data.Email) > 0 {
		address = out.Data.Email[0]
	}
	if strings.TrimSpace(address) == "" {
		return nil, errors.New("email provider returned no address")
	}
	inboxID := firstNonEmpty(out.InboxID, out.ID, out.Data.InboxID, out.Data.ID)
	if inboxID == "" {
		// Gmailnator's public API identifies an inbox by its address; retaining
		// the address as the internal reference keeps polling deterministic.
		inboxID = address
	}
	return &GeneratedInbox{ProviderInboxID: inboxID, EmailAddress: strings.TrimSpace(address), AddressType: addressType, ExpiresAt: time.Now().Add(15 * time.Minute)}, nil
}

func (p *emailnatorProvider) ListMessages(ctx context.Context, req ListMessagesRequest) (*MessageListResult, error) {
	var raw json.RawMessage
	data, err := p.request(ctx, "list_messages", http.MethodPost, p.endpoint("list", "/message-list"), map[string]string{"email": req.EmailAddress, "inbox_id": req.ProviderInboxID}, &raw)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		raw = data
	}
	return parseEmailMessageList(raw)
}
func parseEmailMessageList(raw []byte) (*MessageListResult, error) {
	var envelope struct {
		MessageData []json.RawMessage `json:"messageData"`
		Messages    []json.RawMessage `json:"messages"`
		Data        []json.RawMessage `json:"data"`
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return &MessageListResult{}, nil
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, errors.New("email provider returned malformed message list")
	}
	items := envelope.MessageData
	if len(items) == 0 {
		items = envelope.Messages
	}
	if len(items) == 0 {
		items = envelope.Data
	}
	if len(items) == 0 {
		var array []json.RawMessage
		if json.Unmarshal(raw, &array) == nil {
			items = array
		}
	}
	out := &MessageListResult{Messages: make([]ProviderEmailSummary, 0, len(items))}
	for _, item := range items {
		var m map[string]any
		if json.Unmarshal(item, &m) != nil {
			continue
		}
		id := firstString(m, "id", "message_id", "messageId", "messageID", "messageid")
		subject := firstString(m, "subject", "Subject")
		from := firstString(m, "from", "from_address", "sender", "Email")
		to := firstString(m, "to", "to_address")
		name := firstString(m, "from_name", "name")
		received := parseProviderTime(firstString(m, "received_at", "receivedAt", "time", "date"))
		if id == "" {
			id = hashString(string(item))
		}
		out.Messages = append(out.Messages, ProviderEmailSummary{ProviderMessageID: id, FromAddress: from, FromName: name, ToAddress: to, Subject: subject, ReceivedAt: received})
	}
	return out, nil
}
func (p *emailnatorProvider) GetMessage(ctx context.Context, req GetMessageRequest) (*ProviderEmailMessage, error) {
	var out map[string]any
	body := map[string]string{"email": req.EmailAddress, "inbox_id": req.ProviderInboxID, "message_id": req.ProviderMessageID, "messageid": req.ProviderMessageID, "id": req.ProviderMessageID}
	raw, err := p.request(ctx, "get_message", http.MethodPost, p.endpoint("message", "/message"), body, &out)
	if err != nil {
		return nil, err
	}
	if out == nil {
		_ = json.Unmarshal(raw, &out)
	}
	if data, ok := out["data"].(map[string]any); ok {
		out = data
	}
	if out == nil {
		return nil, errors.New("email provider returned empty message")
	}
	htmlBody := firstString(out, "html", "html_body", "content_html", "content")
	textBody := firstString(out, "text", "text_body", "body")
	if textBody == "" && htmlBody != "" {
		textBody = HTMLToText(htmlBody)
	}
	return &ProviderEmailMessage{ProviderMessageID: firstString(out, "id", "message_id", "messageId", "messageID", "messageid"), FromAddress: firstString(out, "from", "from_address", "sender", "fromEmail"), FromName: firstString(out, "from_name", "name"), ToAddress: firstString(out, "to", "to_address"), Subject: firstString(out, "subject", "Subject"), TextBody: textBody, HTMLBody: htmlBody, ReceivedAt: parseProviderTime(firstString(out, "received_at", "receivedAt", "time", "date")), RawPayload: json.RawMessage(raw)}, nil
}
func (p *emailnatorProvider) DeleteMessage(ctx context.Context, req DeleteMessageRequest) error {
	_, err := p.request(ctx, "delete_message", http.MethodDelete, p.endpoint("delete", "/message"), map[string]string{"email": req.EmailAddress, "inbox_id": req.ProviderInboxID, "message_id": req.ProviderMessageID}, nil)
	return err
}

func firstString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}
func parseProviderTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123, "2006-01-02 15:04:05", "2006-01-02T15:04:05.000Z07:00"} {
		if t, e := time.Parse(layout, v); e == nil {
			return t
		}
	}
	if parsed, err := mail.ParseDate(v); err == nil {
		return parsed
	}
	return time.Time{}
}
func hashString(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }

func emailProviderAPIKey(code, credentialRef string, encryptor SecretEncryptor) string {
	name := "EMAIL_" + strings.ToUpper(strings.ReplaceAll(code, "-", "_")) + "_API_KEY"
	ref := strings.TrimSpace(credentialRef)
	if strings.HasPrefix(ref, "env:") {
		if value := strings.TrimSpace(os.Getenv(strings.TrimPrefix(ref, "env:"))); value != "" {
			return value
		}
	}
	if strings.HasPrefix(ref, "enc:") && encryptor != nil {
		if value, err := encryptor.Decrypt(strings.TrimPrefix(ref, "enc:")); err == nil {
			if value = strings.TrimSpace(value); value != "" {
				return value
			}
		}
	}
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return ""
}
func emailProviderRequiresCredential(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "temp_tf":
		return false
	default:
		return true
	}
}

func emailProviderSupportsAddressType(code, addressType string) bool {
	code = strings.ToLower(strings.TrimSpace(code))
	addressType = strings.ToLower(strings.TrimSpace(addressType))
	switch code {
	case "temp_tf":
		return addressType == "gmail" || addressType == "outlook" || addressType == "hotmail"
	case "sonjj":
		return addressType == "gmail_real" || addressType == "gmail_alias" || addressType == "outlook_real" || addressType == "outlook_alias"
	case "emailnator":
		return addressType == "gmail"
	default:
		return false
	}
}

func emailProviderFor(code, baseURL, credentialRef string, metadata map[string]any, encryptor SecretEncryptor, billingValues ...map[string]any) EmailProvider {
	code = strings.ToLower(strings.TrimSpace(code))
	key := emailProviderAPIKey(code, credentialRef, encryptor)
	concurrency := 2
	interval := time.Second
	if len(billingValues) > 0 && billingValues[0] != nil {
		billing := billingValues[0]
		if rate := numberFromMap(billing, "rate_limit_per_minute"); rate > 0 {
			interval = time.Minute / time.Duration(math.Max(1, rate))
		}
		if rate := numberFromMap(billing, "rate_limit_per_hour"); rate > 0 {
			hourlyInterval := time.Hour / time.Duration(math.Max(1, rate))
			if hourlyInterval > interval {
				interval = hourlyInterval
			}
		}
		if value := numberFromMap(billing, "provider_concurrency"); value > 0 {
			concurrency = int(math.Min(32, math.Max(1, value)))
		}
	}
	limiterKey := code + "|" + strings.TrimRight(baseURL, "/") + "|" + interval.String() + "|" + strconv.Itoa(concurrency)
	limiterValue, _ := emailProviderLimiters.LoadOrStore(limiterKey, newEmailProviderLimiter(concurrency, interval))
	limiter, ok := limiterValue.(*emailProviderLimiter)
	if !ok {
		return nil
	}

	switch code {
	case "temp_tf":
		if baseURL == "" {
			baseURL = "https://temp.tf/api"
		}
		return &tempTFProvider{code: code, baseURL: baseURL, client: &http.Client{Timeout: 15 * time.Second}, limiter: limiter}
	case "sonjj":
		if baseURL == "" {
			baseURL = "https://app.sonjj.com"
		}
		return &sonjjProvider{code: code, baseURL: baseURL, apiKey: key, client: &http.Client{Timeout: 20 * time.Second}, limiter: limiter}
	case "emailnator":
		host := ""
		if v, ok := metadata["rapidapi_host"].(string); ok {
			host = strings.TrimSpace(v)
		}
		endpoints := map[string]string{}
		if raw, ok := metadata["endpoints"].(map[string]any); ok {
			for k, v := range raw {
				if s, ok := v.(string); ok {
					endpoints[k] = s
				}
			}
		}
		if baseURL == "" {
			baseURL = "https://gmailnator.p.rapidapi.com"
		}
		return &emailnatorProvider{code: code, baseURL: baseURL, rapidAPIKey: key, rapidAPIHost: host, client: &http.Client{Timeout: 20 * time.Second}, cap: emailnatorCapabilities(), endpoints: endpoints, limiter: limiter}
	default:
		return nil
	}
}

type emailProviderLimiter struct {
	sem      chan struct{}
	mu       sync.Mutex
	next     time.Time
	interval time.Duration
}

func newEmailProviderLimiter(concurrency int, interval time.Duration) *emailProviderLimiter {
	if concurrency < 1 {
		concurrency = 1
	}
	return &emailProviderLimiter{sem: make(chan struct{}, concurrency), interval: interval}
}
func (l *emailProviderLimiter) acquire(ctx context.Context) error {
	select {
	case l.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	for {
		now := time.Now()
		l.mu.Lock()
		wait := time.Until(l.next)
		if wait <= 0 {
			l.next = now.Add(l.interval)
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		t := time.NewTimer(wait)
		select {
		case <-t.C:
			// Re-check under the mutex: another waiter may have consumed the
			// slot while this goroutine was asleep.
		case <-ctx.Done():
			t.Stop()
			l.release()
			return ctx.Err()
		}
	}
}
func (l *emailProviderLimiter) release() { <-l.sem }

type EmailPublicService struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type EmailPublicChannel struct {
	Code                     string                    `json:"channel_code"`
	PublicName               string                    `json:"public_name"`
	EmailType                string                    `json:"email_type"`
	PrivacyLevel             string                    `json:"privacy_level"`
	SalePrice                float64                   `json:"sale_price"`
	SuccessRate              *float64                  `json:"success_rate,omitempty"`
	SuccessRateGrade         string                    `json:"success_rate_grade,omitempty"`
	SuccessRateSampleCount   int                       `json:"success_rate_sample_count"`
	EstimatedDeliverySeconds int                       `json:"estimated_delivery_seconds"`
	RetentionDescription     string                    `json:"retention_description"`
	RefundPolicyDescription  string                    `json:"refund_policy_description"`
	QuoteID                  string                    `json:"quote_id"`
	QuoteExpiresAt           time.Time                 `json:"quote_expires_at"`
	Capabilities             EmailProviderCapabilities `json:"capabilities"`
	// These fields are used to keep the purchase snapshot identical to the
	// quote. They are deliberately excluded from the user response.
	ProviderCostEstimate float64        `json:"-"`
	PricingRuleSnapshot  map[string]any `json:"-"`
}
type EmailOrder struct {
	ID                     string         `json:"id"`
	OrderNo                string         `json:"order_no"`
	ServiceCode            string         `json:"service_code"`
	ChannelCode            string         `json:"channel_code"`
	ChannelName            string         `json:"channel_name"`
	EmailAddress           string         `json:"email_address"`
	AddressType            string         `json:"address_type"`
	Price                  float64        `json:"price"`
	Status                 string         `json:"status"`
	SuccessRate            *float64       `json:"success_rate,omitempty"`
	SuccessRateGrade       string         `json:"success_rate_grade,omitempty"`
	RefundPolicy           string         `json:"refund_policy"`
	CapturePolicy          string         `json:"capture_policy"`
	ExpiresAt              *time.Time     `json:"expires_at,omitempty"`
	CreatedAt              time.Time      `json:"created_at"`
	FirstMessageAt         *time.Time     `json:"first_message_at,omitempty"`
	Messages               []EmailMessage `json:"messages,omitempty"`
	LatestVerificationCode string         `json:"latest_verification_code,omitempty"`
	RefundStatus           string         `json:"refund_status"`
	RefundReason           string         `json:"refund_reason,omitempty"`
	ErrorPublicMessage     string         `json:"error_message,omitempty"`
}
type EmailOrderPage struct {
	Items    []EmailOrder `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Pages    int          `json:"pages"`
}
type EmailMessage struct {
	ID                     string    `json:"id"`
	FromAddress            string    `json:"from_address"`
	FromName               string    `json:"from_name"`
	ToAddress              string    `json:"to_address"`
	Subject                string    `json:"subject"`
	TextBody               string    `json:"text_body"`
	HTMLBody               string    `json:"html_body,omitempty"`
	VerificationCode       string    `json:"verification_code,omitempty"`
	VerificationURL        string    `json:"verification_url,omitempty"`
	VerificationConfidence float64   `json:"verification_confidence,omitempty"`
	VerificationMethod     string    `json:"verification_method,omitempty"`
	ReceivedAt             time.Time `json:"received_at"`
}

type EmailVerificationService struct {
	db              *sql.DB
	settings        *SettingService
	encryptor       SecretEncryptor
	quoteSigningKey []byte
}

func NewEmailVerificationService(db *sql.DB, settings *SettingService, encryptor SecretEncryptor, cfg *config.Config) *EmailVerificationService {
	return &EmailVerificationService{
		db:              db,
		settings:        settings,
		encryptor:       encryptor,
		quoteSigningKey: resolveEmailQuoteSigningKey(cfg),
	}
}

// recordOrderEvent is deliberately best-effort. Order state changes must not
// fail just because an audit/event row could not be written, but every normal
// transition records an event when the migration is present.
func (s *EmailVerificationService) recordOrderEvent(ctx context.Context, orderID int64, eventType, idempotencyKey string, payload map[string]any) {
	if s == nil || s.db == nil || orderID <= 0 || strings.TrimSpace(eventType) == "" {
		return
	}
	data := []byte(`{}`)
	if payload != nil {
		if encoded, err := json.Marshal(payload); err == nil {
			data = encoded
		}
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO email_order_events(email_order_id,event_type,actor,idempotency_key,payload) VALUES($1,$2,'system',$3,$4) ON CONFLICT DO NOTHING`, orderID, eventType, strings.TrimSpace(idempotencyKey), data)
}

func (s *EmailVerificationService) Enabled(ctx context.Context) bool {
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return false
	}
	v, e := s.settings.settingRepo.GetValue(ctx, SettingKeyEmailServiceEnabled)
	return e == nil && strings.EqualFold(strings.TrimSpace(v), "true")
}

type EmailAdminSettings struct {
	Enabled                   bool `json:"enabled"`
	FreeDailyLimit            int  `json:"free_daily_limit"`
	FreeActiveLimit           int  `json:"free_active_limit"`
	FreeGenerationIntervalSec int  `json:"free_generation_interval_seconds"`
}

func (s *EmailVerificationService) AdminSettings(ctx context.Context) EmailAdminSettings {
	return EmailAdminSettings{
		Enabled:                   s.Enabled(ctx),
		FreeDailyLimit:            s.emailSettingInt(ctx, "email_free_daily_limit", 20),
		FreeActiveLimit:           s.emailSettingInt(ctx, "email_free_active_limit", 3),
		FreeGenerationIntervalSec: s.emailSettingInt(ctx, "email_free_generation_interval_seconds", 5),
	}
}

func (s *EmailVerificationService) UpdateAdminSettings(ctx context.Context, values EmailAdminSettings) error {
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return errors.New("settings repository unavailable")
	}
	if values.FreeDailyLimit < 0 || values.FreeDailyLimit > 100000 {
		return errors.New("invalid free email daily limit")
	}
	if values.FreeActiveLimit < 0 || values.FreeActiveLimit > 1000 {
		return errors.New("invalid free email active limit")
	}
	if values.FreeGenerationIntervalSec < 0 || values.FreeGenerationIntervalSec > 3600 {
		return errors.New("invalid free email generation interval")
	}
	return s.settings.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyEmailServiceEnabled:            strconv.FormatBool(values.Enabled),
		"email_free_daily_limit":                 strconv.Itoa(values.FreeDailyLimit),
		"email_free_active_limit":                strconv.Itoa(values.FreeActiveLimit),
		"email_free_generation_interval_seconds": strconv.Itoa(values.FreeGenerationIntervalSec),
	})
}

func (s *EmailVerificationService) SetEnabled(ctx context.Context, enabled bool) error {
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return errors.New("settings repository unavailable")
	}
	return s.settings.settingRepo.Set(ctx, SettingKeyEmailServiceEnabled, strconv.FormatBool(enabled))
}

func (s *EmailVerificationService) ListServices(ctx context.Context) ([]EmailPublicService, error) {
	if !s.Enabled(ctx) {
		return nil, ErrEmailFeatureDisabled
	}
	rows, e := s.db.QueryContext(ctx, `SELECT code,name FROM email_services WHERE enabled ORDER BY sort_order,code`)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []EmailPublicService{}
	for rows.Next() {
		var v EmailPublicService
		if e = rows.Scan(&v.Code, &v.Name); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Email quote identifiers carry a short-lived, signed expiry.  Keeping the
// expiry in the token avoids a process-local cache and lets every API instance
// enforce the same TTL. The effective JWT secret may be generated and loaded
// from the database during startup, so use the initialized config rather than
// assuming JWT_SECRET is present in the process environment.
func resolveEmailQuoteSigningKey(cfg *config.Config) []byte {
	key := strings.TrimSpace(os.Getenv("EMAIL_QUOTE_SIGNING_KEY"))
	if key == "" && cfg != nil {
		key = strings.TrimSpace(cfg.JWT.Secret)
	}
	if key == "" {
		key = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	return []byte(key)
}

const defaultEmailServiceCode = "other"

func normalizeEmailServiceCode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return defaultEmailServiceCode
	}
	return normalized
}

func makeEmailQuoteID(signingKey []byte, serviceCode, channelCode, addressType string, expires time.Time) string {
	nonce := randomID()
	material := strings.Join([]string{strings.ToLower(serviceCode), channelCode, strings.ToLower(addressType), strconv.FormatInt(expires.Unix(), 10), nonce}, "|")
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte(material))
	return fmt.Sprintf("%d.%s.%x", expires.Unix(), nonce, mac.Sum(nil))
}

func validateEmailQuoteID(signingKey []byte, id, serviceCode, channelCode, addressType string, now time.Time) error {
	parts := strings.Split(strings.TrimSpace(id), ".")
	if len(parts) != 3 {
		return ErrEmailQuoteInvalid
	}
	expiresUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || expiresUnix <= 0 {
		return ErrEmailQuoteInvalid
	}
	if now.Unix() >= expiresUnix {
		return ErrEmailQuoteExpired
	}
	if len(signingKey) == 0 {
		return ErrEmailQuoteInvalid
	}
	material := strings.Join([]string{strings.ToLower(serviceCode), channelCode, strings.ToLower(addressType), parts[0], parts[1]}, "|")
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write([]byte(material))
	expected := fmt.Sprintf("%x", mac.Sum(nil))
	if !hmac.Equal([]byte(strings.ToLower(parts[2])), []byte(expected)) {
		return ErrEmailQuoteInvalid
	}
	return nil
}

func (s *EmailVerificationService) Quote(ctx context.Context, serviceCode, addressType string) ([]EmailPublicChannel, error) {
	if !s.Enabled(ctx) {
		return nil, ErrEmailFeatureDisabled
	}
	if len(s.quoteSigningKey) == 0 {
		return nil, ErrEmailQuoteInvalid
	}
	serviceCode = normalizeEmailServiceCode(serviceCode)
	var normalizeErr error
	addressType, normalizeErr = normalizeEmailAddressType(addressType)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	rows, e := s.db.QueryContext(ctx, `SELECT c.code,c.public_name,c.email_type,c.privacy_level,c.sale_price,c.order_ttl_seconds,c.refund_policy,p.id,p.code,p.base_url,p.credential_ref,p.capabilities,p.metadata,p.billing,c.metadata FROM email_channels c JOIN email_providers p ON p.id=c.provider_id JOIN email_services sv ON sv.code=$1 AND sv.enabled WHERE c.enabled AND c.visible AND c.healthy AND p.enabled`, strings.ToLower(strings.TrimSpace(serviceCode)))
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []EmailPublicChannel{}
	for rows.Next() {
		var code, name, emailType, privacy, refund, pc, base, cred string
		var price float64
		var providerID int64
		var ttl int
		var capsRaw, providerMetaRaw, billingRaw, channelMetaRaw []byte
		if e = rows.Scan(&code, &name, &emailType, &privacy, &price, &ttl, &refund, &providerID, &pc, &base, &cred, &capsRaw, &providerMetaRaw, &billingRaw, &channelMetaRaw); e != nil {
			return nil, e
		}
		var caps EmailProviderCapabilities
		var providerMeta, billing, channelMeta map[string]any
		_ = json.Unmarshal(capsRaw, &caps)
		_ = json.Unmarshal(providerMetaRaw, &providerMeta)
		_ = json.Unmarshal(billingRaw, &billing)
		_ = json.Unmarshal(channelMetaRaw, &channelMeta)
		if !emailProviderSupportsAddressType(pc, addressType) {
			continue
		}
		p := emailProviderFor(pc, base, cred, providerMeta, s.encryptor, billing)
		if p == nil || (emailProviderRequiresCredential(pc) && strings.TrimSpace(emailProviderAPIKey(pc, cred, s.encryptor)) == "") {
			continue
		}
		if !s.emailQuotaAllowsNewOrder(ctx, providerID) {
			continue
		}
		rate, grade, sample := s.successGrade(ctx, code, serviceCode, addressType)
		gradeMultiplier, gradeFixedMarkup := s.emailGradePricing(ctx, grade)
		providerCost := estimateEmailOrderCost(billing)
		finalPrice, pricingSnapshot := emailSalePrice(price, providerCost, billing, channelMeta, grade, gradeMultiplier, gradeFixedMarkup)
		if forceFree, ok := channelMeta["force_free"].(bool); ok && forceFree {
			finalPrice = 0
			pricingSnapshot["sale_price"] = 0.0
			pricingSnapshot["force_free"] = true
		}
		expiresAt := time.Now().Add(30 * time.Second)
		quote := EmailPublicChannel{Code: code, PublicName: publicVerificationChannelName(code), EmailType: emailType, PrivacyLevel: privacy, SalePrice: finalPrice, SuccessRate: rate, SuccessRateGrade: grade, SuccessRateSampleCount: sample, EstimatedDeliverySeconds: 8, RetentionDescription: s.emailRetentionDescription(ctx), RefundPolicyDescription: refund, QuoteID: makeEmailQuoteID(s.quoteSigningKey, serviceCode, code, addressType, expiresAt), QuoteExpiresAt: expiresAt, Capabilities: p.Capabilities(ctx), ProviderCostEstimate: providerCost, PricingRuleSnapshot: pricingSnapshot}
		_ = ttl
		out = append(out, quote)
	}
	return out, rows.Err()
}

// emailGradePricing reuses the SMS grade table so both products retain the
// same S/A/B/C/D pricing semantics. Missing rules intentionally degrade to
// the neutral multiplier and zero fixed markup.
func (s *EmailVerificationService) emailGradePricing(ctx context.Context, grade string) (float64, float64) {
	if strings.TrimSpace(grade) == "" || s == nil || s.db == nil {
		return 1, 0
	}
	var multiplier, fixed sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `SELECT multiplier,fixed_markup FROM sms_success_rate_rules WHERE grade=$1 AND enabled`, grade).Scan(&multiplier, &fixed); err != nil {
		return 1, 0
	}
	if !multiplier.Valid || multiplier.Float64 <= 0 {
		multiplier.Float64 = 1
	}
	if !fixed.Valid || fixed.Float64 < 0 {
		fixed.Float64 = 0
	}
	return multiplier.Float64, fixed.Float64
}

// emailSalePrice keeps the administrator's channel price as a floor while
// applying configurable provider-cost, markup, minimum-profit, and success
// grade rules. The returned snapshot is persisted on the order and never sent
// to ordinary users.
func emailSalePrice(basePrice, providerCost float64, billing, channelMeta map[string]any, grade string, gradeMultiplier, gradeFixedMarkup float64) (float64, map[string]any) {
	if basePrice < 0 {
		basePrice = 0
	}
	if providerCost < 0 {
		providerCost = 0
	}
	lookup := func(key string) float64 {
		if v := numberFromMap(channelMeta, key); v != 0 {
			return v
		}
		return numberFromMap(billing, key)
	}
	baseMarkup := math.Max(0, lookup("base_markup"))
	fixedMarkup := math.Max(0, lookup("fixed_markup"))
	minimumProfit := math.Max(0, lookup("minimum_profit"))
	if gradeMultiplier <= 0 {
		gradeMultiplier = 1
	}
	if gradeFixedMarkup < 0 {
		gradeFixedMarkup = 0
	}
	calculated := providerCost*(1+baseMarkup)*gradeMultiplier + fixedMarkup + gradeFixedMarkup
	calculated = math.Max(calculated, providerCost+minimumProfit)
	finalPrice := math.Max(basePrice, calculated)
	return finalPrice, map[string]any{
		"base_price": basePrice, "provider_cost": providerCost, "base_markup": baseMarkup,
		"fixed_markup": fixedMarkup, "minimum_profit": minimumProfit, "grade": grade,
		"grade_multiplier": gradeMultiplier, "grade_fixed_markup": gradeFixedMarkup,
		"sale_price": finalPrice,
	}
}

func (s *EmailVerificationService) emailRetentionDescription(ctx context.Context) string {
	days := 7
	if s != nil && s.settings != nil && s.settings.settingRepo != nil {
		if raw, err := s.settings.settingRepo.GetValue(ctx, "email_message_retention_days"); err == nil {
			if n, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil && n > 0 {
				days = n
			}
		}
	}
	return fmt.Sprintf("%d days", days)
}
func (s *EmailVerificationService) successGrade(ctx context.Context, channel, service, addressType string) (*float64, string, int) {
	minimum := 20
	if s != nil && s.settings != nil && s.settings.settingRepo != nil {
		if raw, err := s.settings.settingRepo.GetValue(ctx, "email_success_rate_minimum_sample_size"); err == nil {
			if n, parseErr := strconv.Atoi(strings.TrimSpace(raw)); parseErr == nil && n > 0 {
				minimum = n
			}
		}
	}
	// Prefer a recent window and fall back to a wider one only when the
	// configured minimum sample size is not available.
	for _, hours := range []int{24, 24 * 7, 24 * 30} {
		var rate sql.NullFloat64
		var n int
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE status IN ('completed','expired','refunded','failed')), CASE WHEN COUNT(*) FILTER (WHERE status IN ('completed','expired','refunded','failed')) >= $4 THEN COUNT(*) FILTER (WHERE status='completed')::float/NULLIF(COUNT(*) FILTER (WHERE status IN ('completed','expired','refunded','failed')),0) ELSE NULL END FROM email_orders o JOIN email_channels c ON c.id=o.channel_id JOIN email_services sv ON sv.id=o.service_id WHERE c.code=$1 AND sv.code=$2 AND o.address_type=$3 AND o.created_at >= NOW() - ($5 * INTERVAL '1 hour')`, channel, service, addressType, minimum, hours).Scan(&n, &rate)
		if rate.Valid {
			v := rate.Float64
			return &v, s.emailSuccessGrade(ctx, v), n
		}
		if hours == 24*30 {
			return nil, "", n
		}
	}
	return nil, "", 0
}

func (s *EmailVerificationService) emailSuccessGrade(ctx context.Context, v float64) string {
	threshold := func(key string, fallback float64) float64 {
		if s != nil && s.settings != nil && s.settings.settingRepo != nil {
			if raw, err := s.settings.settingRepo.GetValue(ctx, key); err == nil {
				if parsed, parseErr := strconv.ParseFloat(strings.TrimSpace(raw), 64); parseErr == nil && parsed >= 0 && parsed <= 1 {
					return parsed
				}
			}
		}
		return fallback
	}
	// Defaults preserve the existing S/A/B/C/D scale while allowing operators
	// to tune thresholds from the normal settings service.
	if v >= threshold("email_success_grade_s_threshold", .95) {
		return "S"
	}
	if v >= threshold("email_success_grade_a_threshold", .90) {
		return "A"
	}
	if v >= threshold("email_success_grade_b_threshold", .80) {
		return "B"
	}
	if v >= threshold("email_success_grade_c_threshold", .60) {
		return "C"
	}
	return "D"
}

type EmailPurchaseRequest struct {
	ChannelCode, ServiceCode, AddressType string
	ExpectedPrice                         *float64
	QuoteID                               string
}

func normalizeEmailAddressType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "gmail", nil
	}
	switch value {
	case "gmail", "outlook", "hotmail",
		"gmail_real", "gmail_alias", "outlook_real", "outlook_alias":
		return value, nil
	default:
		return "", errors.New("unsupported email address type")
	}
}

func (s *EmailVerificationService) emailSettingInt(ctx context.Context, key string, fallback int) int {
	if s == nil || s.settings == nil || s.settings.settingRepo == nil {
		return fallback
	}
	raw, err := s.settings.settingRepo.GetValue(ctx, key)
	if err != nil {
		return fallback
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func (s *EmailVerificationService) allowFreePublicInbox(ctx context.Context, userID int64) error {
	dailyLimit := s.emailSettingInt(ctx, "email_free_daily_limit", 20)
	activeLimit := s.emailSettingInt(ctx, "email_free_active_limit", 3)
	intervalSeconds := s.emailSettingInt(ctx, "email_free_generation_interval_seconds", 5)

	var daily, active int
	var lastCreated sql.NullTime
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE o.created_at >= CURRENT_DATE),
			COUNT(*) FILTER (WHERE o.status IN ('reserved','generating_inbox','reconciling','waiting_email','email_received','verification_extracted')),
			MAX(o.created_at)
		FROM email_orders o
		JOIN email_providers p ON p.id=o.provider_id
		WHERE o.user_id=$1 AND p.code='temp_tf'`, userID).Scan(&daily, &active, &lastCreated); err != nil {
		return err
	}
	if dailyLimit > 0 && daily >= dailyLimit {
		return ErrEmailFreeDailyLimit
	}
	if activeLimit > 0 && active >= activeLimit {
		return ErrEmailFreeActiveLimit
	}
	if intervalSeconds > 0 && lastCreated.Valid && time.Since(lastCreated.Time) < time.Duration(intervalSeconds)*time.Second {
		return ErrEmailFreeGenerationCooldown
	}
	return nil
}

func (s *EmailVerificationService) Purchase(ctx context.Context, userID int64, req EmailPurchaseRequest, idempotencyKey string) (*EmailOrder, error) {
	if !s.Enabled(ctx) {
		return nil, ErrEmailFeatureDisabled
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		return nil, errors.New("Idempotency-Key is required")
	}
	addressType, normalizeErr := normalizeEmailAddressType(req.AddressType)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	req.AddressType = addressType
	req.ServiceCode = normalizeEmailServiceCode(req.ServiceCode)
	// Idempotent retries return the original order even when the quote used for
	// the first request has since expired.
	var existingID int64
	if lookupErr := s.db.QueryRowContext(ctx, `SELECT id FROM email_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&existingID); lookupErr == nil {
		return s.getOrderByID(ctx, userID, existingID)
	} else if lookupErr != sql.ErrNoRows {
		return nil, lookupErr
	}
	quotes, e := s.Quote(ctx, req.ServiceCode, req.AddressType)
	if e != nil {
		return nil, e
	}
	var selected *EmailPublicChannel
	for i := range quotes {
		if req.ChannelCode == "" || quotes[i].Code == req.ChannelCode {
			selected = &quotes[i]
			break
		}
	}
	if selected == nil {
		return nil, ErrEmailChannelUnavailable
	}
	if err := validateEmailQuoteID(s.quoteSigningKey, req.QuoteID, req.ServiceCode, selected.Code, req.AddressType, time.Now()); err != nil {
		return nil, err
	}
	if req.ExpectedPrice != nil && math.Abs(*req.ExpectedPrice-selected.SalePrice) > 1e-8 {
		return nil, ErrEmailPriceChanged
	}
	var channelID, providerID, serviceID int64
	var providerCode, base, cred string
	var metadataRaw, billingRaw []byte
	var refundPolicy, capturePolicy string
	var ttlSeconds int
	e = s.db.QueryRowContext(ctx, `SELECT c.id,p.id,sv.id,p.code,p.base_url,p.credential_ref,p.metadata,p.billing,c.refund_policy,c.capture_policy,c.order_ttl_seconds FROM email_channels c JOIN email_providers p ON p.id=c.provider_id JOIN email_services sv ON sv.code=$1 WHERE c.code=$2 AND c.enabled AND c.healthy AND p.enabled`, strings.ToLower(req.ServiceCode), selected.Code).Scan(&channelID, &providerID, &serviceID, &providerCode, &base, &cred, &metadataRaw, &billingRaw, &refundPolicy, &capturePolicy, &ttlSeconds)
	if e != nil {
		return nil, ErrEmailChannelUnavailable
	}
	var metadata map[string]any
	_ = json.Unmarshal(metadataRaw, &metadata)
	var billing map[string]any
	_ = json.Unmarshal(billingRaw, &billing)
	p := emailProviderFor(providerCode, base, cred, metadata, s.encryptor, billing)
	if p == nil {
		return nil, ErrEmailChannelUnavailable
	}
	if strings.EqualFold(providerCode, "temp_tf") {
		if err := s.allowFreePublicInbox(ctx, userID); err != nil {
			return nil, err
		}
	}
	if !s.emailQuotaAllowsNewOrder(ctx, providerID) {
		return nil, ErrEmailChannelUnavailable
	}
	orderNo := randomID()
	if ttlSeconds <= 0 {
		ttlSeconds = 15 * 60
	}
	expires := time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return nil, e
	}
	var orderID int64
	refundPolicy = enforceEmailRefundSafety(refundPolicy, selected.SalePrice)
	if capturePolicy == "" {
		capturePolicy = EmailCaptureOnTargetReceived
	}
	providerCostEstimate := selected.ProviderCostEstimate
	if providerCostEstimate <= 0 {
		var billing map[string]any
		_ = json.Unmarshal(billingRaw, &billing)
		providerCostEstimate = estimateEmailOrderCost(billing)
	}
	pricingSnapshot, _ := json.Marshal(selected.PricingRuleSnapshot)
	if len(pricingSnapshot) == 0 || string(pricingSnapshot) == "null" {
		pricingSnapshot = []byte(`{}`)
	}
	e = tx.QueryRowContext(ctx, `INSERT INTO email_orders(order_no,user_id,channel_id,provider_id,service_id,address_type,status,sale_price_snapshot,provider_cost_estimate_snapshot,pricing_rule_snapshot,success_rate_snapshot,success_rate_grade_snapshot,refund_policy_snapshot,capture_policy_snapshot,reserved_amount,expires_at,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,'reserved',$7,$8,$9,$10,$11,$12,$13,$7,$14,$15) RETURNING id`, orderNo, userID, channelID, providerID, serviceID, req.AddressType, selected.SalePrice, providerCostEstimate, pricingSnapshot, selected.SuccessRate, selected.SuccessRateGrade, refundPolicy, capturePolicy, expires, idempotencyKey).Scan(&orderID)
	if e != nil {
		_ = tx.Rollback()
		if e2 := s.db.QueryRowContext(ctx, `SELECT id FROM email_orders WHERE user_id=$1 AND idempotency_key=$2`, userID, idempotencyKey).Scan(&orderID); e2 == nil {
			return s.getOrderByID(ctx, userID, orderID)
		}
		return nil, e
	}
	res, e := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW() WHERE id=$2 AND deleted_at IS NULL AND balance >= $1`, selected.SalePrice, userID)
	if e != nil {
		_ = tx.Rollback()
		return nil, e
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		_ = tx.Rollback()
		return nil, ErrEmailInsufficientBalance
	}
	if e = tx.Commit(); e != nil {
		return nil, e
	}
	s.recordOrderEvent(ctx, orderID, "balance_reserved", "email_reserve:"+orderNo, map[string]any{"amount": selected.SalePrice})
	_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET status='generating_inbox',updated_at=NOW() WHERE id=$1 AND status='reserved'`, orderID)
	s.recordOrderEvent(ctx, orderID, "generate_requested", "email_generate:"+orderNo, nil)
	started := time.Now()
	inbox, e := p.GenerateInbox(ctx, GenerateInboxRequest{AddressType: req.AddressType})
	if e != nil {
		s.recordProviderUsage(ctx, providerID, orderID, "generate_inbox", e, time.Since(started))
		if isEmailProviderTimeout(e) {
			_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET status='reconciling',error_code='EMAIL_PROVIDER_TIMEOUT',error_public_message='当前邮箱通道繁忙，请稍后再试',error_admin_message=$1,updated_at=NOW() WHERE id=$2`, sanitizeEmailProviderError(e), orderID)
			s.recordOrderEvent(ctx, orderID, "recovery_started", "email_generate:"+orderNo, map[string]any{"reason": "provider_timeout"})
			return nil, ErrEmailProviderUnknown
		}
		// Release the hold and mark the order in one transaction. The
		// released_amount guard makes retries/recovery idempotent.
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, txErr
		}
		res, txErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='failed',error_code='EMAIL_GENERATION_FAILED',error_public_message='邮箱生成失败，资金已退回平台余额',error_admin_message=$1,released_amount=sale_price_snapshot,refund_status='released',updated_at=NOW() WHERE id=$2 AND status='generating_inbox' AND released_amount=0`, sanitizeEmailProviderError(e), orderID)
		if txErr == nil {
			if affected, _ := res.RowsAffected(); affected == 1 {
				_, txErr = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, selected.SalePrice, userID)
			}
		}
		if txErr != nil {
			_ = tx.Rollback()
			return nil, txErr
		}
		if txErr = tx.Commit(); txErr != nil {
			return nil, txErr
		}
		s.recordOrderEvent(ctx, orderID, "failed", "email_generate:"+orderNo, map[string]any{"reason": "generation_failed"})
		s.recordOrderEvent(ctx, orderID, "balance_released", "email_release:"+orderNo, map[string]any{"amount": selected.SalePrice})
		return nil, errors.New("email generation failed")
	}
	if inbox == nil || strings.TrimSpace(inbox.EmailAddress) == "" {
		// Treat an empty successful response as a provider failure and release
		// the hold atomically; a malformed provider response must not strand
		// customer funds.
		malformedErr := errors.New("email provider returned no inbox")
		s.recordProviderUsage(ctx, providerID, orderID, "generate_inbox", malformedErr, time.Since(started))
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, txErr
		}
		res, txErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='failed',error_code='EMAIL_GENERATION_FAILED',error_public_message='邮箱生成失败，资金已退回平台余额',error_admin_message='provider returned an empty inbox',released_amount=sale_price_snapshot,refund_status='released',updated_at=NOW() WHERE id=$1 AND status='generating_inbox' AND released_amount=0`, orderID)
		if txErr == nil {
			if affected, _ := res.RowsAffected(); affected == 1 {
				_, txErr = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, selected.SalePrice, userID)
			}
		}
		if txErr != nil {
			_ = tx.Rollback()
			return nil, txErr
		}
		if txErr = tx.Commit(); txErr != nil {
			return nil, txErr
		}
		s.recordOrderEvent(ctx, orderID, "failed", "email_generate:"+orderNo, map[string]any{"reason": "malformed_provider_response"})
		s.recordOrderEvent(ctx, orderID, "balance_released", "email_release:"+orderNo, map[string]any{"amount": selected.SalePrice})
		return nil, errors.New("email provider returned no inbox")
	}
	s.recordProviderUsage(ctx, providerID, orderID, "generate_inbox", nil, time.Since(started))
	if _, e = s.db.ExecContext(ctx, `UPDATE email_orders SET status='waiting_email',provider_inbox_id=$1,email_address=$2,address_type=$3,inbox_created_at=NOW(),waiting_started_at=NOW(),next_poll_at=NOW(),updated_at=NOW() WHERE id=$4`, inbox.ProviderInboxID, inbox.EmailAddress, inbox.AddressType, orderID); e != nil {
		return nil, e
	}
	s.recordOrderEvent(ctx, orderID, "inbox_generated", "email_generate:"+orderNo, map[string]any{"address_type": inbox.AddressType})
	s.recordOrderEvent(ctx, orderID, "inbox_delivered", "email_generate:"+orderNo, nil)
	return s.getOrderByID(ctx, userID, orderID)
}

func isEmailProviderTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var n net.Error
	return errors.As(err, &n) && n.Timeout()
}

// emailQuotaAllowsNewOrder applies the provider stop threshold only to new
// orders. Existing WAITING_EMAIL orders continue polling so an exhausted
// quota cannot strand already delivered inboxes.
func (s *EmailVerificationService) emailQuotaAllowsNewOrder(ctx context.Context, providerID int64) bool {
	var billingRaw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT billing FROM email_providers WHERE id=$1`, providerID).Scan(&billingRaw); err != nil {
		return false
	}
	var billing map[string]any
	if json.Unmarshal(billingRaw, &billing) != nil || billing == nil {
		return true
	}
	limit := numberFromMap(billing, "included_requests_monthly")
	stopPercent := numberFromMap(billing, "quota_stop_percent")
	if stopPercent <= 0 {
		stopPercent = numberFromMap(billing, "stop_new_order_percent")
	}
	if stopPercent <= 0 {
		stopPercent = 95
	}
	var monthUsed, dayUsed, activeOrders int64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE created_at>=date_trunc('month',NOW())), COUNT(*) FILTER (WHERE created_at>=CURRENT_DATE) FROM email_provider_usage WHERE provider_id=$1`, providerID).Scan(&monthUsed, &dayUsed)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_orders WHERE provider_id=$1 AND status IN ('reserved','generating_inbox','reconciling','waiting_email','email_received','verification_extracted')`, providerID).Scan(&activeOrders)
	allowed := !(limit > 0 && float64(monthUsed) >= limit*stopPercent/100)
	if daily := numberFromMap(billing, "included_requests_daily"); daily > 0 && float64(dayUsed) >= daily*stopPercent/100 {
		allowed = false
	}
	if reserve := numberFromMap(billing, "quota_reserved_for_active_orders"); reserve > 0 && limit > 0 && activeOrders > 0 && float64(monthUsed) >= limit*math.Max(0, stopPercent-reserve)/100 {
		allowed = false
	}
	if allowed {
		_, _ = s.db.ExecContext(ctx, `UPDATE email_providers SET health_status='healthy',updated_at=NOW() WHERE id=$1 AND health_status='quota_limited'`, providerID)
	} else {
		_, _ = s.db.ExecContext(ctx, `UPDATE email_providers SET health_status='quota_limited',updated_at=NOW() WHERE id=$1 AND health_status<>'quota_limited'`, providerID)
	}
	return allowed
}

func numberFromMap(values map[string]any, key string) float64 {
	switch value := values[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		v, _ := value.Float64()
		return v
	case string:
		v, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return v
	default:
		return 0
	}
}

func (s *EmailVerificationService) recordProviderUsage(ctx context.Context, providerID, orderID int64, operation string, requestErr error, latency time.Duration) {
	statusCode := 0
	rateLimited := false
	if providerErr, ok := requestErr.(*EmailProviderHTTPError); ok {
		statusCode = providerErr.StatusCode
		rateLimited = providerErr.StatusCode == http.StatusTooManyRequests
	}
	var billingRaw []byte
	var billing map[string]any
	if s != nil && s.db != nil {
		_ = s.db.QueryRowContext(ctx, `SELECT billing FROM email_providers WHERE id=$1`, providerID).Scan(&billingRaw)
		_ = json.Unmarshal(billingRaw, &billing)
	}
	var orderRef any
	if orderID > 0 {
		orderRef = orderID
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO email_provider_usage(provider_id,email_order_id,operation,status_code,success,latency_ms,rate_limited,estimated_request_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, providerID, orderRef, operation, statusCode, requestErr == nil, latency.Milliseconds(), rateLimited, estimateEmailRequestCost(billing))
}

func estimateEmailRequestCost(billing map[string]any) float64 {
	if billing == nil {
		return 0
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(billing["cost_mode"])))
	if mode == "manual" {
		return math.Max(0, numberFromMap(billing, "manual_request_cost"))
	}
	perRequest := math.Max(0, numberFromMap(billing, "overage_price_per_request"))
	if perRequest == 0 {
		perRequest = math.Max(0, numberFromMap(billing, "request_price"))
	}
	monthly := numberFromMap(billing, "monthly_fee")
	included := numberFromMap(billing, "included_requests_monthly")
	if (mode == "monthly_amortized" || mode == "request_based_plus_amortized" || (mode == "" && monthly > 0)) && monthly > 0 && included > 0 {
		perRequest += monthly / included
	}
	return perRequest
}

func estimateEmailOrderCost(billing map[string]any) float64 {
	if billing == nil {
		return 0
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(billing["cost_mode"])))
	if mode == "fixed_per_order" {
		fixed := numberFromMap(billing, "fixed_cost_per_order")
		if fixed == 0 {
			fixed = numberFromMap(billing, "manual_request_cost")
		}
		return math.Max(0, fixed)
	}
	requests := numberFromMap(billing, "estimated_requests_per_order")
	if requests <= 0 {
		requests = 1
	}
	return estimateEmailRequestCost(billing) * requests
}

func sanitizeEmailProviderError(error) string { return "email provider unavailable" }
func (s *EmailVerificationService) getOrderByID(ctx context.Context, userID, id int64) (*EmailOrder, error) {
	var o EmailOrder
	var rate sql.NullFloat64
	var exp, first sql.NullTime
	var errorCode string
	e := s.db.QueryRowContext(ctx, `SELECT o.public_id::text,o.order_no,o.status,c.code,c.public_name,sv.code,o.email_address,o.address_type,o.sale_price_snapshot,o.success_rate_snapshot,o.success_rate_grade_snapshot,o.refund_policy_snapshot,o.capture_policy_snapshot,o.expires_at,o.created_at,o.first_message_at,o.refund_status,o.refund_reason,o.error_code,o.error_public_message FROM email_orders o JOIN email_channels c ON c.id=o.channel_id JOIN email_services sv ON sv.id=o.service_id WHERE o.user_id=$1 AND o.id=$2`, userID, id).Scan(&o.ID, &o.OrderNo, &o.Status, &o.ChannelCode, &o.ChannelName, &o.ServiceCode, &o.EmailAddress, &o.AddressType, &o.Price, &rate, &o.SuccessRateGrade, &o.RefundPolicy, &o.CapturePolicy, &exp, &o.CreatedAt, &first, &o.RefundStatus, &o.RefundReason, &errorCode, &o.ErrorPublicMessage)
	if e != nil {
		return nil, e
	}
	normalizeEmailOrderForUser(&o, errorCode)
	if rate.Valid {
		o.SuccessRate = &rate.Float64
	}
	if exp.Valid {
		o.ExpiresAt = &exp.Time
	}
	if first.Valid {
		o.FirstMessageAt = &first.Time
	}
	msgs, e := s.listMessagesPublic(ctx, id)
	if e == nil {
		o.Messages = msgs
		for _, msg := range msgs {
			if code := strings.TrimSpace(msg.VerificationCode); code != "" {
				o.LatestVerificationCode = code
				break
			}
		}
	}
	return &o, nil
}

func normalizeEmailOrderForUser(o *EmailOrder, errorCode string) {
	if o == nil {
		return
	}
	// Older development snapshots contained mojibake in a few persisted
	// public messages. Normalize by stable error code at the DTO boundary so
	// users never receive unreadable or provider-internal text.
	switch errorCode {
	case "EMAIL_PROVIDER_TIMEOUT":
		o.ErrorPublicMessage = "Email channel is busy; please try again later"
	case "EMAIL_GENERATION_FAILED":
		o.ErrorPublicMessage = "Email generation failed; funds were returned to platform balance"
	case "EMAIL_PROVIDER_REQUEST_LIMIT":
		o.ErrorPublicMessage = "Waiting for email timed out"
	case "EMAIL_RECOVERY_REQUIRED":
		o.ErrorPublicMessage = "Email order is being reconciled"
	}
	o.ChannelName = publicVerificationChannelName(o.ChannelCode)
	o.RefundReason = ""
}

func (s *EmailVerificationService) listMessagesPublic(ctx context.Context, id int64) ([]EmailMessage, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id::text,from_address,from_name,to_address,subject,text_body,html_body,verification_code,verification_url,verification_confidence,verification_method,received_at FROM email_messages WHERE email_order_id=$1 ORDER BY received_at DESC`, id)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []EmailMessage{}
	for rows.Next() {
		var m EmailMessage
		if e = rows.Scan(&m.ID, &m.FromAddress, &m.FromName, &m.ToAddress, &m.Subject, &m.TextBody, &m.HTMLBody, &m.VerificationCode, &m.VerificationURL, &m.VerificationConfidence, &m.VerificationMethod, &m.ReceivedAt); e != nil {
			return nil, e
		}
		// Normalize on read as well so historical rows saved before body
		// normalization do not expose provider CSS/template source to users.
		m.TextBody = normalizeEmailText(m.TextBody, m.HTMLBody)
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *EmailVerificationService) GetOrder(ctx context.Context, userID int64, publicID string) (*EmailOrder, error) {
	var id int64
	if e := s.db.QueryRowContext(ctx, `SELECT id FROM email_orders WHERE user_id=$1 AND public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id); e != nil {
		return nil, ErrEmailNotFound
	}
	return s.getOrderByID(ctx, userID, id)
}

// CancelOrder changes only the local order and balance ledger. Emailnator has
// no provider-side order cancellation/refund contract, so cancellation is
// settled according to the order's snapshotted platform policy.
func (s *EmailVerificationService) CancelOrder(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var status, policy, refundStatus string
	var price, capturedAmount float64
	var first sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT id,status,refund_policy_snapshot,sale_price_snapshot,first_message_at,captured_amount,refund_status FROM email_orders WHERE user_id=$1 AND public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &status, &policy, &price, &first, &capturedAmount, &refundStatus); err != nil {
		return ErrEmailNotFound
	}
	if status == "completed" || status == "refunded" || status == "expired" || status == "cancelled" || status == "failed" {
		return errors.New("email order cannot be cancelled")
	}
	policy = enforceEmailRefundSafety(policy, price)
	if price > 0 && (status == "generating_inbox" || status == "reconciling") {
		return ErrEmailProviderUnknown
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var balanceBack, captureNow bool
	var query string
	if status == "reserved" || status == "generating_inbox" || status == "reconciling" {
		query = `UPDATE email_orders SET status='cancelled',cancelled_at=NOW(),refund_status='released',released_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('reserved','generating_inbox','reconciling') AND released_amount=0`
		balanceBack = true
	} else if policy == EmailRefundIfNoMessage && !first.Valid {
		query = `UPDATE email_orders SET status='cancelled',cancelled_at=NOW(),refund_status='approved',refund_reason='cancelled before target email',refunded_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('waiting_email','email_received','verification_extracted') AND first_message_at IS NULL AND refunded_amount=0 AND captured_amount=0`
		balanceBack = true
	} else if capturedAmount > 0 || refundStatus == "not_applicable" {
		// The first target message may already have settled the hold. Stopping
		// the inbox must not debit the user a second time.
		query = `UPDATE email_orders SET status='cancelled',cancelled_at=NOW(),refund_status='not_applicable',updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('waiting_email','email_received','verification_extracted')`
	} else {
		query = `UPDATE email_orders SET status='cancelled',cancelled_at=NOW(),refund_status='not_applicable',captured_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('waiting_email','email_received','verification_extracted') AND captured_amount=0`
		captureNow = true
	}
	res, err := tx.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errors.New("email order cannot be cancelled")
	}
	if balanceBack {
		if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
			return err
		}
	} else if captureNow {
		if _, err = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.recordOrderEvent(ctx, id, "cancel_requested", "email_cancel:"+strconv.FormatInt(id, 10), map[string]any{"refunded": balanceBack, "captured_now": captureNow})
	return nil
}

func (s *EmailVerificationService) RequestRefund(ctx context.Context, userID int64, publicID string) error {
	var id int64
	var status, policy string
	var price float64
	var first sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT id,status,refund_policy_snapshot,sale_price_snapshot,first_message_at FROM email_orders WHERE user_id=$1 AND public_id=$2::uuid`, userID, strings.TrimSpace(publicID)).Scan(&id, &status, &policy, &price, &first); err != nil {
		return ErrEmailNotFound
	}
	policy = enforceEmailRefundSafety(policy, price)
	if policy != EmailRefundIfNoMessage || first.Valid || (status != "waiting_email" && status != "email_received") {
		return errors.New("email order is not eligible for refund")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE email_orders SET status='refunded',refund_status='approved',refund_reason='user requested refund before target email',refunded_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status IN ('waiting_email','email_received') AND first_message_at IS NULL AND refunded_amount=0 AND captured_amount=0`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return errors.New("email order is not eligible for refund")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.recordOrderEvent(ctx, id, "refund_completed", "email_refund:"+strconv.FormatInt(id, 10), map[string]any{"amount": price})
	return nil
}
func (s *EmailVerificationService) ListOrders(ctx context.Context, userID int64) ([]EmailOrder, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT public_id::text FROM email_orders WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []EmailOrder{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		o, e := s.GetOrder(ctx, userID, id)
		if e == nil {
			out = append(out, *o)
		}
	}
	return out, rows.Err()
}

func (s *EmailVerificationService) ListUserOrdersPage(ctx context.Context, userID int64, page, pageSize int, keyword, status string) (*EmailOrderPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	keyword = strings.TrimSpace(keyword)
	status = strings.TrimSpace(status)
	from := ` FROM email_orders o JOIN email_channels c ON c.id=o.channel_id JOIN email_services sv ON sv.id=o.service_id`
	where := ` WHERE o.user_id=$1`
	args := []any{userID}
	if keyword != "" {
		args = append(args, "%"+keyword+"%")
		where += fmt.Sprintf(` AND (o.order_no ILIKE $%d OR o.email_address ILIKE $%d OR sv.code ILIKE $%d)`, len(args), len(args), len(args))
	}
	if status != "" {
		args = append(args, status)
		where += fmt.Sprintf(` AND o.status=$%d`, len(args))
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	listArgs := append([]any{}, args...)
	listArgs = append(listArgs, pageSize)
	limitPlaceholder := fmt.Sprintf("$%d", len(listArgs))
	listArgs = append(listArgs, (page-1)*pageSize)
	offsetPlaceholder := fmt.Sprintf("$%d", len(listArgs))

	query := `SELECT
		o.public_id::text,o.order_no,o.status,c.code,c.public_name,sv.code,
		o.email_address,o.address_type,o.sale_price_snapshot,o.success_rate_snapshot,
		o.success_rate_grade_snapshot,o.refund_policy_snapshot,o.capture_policy_snapshot,
		o.expires_at,o.created_at,o.first_message_at,o.refund_status,o.refund_reason,
		o.error_code,o.error_public_message,
		COALESCE((
			SELECT m.verification_code
			FROM email_messages m
			WHERE m.email_order_id=o.id AND BTRIM(m.verification_code)<>''
			ORDER BY m.received_at DESC,m.id DESC
			LIMIT 1
		),'')
	` + from + where + ` ORDER BY o.created_at DESC LIMIT ` + limitPlaceholder + ` OFFSET ` + offsetPlaceholder

	rows, err := s.db.QueryContext(ctx, query, listArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]EmailOrder, 0, pageSize)
	for rows.Next() {
		var o EmailOrder
		var rate sql.NullFloat64
		var exp, first sql.NullTime
		var errorCode, latestCode string
		if err := rows.Scan(
			&o.ID, &o.OrderNo, &o.Status, &o.ChannelCode, &o.ChannelName, &o.ServiceCode,
			&o.EmailAddress, &o.AddressType, &o.Price, &rate,
			&o.SuccessRateGrade, &o.RefundPolicy, &o.CapturePolicy,
			&exp, &o.CreatedAt, &first, &o.RefundStatus, &o.RefundReason,
			&errorCode, &o.ErrorPublicMessage, &latestCode,
		); err != nil {
			return nil, err
		}
		normalizeEmailOrderForUser(&o, errorCode)
		o.LatestVerificationCode = strings.TrimSpace(latestCode)
		if rate.Valid {
			o.SuccessRate = &rate.Float64
		}
		if exp.Valid {
			t := exp.Time
			o.ExpiresAt = &t
		}
		if first.Valid {
			t := first.Time
			o.FirstMessageAt = &t
		}
		items = append(items, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if pages < 1 {
		pages = 1
	}
	return &EmailOrderPage{Items: items, Total: total, Page: page, PageSize: pageSize, Pages: pages}, nil
}

func (s *EmailVerificationService) captureEmailOrder(ctx context.Context, orderID, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Billing settlement and inbox lifetime are separate concerns. Capturing
	// the reserved amount must not mark the order completed, otherwise polling
	// would stop after the first message. refund_status is also an idempotency
	// marker for free inboxes whose captured_amount remains zero.
	res, err := tx.ExecContext(ctx, `UPDATE email_orders SET captured_amount=sale_price_snapshot,refund_status='not_applicable',updated_at=NOW() WHERE id=$1 AND status IN ('email_received','verification_extracted') AND captured_amount=0 AND refund_status<>'not_applicable'`, orderID)
	if err != nil {
		return err
	}
	captured, _ := res.RowsAffected()
	if captured == 1 {
		if _, err = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-(SELECT sale_price_snapshot FROM email_orders WHERE id=$1)),updated_at=NOW() WHERE id=$2`, orderID, userID); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if captured == 1 {
		s.recordOrderEvent(ctx, orderID, "balance_captured", "email_capture:"+strconv.FormatInt(orderID, 10), nil)
	}
	return nil
}

func (s *EmailVerificationService) reserveEmailProviderRequest(ctx context.Context, orderID int64, maxRequests int) (bool, error) {
	query := `UPDATE email_orders SET provider_request_count=provider_request_count+1,updated_at=NOW() WHERE id=$1`
	args := []any{orderID}
	if maxRequests > 0 {
		query += ` AND provider_request_count < $2`
		args = append(args, maxRequests)
	}
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected == 1, nil
}

func (s *EmailVerificationService) settleEmailProviderRequestLimit(ctx context.Context, orderID, userID int64, pollCount int) error {
	_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET error_code='EMAIL_PROVIDER_REQUEST_LIMIT',error_public_message='等待邮件超时',error_admin_message='maximum provider requests per order reached',updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted')`, orderID)
	s.recordOrderEvent(ctx, orderID, "provider_request_limit", "email_poll_limit:"+strconv.FormatInt(orderID, 10), map[string]any{"poll_count": pollCount})
	return s.expireOrderV2(ctx, orderID, userID)
}

// PollOrder performs one bounded poll. It is safe to call from a worker;
// unique dedupe hashes and conditional balance updates make duplicate work
// harmless, including a retry after message persistence but before capture.
func emailMessageStableDedupe(providerMessageID, fromAddress, toAddress, subject, textBody, htmlBody string, receivedAt time.Time) string {
	if id := strings.TrimSpace(providerMessageID); id != "" {
		return hashString("provider-id:" + id)
	}
	timePart := ""
	if !receivedAt.IsZero() {
		timePart = receivedAt.UTC().Format(time.RFC3339Nano)
	}
	return hashString(strings.Join([]string{
		"fallback",
		strings.ToLower(strings.TrimSpace(fromAddress)),
		strings.ToLower(strings.TrimSpace(toAddress)),
		strings.TrimSpace(subject),
		strings.TrimSpace(textBody),
		strings.TrimSpace(htmlBody),
		timePart,
	}, "\x1f"))
}

func (s *EmailVerificationService) convergePersistedEmailEvidence(ctx context.Context, orderID, userID int64, capturePolicy string) error {
	var first sql.NullTime
	var hasVerification bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT MIN(received_at),
		       COALESCE(BOOL_OR(BTRIM(verification_code)<>'' OR BTRIM(verification_url)<>''),FALSE)
		FROM email_messages
		WHERE email_order_id=$1`, orderID).Scan(&first, &hasVerification); err != nil {
		return err
	}
	if !first.Valid {
		return nil
	}
	targetStatus := "email_received"
	if hasVerification {
		targetStatus = "verification_extracted"
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE email_orders
		SET first_message_at=COALESCE(first_message_at,$2),
		    status=CASE
		        WHEN status IN ('waiting_email','email_received') AND $3='verification_extracted' THEN 'verification_extracted'
		        WHEN status='waiting_email' THEN 'email_received'
		        ELSE status
		    END,
		    updated_at=NOW()
		WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted')`,
		orderID, first.Time, targetStatus); err != nil {
		return err
	}
	if capturePolicy != EmailCaptureOnExtracted || hasVerification {
		return s.captureEmailOrder(ctx, orderID, userID)
	}
	return nil
}

func (s *EmailVerificationService) PollOrder(ctx context.Context, orderID int64) error {
	var userID, providerID, channelID int64
	var code, base, cred, address, inbox, status, capturePolicy string
	var maxProviderRequests int
	var metaRaw, billingRaw []byte
	var expires, startedAt, inboxCreatedAt sql.NullTime
	var pollCount, providerRequestCount int
	if e := s.db.QueryRowContext(ctx, `SELECT o.user_id,o.provider_id,o.channel_id,p.code,p.base_url,p.credential_ref,p.metadata,p.billing,o.email_address,o.provider_inbox_id,o.status,o.expires_at,o.created_at,o.inbox_created_at,o.capture_policy_snapshot,o.poll_count,o.provider_request_count,c.max_provider_requests_per_order FROM email_orders o JOIN email_providers p ON p.id=o.provider_id JOIN email_channels c ON c.id=o.channel_id WHERE o.id=$1`, orderID).Scan(&userID, &providerID, &channelID, &code, &base, &cred, &metaRaw, &billingRaw, &address, &inbox, &status, &expires, &startedAt, &inboxCreatedAt, &capturePolicy, &pollCount, &providerRequestCount, &maxProviderRequests); e != nil {
		return e
	}
	if status != "waiting_email" && status != "email_received" && status != "verification_extracted" {
		return nil
	}
	// Persisted messages are the source of truth. If a previous process wrote a
	// message and crashed before advancing the order, converge status and
	// settlement before any expiry/refund decision or provider call.
	if err := s.convergePersistedEmailEvidence(ctx, orderID, userID, capturePolicy); err != nil {
		return err
	}
	if expires.Valid && time.Now().After(expires.Time) {
		return s.expireOrderV2(ctx, orderID, userID)
	}
	if maxProviderRequests > 0 && providerRequestCount >= maxProviderRequests {
		return s.settleEmailProviderRequestLimit(ctx, orderID, userID, pollCount)
	}
	var meta map[string]any
	_ = json.Unmarshal(metaRaw, &meta)
	var billing map[string]any
	_ = json.Unmarshal(billingRaw, &billing)
	p := emailProviderFor(code, base, cred, meta, s.encryptor, billing)
	if p == nil {
		return ErrEmailChannelUnavailable
	}

	existingDedupe := map[string]struct{}{}
	existingProviderIDs := map[string]struct{}{}
	rows, err := s.db.QueryContext(ctx, `SELECT dedupe_hash,provider_message_id FROM email_messages WHERE email_order_id=$1`, orderID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var dedupe, providerMessageID string
		if err := rows.Scan(&dedupe, &providerMessageID); err != nil {
			_ = rows.Close()
			return err
		}
		if dedupe = strings.TrimSpace(dedupe); dedupe != "" {
			existingDedupe[dedupe] = struct{}{}
		}
		if providerMessageID = strings.TrimSpace(providerMessageID); providerMessageID != "" {
			existingProviderIDs[providerMessageID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()

	allowed, requestErr := s.reserveEmailProviderRequest(ctx, orderID, maxProviderRequests)
	if requestErr != nil {
		return requestErr
	}
	if !allowed {
		return s.settleEmailProviderRequestLimit(ctx, orderID, userID, pollCount)
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET poll_count=poll_count+1,last_polled_at=NOW(),updated_at=NOW() WHERE id=$1`, orderID)
	listStarted := time.Now()
	list, e := p.ListMessages(ctx, ListMessagesRequest{ProviderInboxID: inbox, EmailAddress: address})
	s.recordProviderUsage(ctx, providerID, orderID, "list_messages", e, time.Since(listStarted))
	if e != nil {
		return e
	}
	minimumReceivedAt := time.Time{}
	if startedAt.Valid {
		minimumReceivedAt = startedAt.Time
	}
	if inboxCreatedAt.Valid && inboxCreatedAt.Time.After(minimumReceivedAt) {
		minimumReceivedAt = inboxCreatedAt.Time
	}
	for _, summary := range list.Messages {
		summaryProviderID := strings.TrimSpace(summary.ProviderMessageID)
		if summaryProviderID != "" {
			if _, exists := existingProviderIDs[summaryProviderID]; exists {
				continue
			}
		} else if !summary.ReceivedAt.IsZero() {
			summaryDedupe := emailMessageStableDedupe("", summary.FromAddress, summary.ToAddress, summary.Subject, "", "", summary.ReceivedAt)
			if _, exists := existingDedupe[summaryDedupe]; exists {
				continue
			}
		}
		if !summary.ReceivedAt.IsZero() && !minimumReceivedAt.IsZero() && summary.ReceivedAt.Before(minimumReceivedAt.Add(-2*time.Minute)) {
			continue
		}
		allowed, requestErr = s.reserveEmailProviderRequest(ctx, orderID, maxProviderRequests)
		if requestErr != nil {
			return requestErr
		}
		if !allowed {
			return s.settleEmailProviderRequestLimit(ctx, orderID, userID, pollCount)
		}
		getStarted := time.Now()
		msg, e := p.GetMessage(ctx, GetMessageRequest{ProviderInboxID: inbox, ProviderMessageID: summary.ProviderMessageID, EmailAddress: address})
		if e != nil {
			s.recordProviderUsage(ctx, providerID, orderID, "get_message", e, time.Since(getStarted))
			return e
		}
		if msg == nil {
			err := errors.New("email provider returned empty message")
			s.recordProviderUsage(ctx, providerID, orderID, "get_message", err, time.Since(getStarted))
			return err
		}
		if strings.TrimSpace(msg.ProviderMessageID) == "" {
			msg.ProviderMessageID = summary.ProviderMessageID
		}
		if strings.TrimSpace(msg.FromAddress) == "" {
			msg.FromAddress = summary.FromAddress
		}
		if strings.TrimSpace(msg.FromName) == "" {
			msg.FromName = summary.FromName
		}
		if strings.TrimSpace(msg.ToAddress) == "" {
			msg.ToAddress = summary.ToAddress
		}
		if strings.TrimSpace(msg.Subject) == "" {
			msg.Subject = summary.Subject
		}
		if msg.ReceivedAt.IsZero() || msg.ReceivedAt.After(time.Now().Add(time.Minute)) {
			msg.ReceivedAt = summary.ReceivedAt
		}
		s.recordProviderUsage(ctx, providerID, orderID, "get_message", nil, time.Since(getStarted))
		if !msg.ReceivedAt.IsZero() && !minimumReceivedAt.IsZero() && msg.ReceivedAt.Before(minimumReceivedAt.Add(-2*time.Minute)) {
			continue
		}
		if !s.messageMatches(ctx, orderID, msg) {
			continue
		}
		safeHTML := SanitizeEmailHTML(msg.HTMLBody)
		normalizedText := normalizeEmailText(msg.TextBody, msg.HTMLBody)
		extract := ExtractVerification(msg.Subject, normalizedText, safeHTML)
		dedupe := emailMessageStableDedupe(msg.ProviderMessageID, msg.FromAddress, msg.ToAddress, msg.Subject, normalizedText, safeHTML, msg.ReceivedAt)
		if _, exists := existingDedupe[dedupe]; exists {
			if err := s.convergePersistedEmailEvidence(ctx, orderID, userID, capturePolicy); err != nil {
				return err
			}
			continue
		}
		receivedAt := msg.ReceivedAt
		if receivedAt.IsZero() {
			receivedAt = time.Now().UTC()
		}
		rawPayload := []byte(`{}`)
		if _, e = s.db.ExecContext(ctx, `INSERT INTO email_messages(email_order_id,provider_message_id,from_address,from_name,to_address,subject,text_body,html_body,verification_code,verification_url,verification_confidence,verification_method,received_at,dedupe_hash,raw_payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(email_order_id,dedupe_hash) DO NOTHING`, orderID, msg.ProviderMessageID, msg.FromAddress, msg.FromName, msg.ToAddress, msg.Subject, normalizedText, safeHTML, extract.Code, extract.URL, extract.Confidence, extract.Method, receivedAt, dedupe, rawPayload); e != nil {
			return e
		}
		existingDedupe[dedupe] = struct{}{}
		if id := strings.TrimSpace(msg.ProviderMessageID); id != "" {
			existingProviderIDs[id] = struct{}{}
		}
		s.recordOrderEvent(ctx, orderID, "message_received", "email_message:"+dedupe, map[string]any{"matched": true})
		if extract.Code != "" || extract.URL != "" {
			s.recordOrderEvent(ctx, orderID, "verification_extracted", "email_message:"+dedupe, map[string]any{"method": extract.Method})
		}
		if err := s.convergePersistedEmailEvidence(ctx, orderID, userID, capturePolicy); err != nil {
			return err
		}
	}
	return nil
}

func (s *EmailVerificationService) messageMatches(ctx context.Context, serviceID int64, msg *ProviderEmailMessage) bool {
	rows, e := s.db.QueryContext(ctx, `SELECT r.sender_exact,r.sender_domain,r.subject_contains,r.subject_regex FROM email_service_match_rules r JOIN email_orders o ON o.service_id=r.service_id WHERE o.id=$1 AND r.enabled ORDER BY r.priority DESC`, serviceID)
	if e != nil {
		return false
	}
	defer func() { _ = rows.Close() }()
	matched := false
	ruleCount := 0
	for rows.Next() {
		ruleCount++
		var exact, domain, contains, rx string
		if rows.Scan(&exact, &domain, &contains, &rx) != nil {
			continue
		}
		sender := strings.ToLower(strings.TrimSpace(msg.FromAddress))
		ok := true
		if exact != "" {
			ok = ok && strings.EqualFold(sender, exact)
		}
		if domain != "" {
			configuredDomain := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "@"))
			senderDomain := ""
			if at := strings.LastIndex(sender, "@"); at >= 0 && at+1 < len(sender) {
				senderDomain = sender[at+1:]
			}
			ok = ok && (senderDomain == configuredDomain || strings.HasSuffix(senderDomain, "."+configuredDomain))
		}
		if contains != "" {
			ok = ok && strings.Contains(strings.ToLower(msg.Subject), strings.ToLower(contains))
		}
		if rx != "" {
			if r, e := regexp.Compile(rx); e == nil {
				ok = ok && r.MatchString(msg.Subject)
			} else {
				ok = false
			}
		}
		if ok {
			matched = true
			break
		}
	}
	// Services without configured rules (for example the generic "other"
	// catalog entry) accept any message in the purchased inbox. Configured
	// services remain strict and only settle on a matching sender/subject.
	if ruleCount == 0 {
		return true
	}
	return matched
}

// Deprecated: expireOrder is retained only for historical migrations. New
// polling uses expireOrderV2, which has idempotent balance settlement.
func (s *EmailVerificationService) expireOrder(ctx context.Context, id, userID int64) error { //nolint:unused
	var status, policy string
	var price float64
	if e := s.db.QueryRowContext(ctx, `SELECT status,refund_policy_snapshot,sale_price_snapshot FROM email_orders WHERE id=$1`, id).Scan(&status, &policy, &price); e != nil {
		return e
	}
	if status == "completed" || status == "refunded" {
		return nil
	}
	if policy == EmailRefundIfNoMessage {
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()
		res, e := tx.ExecContext(ctx, `UPDATE email_orders SET status='refunded',refund_status='approved',refund_reason='未收到目标验证邮件',refunded_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted') AND refund_status='not_requested'`, id)
		if e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n == 1 {
			if _, e = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID); e != nil {
				return e
			}
		}
		if e = tx.Commit(); e != nil {
			return e
		}
		if n, _ := res.RowsAffected(); n == 1 {
			s.recordOrderEvent(ctx, id, "refund_completed", "email_refund:"+strconv.FormatInt(id, 10), map[string]any{"amount": price, "reason": "no_target_message"})
		}
		return nil
	}
	_, e := s.db.ExecContext(ctx, `UPDATE email_orders SET status='expired',updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted')`, id)
	if e == nil {
		s.recordOrderEvent(ctx, id, "expired", "email_expire:"+strconv.FormatInt(id, 10), nil)
	}
	return e
}

// expireOrderV2 settles the frozen hold according to the order's snapshotted
// policy. It is intentionally separate from the legacy implementation above
// so upgrades remain source-compatible while all new polling uses the
// idempotent transaction below.
func (s *EmailVerificationService) expireOrderV2(ctx context.Context, id, userID int64) error {
	var status, policy, refundStatus string
	var price, capturedAmount float64
	var firstMessage sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT status,refund_policy_snapshot,sale_price_snapshot,first_message_at,captured_amount,refund_status FROM email_orders WHERE id=$1`, id).Scan(&status, &policy, &price, &firstMessage, &capturedAmount, &refundStatus); err != nil {
		return err
	}
	if status == "completed" || status == "refunded" || status == "expired" || status == "cancelled" || status == "failed" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	var eventType, eventKey string
	var eventPayload map[string]any
	switch {
	case policy == EmailRefundIfNoMessage && !firstMessage.Valid:
		res, execErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='refunded',refund_status='approved',refund_reason='未收到目标验证邮件',refunded_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted') AND first_message_at IS NULL AND refund_status='not_requested' AND refunded_amount=0`, id)
		if execErr != nil {
			_ = tx.Rollback()
			return execErr
		}
		if n, _ := res.RowsAffected(); n == 1 {
			if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
				_ = tx.Rollback()
				return err
			}
			eventType, eventKey = "refund_completed", "email_refund:"+strconv.FormatInt(id, 10)
			eventPayload = map[string]any{"amount": price, "reason": "no_target_message"}
		}
	case policy == EmailNoRefundAfterDelivery || (policy == EmailRefundIfNoMessage && firstMessage.Valid):
		needsCapture := capturedAmount == 0 && refundStatus != "not_applicable"
		if needsCapture {
			res, execErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='completed',completed_at=NOW(),refund_status='not_applicable',captured_amount=sale_price_snapshot,updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted') AND captured_amount=0`, id)
			if execErr != nil {
				_ = tx.Rollback()
				return execErr
			}
			if n, _ := res.RowsAffected(); n == 1 {
				if _, err = tx.ExecContext(ctx, `UPDATE users SET frozen_balance=GREATEST(0,COALESCE(frozen_balance,0)-$1),updated_at=NOW() WHERE id=$2`, price, userID); err != nil {
					_ = tx.Rollback()
					return err
				}
				eventType, eventKey = "balance_captured", "email_capture:"+strconv.FormatInt(id, 10)
				reason := "delivery_policy"
				if firstMessage.Valid {
					reason = "target_email_received"
				}
				eventPayload = map[string]any{"amount": price, "reason": reason, "receive_window_closed": true}
			}
		} else {
			res, execErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='completed',completed_at=NOW(),refund_status='not_applicable',updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted')`, id)
			if execErr != nil {
				_ = tx.Rollback()
				return execErr
			}
			if n, _ := res.RowsAffected(); n == 1 {
				eventType, eventKey = "inbox_completed", "email_inbox_completed:"+strconv.FormatInt(id, 10)
				eventPayload = map[string]any{"receive_window_closed": true}
			}
		}
	default:
		// Manual review keeps the hold explicit for an operator; it is not
		// silently released or captured by the worker.
		res, execErr := tx.ExecContext(ctx, `UPDATE email_orders SET status='expired',refund_status='manual_review',refund_reason='manual review required',updated_at=NOW() WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted') AND refund_status='not_requested'`, id)
		if execErr != nil {
			_ = tx.Rollback()
			return execErr
		}
		if n, _ := res.RowsAffected(); n == 1 {
			eventType, eventKey = "expired", "email_expire:"+strconv.FormatInt(id, 10)
			eventPayload = map[string]any{"reason": "manual_review"}
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if eventType != "" {
		s.recordOrderEvent(ctx, id, eventType, eventKey, eventPayload)
	}
	return nil
}
func (s *EmailVerificationService) PollDue(ctx context.Context) error {
	if err := s.expireDueEmailOrders(ctx); err != nil {
		return err
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, `WITH due AS (SELECT o.id,c.polling_backoff FROM email_orders o JOIN email_channels c ON c.id=o.channel_id WHERE o.status IN ('waiting_email','email_received','verification_extracted') AND o.next_poll_at<=NOW() AND o.expires_at>NOW() ORDER BY o.next_poll_at FOR UPDATE OF o SKIP LOCKED LIMIT 100) UPDATE email_orders o SET next_poll_at=NOW()+INTERVAL '30 seconds',updated_at=NOW() FROM due WHERE o.id=due.id RETURNING o.id,o.poll_count,due.polling_backoff`)
	if e != nil {
		_ = tx.Rollback()
		return e
	}
	type claimedEmailOrder struct {
		id        int64
		pollCount int
		backoff   []int
	}
	claimed := []claimedEmailOrder{}
	for rows.Next() {
		var id int64
		var pollCount int
		var backoffRaw []byte
		if scanErr := rows.Scan(&id, &pollCount, &backoffRaw); scanErr != nil {
			_ = rows.Close()
			_ = tx.Rollback()
			return scanErr
		}
		claimed = append(claimed, claimedEmailOrder{id: id, pollCount: pollCount, backoff: parseEmailPollingBackoff(backoffRaw)})
	}
	if e = rows.Err(); e != nil {
		_ = rows.Close()
		_ = tx.Rollback()
		return e
	}
	if e = rows.Close(); e != nil {
		_ = tx.Rollback()
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	for _, item := range claimed {
		err := s.PollOrder(ctx, item.id)
		delay := emailPollDelayWithSchedule(item.pollCount, err, item.backoff)
		_, _ = s.db.ExecContext(ctx, `UPDATE email_orders SET next_poll_at=$2 WHERE id=$1 AND status IN ('waiting_email','email_received','verification_extracted')`, item.id, time.Now().Add(delay))
	}
	return nil
}

func (s *EmailVerificationService) expireDueEmailOrders(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id FROM email_orders WHERE status IN ('waiting_email','email_received','verification_extracted') AND expires_at<=NOW() ORDER BY expires_at LIMIT 100`)
	if err != nil {
		return err
	}
	var due [][2]int64
	for rows.Next() {
		var id, userID int64
		if err = rows.Scan(&id, &userID); err != nil {
			_ = rows.Close()
			return err
		}
		due = append(due, [2]int64{id, userID})
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, order := range due {
		if err = s.expireOrderV2(ctx, order[0], order[1]); err != nil {
			return err
		}
	}
	return nil
}

func emailPollDelay(pollCount int, err error) time.Duration { //nolint:unused
	return emailPollDelayWithSchedule(pollCount, err, nil)
}

func parseEmailPollingBackoff(raw []byte) []int {
	var values []int
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value < 1 || value > 3600 {
			return nil
		}
		out = append(out, value)
		if len(out) == 64 {
			break
		}
	}
	return out
}

func emailPollDelayWithSchedule(pollCount int, err error, schedule []int) time.Duration {
	if providerErr, ok := err.(*EmailProviderHTTPError); ok && providerErr.RetryAfter > 0 {
		return providerErr.RetryAfter
	}
	backoff := []time.Duration{2 * time.Second, 4 * time.Second, 7 * time.Second, 10 * time.Second, 15 * time.Second, 20 * time.Second, 30 * time.Second, 45 * time.Second, 60 * time.Second}
	if len(schedule) > 0 {
		backoff = make([]time.Duration, 0, len(schedule))
		for _, seconds := range schedule {
			backoff = append(backoff, time.Duration(seconds)*time.Second)
		}
	}
	if pollCount < 0 {
		pollCount = 0
	}
	if pollCount >= len(backoff) {
		pollCount = len(backoff) - 1
	}
	base := backoff[pollCount]
	// Add bounded jitter so a fleet of orders created together does not stampede
	// the provider on the same second. Retry-After remains authoritative above.
	jitter := time.Duration(mathrand.Int63n(int64(base/4 + 1)))
	return base + jitter
}

type VerificationExtraction struct {
	Code       string
	URL        string
	Confidence float64
	Method     string
}

var otpContextRE = regexp.MustCompile(`(?i)(verification code|security code|one[- ]time password|authentication code|confirm(?:ation)? code|otp|验证码|校验码|动态码|安全码|登录代码)[^0-9A-Za-z]{0,30}([0-9A-Za-z]{4,12})`)
var emailOTPContextRE = regexp.MustCompile(`(?i)(verification code|security code|one[- ]time password|authentication code|confirm(?:ation)? code|otp|验证码|校验码|动态码|安全码|登录代码)[^0-9A-Za-z]{0,30}([0-9A-Za-z]{4,12})`)
var emailOTPContextREUTF8 = regexp.MustCompile(`(?i)(verification code|security code|one[- ]time password|authentication code|confirm(?:ation)? code|otp|\x{9A8C}\x{8BC1}\x{7801}|\x{6821}\x{9A8C}\x{7801}|\x{52A8}\x{6001}\x{7801}|\x{5B89}\x{5168}\x{7801}|\x{767B}\x{5F55}\x{4EE3}\x{7801})[^0-9A-Za-z]{0,30}([0-9A-Za-z]{4,12})`)
var otpRE = regexp.MustCompile(`(?i)\b[0-9]{4}\b|\b[0-9]{6}\b|\b[0-9]{8}\b|\b[A-Z0-9]{4,8}\b`)
var urlRE = regexp.MustCompile(`https?://[^\s"'<>]+`)

var _ = emailOTPContextRE

func ExtractVerification(subject, text, htmlBody string) VerificationExtraction {
	plain := strings.TrimSpace(subject + "\n" + text + "\n" + HTMLToText(htmlBody))
	best := VerificationExtraction{}
	for _, m := range emailOTPContextREUTF8.FindAllStringSubmatch(plain, -1) {
		if len(m) < 3 {
			continue
		}
		candidate := strings.Trim(m[2], ".,:;()[]{}<>")
		if validOTPCandidate(candidate) {
			best = VerificationExtraction{Code: candidate, Confidence: .95, Method: "context_otp"}
			break
		}
	}
	// Keep compatibility with legacy localized templates that used the prior
	// encoded pattern while preferring the canonical UTF-8 context above.
	if best.Code == "" {
		for _, m := range otpContextRE.FindAllStringSubmatch(plain, -1) {
			if len(m) < 3 {
				continue
			}
			candidate := strings.Trim(m[2], ".,:;()[]{}<>")
			if validOTPCandidate(candidate) {
				best = VerificationExtraction{Code: candidate, Confidence: .9, Method: "legacy_context_otp"}
				break
			}
		}
	}
	if best.Code == "" {
		for _, m := range otpRE.FindAllString(plain, -1) {
			if !validOTPCandidate(m) {
				continue
			}
			score := .45
			if strings.Contains(plain, m) {
				score += .05
			}
			if len(m) == 6 {
				score += .1
			}
			if score >= best.Confidence {
				best = VerificationExtraction{Code: m, Confidence: score, Method: "otp_pattern"}
			}
		}
	}
	candidates := verificationURLsFromHTML(htmlBody)
	candidates = append(candidates, urlRE.FindAllString(plain, -1)...)
	for _, candidate := range candidates {
		if normalized, ok := normalizeVerificationURL(candidate); ok {
			best.URL = normalized
		} else {
			continue
		}
		if best.Code != "" {
			best.Confidence = math.Min(1, best.Confidence+.03)
			best.Method += "+url"
		}
		break
	}
	return best
}

func verificationURLsFromHTML(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	doc, err := xhtml.Parse(strings.NewReader(raw))
	if err != nil {
		return nil
	}
	urls := []string{}
	var walk func(*xhtml.Node)
	walk = func(node *xhtml.Node) {
		if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, "a") {
			for _, attr := range node.Attr {
				if strings.EqualFold(attr.Key, "href") {
					urls = append(urls, attr.Val)
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return urls
}

func normalizeVerificationURL(raw string) (string, bool) {
	value := strings.TrimSpace(strings.TrimRight(raw, ".,);]}"))
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return "", false
	}
	lower := strings.ToLower(value)
	for _, ignored := range []string{"unsubscribe", "privacy", "terms", "support", "tracking", "social"} {
		if strings.Contains(lower, ignored) {
			return "", false
		}
	}
	return value, true
}
func validOTPCandidate(v string) bool {
	if len(v) < 4 || len(v) > 8 {
		return false
	}
	digits := true
	for _, r := range v {
		if r < '0' || r > '9' {
			digits = false
			break
		}
	}
	if digits && (len(v) == 4 || len(v) == 6 || len(v) == 8) {
		if len(v) == 4 && (strings.HasPrefix(v, "19") || strings.HasPrefix(v, "20")) {
			return false
		}
		return true
	}
	hasLetter, hasDigit := false, false
	for _, r := range strings.ToUpper(v) {
		hasLetter = hasLetter || (r >= 'A' && r <= 'Z')
		hasDigit = hasDigit || (r >= '0' && r <= '9')
	}
	return hasLetter && hasDigit
}
func HTMLToText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	doc, e := xhtml.Parse(strings.NewReader(raw))
	if e != nil {
		return ""
	}
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode {
			switch strings.ToLower(n.Data) {
			case "style", "script", "head", "noscript", "template", "svg":
				return
			}
		}
		if n.Type == xhtml.TextNode {
			_, _ = b.WriteString(n.Data)
			_ = b.WriteByte(' ')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.Join(strings.Fields(b.String()), " ")
}

func normalizeEmailText(textBody, htmlBody string) string {
	text := strings.TrimSpace(textBody)
	if html := strings.TrimSpace(htmlBody); html != "" {
		if htmlText := HTMLToText(html); htmlText != "" {
			text = htmlText
		}
	} else if looksLikeEmailHTML(text) {
		if htmlText := HTMLToText(text); htmlText != "" {
			text = htmlText
		}
	}
	text = stripEmbeddedEmailCSS(text)
	return strings.Join(strings.Fields(text), " ")
}

func looksLikeEmailHTML(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"<!doctype", "<html", "<body", "<head", "<style", "<table", "<div", "<span", "<p", "<br", "<td", "<a "} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func stripEmbeddedEmailCSS(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	lower := strings.ToLower(value)
	start := -1
	for _, marker := range []string{"@font-face", "@media", ".externalclass", "#outlook", "#bodytable", "#bodycell", "body {", "table {", "img {", "html {"} {
		if idx := strings.Index(lower, marker); idx >= 0 && (start < 0 || idx < start) {
			start = idx
		}
	}
	if start < 0 {
		return value
	}

	pos := start
	end := start
	consumed := false
	for pos < len(value) {
		for pos < len(value) {
			switch value[pos] {
			case ' ', '\t', '\r', '\n':
				pos++
			default:
				goto selector
			}
		}
	selector:
		if pos >= len(value) {
			break
		}
		openRel := strings.IndexByte(value[pos:], '{')
		if openRel < 0 || openRel > 1400 {
			break
		}
		open := pos + openRel
		selectorText := strings.TrimSpace(value[pos:open])
		if !looksLikeCSSSelector(selectorText) {
			break
		}
		depth := 0
		closeAt := -1
		for i := open; i < len(value); i++ {
			switch value[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					closeAt = i + 1
					i = len(value)
				}
			}
		}
		if closeAt < 0 {
			break
		}
		consumed = true
		end = closeAt
		pos = closeAt
	}
	if !consumed {
		return value
	}

	prefix := strings.TrimSpace(value[:start])
	suffix := strings.TrimSpace(value[end:])
	switch {
	case prefix == "":
		return suffix
	case suffix == "":
		return prefix
	default:
		return prefix + " " + suffix
	}
}

func looksLikeCSSSelector(selector string) bool {
	selector = strings.TrimSpace(selector)
	if selector == "" || len(selector) > 1400 {
		return false
	}
	lower := strings.ToLower(selector)
	if strings.HasPrefix(lower, "@") || strings.ContainsAny(selector, ".#[],:>+~*") {
		return true
	}
	first := lower
	if fields := strings.Fields(lower); len(fields) > 0 {
		first = fields[0]
	}
	switch strings.Trim(first, ",") {
	case "html", "body", "table", "tbody", "thead", "tr", "td", "th", "img", "a", "p", "div", "span", "font":
		return true
	default:
		return false
	}
}
func SanitizeEmailHTML(raw string) string {
	doc, e := xhtml.Parse(strings.NewReader(raw))
	if e != nil {
		return ""
	}
	allowed := map[string]bool{"html": true, "body": true, "p": true, "div": true, "span": true, "br": true, "strong": true, "b": true, "em": true, "i": true, "u": true, "ul": true, "ol": true, "li": true, "pre": true, "code": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "a": true}
	var clean func(*xhtml.Node)
	clean = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == xhtml.ElementNode && !allowed[strings.ToLower(c.Data)] {
				n.RemoveChild(c)
			} else {
				if c.Type == xhtml.ElementNode {
					attrs := c.Attr[:0]
					for _, a := range c.Attr {
						if strings.EqualFold(a.Key, "href") && (strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.Val)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.Val)), "https://")) {
							attrs = append(attrs, xhtml.Attribute{Key: "href", Val: a.Val})
						}
					}
					if strings.EqualFold(c.Data, "a") && len(attrs) > 0 {
						attrs = append(attrs, xhtml.Attribute{Key: "rel", Val: "noopener noreferrer"})
					}
					c.Attr = attrs
				}
				clean(c)
			}
			c = next
		}
	}
	clean(doc)
	var b bytes.Buffer
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		_ = xhtml.Render(&b, c)
	}
	return b.String()
}

func (s *EmailVerificationService) AdminProviders(ctx context.Context) ([]map[string]any, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT p.id,p.code,p.name,p.base_url,p.enabled,p.health_status,p.credential_ref,p.billing,p.metadata,COALESCE((SELECT COUNT(*) FROM email_provider_usage u WHERE u.provider_id=p.id AND u.created_at>=CURRENT_DATE),0),COALESCE((SELECT COUNT(*) FROM email_provider_usage u WHERE u.provider_id=p.id AND date_trunc('month',u.created_at)=date_trunc('month',NOW())),0) FROM email_providers p ORDER BY p.id`)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var code, name, base, health, cred string
		var enabled bool
		var billing, metadata []byte
		var today, month int64
		if e = rows.Scan(&id, &code, &name, &base, &enabled, &health, &cred, &billing, &metadata, &today, &month); e != nil {
			return nil, e
		}
		var metadataValues map[string]any
		_ = json.Unmarshal(metadata, &metadataValues)
		portalURL := ""
		if candidate, ok := metadataValues["portal_url"].(string); ok {
			if parsed, parseErr := url.Parse(strings.TrimSpace(candidate)); parseErr == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
				portalURL = parsed.String()
			}
		}
		out = append(out, map[string]any{"id": id, "code": code, "name": name, "base_url": base, "enabled": enabled, "health_status": health, "requires_credential": emailProviderRequiresCredential(code), "credential_configured": !emailProviderRequiresCredential(code) || emailProviderAPIKey(code, cred, s.encryptor) != "", "masked_hint": maskCredential(emailProviderAPIKey(code, cred, s.encryptor)), "billing": json.RawMessage(billing), "portal_url": portalURL, "today_requests": today, "month_requests": month})
	}
	return out, rows.Err()
}
func maskCredential(v string) string {
	if len(v) <= 4 {
		if v == "" {
			return ""
		}
		return "****"
	}
	return "****" + v[len(v)-4:]
}
func (s *EmailVerificationService) AdminUpdateProvider(ctx context.Context, id int64, enabled bool, base, cred string) error {
	return s.AdminUpdateProviderConfig(ctx, id, enabled, base, cred, nil)
}

func (s *EmailVerificationService) AdminUpdateProviderConfig(ctx context.Context, id int64, enabled bool, base, cred string, billing map[string]any) error {
	if id <= 0 {
		return errors.New("invalid email provider id")
	}
	base = strings.TrimSpace(base)
	if base != "" {
		parsed, err := url.Parse(base)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			return errors.New("invalid email provider base URL")
		}
	}
	cred = strings.TrimSpace(cred)
	if strings.HasPrefix(cred, "env:") {
		name := strings.TrimPrefix(cred, "env:")
		validName, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, name)
		if !validName {
			return errors.New("invalid email credential environment reference")
		}
	}
	if cred != "" && !strings.HasPrefix(cred, "env:") && !strings.HasPrefix(cred, "enc:") {
		if s.encryptor == nil {
			return errors.New("credential encryption is unavailable")
		}
		encrypted, err := s.encryptor.Encrypt(cred)
		if err != nil {
			return errors.New("credential encryption failed")
		}
		cred = "enc:" + encrypted
	}
	if billing != nil {
		billing = normalizeEmailBilling(billing)
		encoded, err := json.Marshal(billing)
		if err != nil {
			return err
		}
		_, e := s.db.ExecContext(ctx, `UPDATE email_providers SET enabled=$1,base_url=COALESCE(NULLIF($2,''),base_url),credential_ref=COALESCE(NULLIF($3,''),credential_ref),billing=$4::jsonb,updated_at=NOW() WHERE id=$5`, enabled, base, cred, encoded, id)
		return e
	}
	_, e := s.db.ExecContext(ctx, `UPDATE email_providers SET enabled=$1,base_url=COALESCE(NULLIF($2,''),base_url),credential_ref=COALESCE(NULLIF($3,''),credential_ref),updated_at=NOW() WHERE id=$4`, enabled, base, cred, id)
	return e
}

func normalizeEmailBilling(values map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range values {
		out[k] = v
	}
	for _, key := range []string{"monthly_fee", "included_requests_monthly", "included_requests_daily", "overage_price_per_request", "rate_limit_per_minute", "rate_limit_per_hour", "quota_reserved_for_active_orders", "manual_request_cost", "fixed_cost_per_order", "estimated_requests_per_order", "provider_concurrency"} {
		if _, ok := out[key]; ok {
			out[key] = math.Max(0, numberFromMap(out, key))
		}
	}
	for _, key := range []string{"quota_warning_percent", "quota_stop_percent"} {
		if _, ok := out[key]; ok {
			out[key] = math.Min(100, math.Max(0, numberFromMap(out, key)))
		}
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(out["cost_mode"])))
	switch mode {
	case "request_based_plus_amortized", "request_based", "monthly_amortized", "fixed_per_order", "manual":
		out["cost_mode"] = mode
	default:
		out["cost_mode"] = "request_based_plus_amortized"
	}
	return out
}

// AdminTestProvider performs a bounded authenticated provider contract check.
// Gmailnator allocates a disposable address when no dedicated health endpoint
// is configured, so the cooldown and usage meter prevent accidental churn.
func (s *EmailVerificationService) AdminTestProvider(ctx context.Context, id int64) (map[string]any, error) {
	now := time.Now()

	var code, base, cred string
	var metadataRaw, billingRaw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT code,base_url,credential_ref,metadata,billing FROM email_providers WHERE id=$1`, id).Scan(&code, &base, &cred, &metadataRaw, &billingRaw); err != nil {
		return nil, err
	}
	var metadata map[string]any
	_ = json.Unmarshal(metadataRaw, &metadata)
	var billing map[string]any
	_ = json.Unmarshal(billingRaw, &billing)
	if emailProviderRequiresCredential(code) && emailProviderAPIKey(code, cred, s.encryptor) == "" {
		return nil, ErrEmailProviderCredentialMissing
	}
	emailProviderTestMu.Lock()
	if previous, ok := emailProviderLastTests[id]; ok && now.Sub(previous) < time.Minute {
		emailProviderTestMu.Unlock()
		return nil, ErrEmailProviderTestCooldown
	}
	emailProviderLastTests[id] = now
	emailProviderTestMu.Unlock()
	provider := emailProviderFor(code, base, cred, metadata, s.encryptor, billing)
	if provider == nil {
		return nil, ErrEmailChannelUnavailable
	}
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	started := time.Now()
	err := provider.Health(testCtx)
	latency := time.Since(started)
	s.recordProviderUsage(ctx, id, 0, "health_check", err, latency)
	health := "healthy"
	if err != nil {
		health = "unavailable"
		var providerErr *EmailProviderHTTPError
		if errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusTooManyRequests {
			health = "quota_limited"
		}
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE email_providers SET health_status=$1,updated_at=NOW() WHERE id=$2`, health, id)
	if err != nil {
		return map[string]any{"healthy": false, "health_status": health, "latency_ms": latency.Milliseconds()}, err
	}
	return map[string]any{"healthy": true, "health_status": health, "latency_ms": latency.Milliseconds()}, nil
}

func (s *EmailVerificationService) AdminChannels(ctx context.Context) ([]map[string]any, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT c.id,c.code,c.public_name,p.code,c.enabled,c.visible,c.healthy,c.email_type,c.privacy_level,c.sale_price,c.refund_policy,c.capture_policy,c.order_ttl_seconds,c.polling_backoff,c.max_provider_requests_per_order,c.metadata FROM email_channels c JOIN email_providers p ON p.id=c.provider_id ORDER BY c.sort_order,c.id`)
	if e != nil {
		return nil, e
	}
	defer func() { _ = rows.Close() }()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var code, name, pc, emailType, privacy, refund, capture string
		var enabled, visible, healthy bool
		var price float64
		var ttl int
		var metadataRaw, backoffRaw []byte
		var maxRequests int
		if e = rows.Scan(&id, &code, &name, &pc, &enabled, &visible, &healthy, &emailType, &privacy, &price, &refund, &capture, &ttl, &backoffRaw, &maxRequests, &metadataRaw); e != nil {
			return nil, e
		}
		var metadata map[string]any
		_ = json.Unmarshal(metadataRaw, &metadata)
		var backoff []int
		_ = json.Unmarshal(backoffRaw, &backoff)
		out = append(out, map[string]any{"id": id, "code": code, "public_name": name, "provider_name": pc, "enabled": enabled, "visible": visible, "healthy": healthy, "email_type": emailType, "privacy_level": privacy, "sale_price": price, "refund_policy": refund, "capture_policy": capture, "order_ttl_seconds": ttl, "polling_backoff": backoff, "max_provider_requests_per_order": maxRequests, "base_markup": numberFromMap(metadata, "base_markup"), "fixed_markup": numberFromMap(metadata, "fixed_markup"), "minimum_profit": numberFromMap(metadata, "minimum_profit")})
	}
	return out, rows.Err()
}
func (s *EmailVerificationService) AdminUpdateChannel(ctx context.Context, id int64, enabled, visible, healthy bool, price float64, refund, capture string, ttl, maxRequests *int, backoff []int) error {
	if id <= 0 || math.IsNaN(price) || math.IsInf(price, 0) || price < 0 {
		return errors.New("invalid email channel configuration")
	}
	refund = strings.TrimSpace(refund)
	if refund != "" && refund != EmailRefundIfNoMessage && refund != EmailNoRefundAfterDelivery && refund != EmailManualReview {
		return errors.New("invalid email refund policy")
	}
	capture = strings.TrimSpace(capture)
	if capture != "" && capture != EmailCaptureOnTargetReceived && capture != EmailCaptureOnExtracted {
		return errors.New("invalid email capture policy")
	}
	if ttl != nil && (*ttl < 60 || *ttl > 86400) {
		return errors.New("email order TTL must be between 60 and 86400 seconds")
	}
	if maxRequests != nil && (*maxRequests < 1 || *maxRequests > 10000) {
		return errors.New("invalid email provider request limit")
	}
	if len(backoff) > 64 {
		return errors.New("email polling backoff has too many steps")
	}
	for _, seconds := range backoff {
		if seconds < 1 || seconds > 3600 {
			return errors.New("email polling backoff must contain 1-3600 second values")
		}
	}
	if ttl == nil && maxRequests == nil && backoff == nil {
		_, e := s.db.ExecContext(ctx, `UPDATE email_channels SET enabled=$1,visible=$2,healthy=$3,sale_price=CASE WHEN $4::numeric>=0 THEN $4::numeric ELSE sale_price END,refund_policy=COALESCE(NULLIF($5,''),refund_policy),capture_policy=COALESCE(NULLIF($6,''),capture_policy),updated_at=NOW() WHERE id=$7`, enabled, visible, healthy, price, refund, capture, id)
		return e
	}
	backoffJSON, _ := json.Marshal(backoff)
	_, e := s.db.ExecContext(ctx, `UPDATE email_channels SET enabled=$1,visible=$2,healthy=$3,sale_price=CASE WHEN $4::numeric>=0 THEN $4::numeric ELSE sale_price END,refund_policy=COALESCE(NULLIF($5,''),refund_policy),capture_policy=COALESCE(NULLIF($6,''),capture_policy),order_ttl_seconds=COALESCE(NULLIF($7::integer,0),order_ttl_seconds),max_provider_requests_per_order=COALESCE(NULLIF($8::integer,0),max_provider_requests_per_order),polling_backoff=CASE WHEN $9::jsonb='null'::jsonb THEN polling_backoff ELSE $9::jsonb END,updated_at=NOW() WHERE id=$10`, enabled, visible, healthy, price, refund, capture, emailIntValue(ttl), emailIntValue(maxRequests), backoffJSON, id)
	return e
}

func emailIntValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// AdminUpdateChannelPricing stores pricing controls in the existing channel
// metadata JSON so deployments do not need a second pricing table. A nil
// value leaves that control unchanged.
func (s *EmailVerificationService) AdminUpdateChannelPricing(ctx context.Context, id int64, baseMarkup, fixedMarkup, minimumProfit *float64) error {
	if baseMarkup == nil && fixedMarkup == nil && minimumProfit == nil {
		return nil
	}
	for _, value := range []*float64{baseMarkup, fixedMarkup, minimumProfit} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0) {
			return errors.New("invalid email channel pricing")
		}
	}
	var raw []byte
	if err := s.db.QueryRowContext(ctx, `SELECT metadata FROM email_channels WHERE id=$1`, id).Scan(&raw); err != nil {
		return err
	}
	metadata := map[string]any{}
	_ = json.Unmarshal(raw, &metadata)
	if baseMarkup != nil {
		metadata["base_markup"] = math.Max(0, *baseMarkup)
	}
	if fixedMarkup != nil {
		metadata["fixed_markup"] = math.Max(0, *fixedMarkup)
	}
	if minimumProfit != nil {
		metadata["minimum_profit"] = math.Max(0, *minimumProfit)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE email_channels SET metadata=$1,updated_at=NOW() WHERE id=$2`, encoded, id)
	return err
}
func (s *EmailVerificationService) AdminStats(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}
	out["feature_enabled"] = s.Enabled(ctx)
	var orders, completed, refunded int64
	var sales float64
	if e := s.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER(WHERE status='completed'),COUNT(*) FILTER(WHERE status='refunded'),COALESCE(SUM(captured_amount),0) FROM email_orders`).Scan(&orders, &completed, &refunded, &sales); e != nil {
		return nil, e
	}
	out["orders"] = orders
	out["completed"] = completed
	out["refunded"] = refunded
	out["sale_amount"] = sales
	var today, month, failed, limited int64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER(WHERE created_at>=CURRENT_DATE),COUNT(*) FILTER(WHERE date_trunc('month',created_at)=date_trunc('month',NOW())),COUNT(*) FILTER(WHERE NOT success),COUNT(*) FILTER(WHERE rate_limited) FROM email_provider_usage`).Scan(&today, &month, &failed, &limited)
	out["today_requests"] = today
	out["month_requests"] = month
	out["failed_requests"] = failed
	out["rate_limited_requests"] = limited
	var generate, polling, reads, deletes, avgLatency, usageOrders int64
	var estimatedCost float64
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE operation='generate_inbox'),COUNT(*) FILTER (WHERE operation='list_messages'),COUNT(*) FILTER (WHERE operation='get_message'),COUNT(*) FILTER (WHERE operation='delete_message'),COALESCE(AVG(latency_ms),0)::bigint,COUNT(DISTINCT email_order_id),COALESCE(SUM(estimated_request_cost),0) FROM email_provider_usage WHERE created_at>=date_trunc('month',NOW())`).Scan(&generate, &polling, &reads, &deletes, &avgLatency, &usageOrders, &estimatedCost)
	out["generate_requests"] = generate
	out["polling_requests"] = polling
	out["message_read_requests"] = reads
	out["delete_requests"] = deletes
	out["average_latency_ms"] = avgLatency
	out["requests_per_order"] = float64(month) / math.Max(1, float64(usageOrders))
	out["estimated_provider_cost"] = estimatedCost
	var billingRaw []byte
	_ = s.db.QueryRowContext(ctx, `SELECT billing FROM email_providers WHERE enabled ORDER BY id LIMIT 1`).Scan(&billingRaw)
	var billing map[string]any
	_ = json.Unmarshal(billingRaw, &billing)
	quotaLimit := numberFromMap(billing, "included_requests_monthly")
	if quotaLimit > 0 {
		out["current_quota_usage_percent"] = float64(month) * 100 / quotaLimit
		remainingRequests := math.Max(0, quotaLimit-float64(month))
		requestsPerOrder := float64(month) / math.Max(1, float64(usageOrders))
		out["estimated_remaining_orders"] = math.Floor(remainingRequests / math.Max(1, requestsPerOrder))
		out["estimated_remaining_requests"] = remainingRequests
	} else {
		out["current_quota_usage_percent"] = nil
		out["estimated_remaining_orders"] = nil
		out["estimated_remaining_requests"] = nil
	}
	return out, nil
}

// AdminOrders intentionally returns an operator view separate from the user
// DTO. Provider identifiers and cost telemetry are only available here.
func (s *EmailVerificationService) AdminOrders(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT o.public_id::text,o.order_no,o.user_id,sv.code,c.code,c.public_name,p.code,o.provider_inbox_id,o.email_address,o.address_type,o.status,o.sale_price_snapshot,o.provider_request_count,o.refund_status,o.created_at,o.expires_at FROM email_orders o JOIN email_services sv ON sv.id=o.service_id JOIN email_channels c ON c.id=o.channel_id JOIN email_providers p ON p.id=o.provider_id ORDER BY o.created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	orders := make([]map[string]any, 0)
	for rows.Next() {
		var id, orderNo, serviceCode, channelCode, channelName, providerCode, inbox, address, addressType, status, refundStatus string
		var userID int64
		var price float64
		var requestCount int
		var createdAt time.Time
		var expiresAt sql.NullTime
		if err := rows.Scan(&id, &orderNo, &userID, &serviceCode, &channelCode, &channelName, &providerCode, &inbox, &address, &addressType, &status, &price, &requestCount, &refundStatus, &createdAt, &expiresAt); err != nil {
			return nil, err
		}
		item := map[string]any{"id": id, "order_no": orderNo, "user_id": userID, "service_code": serviceCode, "channel_code": channelCode, "channel_name": channelName, "provider_code": providerCode, "provider_inbox_id": inbox, "email_address": address, "address_type": addressType, "status": status, "sale_price": price, "provider_request_count": requestCount, "refund_status": refundStatus, "created_at": createdAt}
		if expiresAt.Valid {
			item["expires_at"] = expiresAt.Time
		}
		orders = append(orders, item)
	}
	return orders, rows.Err()
}
