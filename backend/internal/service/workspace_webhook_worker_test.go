package service

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWebhookRotationSignatureAcceptsCurrentAndPreviousDuringGrace(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	body := []byte(`{"id":"evt_1"}`)
	signature := SignWebhookPayloadWithPrevious("current", "previous", now.Add(time.Hour), now, body)
	if !VerifyWebhookSignature("current", now, body, signature) {
		t.Fatal("current secret did not verify rotated signature")
	}
	if !VerifyWebhookSignature("previous", now, body, signature) {
		t.Fatal("previous secret did not verify rotated signature during grace")
	}
	if VerifyWebhookSignature("previous", now.Add(2*time.Hour), body, signature) {
		t.Fatal("previous secret verified outside the timestamp encoded by signature")
	}
}

func TestWebhookRotationSignatureExcludesPreviousAtGraceExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	body := []byte(`{"id":"evt_1"}`)
	for _, expires := range []time.Time{now, now.Add(-time.Second)} {
		signature := SignWebhookPayloadWithPrevious("current", "previous", expires, now, body)
		require.True(t, VerifyWebhookSignature("current", now, body, signature))
		require.False(t, VerifyWebhookSignature("previous", now, body, signature))
		require.Equal(t, 1, strings.Count(signature, "v1="))
	}
}

func TestWorkspaceWebhookHTTPClientSecurityBounds(t *testing.T) {
	client := newWorkspaceWebhookHTTPClient()
	t.Cleanup(client.CloseIdleConnections)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Nil(t, transport.Proxy)
	require.Equal(t, 10*time.Second, client.Timeout)
	require.Greater(t, transport.TLSHandshakeTimeout, time.Duration(0))
	require.Greater(t, transport.ResponseHeaderTimeout, time.Duration(0))
	require.GreaterOrEqual(t, transport.TLSClientConfig.MinVersion, uint16(tls.VersionTLS12))
	require.False(t, transport.TLSClientConfig.InsecureSkipVerify)
	require.ErrorIs(t, client.CheckRedirect(&http.Request{}, nil), http.ErrUseLastResponse)
}

func TestWorkspaceWebhookWorkerCancelsInFlightRequest(t *testing.T) {
	repo := &webhookWorkerRepositoryRecorder{active: true}
	worker := NewWorkspaceWebhookWorker(repo, webhookWorkerTestEncryptor{})
	worker.client.Transport = webhookWorkerRoundTripper(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	worker.processClaim(ctx, webhookWorkerTestClaim())
	require.Less(t, time.Since(started), time.Second)
	require.Equal(t, "retrying", repo.state)
	require.Equal(t, "webhook request timed out", repo.reason)
}

func TestSanitizeWebhookPreviewBoundsAndRemovesControls(t *testing.T) {
	got := sanitizeWebhookPreview([]byte("ok\x00\x01\n"+string(make([]byte, 2048))), 32)
	if len(got) > 32+len("...(truncated)") {
		t.Fatalf("preview length = %d, exceeds bound", len(got))
	}
	if got == "" || got[0] != 'o' {
		t.Fatalf("unexpected preview %q", got)
	}
}

type webhookWorkerRepositoryRecorder struct {
	WorkspaceWebhookRepository
	active   bool
	state    string
	status   int
	preview  string
	reason   string
	next     time.Time
	deadline time.Time
}

func (r *webhookWorkerRepositoryRecorder) DeliveryEndpointActive(context.Context, int64, string) (bool, error) {
	return r.active, nil
}

func (r *webhookWorkerRepositoryRecorder) MarkDeliverySuccess(ctx context.Context, _ int64, _ string, status int, preview string) error {
	r.state, r.status, r.preview = "succeeded", status, preview
	r.deadline, _ = ctx.Deadline()
	return nil
}
func (r *webhookWorkerRepositoryRecorder) MarkDeliveryRetry(ctx context.Context, _ int64, _ string, next time.Time, reason string, status int, preview string) error {
	r.state, r.status, r.preview, r.reason, r.next = "retrying", status, preview, reason, next
	r.deadline, _ = ctx.Deadline()
	return nil
}
func (r *webhookWorkerRepositoryRecorder) MarkDeliveryDead(ctx context.Context, _ int64, _ string, status int, preview, reason string) error {
	r.state, r.status, r.preview, r.reason = "dead", status, preview, reason
	r.deadline, _ = ctx.Deadline()
	return nil
}

type webhookWorkerTestEncryptor struct{}

func (webhookWorkerTestEncryptor) Encrypt(value string) (string, error) {
	return "encrypted:" + value, nil
}
func (webhookWorkerTestEncryptor) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "encrypted:") {
		return "", errors.New("bad ciphertext")
	}
	return strings.TrimPrefix(value, "encrypted:"), nil
}

type webhookWorkerRoundTripper func(*http.Request) (*http.Response, error)

func (f webhookWorkerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func webhookWorkerTestClaim() WebhookDeliveryClaim {
	return WebhookDeliveryClaim{
		WorkspaceWebhookDelivery: WorkspaceWebhookDelivery{ID: 17, WebhookID: 19, EventID: "evt_worker", EventType: EventWorkspaceUpdated, Attempts: 1},
		WorkspaceID:              11, URL: "https://8.8.8.8/events?token=secret-url-value", SecretCurrent: "encrypted:whsec_current_test_secret",
		Payload: []byte(`{"id":"evt_worker","type":"workspace.updated","version":1}`), LockOwner: "worker:claim",
	}
}

func TestWorkspaceWebhookWorkerSendsCanonicalHeadersAndRawBody(t *testing.T) {
	repo := &webhookWorkerRepositoryRecorder{active: true}
	worker := NewWorkspaceWebhookWorker(repo, webhookWorkerTestEncryptor{})
	claim := webhookWorkerTestClaim()
	grace := time.Now().UTC().Add(time.Hour)
	claim.SecretPrevious = "encrypted:whsec_previous_test_secret"
	claim.PreviousExpiresAt = &grace
	worker.client.Transport = webhookWorkerRoundTripper(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		require.Equal(t, claim.Payload, body)
		require.Equal(t, claim.EventID, req.Header.Get("X-ModuRelay-Event-ID"))
		require.Equal(t, claim.EventType, req.Header.Get("X-ModuRelay-Event-Type"))
		require.Equal(t, "17", req.Header.Get("X-ModuRelay-Delivery-ID"))
		require.Equal(t, "1", req.Header.Get("X-ModuRelay-Delivery-Attempt"))
		seconds, err := strconv.ParseInt(req.Header.Get("X-ModuRelay-Timestamp"), 10, 64)
		require.NoError(t, err)
		signature := req.Header.Get("X-ModuRelay-Signature")
		require.True(t, strings.HasPrefix(signature, "v1="))
		require.True(t, VerifyWebhookSignature("whsec_current_test_secret", time.Unix(seconds, 0), body, signature))
		require.True(t, VerifyWebhookSignature("whsec_previous_test_secret", time.Unix(seconds, 0), body, signature))
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("accepted")), Header: make(http.Header)}, nil
	})
	worker.processClaim(context.Background(), claim)
	require.Equal(t, "succeeded", repo.state)
	require.False(t, repo.deadline.IsZero(), "status persistence must have a timeout")
}

func TestWorkspaceWebhookWorkerResponseClassification(t *testing.T) {
	for _, tc := range []struct {
		status int
		state  string
	}{
		{200, "succeeded"}, {204, "succeeded"}, {408, "retrying"}, {429, "retrying"}, {500, "retrying"},
		{400, "dead"}, {401, "dead"}, {403, "dead"}, {404, "dead"}, {302, "dead"}, {307, "dead"},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			repo := &webhookWorkerRepositoryRecorder{active: true}
			worker := NewWorkspaceWebhookWorker(repo, webhookWorkerTestEncryptor{})
			calls := 0
			worker.client.Transport = webhookWorkerRoundTripper(func(req *http.Request) (*http.Response, error) {
				calls++
				header := make(http.Header)
				header.Set("Location", "https://169.254.169.254/credentials")
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader("ok")), Header: header, Request: req}, nil
			})
			worker.processClaim(context.Background(), webhookWorkerTestClaim())
			require.Equal(t, 1, calls, "redirect must not issue another request")
			require.Equal(t, tc.state, repo.state)
			require.Equal(t, tc.status, repo.status)
			if tc.state == "retrying" {
				require.GreaterOrEqual(t, time.Until(repo.next), 59*time.Second)
			}
		})
	}
}

func TestWorkspaceWebhookWorkerTransportErrorsAndMaxAttempts(t *testing.T) {
	for _, tc := range []struct {
		name     string
		err      error
		attempts int
		state    string
	}{
		{"connection", errors.New("connection failure: secret-url-value"), 1, "retrying"},
		{"timeout", context.DeadlineExceeded, 1, "retrying"},
		{"max_attempts", context.DeadlineExceeded, WebhookMaxAttempts, "dead"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &webhookWorkerRepositoryRecorder{active: true}
			worker := NewWorkspaceWebhookWorker(repo, webhookWorkerTestEncryptor{})
			worker.client.Transport = webhookWorkerRoundTripper(func(*http.Request) (*http.Response, error) { return nil, tc.err })
			claim := webhookWorkerTestClaim()
			claim.Attempts = tc.attempts
			worker.processClaim(context.Background(), claim)
			require.Equal(t, tc.state, repo.state)
			require.NotContains(t, repo.reason, "secret-url-value")
			require.NotContains(t, repo.reason, "https://")
			require.False(t, repo.deadline.IsZero())
		})
	}
}

func TestWorkspaceWebhookWorkerRejectsDisabledEndpointBeforeNetwork(t *testing.T) {
	repo := &webhookWorkerRepositoryRecorder{active: false}
	worker := NewWorkspaceWebhookWorker(repo, webhookWorkerTestEncryptor{})
	worker.client.Transport = webhookWorkerRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Fatal("disabled endpoint was contacted")
		return nil, errors.New("unexpected network call")
	})
	worker.processClaim(context.Background(), webhookWorkerTestClaim())
	require.Equal(t, "retrying", repo.state)
}

func TestWorkspaceWebhookWorkerBoundsAndRedactsReceiverBody(t *testing.T) {
	repo := &webhookWorkerRepositoryRecorder{active: true}
	worker := NewWorkspaceWebhookWorker(repo, webhookWorkerTestEncryptor{})
	body := `{"api_key":"sk-private-key", "secret":"whsec_current_test_secret", "authorization":"Bearer private-token"}` + strings.Repeat("x", webhookMaxBodyBytes)
	worker.client.Transport = webhookWorkerRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	worker.processClaim(context.Background(), webhookWorkerTestClaim())
	require.Equal(t, "retrying", repo.state)
	require.LessOrEqual(t, len(repo.preview), WebhookResponsePreviewBytes)
	for _, secret := range []string{"sk-private-key", "whsec_current_test_secret", "private-token"} {
		require.NotContains(t, repo.preview, secret)
	}
}

func TestWorkspaceWebhookRetryDelayHasBoundedJitter(t *testing.T) {
	bases := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour}
	for attempt := 1; attempt <= WebhookMaxAttempts; attempt++ {
		index := attempt - 1
		if index >= len(bases) {
			index = len(bases) - 1
		}
		base := bases[index]
		for sample := 0; sample < 16; sample++ {
			delay := webhookRetryDelay(attempt)
			require.GreaterOrEqual(t, delay, base)
			require.LessOrEqual(t, delay, base+base/4)
		}
	}
}
