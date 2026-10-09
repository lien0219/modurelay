package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAdminOperationInputRejectsUnicodeControlReason(t *testing.T) {
	in := AdminOperationInput{
		Action: "suspend", Reason: "operator\u0085note", Confirmation: "suspend:9", IdempotencyKey: uuid.New().String(),
		ExpectedUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := in.Validate(); !errors.Is(err, ErrWorkspaceInvalid) {
		t.Fatalf("expected control character rejection, got %v", err)
	}
}

func TestAdminOperationInputStatusCompatibilityAndCanonicalFingerprint(t *testing.T) {
	key := uuid.New().String()
	in := AdminOperationInput{Status: "suspended", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: key}
	if got := in.CanonicalAction(); got != "suspend" {
		t.Fatalf("canonical action = %q", got)
	}
	a, err := in.Fingerprint("workspace", 9, 9)
	if err != nil {
		t.Fatal(err)
	}
	canonical := in
	canonical.Action, canonical.Status = "suspend", ""
	b, err := canonical.Fingerprint("workspace", 9, 9)
	if err != nil || a != b {
		t.Fatalf("status alias and canonical fingerprint differ: %q %q %v", a, b, err)
	}
}

func TestAdminOperateWorkspaceRequiresTrustedHumanSession(t *testing.T) {
	s := NewWorkspaceService(nil)
	in := AdminOperationInput{Action: "suspend", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: uuid.New().String(), ExpectedUpdatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	_, err := s.AdminOperateWorkspace(context.Background(), 1, 9, in)
	if !errors.Is(err, ErrRecentAuthenticationRequired) {
		t.Fatalf("missing session proof error = %v", err)
	}
}

func TestAdminOperateWorkspaceRejectsMachineSessionEvenWithRecentProof(t *testing.T) {
	now := time.Now().UTC()
	ctx := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: "api_key", AuthenticatedAt: now})
	ctx = WithRecentAuthentication(ctx, now, true)
	s := NewWorkspaceService(nil)
	in := AdminOperationInput{Action: "suspend", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: uuid.New().String(), ExpectedUpdatedAt: now.Format(time.RFC3339Nano)}
	_, err := s.AdminOperateWorkspace(ctx, 1, 9, in)
	if !errors.Is(err, ErrRecentAuthenticationRequired) {
		t.Fatalf("machine session error = %v", err)
	}
}

func TestAdminOperateWorkspaceEnrolledSessionNeedsMFAProof(t *testing.T) {
	now := time.Now().UTC()
	ctx := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now, MFAEnrolled: true})
	ctx = WithRecentAuthentication(ctx, now, false)
	repo := &adminOperationGuardRepository{}
	s := NewWorkspaceService(repo)
	in := AdminOperationInput{Action: "suspend", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: uuid.New().String(), ExpectedUpdatedAt: now.Format(time.RFC3339Nano)}
	_, err := s.AdminOperateWorkspace(ctx, 1, 9, in)
	require.ErrorIs(t, err, ErrRecentAuthenticationRequired)
	require.Zero(t, repo.calls, "enrollment is never accepted as proof at the service boundary")
}

type adminOperationGuardRepository struct {
	WorkspaceRepository
	EnterpriseAdminRepository
	calls int
}

func (r *adminOperationGuardRepository) AdminOperateWorkspace(context.Context, int64, int64, AdminOperationInput) (*AdminOperationReceipt, error) {
	r.calls++
	return &AdminOperationReceipt{ID: "checked"}, nil
}

func (r *adminOperationGuardRepository) AdminRetryWebhook(context.Context, int64, int64, int64, int64, AdminOperationInput) (*AdminOperationReceipt, error) {
	r.calls++
	return &AdminOperationReceipt{ID: "checked"}, nil
}

func TestAdminOperationsServiceRejectsInvalidProofBeforeRepository(t *testing.T) {
	now := time.Now().UTC()
	for _, tc := range []struct {
		name    string
		auth    SessionAuthentication
		proofAt time.Time
		mfa     bool
	}{
		{"missing-session", SessionAuthentication{}, now, true},
		{"machine", SessionAuthentication{AuthMethod: "api_key", AuthenticatedAt: now}, now, true},
		{"enrollment-only", SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now, MFAEnrolled: true}, now, false},
		{"future-proof", SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-time.Hour)}, now.Add(2 * time.Minute), true},
		{"expired-proof", SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-time.Hour)}, now.Add(-11 * time.Minute), true},
		{"missing-proof", SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-time.Hour)}, time.Time{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := WithRecentAuthentication(WithSessionAuthentication(context.Background(), tc.auth), tc.proofAt, tc.mfa)
			repo := &adminOperationGuardRepository{}
			s := NewWorkspaceService(repo)
			in := AdminOperationInput{Action: "suspend", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: uuid.NewString(), ExpectedUpdatedAt: now.Format(time.RFC3339Nano)}
			_, err := s.AdminOperateWorkspace(ctx, 1, 9, in)
			require.ErrorIs(t, err, ErrRecentAuthenticationRequired)
			in.Action, in.Confirmation = "retry_webhook", "retry_webhook:10"
			_, err = s.AdminRetryWebhook(ctx, 1, 9, 2, 10, in)
			require.ErrorIs(t, err, ErrRecentAuthenticationRequired)
			require.Zero(t, repo.calls)
		})
	}
}

func TestAdminOperationInputValidationMatrix(t *testing.T) {
	base := AdminOperationInput{Action: "suspend", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: "10000000-0000-4000-8000-00000000000a", ExpectedUpdatedAt: "2026-10-09T08:00:00.123456Z"}
	for _, tc := range []struct {
		name   string
		mutate func(*AdminOperationInput)
	}{
		{"empty-reason", func(in *AdminOperationInput) { in.Reason = "" }},
		{"blank-reason", func(in *AdminOperationInput) { in.Reason = " \t" }},
		{"over-500-runes", func(in *AdminOperationInput) { in.Reason = strings.Repeat("界", 501) }},
		{"invalid-utf8", func(in *AdminOperationInput) { in.Reason = string([]byte{0xff}) }},
		{"ascii-control", func(in *AdminOperationInput) { in.Reason = "line\nnext" }},
		{"unicode-control", func(in *AdminOperationInput) { in.Reason = "line\u0085next" }},
		{"invalid-key", func(in *AdminOperationInput) { in.IdempotencyKey = "invalid" }},
		{"zero-key", func(in *AdminOperationInput) { in.IdempotencyKey = uuid.Nil.String() }},
		{"uppercase-key", func(in *AdminOperationInput) { in.IdempotencyKey = strings.ToUpper(in.IdempotencyKey) }},
		{"padded-key", func(in *AdminOperationInput) { in.IdempotencyKey += " " }},
		{"compact-key", func(in *AdminOperationInput) { in.IdempotencyKey = strings.ReplaceAll(in.IdempotencyKey, "-", "") }},
		{"empty-confirmation", func(in *AdminOperationInput) { in.Confirmation = "" }},
		{"wrong-confirmation-action", func(in *AdminOperationInput) { in.Confirmation = "resume:9" }},
		{"zero-confirmation-id", func(in *AdminOperationInput) { in.Confirmation = "suspend:0" }},
		{"padded-confirmation-id", func(in *AdminOperationInput) { in.Confirmation = "suspend:09" }},
		{"oversize-confirmation", func(in *AdminOperationInput) { in.Confirmation = "suspend:" + strings.Repeat("1", 60) }},
		{"malformed-version", func(in *AdminOperationInput) { in.ExpectedUpdatedAt = "yesterday" }},
		{"oversize-version", func(in *AdminOperationInput) { in.ExpectedUpdatedAt = strings.Repeat("1", 41) }},
		{"negative-attempts", func(in *AdminOperationInput) { in.ExpectedAttempts = -1 }},
		{"malformed-last-attempt", func(in *AdminOperationInput) { in.ExpectedLastAttemptAt = "yesterday" }},
		{"oversize-last-attempt", func(in *AdminOperationInput) { in.ExpectedLastAttemptAt = strings.Repeat("1", 41) }},
		{"conflicting-status", func(in *AdminOperationInput) { in.Status = "active" }},
		{"unsupported-action", func(in *AdminOperationInput) { in.Action = "archive" }},
	} {
		t.Run(tc.name, func(t *testing.T) { in := base; tc.mutate(&in); require.ErrorIs(t, in.Validate(), ErrWorkspaceInvalid) })
	}
	base.Reason = strings.Repeat("界", 500)
	require.NoError(t, base.Validate(), "limit counts Unicode runes, not UTF8 bytes")
}

func TestAdminOperationFingerprintAliasesTargetsAndFences(t *testing.T) {
	base := AdminOperationInput{Action: "suspend", Reason: "maintenance", Confirmation: "suspend:9", IdempotencyKey: uuid.NewString(), ExpectedUpdatedAt: "2026-10-09T08:00:00.123456Z"}
	a, err := base.Fingerprint("workspace", 9, 9)
	require.NoError(t, err)
	for _, alias := range []AdminOperationInput{
		{Status: "suspended", Reason: base.Reason, Confirmation: base.Confirmation, IdempotencyKey: base.IdempotencyKey, ExpectedUpdatedAt: base.ExpectedUpdatedAt},
		{Action: "suspend", Status: "suspended", Reason: base.Reason, Confirmation: base.Confirmation, IdempotencyKey: base.IdempotencyKey, ExpectedUpdatedAt: base.ExpectedUpdatedAt},
	} {
		got, err := alias.Fingerprint("workspace", 9, 9)
		require.NoError(t, err)
		require.Equal(t, a, got)
	}
	for _, tc := range []struct {
		name                  string
		mutate                func(*AdminOperationInput)
		targetType            string
		targetID, workspaceID int64
	}{
		{"target", func(*AdminOperationInput) {}, "workspace", 10, 9},
		{"workspace", func(*AdminOperationInput) {}, "workspace", 9, 10},
		{"target-type", func(*AdminOperationInput) {}, "webhook_delivery", 9, 9},
		{"reason", func(in *AdminOperationInput) { in.Reason = "different" }, "workspace", 9, 9},
		{"confirmation", func(in *AdminOperationInput) { in.Confirmation = "suspend:10" }, "workspace", 9, 9},
		{"version", func(in *AdminOperationInput) { in.ExpectedUpdatedAt = "2026-10-09T08:00:00.123457Z" }, "workspace", 9, 9},
		{"attempts", func(in *AdminOperationInput) { in.ExpectedAttempts = 2 }, "workspace", 9, 9},
		{"last-attempt", func(in *AdminOperationInput) { in.ExpectedLastAttemptAt = "2026-10-09T08:00:00Z" }, "workspace", 9, 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			tc.mutate(&in)
			got, err := in.Fingerprint(tc.targetType, tc.targetID, tc.workspaceID)
			require.NoError(t, err)
			require.NotEqual(t, a, got)
		})
	}
}

func TestAdministratorRetryDomainEventRequiresExactSafePayload(t *testing.T) {
	const eventType = "webhook.administrator_retried"
	e, err := NewDomainEvent(eventType, 9, 0, 1, "webhook_delivery", "10", DomainEventData{"previous_status": "dead", "status": "retrying", "delivery_id": int64(10)})
	require.NoError(t, err)
	require.True(t, IsWorkspaceVisibleEvent(eventType))
	for _, tc := range []struct {
		name   string
		mutate func(*DomainEvent)
	}{
		{"wrong-subject", func(e *DomainEvent) { e.Subject.Type = "workspace" }},
		{"wrong-id", func(e *DomainEvent) { e.Subject.ID = "11" }},
		{"wrong-status", func(e *DomainEvent) { e.Data["status"] = "dead" }},
		{"wrong-previous", func(e *DomainEvent) { e.Data["previous_status"] = "pending" }},
		{"missing-id", func(e *DomainEvent) { delete(e.Data, "delivery_id") }},
		{"string-id", func(e *DomainEvent) { e.Data["delivery_id"] = "10" }},
		{"float-id", func(e *DomainEvent) { e.Data["delivery_id"] = float64(10) }},
		{"fraction-id", func(e *DomainEvent) { e.Data["delivery_id"] = json.Number("10.1") }},
		{"extra-field", func(e *DomainEvent) { e.Data["name"] = "secret" }},
		{"missing-workspace", func(e *DomainEvent) { e.WorkspaceID = nil }},
		{"missing-actor", func(e *DomainEvent) { e.ActorUserID = nil }},
		{"project-scope", func(e *DomainEvent) { v := int64(2); e.ProjectID = &v }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := *e
			copy.Data = DomainEventData{}
			for k, v := range e.Data {
				copy.Data[k] = v
			}
			tc.mutate(&copy)
			_, err := copy.MarshalPayload()
			require.Error(t, err)
		})
	}
	e.Data["delivery_id"] = json.Number("10")
	require.NoError(t, e.Validate(), "dispatcher UseNumber decoding remains valid")
}
