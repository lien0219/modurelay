package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// Domain event names are stable protocol identifiers. Their payload schema is
// versioned independently from the Go implementation.
const (
	EventVersion = 1

	EventServiceAccountCreated            = "service_account.created"
	EventServiceAccountUpdated            = "service_account.updated"
	EventServiceAccountDisabled           = "service_account.disabled"
	EventServiceAccountEnabled            = "service_account.enabled"
	EventServiceAccountCredentialCreated  = "service_account.credential.created"
	EventServiceAccountCredentialUpdated  = "service_account.credential.updated"
	EventServiceAccountCredentialRevoked  = "service_account.credential.revoked"
	EventServiceAccountCredentialRotated  = "service_account.credential.rotated"
	EventServiceAccountCredentialExpiring = "service_account.credential.expiring"
	EventServiceAccountCredentialExpired  = "service_account.credential.expired"

	EventWorkspaceCreated               = "workspace.created"
	EventWorkspaceUpdated               = "workspace.updated"
	EventWorkspaceSuspended             = "workspace.suspended"
	EventWorkspaceResumed               = "workspace.resumed"
	EventWorkspaceArchived              = "workspace.archived"
	EventMemberInvited                  = "member.invited"
	EventMemberJoined                   = "member.joined"
	EventMemberRoleChanged              = "member.role_changed"
	EventMemberSuspended                = "member.suspended"
	EventMemberRemoved                  = "member.removed"
	EventProjectCreated                 = "project.created"
	EventProjectUpdated                 = "project.updated"
	EventProjectArchived                = "project.archived"
	EventProjectRestored                = "project.restored"
	EventAPIKeyCreated                  = "api_key.created"
	EventAPIKeyUpdated                  = "api_key.updated"
	EventAPIKeyRevoked                  = "api_key.revoked"
	EventBudgetThreshold                = "budget.threshold_reached"
	EventBudgetSoftLimit                = "budget.soft_limit_exceeded"
	EventBudgetHardLimit                = "budget.hard_limit_reached"
	EventBudgetUpdated                  = "budget.updated"
	EventBillingPending                 = "billing.settlement_pending"
	EventBillingRecovered               = "billing.settlement_recovered"
	EventQuotaThreshold                 = "quota.threshold_reached"
	EventQuotaExhausted                 = "quota.exhausted"
	EventPolicyUpdated                  = "policy.updated"
	EventWebhookTest                    = "webhook.test"
	EventWorkspaceTeamCreated           = "workspace.team.created"
	EventWorkspaceTeamUpdated           = "workspace.team.updated"
	EventWorkspaceTeamArchived          = "workspace.team.archived"
	EventWorkspaceTeamMemberAdded       = "workspace.team.member_added"
	EventWorkspaceTeamMemberRemoved     = "workspace.team.member_removed"
	EventWorkspaceProjectAccessCreated  = "workspace.project_access.grant_created"
	EventWorkspaceProjectAccessUpdated  = "workspace.project_access.grant_updated"
	EventWorkspaceProjectAccessDeleted  = "workspace.project_access.grant_deleted"
	EventWorkspaceProjectAccessMode     = "workspace.project_access_mode.updated"
	EventWorkspaceDomainCreated         = "workspace.domain.created"
	EventWorkspaceDomainRegenerated     = "workspace.domain.regenerated"
	EventWorkspaceDomainVerified        = "workspace.domain.verified"
	EventWorkspaceDomainRevoked         = "workspace.domain.revoked"
	EventIdentityProviderCreated        = "workspace.identity_provider.created"
	EventIdentityProviderUpdated        = "workspace.identity_provider.updated"
	EventIdentityProviderDisabled       = "workspace.identity_provider.disabled"
	EventSAMLMetadataUpdated            = "workspace.saml.metadata.updated"
	EventSAMLCertificateRotated         = "workspace.saml.certificate.rotated"
	EventSSOEnforcementEnabled          = "workspace.sso.enforcement_enabled"
	EventSSOEnforcementDisabled         = "workspace.sso.enforcement_disabled"
	EventSSOBreakGlassUsed              = "workspace.sso.break_glass_used"
	EventWorkspaceSecurityPolicyUpdated = "workspace.security_policy.updated"
	EventOIDCJITProvisioned             = "workspace.member.jit_provisioned"
	EventOIDCIdentityLinked             = "workspace.identity.linked"
	EventOIDCMappingsUpdated            = "workspace.identity_provider.mappings_updated"
	EventOIDCRoleReconciled             = "workspace.identity.role_reconciled"
	EventOIDCTeamsReconciled            = "workspace.identity.teams_reconciled"
	EventSCIMConnectorCreated           = "workspace.scim.connector.created"
	EventSCIMConnectorDisabled          = "workspace.scim.connector.disabled"
	EventSCIMTokenCreated               = "workspace.scim.token.created"
	EventSCIMTokenRevoked               = "workspace.scim.token.revoked"
	EventSCIMSyncFailed                 = "workspace.scim.sync.failed"
	EventSCIMTokenExpiring              = "workspace.scim.token.expiring"
	EventSCIMSecurityConflict           = "workspace.scim.security.conflict"
)

var allowedDomainEventTypes = map[string]struct{}{
	EventServiceAccountCreated:            {},
	EventServiceAccountUpdated:            {},
	EventServiceAccountDisabled:           {},
	EventServiceAccountEnabled:            {},
	EventServiceAccountCredentialCreated:  {},
	EventServiceAccountCredentialUpdated:  {},
	EventServiceAccountCredentialRevoked:  {},
	EventServiceAccountCredentialRotated:  {},
	EventServiceAccountCredentialExpiring: {},
	EventServiceAccountCredentialExpired:  {},

	EventWorkspaceCreated: {}, EventWorkspaceUpdated: {}, EventWorkspaceSuspended: {},
	EventWorkspaceResumed: {}, EventWorkspaceArchived: {}, EventMemberInvited: {},
	EventMemberJoined: {}, EventMemberRoleChanged: {}, EventMemberSuspended: {},
	EventMemberRemoved: {}, EventProjectCreated: {}, EventProjectUpdated: {},
	EventProjectArchived: {}, EventProjectRestored: {}, EventAPIKeyCreated: {},
	EventAPIKeyUpdated: {}, EventAPIKeyRevoked: {}, EventBudgetThreshold: {},
	EventBudgetSoftLimit: {}, EventBudgetHardLimit: {}, EventBudgetUpdated: {},
	EventBillingPending: {}, EventBillingRecovered: {}, EventQuotaThreshold: {},
	EventQuotaExhausted: {}, EventPolicyUpdated: {}, EventWebhookTest: {},
	EventWorkspaceTeamCreated: {}, EventWorkspaceTeamUpdated: {}, EventWorkspaceTeamArchived: {},
	EventWorkspaceTeamMemberAdded: {}, EventWorkspaceTeamMemberRemoved: {},
	EventWorkspaceProjectAccessCreated: {}, EventWorkspaceProjectAccessUpdated: {}, EventWorkspaceProjectAccessDeleted: {},
	EventWorkspaceProjectAccessMode: {},
	EventWorkspaceDomainCreated:     {}, EventWorkspaceDomainRegenerated: {}, EventWorkspaceDomainVerified: {}, EventWorkspaceDomainRevoked: {},
	EventIdentityProviderCreated: {}, EventIdentityProviderUpdated: {}, EventIdentityProviderDisabled: {},
	EventSAMLMetadataUpdated: {}, EventSAMLCertificateRotated: {},
	EventSSOEnforcementEnabled: {}, EventSSOEnforcementDisabled: {}, EventSSOBreakGlassUsed: {},
	EventWorkspaceSecurityPolicyUpdated: {},
	EventOIDCJITProvisioned:             {}, EventOIDCIdentityLinked: {}, EventOIDCMappingsUpdated: {}, EventOIDCRoleReconciled: {}, EventOIDCTeamsReconciled: {},
	EventSCIMConnectorCreated: {}, EventSCIMConnectorDisabled: {}, EventSCIMTokenCreated: {}, EventSCIMTokenRevoked: {},
	EventSCIMSyncFailed: {}, EventSCIMTokenExpiring: {}, EventSCIMSecurityConflict: {},
}

type EventSubject struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

// DomainEventData is intentionally a map over a closed, documented set of
// scalar fields. New fields are additive; database entities and credentials
// must never be marshaled into this structure.
type DomainEventData map[string]any

var allowedDomainEventDataKeys = map[string]struct{}{
	"service_account_id": {}, "credential_id": {}, "credential_name": {}, "old_credential_id": {}, "new_credential_id": {}, "expires_at": {},
	"name": {}, "slug": {}, "status": {}, "role": {}, "user_id": {},
	"member_id": {}, "invitation_id": {}, "key_id": {}, "key_name": {},
	"project_id": {}, "workspace_id": {}, "scope_type": {}, "scope_id": {},
	"period_start": {}, "policy_revision": {}, "threshold": {}, "amount": {},
	"spent": {}, "reserved": {}, "estimated_amount": {}, "actual_amount": {},
	"reason_code": {}, "request_id": {}, "task_id": {}, "model": {},
	"platform": {}, "previous_status": {}, "delivery_id": {}, "category": {},
	"quota_type": {}, "period_end": {}, "used": {}, "limit": {},
	"team_id": {}, "grant_id": {}, "subject_type": {}, "project_access_mode": {},
	"domain_id": {}, "domain": {}, "normalized_domain": {}, "provider_id": {}, "provider_revision": {},
	"require_sso": {}, "role_source": {}, "source_provider_id": {}, "role_count": {}, "team_count": {}, "mapping_revision": {},
	"connector_id": {}, "token_id": {}, "resource_id": {}, "operation": {}, "added_count": {}, "removed_count": {}, "failure_count": {},
	"previous_revision": {}, "require_mfa": {}, "session_max_age_seconds": {}, "invitation_policy": {}, "allow_external_members": {}, "workspace_jit_enabled": {}, "approved_identity_provider_mode": {}, "changed_fields": {},
}

type DomainEvent struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Version     int             `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	WorkspaceID *int64          `json:"workspace_id,omitempty"`
	ProjectID   *int64          `json:"project_id,omitempty"`
	ActorUserID *int64          `json:"actor_user_id,omitempty"`
	Subject     EventSubject    `json:"subject"`
	Data        DomainEventData `json:"data"`
}

func NewDomainEvent(eventType string, workspaceID, projectID, actorUserID int64, subjectType, subjectID string, data DomainEventData) (*DomainEvent, error) {
	eventType = strings.TrimSpace(eventType)
	if _, ok := allowedDomainEventTypes[eventType]; !ok {
		return nil, errors.New("unsupported domain event type")
	}
	if strings.TrimSpace(subjectType) == "" || strings.TrimSpace(subjectID) == "" {
		return nil, errors.New("domain event subject is required")
	}
	cleanData, err := validateDomainEventData(data)
	if err != nil {
		return nil, err
	}
	rawID := make([]byte, 16)
	if _, err := rand.Read(rawID); err != nil {
		return nil, err
	}
	id := "evt_" + hex.EncodeToString(rawID)
	e := &DomainEvent{ID: id, Type: eventType, Version: EventVersion, CreatedAt: time.Now().UTC(), Subject: EventSubject{Type: strings.TrimSpace(subjectType), ID: strings.TrimSpace(subjectID)}, Data: cleanData}
	if workspaceID > 0 {
		e.WorkspaceID = &workspaceID
	}
	if projectID > 0 {
		e.ProjectID = &projectID
	}
	if actorUserID > 0 {
		e.ActorUserID = &actorUserID
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return e, nil
}

// Validate runs both before persistence and after decoding durable events. The
// allowlist is enforced at serialization too, so mutating a constructed event
// cannot introduce entity dumps or custom JSON marshalers into external data.
func (e *DomainEvent) Validate() error {
	if e == nil || !safeDomainEventID(e.ID) || e.Version != EventVersion || e.CreatedAt.IsZero() {
		return errors.New("invalid domain event")
	}
	if _, ok := allowedDomainEventTypes[e.Type]; !ok {
		return errors.New("unsupported domain event type")
	}
	if strings.TrimSpace(e.Subject.Type) == "" || strings.TrimSpace(e.Subject.ID) == "" || len(e.Subject.Type) > 64 || len(e.Subject.ID) > 256 {
		return errors.New("invalid domain event subject")
	}
	if (e.WorkspaceID != nil && *e.WorkspaceID <= 0) || (e.ProjectID != nil && (*e.ProjectID <= 0 || e.WorkspaceID == nil)) || (e.ActorUserID != nil && *e.ActorUserID <= 0) {
		return errors.New("invalid domain event scope")
	}
	_, err := validateDomainEventData(e.Data)
	return err
}

func validateDomainEventData(data DomainEventData) (DomainEventData, error) {
	clean := make(DomainEventData, len(data))
	for key, value := range data {
		if _, ok := allowedDomainEventDataKeys[key]; !ok {
			return nil, errors.New("unsupported domain event data field")
		}
		switch v := value.(type) {
		case nil:
			continue
		case string:
			if len(v) > 4096 {
				return nil, fmt.Errorf("domain event data field %s is too long", key)
			}
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		case float32:
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("domain event data field %s must be finite", key)
			}
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fmt.Errorf("domain event data field %s must be finite", key)
			}
		case json.Number:
			n, err := strconv.ParseFloat(string(v), 64)
			if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || !json.Valid([]byte(v)) {
				return nil, fmt.Errorf("domain event data field %s must be a number", key)
			}
		default:
			return nil, fmt.Errorf("domain event data field %s must be a scalar", key)
		}
		clean[key] = value
	}
	return clean, nil
}

func (e *DomainEvent) MarshalPayload() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}

func IsWorkspaceVisibleEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case EventServiceAccountCreated, EventServiceAccountUpdated, EventServiceAccountDisabled, EventServiceAccountEnabled, EventServiceAccountCredentialCreated, EventServiceAccountCredentialUpdated, EventServiceAccountCredentialRevoked, EventServiceAccountCredentialRotated, EventServiceAccountCredentialExpiring, EventServiceAccountCredentialExpired,
		EventWorkspaceCreated, EventWorkspaceUpdated, EventWorkspaceSuspended, EventWorkspaceResumed, EventWorkspaceArchived,
		EventMemberInvited, EventMemberJoined, EventMemberRoleChanged, EventMemberSuspended, EventMemberRemoved,
		EventProjectCreated, EventProjectUpdated, EventProjectArchived, EventProjectRestored,
		EventAPIKeyCreated, EventAPIKeyUpdated, EventAPIKeyRevoked, EventPolicyUpdated, EventBudgetThreshold, EventBudgetSoftLimit,
		EventBudgetHardLimit, EventBudgetUpdated, EventBillingPending, EventBillingRecovered, EventQuotaThreshold, EventQuotaExhausted,
		EventWebhookTest,
		EventWorkspaceTeamCreated, EventWorkspaceTeamUpdated, EventWorkspaceTeamArchived, EventWorkspaceTeamMemberAdded, EventWorkspaceTeamMemberRemoved,
		EventWorkspaceProjectAccessCreated, EventWorkspaceProjectAccessUpdated, EventWorkspaceProjectAccessDeleted, EventWorkspaceProjectAccessMode,
		EventWorkspaceDomainCreated, EventWorkspaceDomainRegenerated, EventWorkspaceDomainVerified, EventWorkspaceDomainRevoked,
		EventIdentityProviderCreated, EventIdentityProviderUpdated, EventIdentityProviderDisabled, EventOIDCMappingsUpdated,
		EventSAMLMetadataUpdated, EventSAMLCertificateRotated,
		EventSSOEnforcementEnabled, EventSSOEnforcementDisabled, EventSSOBreakGlassUsed, EventWorkspaceSecurityPolicyUpdated, EventOIDCJITProvisioned,
		EventOIDCIdentityLinked, EventOIDCRoleReconciled, EventOIDCTeamsReconciled,
		EventSCIMConnectorCreated, EventSCIMConnectorDisabled, EventSCIMTokenCreated, EventSCIMTokenRevoked,
		EventSCIMSyncFailed, EventSCIMTokenExpiring, EventSCIMSecurityConflict:
		return true
	default:
		return false
	}
}

type UserNotification struct {
	ID              int64          `json:"id"`
	EventID         string         `json:"event_id"`
	RecipientUserID int64          `json:"recipient_user_id"`
	WorkspaceID     *int64         `json:"workspace_id,omitempty"`
	ProjectID       *int64         `json:"project_id,omitempty"`
	Category        string         `json:"category"`
	TitleKey        string         `json:"title_key"`
	BodyKey         string         `json:"body_key"`
	Data            map[string]any `json:"data,omitempty"`
	ReadAt          *time.Time     `json:"read_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

type NotificationListFilter struct {
	Page        int
	PageSize    int
	Category    string
	Unread      *bool
	WorkspaceID *int64
	ProjectID   *int64
}

func (f NotificationListFilter) Normalized() NotificationListFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize <= 0 {
		f.PageSize = 20
	} else if f.PageSize > 100 {
		f.PageSize = 100
	}
	// Multiplication below must remain representable for SQL OFFSET.
	if f.Page > math.MaxInt/f.PageSize {
		f.Page = math.MaxInt / f.PageSize
	}
	f.Category = strings.TrimSpace(f.Category)
	return f
}

var ErrNotificationNotFound = infraerrors.NotFound("NOTIFICATION_NOT_FOUND", "notification not found")

type NotificationRepository interface {
	List(context.Context, int64, NotificationListFilter) ([]UserNotification, int64, error)
	UnreadCount(context.Context, int64, *int64, *int64) (int64, error)
	MarkRead(context.Context, int64, int64) error
	MarkAllRead(context.Context, int64, *int64, *int64) (int64, error)
	CreateForRecipients(context.Context, []UserNotification) error
}

// NotificationRecipientResolver is the single authority for mapping an
// immutable domain event to tenant users. Handlers and workers must not
// implement their own recipient queries.
type NotificationRecipientResolver interface {
	Resolve(context.Context, *DomainEvent) ([]int64, error)
}

type NotificationEventFanout struct {
	EventID         string
	RecipientUserID int64
	WorkspaceID     *int64
	ProjectID       *int64
	Category        string
	TitleKey        string
	BodyKey         string
	Data            map[string]any
}

// NotificationCenterService exposes the durable user inbox without coupling
// HTTP handlers to SQL or event fanout details.
type NotificationCenterService struct{ repo NotificationRepository }

func NewNotificationCenterService(repo NotificationRepository) *NotificationCenterService {
	return &NotificationCenterService{repo: repo}
}

func (s *NotificationCenterService) List(ctx context.Context, userID int64, f NotificationListFilter) ([]UserNotification, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, errors.New("notification repository is unavailable")
	}
	return s.repo.List(ctx, userID, f.Normalized())
}

func (s *NotificationCenterService) UnreadCount(ctx context.Context, userID int64, workspaceID, projectID *int64) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("notification repository is unavailable")
	}
	return s.repo.UnreadCount(ctx, userID, workspaceID, projectID)
}

func (s *NotificationCenterService) MarkRead(ctx context.Context, userID, id int64) error {
	if s == nil || s.repo == nil {
		return errors.New("notification repository is unavailable")
	}
	return s.repo.MarkRead(ctx, userID, id)
}

func (s *NotificationCenterService) MarkAllRead(ctx context.Context, userID int64, workspaceID, projectID *int64) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("notification repository is unavailable")
	}
	return s.repo.MarkAllRead(ctx, userID, workspaceID, projectID)
}

// DomainEventOutboxRecord is the leased dispatcher payload. LockToken is
// opaque and must be presented when acknowledging or retrying a delivery.
type DomainEventOutboxRecord struct {
	EventID     string
	EventType   string
	Payload     []byte
	Attempts    int
	LockToken   string
	LockedUntil time.Time
}

type DomainEventOutboxRepository interface {
	Claim(context.Context, int, time.Duration) ([]DomainEventOutboxRecord, error)
	Ack(context.Context, string, string) error
	Retry(context.Context, string, string, time.Duration, error) error
}

type DomainEventRetentionResult struct {
	Notifications     int64
	WebhookDeliveries int64
	Outbox            int64
	Events            int64
}

// Retention is optional for test/non-SQL outboxes. Production implements it on
// the outbox repository so the dispatcher lifecycle owns the periodic cleanup.
type DomainEventRetentionRepository interface {
	Cleanup(context.Context, int) (DomainEventRetentionResult, error)
}

var ErrDomainEventLeaseLost = errors.New("domain event outbox lease lost")

// DomainEventFailureCode exposes a bounded, safe diagnostic for database retry
// state and logs. Dependency errors can contain SQL parameters or endpoint
// credentials, so their raw Error text must never be persisted or logged here.
func DomainEventFailureCode(err error) string {
	var failure *domainEventDispatchError
	if errors.As(err, &failure) {
		return failure.code
	}
	if errors.Is(err, context.Canceled) {
		return "dispatch_canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "dispatch_timeout"
	}
	if errors.Is(err, ErrDomainEventLeaseLost) {
		return "outbox_lease_lost"
	}
	return "dispatch_failed"
}

type domainEventDispatchError struct {
	code  string
	cause error
}

func (e *domainEventDispatchError) Error() string { return e.code }
func (e *domainEventDispatchError) Unwrap() error { return e.cause }

func domainEventFailure(code string, cause error) error {
	return &domainEventDispatchError{code: code, cause: cause}
}

type DomainEventWebhookEnqueuer interface {
	EnqueueEventDeliveries(context.Context, *DomainEvent) error
}

const (
	domainEventDispatcherPollInterval = 500 * time.Millisecond
	domainEventDispatcherLease        = 2 * time.Minute
	domainEventDispatcherBatchSize    = 100
	domainEventRetentionInterval      = time.Hour
	domainEventRetentionBatchSize     = 500
)

// DomainEventDispatcher reliably fans out claimed events to both durable
// notification inbox rows and webhook delivery rows. A claim is acked only
// after all enabled consumers have accepted the immutable event.
type DomainEventDispatcher struct {
	outbox        DomainEventOutboxRepository
	notifications NotificationRepository
	recipients    NotificationRecipientResolver
	webhooks      DomainEventWebhookEnqueuer
	ctx           context.Context
	cancel        context.CancelFunc
	lifecycle     sync.Mutex
	started       bool
	stopped       bool
	log           *zap.Logger
	wg            sync.WaitGroup
}

func NewDomainEventDispatcher(outbox DomainEventOutboxRepository, notifications NotificationRepository, recipients NotificationRecipientResolver, webhooks DomainEventWebhookEnqueuer) *DomainEventDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &DomainEventDispatcher{outbox: outbox, notifications: notifications, recipients: recipients, webhooks: webhooks, ctx: ctx, cancel: cancel, log: logger.With(zap.String("component", "domain_event_dispatcher"))}
}

func (d *DomainEventDispatcher) Start() {
	if d == nil || d.outbox == nil || d.notifications == nil || d.recipients == nil {
		return
	}
	d.lifecycle.Lock()
	defer d.lifecycle.Unlock()
	if d.started || d.stopped {
		return
	}
	d.started = true
	d.wg.Add(1)
	go d.run()
}

func (d *DomainEventDispatcher) Stop() {
	if d == nil {
		return
	}
	d.lifecycle.Lock()
	d.stopped = true
	d.cancel()
	d.lifecycle.Unlock()
	d.wg.Wait()
}

func (d *DomainEventDispatcher) run() {
	defer d.wg.Done()
	ticker := time.NewTicker(domainEventDispatcherPollInterval)
	defer ticker.Stop()
	nextCleanup := time.Now()
	for {
		if err := d.ProcessBatch(d.ctx); err != nil && d.ctx.Err() == nil {
			d.log.Warn("domain event dispatch batch failed", zap.String("failure_code", DomainEventFailureCode(err)))
		}
		if time.Now().After(nextCleanup) && d.ctx.Err() == nil {
			if retention, ok := d.outbox.(DomainEventRetentionRepository); ok {
				cleanupCtx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
				counts, err := retention.Cleanup(cleanupCtx, domainEventRetentionBatchSize)
				cancel()
				if err != nil && d.ctx.Err() == nil {
					d.log.Warn("domain event retention cleanup failed", zap.String("failure_code", "retention_cleanup_failed"))
				} else if counts.Notifications+counts.WebhookDeliveries+counts.Outbox+counts.Events > 0 {
					d.log.Info("domain event retention cleanup completed", zap.Int64("notifications", counts.Notifications), zap.Int64("webhook_deliveries", counts.WebhookDeliveries), zap.Int64("outbox", counts.Outbox), zap.Int64("events", counts.Events))
				}
			}
			nextCleanup = time.Now().Add(domainEventRetentionInterval)
		}
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *DomainEventDispatcher) ProcessBatch(ctx context.Context) error {
	if d == nil || d.outbox == nil || d.notifications == nil || d.recipients == nil {
		return errors.New("domain event dispatcher is unavailable")
	}
	claimed, err := d.outbox.Claim(ctx, domainEventDispatcherBatchSize, domainEventDispatcherLease)
	if err != nil {
		return domainEventFailure("outbox_claim_failed", err)
	}
	var failures []error
	for _, item := range claimed {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, domainEventFailure("dispatch_canceled", err))...)
		}
		if err := d.processClaim(ctx, item); err != nil {
			delay := time.Duration(item.Attempts) * time.Second
			if delay < time.Second {
				delay = time.Second
			}
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
			if retryErr := d.outbox.Retry(ctx, item.EventID, item.LockToken, delay, err); retryErr != nil {
				return domainEventFailure("outbox_retry_failed", retryErr)
			}
			fields := []zap.Field{zap.Int("attempt", item.Attempts), zap.String("failure_code", DomainEventFailureCode(err))}
			if safeDomainEventID(item.EventID) {
				fields = append(fields, zap.String("event_id", item.EventID))
			}
			if _, ok := allowedDomainEventTypes[item.EventType]; ok {
				fields = append(fields, zap.String("event_type", item.EventType))
			}
			d.log.Warn("domain event consumer failed; retry scheduled", fields...)
			failures = append(failures, err)
			continue
		}
		if err := d.outbox.Ack(ctx, item.EventID, item.LockToken); err != nil {
			return domainEventFailure("outbox_ack_failed", err)
		}
	}
	return errors.Join(failures...)
}

func (d *DomainEventDispatcher) processClaim(ctx context.Context, item DomainEventOutboxRecord) error {
	var event DomainEvent
	decoder := json.NewDecoder(bytes.NewReader(item.Payload))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&event); err != nil {
		return domainEventFailure("event_decode_failed", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return domainEventFailure("event_decode_failed", errors.New("unexpected trailing event data"))
	}
	if event.ID != item.EventID || event.Type != item.EventType || event.Version != EventVersion {
		return domainEventFailure("event_invalid", errors.New("domain event envelope mismatch"))
	}
	if err := event.Validate(); err != nil {
		return domainEventFailure("event_invalid", err)
	}
	recipients, err := d.recipients.Resolve(ctx, &event)
	if err != nil {
		return domainEventFailure("notification_recipients_failed", err)
	}
	rows := make([]UserNotification, 0, len(recipients))
	for _, userID := range recipients {
		if userID <= 0 {
			continue
		}
		category, title, body := notificationPresentation(event.Type)
		data := make(map[string]any, len(event.Data)+2)
		for key, value := range event.Data {
			data[key] = value
		}
		data["event_type"] = event.Type
		data["event_version"] = event.Version
		rows = append(rows, UserNotification{EventID: event.ID, RecipientUserID: userID, WorkspaceID: event.WorkspaceID, ProjectID: event.ProjectID, Category: category, TitleKey: title, BodyKey: body, Data: data, CreatedAt: event.CreatedAt})
	}
	if len(rows) > 0 {
		if err := d.notifications.CreateForRecipients(ctx, rows); err != nil {
			return domainEventFailure("notification_fanout_failed", err)
		}
	}
	if d.webhooks != nil {
		if err := d.webhooks.EnqueueEventDeliveries(ctx, &event); err != nil {
			return domainEventFailure("webhook_enqueue_failed", err)
		}
	}
	return nil
}

func safeDomainEventID(id string) bool {
	if !strings.HasPrefix(id, "evt_") || len(id) <= 4 || len(id) > 128 {
		return false
	}
	for _, char := range id[4:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func notificationPresentation(eventType string) (category, titleKey, bodyKey string) {
	switch eventType {
	case EventWorkspaceSecurityPolicyUpdated:
		return "security", "notifications.workspaceSecurity.title", "notifications.workspaceSecurity.body"
	case EventSCIMConnectorDisabled, EventSCIMSyncFailed, EventSCIMTokenExpiring, EventSCIMSecurityConflict:
		return "security", "notifications.scim.title", "notifications.scim.body"
	case EventWorkspaceDomainVerified:
		return "security", "notifications.identity_domain.title", "notifications.identity_domain.body"
	case EventIdentityProviderDisabled:
		return "security", "notifications.identity_provider.title", "notifications.identity_provider.body"
	case EventSAMLCertificateRotated:
		return "security", "notifications.saml_certificate.title", "notifications.saml_certificate.body"
	case EventSSOEnforcementEnabled, EventSSOEnforcementDisabled:
		return "security", "notifications.sso_enforcement.title", "notifications.sso_enforcement.body"
	case EventSSOBreakGlassUsed:
		return "security", "notifications.sso_recovery.title", "notifications.sso_recovery.body"
	case EventBudgetThreshold, EventBudgetSoftLimit, EventBudgetHardLimit, EventBudgetUpdated:
		return "budget", "notifications.budget.title", "notifications.budget.body"
	case EventBillingPending, EventBillingRecovered:
		return "billing", "notifications.billing.title", "notifications.billing.body"
	case EventAPIKeyCreated, EventAPIKeyUpdated, EventAPIKeyRevoked:
		return "api_key", "notifications.api_key.title", "notifications.api_key.body"
	case EventMemberInvited, EventMemberJoined, EventMemberRoleChanged, EventMemberSuspended, EventMemberRemoved:
		return "workspace", "notifications.member.title", "notifications.member.body"
	case EventProjectCreated, EventProjectUpdated, EventProjectArchived, EventProjectRestored:
		return "project", "notifications.project.title", "notifications.project.body"
	case EventWorkspaceTeamCreated, EventWorkspaceTeamUpdated, EventWorkspaceTeamArchived, EventWorkspaceTeamMemberAdded, EventWorkspaceTeamMemberRemoved:
		return "workspace", "notifications.workspace.title", "notifications.workspace.body"
	case EventWorkspaceProjectAccessCreated, EventWorkspaceProjectAccessUpdated, EventWorkspaceProjectAccessDeleted, EventWorkspaceProjectAccessMode:
		return "project", "notifications.project.title", "notifications.project.body"
	case EventPolicyUpdated:
		return "policy", "notifications.policy.title", "notifications.policy.body"
	case EventQuotaThreshold, EventQuotaExhausted:
		return "quota", "notifications.quota.title", "notifications.quota.body"
	case EventWebhookTest:
		return "system", "notifications.webhook_test.title", "notifications.webhook_test.body"
	default:
		return "workspace", "notifications.workspace.title", "notifications.workspace.body"
	}
}

// ServiceAccountEventTypes includes the reserved optional expiration notices.
func ServiceAccountEventTypes() []string {
	return []string{EventServiceAccountCreated, EventServiceAccountUpdated, EventServiceAccountDisabled, EventServiceAccountEnabled, EventServiceAccountCredentialCreated, EventServiceAccountCredentialUpdated, EventServiceAccountCredentialRevoked, EventServiceAccountCredentialRotated, EventServiceAccountCredentialExpiring, EventServiceAccountCredentialExpired}
}
