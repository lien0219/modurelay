package service

import (
	"errors"
	"net"
	"strings"
	"testing"
	"time"
)

var errWebhookTestSentinel = errors.New("transport failed")

func TestValidateWorkspaceWebhookURLRejectsUnsafeForms(t *testing.T) {
	for _, raw := range []string{
		"http://hooks.example.test/events",
		"ftp://hooks.example.test/events",
		"https://localhost/events",
		"https://user:pass@hooks.example.test/events",
		"https://127.0.0.1/events",
		"https://10.0.0.1/events",
		"https://172.16.0.1/events",
		"https://192.168.0.1/events",
		"https://169.254.169.254/events",
		"https://[::1]/events",
		"https://[fe80::1]/events",
		"https://[::ffff:127.0.0.1]/events",
		"https://[::ffff:8.8.8.8]/events",
		"https://192.0.2.1/events",
		"https://[2001:db8::1]/events",
		"https://hooks.example.test/events#fragment",
	} {
		if _, err := ValidateWorkspaceWebhookURL(raw); err == nil {
			t.Fatalf("ValidateWorkspaceWebhookURL(%q) accepted unsafe URL", raw)
		}
	}

	got, err := ValidateWorkspaceWebhookURL("https://hooks.example.test/events/")
	if err != nil {
		t.Fatalf("ValidateWorkspaceWebhookURL(public) error: %v", err)
	}
	if got != "https://hooks.example.test/events" {
		t.Fatalf("normalized URL = %q, want trailing slash removed", got)
	}
}

func TestWebhookIPPolicyRejectsEveryReservedNetwork(t *testing.T) {
	for _, network := range reservedWebhookNetworks {
		if webhookIPAllowed(network.IP) {
			t.Errorf("reserved network %s was allowed", network.String())
		}
	}
}

func TestWebhookIPPolicyRejectsReservedNetworks(t *testing.T) {
	for _, raw := range []string{"192.0.2.1", "198.18.0.1", "198.51.100.1", "203.0.113.2", "240.0.0.1", "2001:db8::1"} {
		if webhookIPAllowed(net.ParseIP(raw)) {
			t.Errorf("webhookIPAllowed(%s)=true, want false", raw)
		}
	}
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		if !webhookIPAllowed(net.ParseIP(raw)) {
			t.Errorf("webhookIPAllowed(%s)=false, want true", raw)
		}
	}
}

func TestSignWebhookPayloadUsesTimestampAndRawBody(t *testing.T) {
	timestamp := time.Unix(1700000000, 0).UTC()
	body := []byte(`{"event":"workspace.updated"}`)
	got := SignWebhookPayload("whsec_test-secret", timestamp, body)
	if !strings.HasPrefix(got, "t=1700000000,v1=") {
		t.Fatalf("signature = %q, want timestamp-prefixed v1 signature", got)
	}
	if !VerifyWebhookSignature("whsec_test-secret", timestamp, body, got) {
		t.Fatal("VerifyWebhookSignature rejected signature generated from the same raw body")
	}
	if VerifyWebhookSignature("whsec_test-secret", timestamp, []byte(`{"event":"workspace.updated"} `), got) {
		t.Fatal("VerifyWebhookSignature accepted a changed raw body")
	}
	if VerifyWebhookSignature("wrong-secret", timestamp, body, got) {
		t.Fatal("VerifyWebhookSignature accepted the wrong secret")
	}
	if VerifyWebhookSignature("whsec_test-secret", timestamp.Add(time.Second), body, got) {
		t.Fatal("VerifyWebhookSignature accepted a changed timestamp")
	}
}

func TestWebhookResponseRetryPolicy(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 599} {
		if !ShouldRetryWebhookResponse(status, nil) {
			t.Fatalf("status %d should be retryable", status)
		}
	}
	for _, status := range []int{200, 201, 400, 401, 404, 499} {
		if ShouldRetryWebhookResponse(status, nil) {
			t.Fatalf("status %d should be terminal", status)
		}
	}
	if !ShouldRetryWebhookResponse(0, errWebhookTestSentinel) {
		t.Fatal("transport error should be retryable")
	}
}
