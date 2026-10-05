package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const (
	WorkspaceWebhookMaxEndpoints   = 10
	WebhookMaxAttempts             = 10
	WebhookResponsePreviewBytes    = 1024
	WebhookTestCooldown            = time.Minute
	WebhookWorkspaceTestsPerMinute = 10
)

var (
	ErrWebhookSecretUnavailable = errors.New("webhook secret encryption is not configured")
	ErrWebhookDeliveryNotFound  = ErrWorkspaceNotFound
	ErrWebhookDeliveryLeaseLost = errors.New("webhook delivery lease lost")
	ErrWebhookTestRateLimited   = infraerrors.TooManyRequests("WEBHOOK_TEST_RATE_LIMITED", "webhook test rate limit exceeded")
	reservedWebhookNetworks     = []*net.IPNet{
		mustWebhookCIDR("0.0.0.0/8"), mustWebhookCIDR("10.0.0.0/8"), mustWebhookCIDR("100.64.0.0/10"),
		mustWebhookCIDR("127.0.0.0/8"), mustWebhookCIDR("169.254.0.0/16"), mustWebhookCIDR("172.16.0.0/12"),
		mustWebhookCIDR("192.0.0.0/24"), mustWebhookCIDR("192.0.2.0/24"), mustWebhookCIDR("192.88.99.0/24"),
		mustWebhookCIDR("192.168.0.0/16"), mustWebhookCIDR("198.18.0.0/15"), mustWebhookCIDR("198.51.100.0/24"),
		mustWebhookCIDR("203.0.113.0/24"), mustWebhookCIDR("224.0.0.0/4"), mustWebhookCIDR("240.0.0.0/4"),
		mustWebhookCIDR("::/128"), mustWebhookCIDR("::1/128"),
		mustWebhookCIDR("64:ff9b::/96"), mustWebhookCIDR("64:ff9b:1::/48"), mustWebhookCIDR("100::/64"),
		mustWebhookCIDR("2001::/23"), mustWebhookCIDR("2001:db8::/32"), mustWebhookCIDR("2002::/16"),
		mustWebhookCIDR("3fff::/20"), mustWebhookCIDR("fc00::/7"), mustWebhookCIDR("fe80::/10"), mustWebhookCIDR("ff00::/8"),
	}
)

type WorkspaceWebhook struct {
	ID                  int64      `json:"id"`
	WorkspaceID         int64      `json:"workspace_id"`
	Name                string     `json:"name"`
	URL                 string     `json:"url"`
	Enabled             bool       `json:"enabled"`
	EventTypes          []string   `json:"event_types"`
	CreatedByUserID     int64      `json:"created_by_user_id"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DisabledAt          *time.Time `json:"disabled_at"`
	PreviousSecretUntil *time.Time `json:"previous_secret_until,omitempty"`
}

type WorkspaceWebhookDelivery struct {
	ID              int64      `json:"id"`
	WebhookID       int64      `json:"webhook_id"`
	EventID         string     `json:"event_id"`
	EventType       string     `json:"event_type"`
	Status          string     `json:"status"`
	Attempts        int        `json:"attempts"`
	NextAttemptAt   time.Time  `json:"next_attempt_at"`
	ResponseStatus  *int       `json:"response_status,omitempty"`
	ResponsePreview string     `json:"response_preview,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	ErrorCode       string     `json:"error_code,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	DeliveredAt     *time.Time `json:"delivered_at,omitempty"`
	LastAttemptAt   *time.Time `json:"last_attempt_at"`
}

type CreateWorkspaceWebhookInput struct {
	Name       string
	URL        string
	EventTypes []string
}

type UpdateWorkspaceWebhookInput struct {
	Name       *string
	URL        *string
	Enabled    *bool
	EventTypes *[]string
}

type WorkspaceWebhookRepository interface {
	ListWebhooks(context.Context, int64, int64) ([]WorkspaceWebhook, error)
	GetWebhook(context.Context, int64, int64, int64) (*WorkspaceWebhookRecord, error)
	CreateWebhook(context.Context, int64, int64, CreateWorkspaceWebhookInput, string, []string) (*WorkspaceWebhook, error)
	UpdateWebhook(context.Context, int64, int64, int64, UpdateWorkspaceWebhookInput, []string) (*WorkspaceWebhook, error)
	DeleteWebhook(context.Context, int64, int64, int64) error
	RotateWebhookSecret(context.Context, int64, int64, int64, string, []string, time.Time) (*WorkspaceWebhook, error)
	CreateTestDelivery(context.Context, int64, int64, int64, *DomainEvent) (*WorkspaceWebhookDelivery, error)
	ListDeliveries(context.Context, int64, int64, int64, int, int) ([]WorkspaceWebhookDelivery, int64, error)
	RetryDelivery(context.Context, int64, int64, int64, int64) (*WorkspaceWebhookDelivery, error)
	EnqueueEventDeliveries(context.Context, *DomainEvent) error
	ClaimDeliveries(context.Context, string, int, time.Duration) ([]WebhookDeliveryClaim, error)
	DeliveryEndpointActive(context.Context, int64, string) (bool, error)
	MarkDeliverySuccess(context.Context, int64, string, int, string) error
	MarkDeliveryRetry(context.Context, int64, string, time.Time, string, int, string) error
	MarkDeliveryDead(context.Context, int64, string, int, string, string) error
}

type WorkspaceWebhookRecord struct {
	WorkspaceWebhook
	SecretCurrentEncrypted  string `json:"-"`
	SecretPreviousEncrypted string `json:"-"`
}

type WebhookDeliveryClaim struct {
	WorkspaceWebhookDelivery
	WorkspaceID       int64
	URL               string
	SecretCurrent     string
	SecretPrevious    string
	PreviousExpiresAt *time.Time
	Payload           []byte
	LockOwner         string
}

type WorkspaceWebhookService struct {
	repo      WorkspaceWebhookRepository
	access    *WorkspaceAccessService
	encryptor SecretEncryptor
	keyReady  bool
}

func NewWorkspaceWebhookService(repo WorkspaceWebhookRepository, access *WorkspaceAccessService, encryptor SecretEncryptor, keyReady bool) *WorkspaceWebhookService {
	if access == nil {
		access = NewWorkspaceAccessService(nil)
	}
	return &WorkspaceWebhookService{repo: repo, access: access, encryptor: encryptor, keyReady: keyReady}
}

func ValidateWorkspaceWebhookURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.User != nil || parsed.Fragment != "" || strings.Contains(raw, "#") {
		return "", ErrWorkspaceInvalid
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil {
		// Reject mapped IPv6 literals, including those that embed a public IPv4
		// address. DNS A records remain ordinary IPv4 dial targets.
		if (strings.Contains(parsed.Hostname(), ":") && ip.To4() != nil) || !webhookIPAllowed(ip) {
			return "", ErrWorkspaceInvalid
		}
	}
	validated, err := urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{AllowPrivate: false})
	if err != nil {
		return "", ErrWorkspaceInvalid
	}
	return validated, nil
}

func webhookIPAllowed(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range reservedWebhookNetworks {
		if network.Contains(ip) {
			return false
		}
	}
	return ip.IsGlobalUnicast()
}

func mustWebhookCIDR(raw string) *net.IPNet {
	_, network, err := net.ParseCIDR(raw)
	if err != nil {
		panic(err)
	}
	return network
}

func SignWebhookPayload(secret string, timestamp time.Time, body []byte) string {
	message := fmt.Sprintf("%d.", timestamp.Unix())
	h := hmac.New(sha256.New, []byte(secret))
	_, _ = h.Write([]byte(message))
	_, _ = h.Write(body)
	return "t=" + fmt.Sprintf("%d", timestamp.Unix()) + ",v1=" + hex.EncodeToString(h.Sum(nil))
}

func SignWebhookPayloadWithPrevious(current, previous string, previousUntil, timestamp time.Time, body []byte) string {
	primary := SignWebhookPayload(current, timestamp, body)
	if strings.TrimSpace(previous) == "" || !previousUntil.After(timestamp) {
		return primary
	}
	secondary := SignWebhookPayload(previous, timestamp, body)
	return primary + "," + strings.TrimPrefix(secondary, "t="+fmt.Sprintf("%d", timestamp.Unix())+",")
}

func VerifyWebhookSignature(secret string, timestamp time.Time, body []byte, signature string) bool {
	parts := strings.Split(signature, ",")
	if len(parts) == 0 {
		return false
	}
	if strings.HasPrefix(parts[0], "t=") {
		if parts[0] != "t="+fmt.Sprintf("%d", timestamp.Unix()) {
			return false
		}
		parts = parts[1:]
	}
	expected := strings.TrimPrefix(SignWebhookPayload(secret, timestamp, body), "t="+fmt.Sprintf("%d", timestamp.Unix())+",")
	for _, part := range parts {
		if strings.HasPrefix(part, "v1=") && hmac.Equal([]byte(expected), []byte(part)) {
			return true
		}
	}
	return false
}

func ShouldRetryWebhookResponse(status int, err error) bool {
	if err != nil {
		return true
	}
	return status == 408 || status == 429 || status >= 500
}

func GenerateWebhookSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "whsec_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func normalizeWebhookEvents(events []string) ([]string, error) {
	if len(events) == 0 {
		return nil, ErrWorkspaceInvalid
	}
	seen := make(map[string]struct{}, len(events))
	out := make([]string, 0, len(events))
	for _, eventType := range events {
		eventType = strings.TrimSpace(eventType)
		if !IsWorkspaceVisibleEvent(eventType) {
			return nil, ErrWorkspaceInvalid
		}
		if _, ok := seen[eventType]; ok {
			continue
		}
		seen[eventType] = struct{}{}
		out = append(out, eventType)
	}
	if len(out) == 0 {
		return nil, ErrWorkspaceInvalid
	}
	return out, nil
}

func validateWorkspaceWebhookDNS(ctx context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrWorkspaceInvalid
	}
	pinnedCtx, err := urlvalidator.ResolveAndPinHost(ctx, u.Hostname())
	if err != nil {
		return ErrWorkspaceInvalid
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	addresses, pinned, err := urlvalidator.PinnedDialAddresses(pinnedCtx, net.JoinHostPort(u.Hostname(), port))
	if err != nil || !pinned || len(addresses) == 0 {
		return ErrWorkspaceInvalid
	}
	for _, address := range addresses {
		host, _, splitErr := net.SplitHostPort(address)
		if splitErr != nil || !webhookIPAllowed(net.ParseIP(host)) {
			return ErrWorkspaceInvalid
		}
	}
	return nil
}

func (s *WorkspaceWebhookService) Create(ctx context.Context, actorID, workspaceID int64, input CreateWorkspaceWebhookInput) (*WorkspaceWebhook, string, error) {
	if s == nil || s.repo == nil || !s.keyReady || s.encryptor == nil {
		return nil, "", ErrWebhookSecretUnavailable
	}
	urlValue, err := ValidateWorkspaceWebhookURL(input.URL)
	if err != nil {
		return nil, "", err
	}
	events, err := normalizeWebhookEvents(input.EventTypes)
	if err != nil || strings.TrimSpace(input.Name) == "" || len(input.Name) > 100 {
		return nil, "", ErrWorkspaceInvalid
	}
	if _, err = s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.create"); err != nil {
		return nil, "", err
	}
	if err = validateWorkspaceWebhookDNS(ctx, urlValue); err != nil {
		return nil, "", err
	}
	secret, err := GenerateWebhookSecret()
	if err != nil {
		return nil, "", err
	}
	encrypted, err := s.encryptor.Encrypt(secret)
	if err != nil {
		return nil, "", err
	}
	item, err := s.repo.CreateWebhook(ctx, actorID, workspaceID, CreateWorkspaceWebhookInput{Name: strings.TrimSpace(input.Name), URL: urlValue, EventTypes: events}, encrypted, events)
	return item, secret, err
}

func (s *WorkspaceWebhookService) List(ctx context.Context, actorID, workspaceID int64) ([]WorkspaceWebhook, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.read"); err != nil {
		return nil, err
	}
	return s.repo.ListWebhooks(ctx, actorID, workspaceID)
}

func (s *WorkspaceWebhookService) Update(ctx context.Context, actorID, workspaceID, webhookID int64, input UpdateWorkspaceWebhookInput) (*WorkspaceWebhook, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.update"); err != nil {
		return nil, err
	}
	if input.URL != nil {
		value, err := ValidateWorkspaceWebhookURL(*input.URL)
		if err != nil {
			return nil, err
		}
		if err = validateWorkspaceWebhookDNS(ctx, value); err != nil {
			return nil, err
		}
		input.URL = &value
	}
	if input.EventTypes != nil {
		events, err := normalizeWebhookEvents(*input.EventTypes)
		if err != nil {
			return nil, err
		}
		input.EventTypes = &events
	}
	if input.Name != nil && (strings.TrimSpace(*input.Name) == "" || len(*input.Name) > 100) {
		return nil, ErrWorkspaceInvalid
	}
	return s.repo.UpdateWebhook(ctx, actorID, workspaceID, webhookID, input, nil)
}

func (s *WorkspaceWebhookService) Delete(ctx context.Context, actorID, workspaceID, webhookID int64) error {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.delete"); err != nil {
		return err
	}
	return s.repo.DeleteWebhook(ctx, actorID, workspaceID, webhookID)
}

func (s *WorkspaceWebhookService) Rotate(ctx context.Context, actorID, workspaceID, webhookID int64) (*WorkspaceWebhook, string, error) {
	if s == nil || !s.keyReady || s.encryptor == nil {
		return nil, "", ErrWebhookSecretUnavailable
	}
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.secret.rotate"); err != nil {
		return nil, "", err
	}
	secret, err := GenerateWebhookSecret()
	if err != nil {
		return nil, "", err
	}
	encrypted, err := s.encryptor.Encrypt(secret)
	if err != nil {
		return nil, "", err
	}
	item, err := s.repo.RotateWebhookSecret(ctx, actorID, workspaceID, webhookID, encrypted, nil, time.Now().UTC().Add(24*time.Hour))
	return item, secret, err
}

func (s *WorkspaceWebhookService) Deliveries(ctx context.Context, actorID, workspaceID, webhookID int64, page, pageSize int) ([]WorkspaceWebhookDelivery, int64, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.delivery.read"); err != nil {
		return nil, 0, err
	}
	return s.repo.ListDeliveries(ctx, actorID, workspaceID, webhookID, page, pageSize)
}

func (s *WorkspaceWebhookService) Retry(ctx context.Context, actorID, workspaceID, webhookID, deliveryID int64) (*WorkspaceWebhookDelivery, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.delivery.retry"); err != nil {
		return nil, err
	}
	return s.repo.RetryDelivery(ctx, actorID, workspaceID, webhookID, deliveryID)
}

func (s *WorkspaceWebhookService) Test(ctx context.Context, actorID, workspaceID, webhookID int64) (*WorkspaceWebhookDelivery, error) {
	if _, err := s.access.RequireWorkspace(ctx, actorID, workspaceID, "webhook.test"); err != nil {
		return nil, err
	}
	event, err := NewDomainEvent(EventWebhookTest, workspaceID, 0, actorID, "webhook", fmt.Sprintf("%d", webhookID), DomainEventData{"category": "test"})
	if err != nil {
		return nil, err
	}
	return s.repo.CreateTestDelivery(ctx, actorID, workspaceID, webhookID, event)
}
