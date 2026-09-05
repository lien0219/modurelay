package repository

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/stretchr/testify/require"
)

func pinnedUpstreamContext(t *testing.T) context.Context {
	t.Helper()
	ctx, err := urlvalidator.WithValidatedResolvedHostIPs(t.Context(), "relay.example", []net.IP{
		net.ParseIP("203.0.113.10"),
		net.ParseIP("203.0.113.11"),
	})
	require.NoError(t, err)
	return ctx
}

func TestResolvedIPPinnedTransportDirectAndSOCKSUseOnlyValidatedAddresses(t *testing.T) {
	for _, tt := range []struct {
		name     string
		proxyURL *url.URL
	}{
		{name: "direct"},
		{name: "SOCKS", proxyURL: &url.URL{Scheme: "socks5h", Host: "proxy.example:1080"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var dialed []string
			transport := &http.Transport{DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
				dialed = append(dialed, address)
				if address == "203.0.113.10:443" {
					return nil, fmt.Errorf("first address unavailable")
				}
				client, server := net.Pipe()
				_ = server.Close()
				return client, nil
			}}
			configureResolvedIPPinnedTransport(transport, tt.proxyURL)

			conn, err := transport.DialContext(pinnedUpstreamContext(t), "tcp", "relay.example:443")

			require.NoError(t, err)
			require.NoError(t, conn.Close())
			require.Equal(t, []string{"203.0.113.10:443", "203.0.113.11:443"}, dialed)
		})
	}
}

func TestResolvedIPPinnedTransportHTTPProxyUsesValidatedAbsoluteTarget(t *testing.T) {
	proxyURL, err := url.Parse("http://proxy.example:8080")
	require.NoError(t, err)
	transport := &http.Transport{DialContext: newUpstreamDialer().DialContext}
	configureResolvedIPPinnedTransport(transport, proxyURL)
	req, err := http.NewRequestWithContext(pinnedUpstreamContext(t), http.MethodGet, "http://relay.example/v1/usage", nil)
	require.NoError(t, err)

	resolvedProxy, err := transport.Proxy(req)

	require.NoError(t, err)
	require.Equal(t, proxyURL.String(), resolvedProxy.String())
	require.Equal(t, "relay.example", req.Host)
	require.Equal(t, "203.0.113.10:80", req.URL.Host)
}

func TestResolvedIPPinnedTransportHTTPSProxyConnectsToValidatedTarget(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	connectTarget := make(chan string, 1)
	proxyErr := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			proxyErr <- acceptErr
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		req, readErr := http.ReadRequest(bufio.NewReader(conn))
		if readErr != nil {
			proxyErr <- readErr
			return
		}
		connectTarget <- req.Host
		_, writeErr := fmt.Fprint(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
		proxyErr <- writeErr
	}()

	proxyURL, err := url.Parse("http://" + listener.Addr().String())
	require.NoError(t, err)
	transport := &http.Transport{DialContext: newUpstreamDialer().DialContext}
	configureResolvedIPPinnedTransport(transport, proxyURL)
	req, err := http.NewRequestWithContext(pinnedUpstreamContext(t), http.MethodGet, "https://relay.example/v1/usage", nil)
	require.NoError(t, err)

	resolvedProxy, err := transport.Proxy(req)
	require.NoError(t, err)
	require.Nil(t, resolvedProxy)
	conn, err := transport.DialContext(req.Context(), "tcp", "relay.example:443")
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	require.Equal(t, "203.0.113.10:443", <-connectTarget)
	require.NoError(t, <-proxyErr)
}

func TestResolvedIPPinnedRequestUsesFreshTransportConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	protocols := &http.Protocols{}
	protocols.SetHTTP1(true)
	protocols.SetHTTP2(true)
	var dialCount atomic.Int32
	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		Protocols:         protocols,
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{
			"h2": func(string, *tls.Conn) http.RoundTripper { return nil },
		},
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			dialCount.Add(1)
			return newUpstreamDialer().DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	configureResolvedIPPinnedTransport(transport, nil)
	client := &http.Client{Transport: transport}
	targetURL := "http://relay.example:" + serverURL.Port() + "/v1/usage"

	firstReq, err := http.NewRequest(http.MethodGet, targetURL, nil)
	require.NoError(t, err)
	firstResp, err := client.Do(firstReq)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, firstResp.Body)
	require.NoError(t, firstResp.Body.Close())
	require.Equal(t, int32(1), dialCount.Load())

	secondCtx := service.WithHTTPUpstreamResolvedIPPinning(pinnedUpstreamContext(t))
	secondReq, err := http.NewRequestWithContext(secondCtx, http.MethodGet, targetURL, nil)
	require.NoError(t, err)
	pinnedClient := (&httpUpstreamService{}).httpClientForUpstreamRequest(client, secondReq)
	pinnedTransport, ok := pinnedClient.Transport.(*http.Transport)
	require.True(t, ok)
	require.True(t, pinnedTransport.DisableKeepAlives)
	require.True(t, pinnedTransport.ForceAttemptHTTP2)
	require.Nil(t, pinnedTransport.TLSNextProto)
	require.Nil(t, pinnedTransport.Protocols)

	secondResp, err := pinnedClient.Do(secondReq)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, secondResp.Body)
	require.NoError(t, secondResp.Body.Close())
	// The original client had an idle connection available. Pinning must
	// still force a new dial through the validated address.
	require.Equal(t, int32(2), dialCount.Load())
}

func TestResolvedIPPinnedRequestNegotiatesFreshHTTP2(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		require.Equal(t, 2, req.ProtoMajor)
		_, _ = io.WriteString(w, "ok")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	var dialCount atomic.Int32
	transport := &http.Transport{
		ForceAttemptHTTP2: true,
		// The test server uses a self-signed httptest certificate; production
		// transports never use this test-only TLS configuration.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Test server certificate.
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			dialCount.Add(1)
			return newUpstreamDialer().DialContext(ctx, network, server.Listener.Addr().String())
		},
	}
	_, err = enableOpenAIHTTP2KeepAlive(transport)
	require.NoError(t, err)
	configureResolvedIPPinnedTransport(transport, nil)

	ctx := service.WithHTTPUpstreamResolvedIPPinning(pinnedUpstreamContext(t))
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://relay.example:"+serverURL.Port()+"/v1/sub2api/billing",
		nil,
	)
	require.NoError(t, err)
	client := (&httpUpstreamService{}).httpClientForUpstreamRequest(&http.Client{Transport: transport}, req)

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	require.Equal(t, 2, resp.ProtoMajor)
	require.Equal(t, int32(1), dialCount.Load())
}
