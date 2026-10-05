package service

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

// A controlled DNS server exercises the real resolver and pinning path without
// contacting an external receiver or adding a bypass to production transport.
func webhookControlledDNS(t *testing.T) *atomic.Value {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	var address atomic.Value
	address.Store(net.ParseIP("8.8.8.8"))
	done := make(chan struct{})
	go func() {
		defer close(done)
		packet := make([]byte, 4096)
		for {
			n, peer, readErr := conn.ReadFrom(packet)
			if readErr != nil {
				return
			}
			var query dnsmessage.Message
			if query.Unpack(packet[:n]) != nil {
				continue
			}
			response := dnsmessage.Message{
				Header:    dnsmessage.Header{ID: query.ID, Response: true, Authoritative: true, RecursionAvailable: true},
				Questions: query.Questions,
			}
			for _, question := range query.Questions {
				if question.Type != dnsmessage.TypeA {
					continue
				}
				storedIP, ok := address.Load().(net.IP)
				if !ok {
					continue
				}
				ip := storedIP.To4()
				if ip == nil {
					continue
				}
				response.Answers = append(response.Answers, dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 0},
					Body:   &dnsmessage.AResource{A: [4]byte{ip[0], ip[1], ip[2], ip[3]}},
				})
			}
			if packed, packErr := response.Pack(); packErr == nil {
				_, _ = conn.WriteTo(packed, peer)
			}
		}
	}()
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "udp", conn.LocalAddr().String())
	}}
	t.Cleanup(func() {
		net.DefaultResolver = previous
		_ = conn.Close()
		<-done
	})
	return &address
}

type webhookDNSAccessRepository struct{ WorkspaceRepository }

func (webhookDNSAccessRepository) GetAccess(context.Context, int64, int64, int64) (*WorkspaceAccess, error) {
	return &WorkspaceAccess{Workspace: &Workspace{ID: 11, Status: "active"}, Member: &WorkspaceMember{Role: "owner", Status: "active"}}, nil
}

type webhookDNSWriteRepository struct {
	WorkspaceWebhookRepository
	writes int
}

func (r *webhookDNSWriteRepository) CreateWebhook(context.Context, int64, int64, CreateWorkspaceWebhookInput, string, []string) (*WorkspaceWebhook, error) {
	r.writes++
	return &WorkspaceWebhook{ID: 19}, nil
}

func (r *webhookDNSWriteRepository) UpdateWebhook(context.Context, int64, int64, int64, UpdateWorkspaceWebhookInput, []string) (*WorkspaceWebhook, error) {
	r.writes++
	return &WorkspaceWebhook{ID: 19}, nil
}

func TestWorkspaceWebhookRejectsPrivateDNSBeforeCreateOrUpdate(t *testing.T) {
	dns := webhookControlledDNS(t)
	dns.Store(net.ParseIP("127.0.0.1"))
	repo := &webhookDNSWriteRepository{}
	svc := NewWorkspaceWebhookService(repo, NewWorkspaceAccessService(webhookDNSAccessRepository{}), webhookWorkerTestEncryptor{}, true)
	url := "https://receiver.modurelay.test/events"
	_, _, err := svc.Create(context.Background(), 7, 11, CreateWorkspaceWebhookInput{Name: "Private", URL: url, EventTypes: []string{EventWorkspaceUpdated}})
	require.ErrorIs(t, err, ErrWorkspaceInvalid)
	_, err = svc.Update(context.Background(), 7, 11, 19, UpdateWorkspaceWebhookInput{URL: &url})
	require.ErrorIs(t, err, ErrWorkspaceInvalid)
	require.Zero(t, repo.writes)
}

func TestWorkspaceWebhookWorkerRejectsDNSRebindingAndPinsPublicAddress(t *testing.T) {
	dns := webhookControlledDNS(t)
	url := "https://receiver.modurelay.test/events"
	repo := &webhookDNSWriteRepository{}
	svc := NewWorkspaceWebhookService(repo, NewWorkspaceAccessService(webhookDNSAccessRepository{}), webhookWorkerTestEncryptor{}, true)
	_, _, err := svc.Create(context.Background(), 7, 11, CreateWorkspaceWebhookInput{Name: "Public", URL: url, EventTypes: []string{EventWorkspaceUpdated}})
	require.NoError(t, err)
	claim := webhookWorkerTestClaim()
	claim.URL = url
	workerRepo := &webhookWorkerRepositoryRecorder{active: true}
	worker := NewWorkspaceWebhookWorker(workerRepo, webhookWorkerTestEncryptor{})
	requests := 0
	worker.client.Transport = webhookWorkerRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		addresses, pinned, pinErr := urlvalidator.PinnedDialAddresses(req.Context(), "receiver.modurelay.test:443")
		require.NoError(t, pinErr)
		require.True(t, pinned)
		require.Equal(t, []string{"8.8.8.8:443"}, addresses)
		dns.Store(net.ParseIP("127.0.0.1"))
		// A DNS change after validation cannot change this request's dial target.
		addresses, pinned, pinErr = urlvalidator.PinnedDialAddresses(req.Context(), "receiver.modurelay.test:443")
		require.NoError(t, pinErr)
		require.True(t, pinned)
		require.Equal(t, []string{"8.8.8.8:443"}, addresses)
		return &http.Response{StatusCode: 204, Body: http.NoBody}, nil
	})
	worker.processClaim(context.Background(), claim)
	require.Equal(t, "succeeded", workerRepo.state)
	worker.processClaim(context.Background(), claim)
	require.Equal(t, 1, requests, "subsequent attempts must reject the rebound private address before HTTP")
	require.Equal(t, "retrying", workerRepo.state)
	require.False(t, strings.Contains(workerRepo.reason, "127.0.0.1"))
}
