package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// EnterpriseAdminRepository is optional: legacy WorkspaceRepository doubles
// and tenant authorization remain unchanged.
type EnterpriseAdminRepository interface {
	AdminSearchWorkspaces(context.Context, int64, AdminWorkspaceFilter) (*AdminWorkspacePage, error)
	AdminDiagnosticsOverview(context.Context, int64) (*AdminDiagnosticsOverview, error)
	AdminWorkspaceDiagnostics(context.Context, int64, int64) (*AdminWorkspaceDiagnostics, error)
	AdminOperationsJobs(context.Context, int64) (*AdminJobsDiagnostics, error)
	AdminOperateWorkspace(context.Context, int64, int64, AdminOperationInput) (*AdminOperationReceipt, error)
	AdminRetryWebhook(context.Context, int64, int64, int64, int64, AdminOperationInput) (*AdminOperationReceipt, error)
}

// AdminOperationInput is the complete, canonical request for a guarded
// administrator operation. Expected values are version fences read by the
// caller; the repository rechecks them while holding the target locks.
type AdminOperationInput struct {
	Action                string `json:"action"`
	Status                string `json:"status,omitempty"` // legacy status PATCH compatibility
	Reason                string `json:"reason"`
	Confirmation          string `json:"confirmation"`
	IdempotencyKey        string `json:"idempotency_key"`
	ExpectedUpdatedAt     string `json:"expected_updated_at,omitempty"`
	ExpectedAttempts      int    `json:"expected_attempts,omitempty"`
	ExpectedLastAttemptAt string `json:"expected_last_attempt_at,omitempty"`
}

// AdminOperationReceipt is a safe immutable result. It intentionally contains
// no audit metadata, request payload, credentials, or arbitrary errors.
type AdminOperationReceipt struct {
	ID              string    `json:"id"`
	Action          string    `json:"action"`
	TargetType      string    `json:"target_type"`
	TargetID        int64     `json:"target_id"`
	WorkspaceID     int64     `json:"workspace_id"`
	PreviousStatus  string    `json:"previous_status"`
	ResultStatus    string    `json:"result_status"`
	ResultUpdatedAt time.Time `json:"result_updated_at"`
	CreatedAt       time.Time `json:"created_at"`
}

func (in AdminOperationInput) CanonicalAction() string {
	a := strings.TrimSpace(in.Action)
	if a == "" {
		a = strings.TrimSpace(in.Status)
	}
	if a == "active" {
		a = "resume"
	}
	if a == "suspended" {
		a = "suspend"
	}
	return a
}

func (in AdminOperationInput) Validate() error {
	a := in.CanonicalAction()
	if a != "suspend" && a != "resume" && a != "retry_webhook" {
		return ErrWorkspaceInvalid
	}
	prefix := a + ":"
	if !strings.HasPrefix(in.Confirmation, prefix) {
		return ErrWorkspaceInvalid
	}
	confirmationID, parseConfirmationErr := strconv.ParseInt(strings.TrimPrefix(in.Confirmation, prefix), 10, 64)
	if parseConfirmationErr != nil || confirmationID <= 0 || strconv.FormatInt(confirmationID, 10) != strings.TrimPrefix(in.Confirmation, prefix) {
		return ErrWorkspaceInvalid
	}
	if strings.TrimSpace(in.Reason) == "" || utf8.RuneCountInString(in.Reason) > 500 || !utf8.ValidString(in.Reason) {
		return ErrWorkspaceInvalid
	}
	if strings.TrimSpace(in.Action) != "" && strings.TrimSpace(in.Status) != "" {
		compat := strings.TrimSpace(in.Status)
		switch compat {
		case "active":
			compat = "resume"
		case "suspended":
			compat = "suspend"
		}
		if strings.TrimSpace(in.Action) != compat {
			return ErrWorkspaceInvalid
		}
	}
	for _, r := range in.Reason {
		if unicode.IsControl(r) {
			return ErrWorkspaceInvalid
		}
	}
	if strings.TrimSpace(in.IdempotencyKey) != in.IdempotencyKey {
		return ErrWorkspaceInvalid
	}
	u, err := uuid.Parse(in.IdempotencyKey)
	if err != nil || u == uuid.Nil || u.String() != in.IdempotencyKey {
		return ErrWorkspaceInvalid
	}
	if in.ExpectedUpdatedAt != "" {
		if len(in.ExpectedUpdatedAt) > 40 {
			return ErrWorkspaceInvalid
		}
		if _, err := time.Parse(time.RFC3339Nano, in.ExpectedUpdatedAt); err != nil {
			return ErrWorkspaceInvalid
		}
	}
	if in.ExpectedAttempts < 0 || len(in.ExpectedLastAttemptAt) > 40 {
		return ErrWorkspaceInvalid
	}
	if in.ExpectedLastAttemptAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, in.ExpectedLastAttemptAt); err != nil {
			return ErrWorkspaceInvalid
		}
	}
	return nil
}

func (in AdminOperationInput) Fingerprint(targetType string, targetID, workspaceID int64) (string, error) {
	return in.fingerprint(targetType, targetID, workspaceID, 0)
}

func (in AdminOperationInput) WebhookFingerprint(workspaceID, webhookID, deliveryID int64) (string, error) {
	return in.fingerprint("webhook_delivery", deliveryID, workspaceID, webhookID)
}

func (in AdminOperationInput) fingerprint(targetType string, targetID, workspaceID, webhookID int64) (string, error) {
	if err := in.Validate(); err != nil {
		return "", err
	}
	v := struct {
		Action                string `json:"action"`
		Status                string `json:"status"`
		Reason                string `json:"reason"`
		Confirmation          string `json:"confirmation"`
		TargetType            string `json:"target_type"`
		TargetID              int64  `json:"target_id"`
		WorkspaceID           int64  `json:"workspace_id"`
		WebhookID             int64  `json:"webhook_id,omitempty"`
		ExpectedUpdatedAt     string `json:"expected_updated_at"`
		ExpectedAttempts      int    `json:"expected_attempts"`
		ExpectedLastAttemptAt string `json:"expected_last_attempt_at"`
	}{in.CanonicalAction(), "", in.Reason, in.Confirmation, targetType, targetID, workspaceID, webhookID, in.ExpectedUpdatedAt, in.ExpectedAttempts, in.ExpectedLastAttemptAt}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

var ErrAdminOperationIdempotencyConflict = errors.New("administrator operation idempotency key conflicts with a different request")

type AdminWorkspaceFilter struct {
	WorkspaceID        int64  `json:"workspace_id"`
	OwnerUserID        int64  `json:"owner_user_id"`
	BillingOwnerUserID int64  `json:"billing_owner_user_id"`
	NamePrefix         string `json:"name_prefix"`
	SlugPrefix         string `json:"slug_prefix"`
	Status             string `json:"status"`
	Type               string `json:"type"`
	CreatedFrom        string `json:"created_from"`
	CreatedTo          string `json:"created_to"`
	UpdatedFrom        string `json:"updated_from"`
	UpdatedTo          string `json:"updated_to"`
	Sort               string `json:"sort"`
	Direction          string `json:"direction"`
	Page               int    `json:"page"`
	PageSize           int    `json:"page_size"`
}

func (f AdminWorkspaceFilter) Normalized() AdminWorkspaceFilter {
	if f.Page == 0 {
		f.Page = 1
	}
	if f.PageSize == 0 {
		f.PageSize = 20
	}
	if f.Sort == "" {
		f.Sort = "id"
	}
	if f.Direction == "" {
		f.Direction = "desc"
	}
	return f
}
func (f AdminWorkspaceFilter) Validate() error {
	f = f.Normalized()
	if f.WorkspaceID < 0 || f.OwnerUserID < 0 || f.BillingOwnerUserID < 0 || f.Page < 1 || f.PageSize < 1 || f.PageSize > 100 || f.Page > 10001 || int64(f.Page-1)*int64(f.PageSize) > 10000 {
		return ErrWorkspaceInvalid
	}
	for _, p := range []string{f.NamePrefix, f.SlugPrefix} {
		if !utf8.ValidString(p) || utf8.RuneCountInString(p) > 100 || strings.ContainsRune(p, 0) {
			return ErrWorkspaceInvalid
		}
	}
	if !adminAllowed(f.Status, "", "active", "suspended", "archived", "pending_deletion", "purging", "deleted") || !adminAllowed(f.Type, "", "personal", "organization") || !adminAllowed(f.Sort, "id", "name", "slug", "created_at", "updated_at") || !adminAllowed(f.Direction, "asc", "desc") {
		return ErrWorkspaceInvalid
	}
	for _, pair := range [][2]string{{f.CreatedFrom, f.CreatedTo}, {f.UpdatedFrom, f.UpdatedTo}} {
		var parsed [2]time.Time
		for i, s := range pair {
			if s != "" {
				if len(s) > 40 {
					return ErrWorkspaceInvalid
				}
				v, e := time.Parse(time.RFC3339Nano, s)
				if e != nil || v.Year() < 1 || v.Year() > 9999 {
					return ErrWorkspaceInvalid
				}
				parsed[i] = v
			}
		}
		if pair[0] != "" && pair[1] != "" && (parsed[1].Before(parsed[0]) || parsed[1].Sub(parsed[0]) > 366*24*time.Hour) {
			return ErrWorkspaceInvalid
		}
	}
	return nil
}
func adminAllowed(v string, values ...string) bool {
	for _, x := range values {
		if v == x {
			return true
		}
	}
	return false
}
func adminEscapedPrefix(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(s)) + "%"
}
func (f AdminWorkspaceFilter) EscapedNamePrefix() string { return adminEscapedPrefix(f.NamePrefix) }
func (f AdminWorkspaceFilter) EscapedSlugPrefix() string { return adminEscapedPrefix(f.SlugPrefix) }

type AdminWorkspacePage struct {
	Items       []Workspace `json:"items"`
	Total       int64       `json:"total"`
	Page        int         `json:"page"`
	PageSize    int         `json:"page_size"`
	TotalPages  int64       `json:"total_pages"`
	TotalCapped bool        `json:"total_capped"`
	ObservedAt  time.Time   `json:"observed_at"`
}

type AdminCount struct {
	Value      *int64    `json:"value"`
	Available  bool      `json:"available"`
	Capped     bool      `json:"capped"`
	ObservedAt time.Time `json:"observed_at"`
	Source     string    `json:"source"`
	Coverage   string    `json:"coverage"`
	Freshness  string    `json:"freshness"`
}
type AdminIssue struct {
	Code              string    `json:"code"`
	Severity          string    `json:"severity"`
	Scope             string    `json:"scope"`
	ObservedAt        time.Time `json:"observed_at"`
	Reason            string    `json:"reason"`
	RecommendedAction string    `json:"recommended_action"`
	Runbook           string    `json:"runbook"`
}
type AdminJobSummary struct {
	ID          string     `json:"id"`
	WorkspaceID int64      `json:"workspace_id"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	// Only the webhook terminal dead state has an actual failure clock.
	FailedAt         *time.Time     `json:"failed_at"`
	FailureCode      string         `json:"failure_code"`
	WebhookID        *int64         `json:"webhook_id,omitempty"`
	DeliveryID       *int64         `json:"delivery_id,omitempty"`
	Attempts         *int           `json:"attempts,omitempty"`
	LastAttemptAt    *time.Time     `json:"last_attempt_at,omitempty"`
	EndpointEnabled  *bool          `json:"endpoint_enabled,omitempty"`
	RetryEligibility string         `json:"retry_eligibility,omitempty"`
	Phase            string         `json:"phase,omitempty"`
	Progress         *int64         `json:"progress,omitempty"`
	Blockers         []AdminBlocker `json:"blockers,omitempty"`
	SourceTime       *time.Time     `json:"source_time,omitempty"`
	ObservedAt       time.Time      `json:"observed_at"`
	Freshness        string         `json:"freshness"`
}
type AdminTimeEvidence struct {
	Value      *time.Time `json:"value"`
	AgeSeconds *int64     `json:"age_seconds"`
	Available  bool       `json:"available"`
	ObservedAt time.Time  `json:"observed_at"`
	Source     string     `json:"source"`
	Coverage   string     `json:"coverage"`
	Freshness  string     `json:"freshness"`
}
type AdminFailureEvidence struct {
	Code       *string    `json:"code"`
	SourceTime *time.Time `json:"source_time"`
	Available  bool       `json:"available"`
	ObservedAt time.Time  `json:"observed_at"`
	Source     string     `json:"source"`
	Coverage   string     `json:"coverage"`
	Freshness  string     `json:"freshness"`
}
type AdminBlocker struct {
	Code       string      `json:"code"`
	Reason     string      `json:"reason"`
	Count      *AdminCount `json:"count,omitempty"`
	ObservedAt time.Time   `json:"observed_at"`
	SourceTime *time.Time  `json:"source_time"`
	Source     string      `json:"source"`
	Coverage   string      `json:"coverage"`
	Freshness  string      `json:"freshness"`
	Runbook    string      `json:"runbook"`
}
type AdminPurgeDiagnostics struct {
	State      string         `json:"state"`
	Available  bool           `json:"available"`
	ObservedAt time.Time      `json:"observed_at"`
	Coverage   string         `json:"coverage"`
	Blockers   []AdminBlocker `json:"blockers"`
	Issues     []AdminIssue   `json:"issues"`
}
type AdminIdentityConnection struct {
	ID                  int64        `json:"id"`
	WorkspaceID         int64        `json:"workspace_id"`
	Type                string       `json:"type"`
	Status              string       `json:"status"`
	State               string       `json:"state"`
	ObservedAt          time.Time    `json:"observed_at"`
	Source              string       `json:"source"`
	Freshness           string       `json:"freshness"`
	LastValidatedAt     *time.Time   `json:"last_validated_at"`
	LastValidationCode  string       `json:"last_validation_code"`
	LastSyncAt          *time.Time   `json:"last_sync_at"`
	LastErrorCode       string       `json:"last_error_code"`
	ConsecutiveFailures *int64       `json:"consecutive_failures"`
	Coverage            string       `json:"coverage"`
	Issues              []AdminIssue `json:"issues"`
}
type AdminIdentitySection struct {
	State      string                    `json:"state"`
	Available  bool                      `json:"available"`
	Capped     bool                      `json:"capped"`
	ObservedAt time.Time                 `json:"observed_at"`
	Source     string                    `json:"source"`
	Coverage   string                    `json:"coverage"`
	Freshness  string                    `json:"freshness"`
	Items      []AdminIdentityConnection `json:"items"`
	Counts     map[string]AdminCount     `json:"counts"`
	Issues     []AdminIssue              `json:"issues"`
}
type AdminIdentityDiagnostics struct {
	Providers AdminIdentitySection `json:"providers"`
	SCIM      AdminIdentitySection `json:"scim"`
}
type AdminWorkerDiagnostics struct {
	Worker               string                `json:"worker"`
	State                string                `json:"state"`
	Liveness             string                `json:"liveness"`
	ObservedAt           time.Time             `json:"observed_at"`
	Source               string                `json:"source"`
	Coverage             string                `json:"coverage"`
	Freshness            string                `json:"freshness"`
	Counts               map[string]AdminCount `json:"counts"`
	DueLagSeconds        *int64                `json:"due_lag_seconds"`
	OldestPending        AdminTimeEvidence     `json:"oldest_pending"`
	PendingAlertAge      AdminTimeEvidence     `json:"pending_alert_age"`
	LastFailure          AdminFailureEvidence  `json:"last_failure"`
	LatestFailureTime    AdminTimeEvidence     `json:"latest_failure_time"`
	StoredScanLagSeconds *int64                `json:"stored_scan_lag_seconds,omitempty"`
	LastSuccessfulJobAt  *time.Time            `json:"last_successful_job_at"`
	LastSuccessfulScanAt *time.Time            `json:"last_successful_scan_at"`
	LastFailedAt         *time.Time            `json:"last_failed_at"`
	Jobs                 []AdminJobSummary     `json:"jobs"`
	JobsCapped           bool                  `json:"jobs_capped"`
	Issues               []AdminIssue          `json:"issues"`
	Runbook              string                `json:"runbook"`
}
type AdminFailureBucket struct {
	Hour  time.Time  `json:"hour"`
	Count AdminCount `json:"count"`
}
type AdminInstanceCapabilities struct {
	Available     bool   `json:"available"`
	Scope         string `json:"scope"`
	ExportEnabled bool   `json:"export_enabled"`
	PurgeEnabled  bool   `json:"purge_enabled"`
	KeyAvailable  bool   `json:"key_available"`
}
type AdminExportRotation struct {
	State             string                    `json:"state"`
	ObservedAt        time.Time                 `json:"observed_at"`
	RotationCertified bool                      `json:"rotation_certified"`
	Format            string                    `json:"format"`
	SingleKeyNoID     bool                      `json:"single_key_no_id"`
	Capabilities      AdminInstanceCapabilities `json:"capabilities"`
	Counts            map[string]AdminCount     `json:"counts"`
	Coverage          string                    `json:"coverage"`
	Prerequisites     []string                  `json:"prerequisites"`
	Issues            []AdminIssue              `json:"issues"`
	Runbook           string                    `json:"runbook"`
}
type AdminJobsDiagnostics struct {
	State               string                   `json:"state"`
	ObservedAt          time.Time                `json:"observed_at"`
	Workers             []AdminWorkerDiagnostics `json:"workers"`
	WebhookFailureTrend []AdminFailureBucket     `json:"webhook_failure_trend"`
	ExportRotation      AdminExportRotation      `json:"export_rotation"`
	Issues              []AdminIssue             `json:"issues"`
}
type AdminDiagnosticsOverview struct {
	State                 string                   `json:"state"`
	ObservedAt            time.Time                `json:"observed_at"`
	Counts                map[string]AdminCount    `json:"counts"`
	Jobs                  *AdminJobsDiagnostics    `json:"jobs"`
	Issues                []AdminIssue             `json:"issues"`
	Identity              AdminIdentityDiagnostics `json:"identity"`
	WorkerExceptions      AdminCount               `json:"worker_exceptions"`
	UnknownWorkerLiveness AdminCount               `json:"unknown_worker_liveness"`
}
type AdminSecuritySummary struct {
	RequireSSO                   bool   `json:"require_sso"`
	RequireMFA                   bool   `json:"require_mfa"`
	SessionMaxAgeSeconds         *int   `json:"session_max_age_seconds"`
	InvitationPolicy             string `json:"invitation_policy"`
	AllowExternalMembers         bool   `json:"allow_external_members"`
	WorkspaceJITEnabled          bool   `json:"workspace_jit_enabled"`
	ApprovedIdentityProviderMode string `json:"approved_identity_provider_mode"`
}
type AdminAuditSummary struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   *int64    `json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type AdminRetentionSummary struct {
	Category      string `json:"category"`
	MinimumDays   int    `json:"minimum_days"`
	EffectiveDays int    `json:"effective_days"`
	Protected     bool   `json:"protected"`
}
type AdminWorkspaceDiagnostics struct {
	State             string                   `json:"state"`
	ObservedAt        time.Time                `json:"observed_at"`
	Workspace         *Workspace               `json:"workspace"`
	Counts            map[string]AdminCount    `json:"counts"`
	BillingOwnerValid *bool                    `json:"billing_owner_valid"`
	OwnerValid        *bool                    `json:"owner_valid"`
	AllowedOperations []string                 `json:"allowed_operations"`
	Identity          AdminIdentityDiagnostics `json:"identity"`
	Purge             AdminPurgeDiagnostics    `json:"purge"`
	SecurityPolicy    *AdminSecuritySummary    `json:"security_policy"`
	Retention         []AdminRetentionSummary  `json:"retention"`
	RecentAudit       []AdminAuditSummary      `json:"recent_audit"`
	Jobs              *AdminJobsDiagnostics    `json:"jobs"`
	Issues            []AdminIssue             `json:"issues"`
}

func AdminEvidenceFreshness(at *time.Time, now time.Time, maxAge time.Duration) string {
	if at == nil || at.IsZero() || at.After(now) {
		return "unknown"
	}
	if now.Sub(*at) > maxAge {
		return "stale"
	}
	return "fresh"
}
