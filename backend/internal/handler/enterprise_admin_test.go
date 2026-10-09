package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Only external persistence is replaced: HTTP parsing, service validation,
// trusted session proof, response serialization and route registration are real.
type enterpriseAdminHTTPRepo struct {
	service.WorkspaceRepository
	service.EnterpriseAdminRepository
	mu              sync.Mutex
	calls           []string
	filter          service.AdminWorkspaceFilter
	err, inspectErr error
	block           bool
	deadline        time.Duration
	mutations       int
	receipts        map[string]*service.AdminOperationReceipt
	entered         chan struct{}
}

func (r *enterpriseAdminHTTPRepo) AdminList(context.Context, int64, pagination.PaginationParams) ([]service.Workspace, int64, error) {
	return []service.Workspace{}, 0, nil
}
func (r *enterpriseAdminHTTPRepo) AdminSetStatus(context.Context, int64, int64, string) error {
	r.calls = append(r.calls, "legacy-status")
	return service.ErrWorkspaceConflict
}

func (r *enterpriseAdminHTTPRepo) read(ctx context.Context, actor int64, action string) error {
	r.mu.Lock()
	r.calls = append(r.calls, action)
	if end, ok := ctx.Deadline(); ok {
		r.deadline = time.Until(end)
	}
	r.mu.Unlock()
	if actor != 7 {
		return service.ErrWorkspaceForbidden
	}
	if r.block {
		if r.entered != nil {
			r.entered <- struct{}{}
		}
		<-ctx.Done()
		return ctx.Err()
	}
	return r.err
}
func (r *enterpriseAdminHTTPRepo) AdminSearchWorkspaces(ctx context.Context, actor int64, f service.AdminWorkspaceFilter) (*service.AdminWorkspacePage, error) {
	if err := r.read(ctx, actor, "search"); err != nil {
		return nil, err
	}
	r.filter = f
	return &service.AdminWorkspacePage{Items: []service.Workspace{}, Page: f.Page, PageSize: f.PageSize, TotalPages: 0, TotalCapped: true, ObservedAt: time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)}, nil
}
func (r *enterpriseAdminHTTPRepo) AdminDiagnosticsOverview(ctx context.Context, actor int64) (*service.AdminDiagnosticsOverview, error) {
	if err := r.read(ctx, actor, "overview"); err != nil {
		return nil, err
	}
	return &service.AdminDiagnosticsOverview{State: "unknown"}, nil
}
func (r *enterpriseAdminHTTPRepo) AdminWorkspaceDiagnostics(ctx context.Context, actor, workspace int64) (*service.AdminWorkspaceDiagnostics, error) {
	if err := r.read(ctx, actor, "diagnostics"); err != nil {
		return nil, err
	}
	return &service.AdminWorkspaceDiagnostics{State: "unknown", Workspace: &service.Workspace{ID: workspace, Status: "active"}}, nil
}
func (r *enterpriseAdminHTTPRepo) AdminOperationsJobs(ctx context.Context, actor int64) (*service.AdminJobsDiagnostics, error) {
	if err := r.read(ctx, actor, "jobs"); err != nil {
		return nil, err
	}
	return &service.AdminJobsDiagnostics{State: "unknown", Workers: []service.AdminWorkerDiagnostics{}}, nil
}
func (r *enterpriseAdminHTTPRepo) AdminInspect(ctx context.Context, actor, workspace int64) (*service.Workspace, error) {
	if err := r.read(ctx, actor, "inspect"); err != nil {
		return nil, err
	}
	if r.inspectErr != nil {
		return nil, r.inspectErr
	}
	return &service.Workspace{ID: workspace, Name: "Tenant", Slug: "tenant", Status: "active", Permissions: []string{"private-permission"}}, nil
}
func (r *enterpriseAdminHTTPRepo) operate(ctx context.Context, actor, workspace, target int64, in service.AdminOperationInput) (*service.AdminOperationReceipt, error) {
	if err := r.read(ctx, actor, in.CanonicalAction()); err != nil {
		return nil, err
	}
	if r.receipts == nil {
		r.receipts = map[string]*service.AdminOperationReceipt{}
	}
	if receipt := r.receipts[in.IdempotencyKey]; receipt != nil {
		return receipt, nil
	}
	r.mutations++
	receipt := &service.AdminOperationReceipt{ID: "c3d86901-682f-4f99-839b-5232490c8611", Action: in.CanonicalAction(), WorkspaceID: workspace, TargetID: target, ResultStatus: "suspended", ResultUpdatedAt: time.Date(2026, 10, 9, 1, 2, 3, 456, time.UTC)}
	r.receipts[in.IdempotencyKey] = receipt
	return receipt, nil
}
func (r *enterpriseAdminHTTPRepo) AdminOperateWorkspace(ctx context.Context, actor, workspace int64, in service.AdminOperationInput) (*service.AdminOperationReceipt, error) {
	return r.operate(ctx, actor, workspace, workspace, in)
}
func (r *enterpriseAdminHTTPRepo) AdminRetryWebhook(ctx context.Context, actor, workspace, webhook, delivery int64, in service.AdminOperationInput) (*service.AdminOperationReceipt, error) {
	return r.operate(ctx, actor, workspace, delivery, in)
}

func enterpriseAdminTestRouter(repo *enterpriseAdminHTTPRepo, subject *middleware.AuthSubject, role, method string, guard func(*gin.Context) bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewWorkspaceHandler(service.NewWorkspaceService(repo), nil)
	h.SetIdentityRecentAuthentication(guard)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if subject != nil {
			c.Set(string(middleware.ContextKeyUser), *subject)
			c.Set(string(middleware.ContextKeyUserRole), role)
			c.Set("auth_method", method)
			c.Request = c.Request.WithContext(service.WithSessionAuthentication(c.Request.Context(), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().UTC()}))
		}
	})
	h.RegisterAdminRoutes(r.Group("/api/v1/admin"))
	return r
}
func enterpriseAdminProof(c *gin.Context) bool {
	c.Request = c.Request.WithContext(service.WithRecentAuthentication(c.Request.Context(), time.Now().UTC(), true))
	return true
}
func enterpriseAdminRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/api/v1/admin"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}
func enterpriseAdminData(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	return envelope.Data
}

var enterpriseAdminHuman = middleware.AuthSubject{UserID: 7, PrincipalType: service.PrincipalHuman}

const enterpriseAdminStatusBody = `{"status":"suspended","reason":"maintenance","confirmation":"suspend:12","idempotency_key":"91b7f47a-a50c-4b16-8407-8924598c36ab","expected_updated_at":"2026-10-09T00:00:00Z"}`
const enterpriseAdminRetryBody = `{"action":"retry_webhook","reason":"manual retry","confirmation":"retry_webhook:34","idempotency_key":"c8a2e352-20a2-4edb-a7d0-65251291b243","expected_attempts":2}`

func TestEnterpriseAdminHTTPRoutesAndPublicEnvelopes(t *testing.T) {
	repo := &enterpriseAdminHTTPRepo{}
	r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
	for _, tc := range []struct{ path, source string }{{"/workspaces/diagnostics/overview", "overview"}, {"/workspaces/12/diagnostics", "diagnostics"}, {"/operations/jobs", "jobs"}, {"/operations/health", "jobs"}, {"/workspaces/12", "inspect"}} {
		t.Run(tc.path, func(t *testing.T) {
			w := enterpriseAdminRequest(r, "GET", tc.path, "")
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
			require.Equal(t, tc.source, repo.calls[len(repo.calls)-1])
			require.Greater(t, repo.deadline, time.Duration(0))
			require.LessOrEqual(t, repo.deadline, 10*time.Second)
			if tc.path == "/operations/health" {
				data := enterpriseAdminData(t, w)
				require.Equal(t, "unknown", data["state"])
				require.Contains(t, data, "workers")
				require.Contains(t, data, "export_rotation")
				require.Contains(t, data, "webhook_failure_trend")
			}
			w = enterpriseAdminRequest(r, "GET", tc.path+"?timezone=Asia%2FShanghai", "")
			require.Equal(t, 200, w.Code, w.Body.String())
		})
	}
	w := enterpriseAdminRequest(r, "GET", "/workspaces?owner_user_id=3&name_prefix=abc&page=2&page_size=10&sort=name&direction=asc", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	data := enterpriseAdminData(t, w)
	require.EqualValues(t, 1, data["pages"])
	require.EqualValues(t, 2, data["page"])
	require.EqualValues(t, 10, data["page_size"])
	require.EqualValues(t, 0, data["total"])
	require.Equal(t, true, data["total_capped"])
	require.Contains(t, data, "observed_at")
	require.NotContains(t, data, "total_pages")
	require.Equal(t, int64(3), repo.filter.OwnerUserID)
	require.Equal(t, "abc", repo.filter.NamePrefix)
	w = enterpriseAdminRequest(r, "GET", "/workspaces?timezone=Asia%2FShanghai&page=1&page_size=20", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	w = enterpriseAdminRequest(r, "GET", "/operations/metrics", "")
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Header().Get("Content-Type"), "text/plain")
	require.Contains(t, w.Body.String(), "modurelay_enterprise_admin_")
	require.NotContains(t, w.Body.String(), "go_goroutines")
	require.NotContains(t, w.Body.String(), "workspace_id=")
	w = enterpriseAdminRequest(r, "GET", "/operations/metrics?timezone=Asia%2FShanghai", "")
	require.Equal(t, 200, w.Code, w.Body.String())
}

func TestEnterpriseAdminHTTPRejectsMalformedBoundsBeforePersistence(t *testing.T) {
	for _, path := range []string{"/workspaces/01/diagnostics", "/workspaces/+1", "/workspaces/0", "/workspaces/9223372036854775808", "/workspaces?workspace_id=0", "/workspaces?owner_user_id=01", "/workspaces?page=0", "/workspaces?page=-1", "/workspaces?page=x", "/workspaces?page_size=101", "/workspaces?page=502&page_size=20", "/workspaces?page=1&page=2", "/workspaces?status=wat", "/workspaces?sort=sql", "/workspaces?unknown=canary", "/workspaces?created_from=bad", "/workspaces?name_prefix=" + strings.Repeat("a", 101), "/workspaces?name_prefix=" + strings.Repeat("a", 17000), "/operations/jobs?raw_payload=true", "/workspaces?name_prefix=%ZZ"} {
		t.Run(path[:min(len(path), 100)], func(t *testing.T) {
			repo := &enterpriseAdminHTTPRepo{}
			r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
			w := enterpriseAdminRequest(r, "GET", path, "")
			require.Equal(t, 400, w.Code, w.Body.String())
			require.Empty(t, repo.calls)
		})
	}
}

func TestEnterpriseAdminHTTPUnknownPositiveWorkspaceIDsAreStableNotFound(t *testing.T) {
	for _, path := range []string{"/workspaces/9223372036854770000", "/workspaces/9223372036854770000/diagnostics"} {
		for _, tc := range []struct {
			name string
			err  error
		}{{"native", sql.ErrNoRows}, {"wrapped", fmt.Errorf("SQL private-canary: %w", sql.ErrNoRows)}} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				repo := &enterpriseAdminHTTPRepo{err: tc.err}
				r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
				w := enterpriseAdminRequest(r, "GET", path, "")
				require.Equal(t, 404, w.Code, w.Body.String())
				var envelope struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
					Reason  string `json:"reason"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
				require.Equal(t, 404, envelope.Code)
				require.Equal(t, "WORKSPACE_NOT_FOUND", envelope.Reason)
				require.Equal(t, "Workspace resource not found", envelope.Message)
				require.NotContains(t, w.Body.String(), "SQL")
				require.NotContains(t, w.Body.String(), "no rows")
				require.NotContains(t, w.Body.String(), "private-canary")
				require.NotContains(t, w.Body.String(), "9223372036854770000")
			})
		}
	}
}

func TestEnterpriseAdminHTTPAuthenticationAndLiveRepositoryAuthority(t *testing.T) {
	misconfiguredAuth := &AuthHandler{}
	for _, tc := range []struct {
		name         string
		subject      *middleware.AuthSubject
		role, method string
		guard        func(*gin.Context) bool
		want         int
	}{
		{"absent", nil, "", "", enterpriseAdminProof, 401},
		{"member", &enterpriseAdminHuman, "user", "jwt", enterpriseAdminProof, 403},
		{"machine", &middleware.AuthSubject{UserID: 7, PrincipalType: service.PrincipalTypeServiceAccount, ServiceAccountID: 9}, "admin", "jwt", enterpriseAdminProof, 403},
		{"shared-key", &enterpriseAdminHuman, "admin", "admin_api_key", enterpriseAdminProof, 403},
		{"missing-guard", &enterpriseAdminHuman, "admin", "jwt", nil, 403},
		{"misconfigured-guard", &enterpriseAdminHuman, "admin", "jwt", misconfiguredAuth.RequireEnterpriseRecentAuthentication, 503},
		{"denied-guard", &enterpriseAdminHuman, "admin", "jwt", func(c *gin.Context) bool { c.JSON(403, gin.H{"reason": "RECENT_AUTH_REQUIRED"}); return false }, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &enterpriseAdminHTTPRepo{}
			r := enterpriseAdminTestRouter(repo, tc.subject, tc.role, tc.method, tc.guard)
			for _, req := range []struct{ method, path, body string }{{"PATCH", "/workspaces/12/status", enterpriseAdminStatusBody}, {"POST", "/workspaces/12/webhooks/23/deliveries/34/retry", enterpriseAdminRetryBody}} {
				w := enterpriseAdminRequest(r, req.method, req.path, req.body)
				require.Equal(t, tc.want, w.Code, w.Body.String())
			}
			require.Empty(t, repo.calls)
		})
	}
	// Context role hints cannot bypass the repository's live Global Admin check.
	subject := middleware.AuthSubject{UserID: 8, PrincipalType: service.PrincipalHuman}
	repo := &enterpriseAdminHTTPRepo{}
	r := enterpriseAdminTestRouter(repo, &subject, "admin", "jwt", enterpriseAdminProof)
	for _, path := range []string{"/workspaces", "/workspaces/12", "/workspaces/12/diagnostics", "/workspaces/diagnostics/overview", "/operations/jobs", "/operations/health", "/operations/metrics"} {
		w := enterpriseAdminRequest(r, "GET", path, "")
		require.Equal(t, 403, w.Code, w.Body.String())
	}
	for _, tc := range []struct {
		subject *middleware.AuthSubject
		role    string
		want    int
	}{{nil, "", 401}, {&enterpriseAdminHuman, "user", 403}, {&middleware.AuthSubject{UserID: 7, PrincipalType: service.PrincipalTypeServiceAccount, ServiceAccountID: 9}, "admin", 403}} {
		repo := &enterpriseAdminHTTPRepo{}
		r := enterpriseAdminTestRouter(repo, tc.subject, tc.role, "jwt", enterpriseAdminProof)
		for _, path := range []string{"/workspaces", "/workspaces/12", "/workspaces/12/diagnostics", "/workspaces/diagnostics/overview", "/operations/jobs", "/operations/health", "/operations/metrics"} {
			w := enterpriseAdminRequest(r, "GET", path, "")
			require.Equal(t, tc.want, w.Code, w.Body.String())
		}
		require.Empty(t, repo.calls)
	}
}

func TestEnterpriseAdminHTTPStatusOnlyAndUnsafeBodiesCannotBypassGuardedOperation(t *testing.T) {
	for _, body := range []string{`{"status":"suspended"}`, strings.TrimSuffix(enterpriseAdminStatusBody, "}") + `,"secret":"audit-canary"}`, enterpriseAdminStatusBody + `{}`, strings.TrimSuffix(enterpriseAdminStatusBody, "}") + `,"padding":"` + strings.Repeat("a", 17000) + `"}`} {
		repo := &enterpriseAdminHTTPRepo{}
		r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
		w := enterpriseAdminRequest(r, "PATCH", "/workspaces/12/status", body)
		require.Contains(t, []int{400, 413}, w.Code, w.Body.String())
		require.Empty(t, repo.calls)
		require.NotContains(t, w.Body.String(), "audit-canary")
	}
	// A true guard without trusted proof must still be rejected by the service.
	repo := &enterpriseAdminHTTPRepo{}
	r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", func(c *gin.Context) bool {
		c.Request = c.Request.WithContext(service.WithSessionAuthentication(c.Request.Context(), service.SessionAuthentication{AuthMethod: "password", AuthenticatedAt: time.Now().Add(-time.Hour)}))
		return true
	})
	w := enterpriseAdminRequest(r, "PATCH", "/workspaces/12/status", enterpriseAdminStatusBody)
	require.Equal(t, 403, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "RECENT_AUTH_REQUIRED")
	require.Empty(t, repo.calls)
}

func TestEnterpriseAdminHTTPStatusResultAndCommittedInspectionFailure(t *testing.T) {
	repo := &enterpriseAdminHTTPRepo{inspectErr: errors.New("sql/provider/audit-canary")}
	r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
	w := enterpriseAdminRequest(r, "PATCH", "/workspaces/12/status", enterpriseAdminStatusBody)
	require.Equal(t, 502, w.Code, w.Body.String())
	require.Equal(t, "c3d86901-682f-4f99-839b-5232490c8611", w.Header().Get("X-Admin-Operation-Receipt-ID"))
	require.Contains(t, w.Body.String(), "ADMIN_OPERATION_RESULT_UNAVAILABLE")
	require.Contains(t, w.Body.String(), `"mutation_committed":"true"`)
	require.NotContains(t, w.Body.String(), "audit-canary")
	require.Equal(t, 1, repo.mutations)
	repo.inspectErr = nil
	w = enterpriseAdminRequest(r, "PATCH", "/workspaces/12/status", enterpriseAdminStatusBody)
	require.Equal(t, 200, w.Code, w.Body.String())
	data := enterpriseAdminData(t, w)
	require.Equal(t, "suspended", data["status"])
	require.Equal(t, "2026-10-09T01:02:03.000000456Z", data["updated_at"])
	require.Equal(t, "Tenant", data["name"])
	require.Empty(t, data["permissions"])
	require.NotContains(t, data, "receipt")
	require.Equal(t, 1, repo.mutations, "same-key replay must not create a second transition")
	w = enterpriseAdminRequest(r, "POST", "/workspaces/12/webhooks/23/deliveries/34/retry", enterpriseAdminRetryBody)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "retry_webhook", enterpriseAdminData(t, w)["action"])
	require.NotContains(t, w.Body.String(), "manual retry")
	for _, path := range []string{"/workspaces/01/webhooks/23/deliveries/34/retry", "/workspaces/12/webhooks/+23/deliveries/34/retry", "/workspaces/12/webhooks/23/deliveries/034/retry"} {
		before := len(repo.calls)
		w = enterpriseAdminRequest(r, "POST", path, enterpriseAdminRetryBody)
		require.Equal(t, 400, w.Code, w.Body.String())
		require.Len(t, repo.calls, before)
	}
}

func TestEnterpriseAdminHTTPConcurrentRequestsAreBoundedAndReleaseSlots(t *testing.T) {
	repo := &enterpriseAdminHTTPRepo{block: true, entered: make(chan struct{}, 4)}
	r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/api/v1/admin/operations/jobs", nil).WithContext(ctx)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
		}()
	}
	for range 4 {
		select {
		case <-repo.entered:
		case <-time.After(2 * time.Second):
			t.Fatal("four admitted requests did not reach their bounded source")
		}
	}
	w := enterpriseAdminRequest(r, "GET", "/operations/jobs", "")
	require.Equal(t, 429, w.Code, w.Body.String())
	require.Equal(t, "1", w.Header().Get("Retry-After"))
	require.Contains(t, w.Body.String(), "ADMIN_DIAGNOSTICS_BUSY")
	repo.mu.Lock()
	require.Len(t, repo.calls, 4)
	repo.mu.Unlock()
	cancel()
	wg.Wait()
	repo.block = false
	w = enterpriseAdminRequest(r, "GET", "/operations/jobs", "")
	require.Equal(t, 200, w.Code, w.Body.String())
}

func TestEnterpriseAdminHTTPSanitizesBackendErrorsAndTimeouts(t *testing.T) {
	for _, tc := range []struct {
		err    error
		want   int
		reason string
	}{{errors.New("SQL password=audit-canary"), 500, "ADMIN_DIAGNOSTICS_UNAVAILABLE"}, {sql.ErrNoRows, 500, "ADMIN_DIAGNOSTICS_UNAVAILABLE"}, {context.DeadlineExceeded, 504, "ADMIN_DIAGNOSTICS_TIMEOUT"}, {service.ErrAdminOperationIdempotencyConflict, 409, "ADMIN_OPERATION_IDEMPOTENCY_CONFLICT"}} {
		repo := &enterpriseAdminHTTPRepo{err: tc.err}
		r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
		w := enterpriseAdminRequest(r, "GET", "/operations/jobs", "")
		require.Equal(t, tc.want, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), tc.reason)
		require.NotContains(t, w.Body.String(), "audit-canary")
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
	for _, tc := range []struct {
		err  error
		want int
		code string
	}{{errors.New("SQL password=audit-canary"), 500, "ADMIN_OPERATION_FAILED"}, {service.ErrAdminOperationIdempotencyConflict, 409, "ADMIN_OPERATION_IDEMPOTENCY_CONFLICT"}, {service.ErrWorkspaceConflict, 409, "WORKSPACE_CONFLICT"}, {context.DeadlineExceeded, 504, "ADMIN_DIAGNOSTICS_TIMEOUT"}} {
		repo := &enterpriseAdminHTTPRepo{err: tc.err}
		r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
		for _, request := range []struct{ method, path, body string }{{"PATCH", "/workspaces/12/status", enterpriseAdminStatusBody}, {"POST", "/workspaces/12/webhooks/23/deliveries/34/retry", enterpriseAdminRetryBody}} {
			w := enterpriseAdminRequest(r, request.method, request.path, request.body)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			require.Contains(t, w.Body.String(), tc.code)
			require.NotContains(t, w.Body.String(), "audit-canary")
		}
		require.Zero(t, repo.mutations)
	}
	repo := &enterpriseAdminHTTPRepo{block: true}
	r := enterpriseAdminTestRouter(repo, &enterpriseAdminHuman, "admin", "jwt", enterpriseAdminProof)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("GET", "/api/v1/admin/operations/jobs", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 504, w.Code, w.Body.String())
}
