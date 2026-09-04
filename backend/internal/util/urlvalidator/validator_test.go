package urlvalidator

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type staticIPResolver struct {
	ips   []net.IP
	err   error
	calls int
}

func (r *staticIPResolver) LookupIP(context.Context, string, string) ([]net.IP, error) {
	r.calls++
	return r.ips, r.err
}

func TestValidateURLFormat(t *testing.T) {
	if _, err := ValidateURLFormat("", false); err == nil {
		t.Fatalf("expected empty url to fail")
	}
	if _, err := ValidateURLFormat("://bad", false); err == nil {
		t.Fatalf("expected invalid url to fail")
	}
	if _, err := ValidateURLFormat("http://example.com", false); err == nil {
		t.Fatalf("expected http to fail when allow_insecure_http is false")
	}
	if _, err := ValidateURLFormat("https://example.com", false); err != nil {
		t.Fatalf("expected https to pass, got %v", err)
	}
	if _, err := ValidateURLFormat("http://example.com", true); err != nil {
		t.Fatalf("expected http to pass when allow_insecure_http is true, got %v", err)
	}
	if _, err := ValidateURLFormat("https://example.com:bad", true); err == nil {
		t.Fatalf("expected invalid port to fail")
	}

	// 验证末尾斜杠被移除
	normalized, err := ValidateURLFormat("https://example.com/", false)
	if err != nil {
		t.Fatalf("expected trailing slash url to pass, got %v", err)
	}
	if normalized != "https://example.com" {
		t.Fatalf("expected trailing slash to be removed, got %s", normalized)
	}

	// 验证多个末尾斜杠被移除
	normalized, err = ValidateURLFormat("https://example.com///", false)
	if err != nil {
		t.Fatalf("expected multiple trailing slashes to pass, got %v", err)
	}
	if normalized != "https://example.com" {
		t.Fatalf("expected all trailing slashes to be removed, got %s", normalized)
	}

	// 验证带路径的 URL 末尾斜杠被移除
	normalized, err = ValidateURLFormat("https://example.com/api/v1/", false)
	if err != nil {
		t.Fatalf("expected trailing slash url with path to pass, got %v", err)
	}
	if normalized != "https://example.com/api/v1" {
		t.Fatalf("expected trailing slash to be removed from path, got %s", normalized)
	}
}

func TestValidateHTTPURL(t *testing.T) {
	if _, err := ValidateHTTPURL("http://example.com", false, ValidationOptions{}); err == nil {
		t.Fatalf("expected http to fail when allow_insecure_http is false")
	}
	if _, err := ValidateHTTPURL("http://example.com", true, ValidationOptions{}); err != nil {
		t.Fatalf("expected http to pass when allow_insecure_http is true, got %v", err)
	}
	if _, err := ValidateHTTPURL("https://example.com", false, ValidationOptions{RequireAllowlist: true}); err == nil {
		t.Fatalf("expected require allowlist to fail when empty")
	}
	if _, err := ValidateHTTPURL("https://example.com", false, ValidationOptions{AllowedHosts: []string{"api.example.com"}}); err == nil {
		t.Fatalf("expected host not in allowlist to fail")
	}
	if _, err := ValidateHTTPURL("https://api.example.com", false, ValidationOptions{AllowedHosts: []string{"api.example.com"}}); err != nil {
		t.Fatalf("expected allowlisted host to pass, got %v", err)
	}
	if _, err := ValidateHTTPURL("https://sub.api.example.com", false, ValidationOptions{AllowedHosts: []string{"*.example.com"}}); err != nil {
		t.Fatalf("expected wildcard allowlist to pass, got %v", err)
	}
	if _, err := ValidateHTTPURL("https://localhost", false, ValidationOptions{AllowPrivate: false}); err == nil {
		t.Fatalf("expected localhost to be blocked when allow_private_hosts is false")
	}
}

func TestRejectSameHTTPOrigin(t *testing.T) {
	tests := []struct {
		name       string
		candidate  string
		deployment string
		wantSame   bool
		wantErr    bool
	}{
		{name: "same HTTPS origin", candidate: "https://app.example.com/v1", deployment: "https://app.example.com", wantSame: true},
		{name: "default HTTPS port", candidate: "https://APP.example.com.:443/v1", deployment: "https://app.example.com", wantSame: true},
		{name: "userinfo does not hide origin", candidate: "https://token:secret@app.example.com/v1", deployment: "https://app.example.com", wantSame: true},
		{name: "same HTTP origin", candidate: "http://app.example.com/v1", deployment: "http://app.example.com:80", wantSame: true},
		{name: "different host", candidate: "https://upstream.example.com/v1", deployment: "https://app.example.com"},
		{name: "different scheme", candidate: "http://app.example.com/v1", deployment: "https://app.example.com"},
		{name: "different port", candidate: "https://app.example.com:8443/v1", deployment: "https://app.example.com"},
		{name: "deployment not configured", candidate: "https://app.example.com/v1", deployment: ""},
		{name: "invalid candidate", candidate: "://bad", deployment: "https://app.example.com", wantErr: true},
		{name: "invalid deployment", candidate: "https://app.example.com", deployment: "://bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RejectSameHTTPOrigin(tt.candidate, tt.deployment)
			if tt.wantSame {
				if !errors.Is(err, ErrSameHTTPOrigin) {
					t.Fatalf("expected same-origin error, got %v", err)
				}
				if err != nil && (strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "secret")) {
					t.Fatalf("same-origin error leaked URL userinfo: %v", err)
				}
				return
			}
			if tt.wantErr && err == nil {
				t.Fatal("expected invalid origin error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected origins to be accepted, got %v", err)
			}
		})
	}
}

func TestResolveAllowedIPsRejectsEntireMixedResult(t *testing.T) {
	resolver := &staticIPResolver{ips: []net.IP{
		net.ParseIP("8.8.8.8"),
		net.ParseIP("10.0.0.10"),
	}}

	addresses, err := resolveAllowedIPs(context.Background(), "relay.example", resolver)

	if err == nil || !strings.Contains(err.Error(), "10.0.0.10") {
		t.Fatalf("expected mixed DNS result to reject the private address, got %v", err)
	}
	if addresses != nil {
		t.Fatalf("expected no addresses from a rejected DNS result, got %v", addresses)
	}
	if resolver.calls != 1 {
		t.Fatalf("expected one DNS lookup, got %d", resolver.calls)
	}
}

func TestIsBlockedIPRejectsNonPublicIPv4Ranges(t *testing.T) {
	for _, raw := range []string{
		"0.0.0.1",
		"100.64.0.1",
		"100.127.255.254",
	} {
		if !isBlockedIP(net.ParseIP(raw)) {
			t.Fatalf("expected %s to be blocked", raw)
		}
	}
	if isBlockedIP(net.ParseIP("100.128.0.1")) {
		t.Fatal("expected address outside CGNAT range to remain allowed")
	}
}

func TestDialContextWithPinnedIPsUsesOnlyValidatedAddresses(t *testing.T) {
	ctx, err := WithValidatedResolvedHostIPs(context.Background(), "relay.example", []net.IP{
		net.ParseIP("203.0.113.10"),
		net.ParseIP("203.0.113.11"),
	})
	if err != nil {
		t.Fatalf("expected test addresses to pass validation, got %v", err)
	}
	var dialed []string
	dial := func(_ context.Context, _, address string) (net.Conn, error) {
		dialed = append(dialed, address)
		if strings.HasPrefix(address, "203.0.113.10:") {
			return nil, errors.New("first address unavailable")
		}
		client, server := net.Pipe()
		_ = server.Close()
		return client, nil
	}

	conn, err := DialContextWithPinnedIPs(ctx, "tcp", "relay.example:443", dial)
	if err != nil {
		t.Fatalf("expected a validated address to connect, got %v", err)
	}
	_ = conn.Close()
	want := []string{"203.0.113.10:443", "203.0.113.11:443"}
	if strings.Join(dialed, ",") != strings.Join(want, ",") {
		t.Fatalf("expected only pinned addresses %v, got %v", want, dialed)
	}
}

func TestDialContextWithPinnedIPsFallsBackBeforeFirstDialTimesOut(t *testing.T) {
	ctx, err := WithValidatedResolvedHostIPs(t.Context(), "relay.example", []net.IP{
		net.ParseIP("203.0.113.10"),
		net.ParseIP("203.0.113.11"),
	})
	require.NoError(t, err)

	started := time.Now()
	var dialed atomic.Int32
	dial := func(ctx context.Context, _, address string) (net.Conn, error) {
		dialed.Add(1)
		if address == "203.0.113.10:443" {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		client, server := net.Pipe()
		_ = server.Close()
		return client, nil
	}

	conn, err := DialContextWithPinnedIPs(ctx, "tcp", "relay.example:443", dial)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.Equal(t, int32(2), dialed.Load())
	require.Less(t, time.Since(started), time.Second)
}
