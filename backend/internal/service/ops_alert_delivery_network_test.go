//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type monitoringSMTPSettings struct {
	SettingRepository
	values map[string]string
}

func (s *monitoringSMTPSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return s.values, nil
}

// Only a loopback SMTP fixture receives these messages; no external mail is sent.
func TestMonitoringAlertEmailActualSMTPFiringAndRecovery(t *testing.T) {
	server := startNotificationEmailTestSMTPServer(t)
	settings := &monitoringSMTPSettings{values: server.settings()}
	evaluator := &OpsAlertEvaluatorService{emailService: &EmailService{settingRepo: settings}}
	rule := &OpsAlertRule{ID: 17, Name: "测试服务延迟", Severity: "P1"}
	for _, status := range []string{OpsAlertStatusFiring, OpsAlertStatusResolved} {
		event := &OpsAlertEvent{RuleID: 17, Status: status, Severity: "P1", FiredAt: time.Now().UTC()}
		snapshot := opsAlertEmailPayload{Subject: opsAlertEmailSubject(rule, event, notificationEmailLocaleChinese), Body: buildOpsAlertEmailBody(rule, event, notificationEmailLocaleChinese)}
		payload, err := json.Marshal(snapshot)
		require.NoError(t, err)
		require.NoError(t, evaluator.deliverAlertNotification(context.Background(), &OpsAlertDelivery{Channel: "email", Target: "receiver@example.com", Payload: payload}))
		message, err := mail.ReadMessage(strings.NewReader(server.lastMessage()))
		require.NoError(t, err)
		subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject"))
		require.NoError(t, err)
		require.Equal(t, snapshot.Subject, subject)
		require.Contains(t, subject, "测试服务延迟")
		require.Contains(t, server.lastMessageBody(t), "测试服务延迟")
		if status == OpsAlertStatusResolved {
			require.Contains(t, subject, "恢复")
		}
	}
	require.Equal(t, int64(2), server.messageCount())
	server.close()
	payload, err := json.Marshal(opsAlertEmailPayload{Subject: "retry", Body: "test"})
	require.NoError(t, err)
	err = evaluator.deliverAlertNotification(context.Background(), &OpsAlertDelivery{Channel: "email", Target: "receiver@example.com", Payload: payload})
	require.ErrorContains(t, err, "SMTP delivery failed")
	require.NotContains(t, err.Error(), "password")
	replacement := startNotificationEmailTestSMTPServer(t)
	settings.values = replacement.settings()
	require.NoError(t, evaluator.deliverAlertNotification(context.Background(), &OpsAlertDelivery{Channel: "email", Target: "receiver@example.com", Payload: payload}))
	require.Equal(t, int64(1), replacement.messageCount())
}

// Exercise a real TLS connection and HTTP response, replacing only DNS/dialing
// with a loopback fixture. Production public-IP restrictions remain unchanged.
func TestMonitoringWebhookActualTLSStatusSignatureAndDeadline(t *testing.T) {
	secret := strings.Repeat("test-only-", 4)
	t.Setenv("OPS_ALERT_WEBHOOK_SECRET", secret)
	var status atomic.Int64
	var delay atomic.Bool
	type receivedRequest struct{ payload, timestamp, signature, key string }
	received := make(chan receivedRequest, 8)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- receivedRequest{string(body), r.Header.Get("X-ModuRelay-Timestamp"), r.Header.Get("X-ModuRelay-Signature"), r.Header.Get("Idempotency-Key")}
		if delay.Load() {
			<-r.Context().Done()
			return
		}
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte("remote-secret-must-not-enter-errors"))
	}))
	defer server.Close()
	client := server.Client()
	transport := client.Transport.(*http.Transport)
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	transport.TLSClientConfig.ServerName = "example.com"
	defer client.CloseIdleConnections()
	payload := []byte(`{"version":1,"transition":"firing","message":"测试"}`)
	for _, code := range []int{429, 500, 200} {
		status.Store(int64(code))
		err := sendOpsAlertWebhookWithClient(context.Background(), "https://hooks.example.com/hook", payload, "17", client)
		if code == 200 {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			require.NotContains(t, err.Error(), "remote-secret")
		}
		request := <-received
		require.Equal(t, string(payload), request.payload)
		require.Equal(t, "ops-alert-17", request.key)
		require.Equal(t, opsWebhookSignature(secret, request.timestamp, payload), request.signature)
	}
	delay.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.Error(t, sendOpsAlertWebhookWithClient(ctx, "https://hooks.example.com/hook", payload, "17", client))
}
