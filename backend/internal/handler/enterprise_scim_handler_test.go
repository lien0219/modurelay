package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	rate "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type scimHTTPRepo struct {
	service.SCIMRepository
	mutations     []service.SCIMUserMutation
	outcomes      []string
	successes     []bool
	authOverride  bool
	recordError   error
	authPrincipal *service.SCIMPrincipal
}

func (r *scimHTTPRepo) Authenticate(_ context.Context, id string, _ []byte, _ time.Time) (*service.SCIMPrincipal, error) {
	if r.authOverride {
		return r.authPrincipal, nil
	}
	if id != strings.Repeat("p", 43) {
		return nil, service.NewSCIMError(401, "", "invalid provisioning credential")
	}
	return &service.SCIMPrincipal{WorkspaceID: 1, ConnectorID: 2, TokenID: 3, ConnectorRevision: 1, PublicID: id}, nil
}
func (r *scimHTTPRepo) GetUser(context.Context, *service.SCIMPrincipal, string) (*service.SCIMUser, error) {
	return &service.SCIMUser{Schemas: []string{service.SCIMUserSchema}, ID: strings.Repeat("u", 43), UserName: "person@example.com", Active: true, Emails: []service.SCIMEmail{{Value: "person@example.com", Primary: true}}, Revision: 7}, nil
}
func (r *scimHTTPRepo) ListUsers(context.Context, *service.SCIMPrincipal, service.SCIMListQuery) ([]service.SCIMUser, int, error) {
	return []service.SCIMUser{}, 0, nil
}
func (r *scimHTTPRepo) MutateUser(_ context.Context, _ *service.SCIMPrincipal, _ string, m service.SCIMUserMutation) (*service.SCIMUser, error) {
	r.mutations = append(r.mutations, m)
	if m.IfMatch == 6 {
		return nil, service.NewSCIMError(412, "", "resource version changed")
	}
	return r.GetUser(context.Background(), nil, "")
}
func (r *scimHTTPRepo) RecordSyncOutcome(_ context.Context, _ *service.SCIMPrincipal, ok bool, code string) error {
	r.outcomes = append(r.outcomes, code)
	r.successes = append(r.successes, ok)
	return r.recordError
}

type scimHTTPLimiter struct {
	err     error
	allowed bool
	keys    []string
	limits  []int
	denyAt  int
}

func (l *scimHTTPLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (rate.AllowResult, error) {
	l.keys = append(l.keys, key)
	l.limits = append(l.limits, limit)
	allowed := l.allowed
	if l.denyAt > 0 && len(l.keys) == l.denyAt {
		allowed = false
	}
	return rate.AllowResult{Allowed: allowed, RetryAfter: time.Minute}, l.err
}

func scimHTTPFixture(repo *scimHTTPRepo, limiter *scimHTTPLimiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewEnterpriseSCIMHandler(service.NewEnterpriseSCIMService(repo), limiter, "https://console.example.com/api/v1/auth/sso/callback").Register(r)
	return r
}
func scimHTTPRequest(r *gin.Engine, method, path, body, token, etag string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/scim/v2/"+strings.Repeat("p", 43)+path, strings.NewReader(body))
	req.Host = "untrusted.example"
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", "application/scim+json")
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestSCIMHTTPClientFixtureMediaAuthSchemasAndVersions(t *testing.T) {
	repo := &scimHTTPRepo{}
	limit := &scimHTTPLimiter{allowed: true}
	r := scimHTTPFixture(repo, limit)
	token := "Bearer mrc_scim_" + strings.Repeat("t", 43)
	for _, path := range []string{"/ServiceProviderConfig", "/Schemas", "/ResourceTypes", "/Users?count=0", "/Users/" + strings.Repeat("u", 43)} {
		w := scimHTTPRequest(r, http.MethodGet, path, "", token, "")
		require.Equal(t, 200, w.Code, w.Body.String())
		require.Equal(t, "application/scim+json", w.Header().Get("Content-Type"))
		require.NotContains(t, w.Body.String(), "untrusted.example")
	}
	w := scimHTTPRequest(r, "GET", "/ServiceProviderConfig", "", "", "")
	require.Equal(t, 401, w.Code)
	require.Contains(t, w.Body.String(), service.SCIMErrorSchema)
	w = scimHTTPRequest(r, "GET", "/Users/"+strings.Repeat("u", 43), "", token, "")
	require.Equal(t, `W/"7"`, w.Header().Get("ETag"))
	require.Equal(t, "https://console.example.com/scim/v2/"+strings.Repeat("p", 43)+"/Users/"+strings.Repeat("u", 43), w.Header().Get("Location"))
	body := `{"schemas":["` + service.SCIMPatchSchema + `"],"Operations":[{"op":"Replace","path":"active","value":false}]}`
	w = scimHTTPRequest(r, "PATCH", "/Users/"+strings.Repeat("u", 43), body, token, `W/"7"`)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.EqualValues(t, 7, repo.mutations[0].IfMatch)
	require.NotNil(t, repo.mutations[0].Patch)
	w = scimHTTPRequest(r, "PATCH", "/Users/"+strings.Repeat("u", 43), body, token, `W/"6"`)
	require.Equal(t, 412, w.Code)
	require.Contains(t, w.Body.String(), service.SCIMErrorSchema)
	w = scimHTTPRequest(r, "DELETE", "/Users/"+strings.Repeat("u", 43), "", token, "")
	require.Equal(t, 204, w.Code)
	require.Empty(t, w.Body.String())
	for _, key := range limit.keys {
		require.NotContains(t, key, "mrc_scim_")
	}
	require.Equal(t, []int{600, 300, 150}, limit.limits[:3])
}
func TestSCIMHTTPRejectsAmbiguousPayloadAndFailsClosedWithoutLimiter(t *testing.T) {
	token := "Bearer mrc_scim_" + strings.Repeat("t", 43)
	missing := gin.New()
	NewEnterpriseSCIMHandler(service.NewEnterpriseSCIMService(&scimHTTPRepo{}), nil, "https://console.example.com/callback").Register(missing)
	wMissing := scimHTTPRequest(missing, "GET", "/Schemas", "", token, "")
	require.Equal(t, 503, wMissing.Code)
	for _, limit := range []*scimHTTPLimiter{{allowed: false}, {err: errors.New("Redis unavailable")}} {
		w := scimHTTPRequest(scimHTTPFixture(&scimHTTPRepo{}, limit), "GET", "/Schemas", "", token, "")
		if limit.err != nil {
			require.Equal(t, 503, w.Code)
		} else {
			require.Equal(t, 429, w.Code)
			require.Equal(t, "60", w.Header().Get("Retry-After"))
		}
		require.Contains(t, w.Body.String(), service.SCIMErrorSchema)
	}
	r := scimHTTPFixture(&scimHTTPRepo{}, &scimHTTPLimiter{allowed: true})
	for _, body := range []string{`{"schemas":[],"workspace_id":2}`, `{"active":true,"active":false}`, strings.Repeat(" ", service.SCIMMaxPayloadBytes+1)} {
		w := scimHTTPRequest(r, "POST", "/Users", body, token, "")
		require.Equal(t, 400, w.Code, w.Body.String())
		var data map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &data))
		require.Contains(t, data["schemas"], service.SCIMErrorSchema)
	}
}

func TestSCIMHTTPHonestCapabilitiesAndUnsupportedMutations(t *testing.T) {
	r := scimHTTPFixture(&scimHTTPRepo{}, &scimHTTPLimiter{allowed: true})
	token := "Bearer mrc_scim_" + strings.Repeat("t", 43)
	w := scimHTTPRequest(r, "GET", "/ServiceProviderConfig", "", token, "")
	var data map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &data))
	for _, name := range []string{"patch", "filter", "etag"} {
		capability, ok := data[name].(map[string]any)
		require.True(t, ok)
		require.Equal(t, true, capability["supported"])
	}
	for _, name := range []string{"bulk", "sort", "changePassword"} {
		capability, ok := data[name].(map[string]any)
		require.True(t, ok)
		require.Equal(t, false, capability["supported"])
	}
	for _, badToken := range []string{"Bearer mrc_scim_short", "Basic ignored", "Bearer " + strings.Repeat("t", 52)} {
		w = scimHTTPRequest(r, "GET", "/Schemas", "", badToken, "")
		require.Equal(t, 401, w.Code)
	}
	w = scimHTTPRequest(r, "POST", "/Bulk", `{}`, token, "")
	require.Equal(t, 501, w.Code)
	w = scimHTTPRequest(r, "GET", "/unknown/nested/path", "", token, "")
	require.Equal(t, 404, w.Code)
	require.Contains(t, w.Body.String(), service.SCIMErrorSchema)
	w = scimHTTPRequest(r, "PUT", "/ServiceProviderConfig", `{}`, token, "")
	require.Equal(t, 404, w.Code)
	require.Contains(t, w.Body.String(), service.SCIMErrorSchema)
	w = scimHTTPRequest(r, "POST", "/Groups", `{"schemas":["`+service.SCIMGroupSchema+`"],"displayName":"Engineering","role":"owner"}`, token, "")
	require.Equal(t, 400, w.Code)
	for _, callback := range []string{"", "http://console.example.com/callback", "https://user:pass@console.example.com/callback", "https://console.example.com/callback?q=1"} {
		_, err := SCIMBaseURL(strings.Repeat("p", 43), callback)
		require.Error(t, err)
	}
}

func TestSCIMHTTPAuthenticatedValidationRecordsExactlyOneSafeOutcome(t *testing.T) {
	token := "Bearer mrc_scim_" + strings.Repeat("t", 43)
	for _, tc := range []struct {
		name, method, path, body, media, etag, code string
		status                                      int
	}{{"media", "POST", "/Users", `{}`, "text/plain", "", "sync_failed", 415}, {"duplicateJSON", "PUT", "/Users/" + strings.Repeat("u", 43), `{"active":true,"active":false}`, "application/json", "", "invalidValue", 400}, {"schema", "POST", "/Users", `{"schemas":["unsupported"],"userName":"private@example.com","emails":[{"value":"private@example.com","primary":true}]}`, scimMediaType, "", "invalidValue", 400}, {"attribute", "POST", "/Groups", `{"schemas":["` + service.SCIMGroupSchema + `"],"displayName":"Private","role":"owner"}`, scimMediaType, "", "invalidValue", 400}, {"id", "DELETE", "/Users/invalid", "", scimMediaType, "", "not_found", 404}, {"etag", "DELETE", "/Users/" + strings.Repeat("u", 43), "", scimMediaType, "invalid", "invalidValue", 400}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &scimHTTPRepo{}
			r := scimHTTPFixture(repo, &scimHTTPLimiter{allowed: true})
			for range 3 {
				req := httptest.NewRequest(tc.method, "/scim/v2/"+strings.Repeat("p", 43)+tc.path, strings.NewReader(tc.body))
				req.Header.Set("Authorization", token)
				req.Header.Set("Content-Type", tc.media)
				if tc.etag != "" {
					req.Header.Set("If-Match", tc.etag)
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				require.Equal(t, tc.status, w.Code, w.Body.String())
			}
			require.Equal(t, []string{tc.code, tc.code, tc.code}, repo.outcomes)
			require.Equal(t, []bool{false, false, false}, repo.successes)
			require.Empty(t, repo.mutations)
			out, e := json.Marshal(repo.outcomes)
			require.NoError(t, e)
			require.NotContains(t, string(out), "private@example.com")
			require.NotContains(t, string(out), "mrc_scim_")
		})
	}
	repo := &scimHTTPRepo{}
	r := scimHTTPFixture(repo, &scimHTTPLimiter{allowed: true})
	scimHTTPRequest(r, "GET", "/Users/invalid", "", token, "")
	scimHTTPRequest(r, "POST", "/Users", `{}`, "", "")
	require.Empty(t, repo.outcomes)
	body := `{"schemas":["` + service.SCIMUserSchema + `"],"userName":"person","emails":[{"value":"person@example.com","primary":true}]}`
	w := scimHTTPRequest(r, "POST", "/Users", body, token, "")
	require.Equal(t, 201, w.Code)
	require.Equal(t, []string{""}, repo.outcomes)
	require.Equal(t, []bool{true}, repo.successes)
	w = scimHTTPRequest(r, "POST", "/Bulk", `{}`, token, "")
	require.Equal(t, 501, w.Code)
	require.Equal(t, []string{"", "sync_failed"}, repo.outcomes)
	for _, limit := range []*scimHTTPLimiter{{allowed: false}, {err: errors.New("unavailable")}} {
		repo := &scimHTTPRepo{}
		scimHTTPRequest(scimHTTPFixture(repo, limit), "POST", "/Users", `{}`, token, "")
		require.Empty(t, repo.outcomes)
	}
}
func TestSCIMHTTPMalformedPrincipalNeverPanics(t *testing.T) {
	for _, value := range []any{nil, "wrong", (*service.SCIMPrincipal)(nil), &service.SCIMPrincipal{}} {
		t.Run(fmt.Sprintf("%T-%v", value, value), func(t *testing.T) {
			repo := &scimHTTPRepo{}
			h := NewEnterpriseSCIMHandler(service.NewEnterpriseSCIMService(repo), &scimHTTPLimiter{allowed: true}, "https://console.example.com/callback")
			r := gin.New()
			r.POST("/Users", func(c *gin.Context) {
				if value != nil {
					c.Set("scim_principal", value)
				}
			}, h.resources("Users", "create"))
			w := httptest.NewRecorder()
			require.NotPanics(t, func() { r.ServeHTTP(w, httptest.NewRequest("POST", "/Users", strings.NewReader(`{}`))) })
			require.Equal(t, 401, w.Code)
			require.Empty(t, repo.outcomes)
		})
	}
}

func TestSCIMHTTPMalformedAuthenticatedPrincipalAndRateLimitsStaySilent(t *testing.T) {
	token := "Bearer mrc_scim_" + strings.Repeat("t", 43)
	for _, p := range []*service.SCIMPrincipal{nil, {}, {WorkspaceID: 1, ConnectorID: 2, TokenID: 3, ConnectorRevision: 1, PublicID: "invalid"}} {
		repo := &scimHTTPRepo{authOverride: true, authPrincipal: p}
		engine := scimHTTPFixture(repo, &scimHTTPLimiter{allowed: true})
		response := httptest.NewRecorder()
		require.NotPanics(t, func() { response = scimHTTPRequest(engine, "POST", "/Users", `{}`, token, "") })
		require.Equal(t, 401, response.Code)
		require.Empty(t, repo.outcomes)
	}
	for _, deny := range []int{1, 2, 3} {
		repo := &scimHTTPRepo{}
		response := scimHTTPRequest(scimHTTPFixture(repo, &scimHTTPLimiter{allowed: true, denyAt: deny}), "POST", "/Users", `{}`, token, "")
		require.Equal(t, 429, response.Code)
		require.Empty(t, repo.outcomes, "IP/connector/token rate rejection must not create sync failures")
	}
}

func TestSCIMHTTPSuccessIsPreservedWhenOutcomeObserverFails(t *testing.T) {
	repo := &scimHTTPRepo{recordError: errors.New("internal observer failure")}
	engine := scimHTTPFixture(repo, &scimHTTPLimiter{allowed: true})
	token := "Bearer mrc_scim_" + strings.Repeat("t", 43)
	body := `{"schemas":["` + service.SCIMUserSchema + `"],"userName":"person","emails":[{"value":"person@example.com","primary":true}]}`
	created := scimHTTPRequest(engine, "POST", "/Users", body, token, "")
	require.Equal(t, 201, created.Code)
	require.NotContains(t, created.Body.String(), "internal observer failure")
	deleted := scimHTTPRequest(engine, "DELETE", "/Users/"+strings.Repeat("u", 43), "", token, "")
	require.Equal(t, 204, deleted.Code)
	require.Empty(t, deleted.Body.String())
	require.Equal(t, []string{"", ""}, repo.outcomes)
	require.Equal(t, []bool{true, true}, repo.successes)
}
