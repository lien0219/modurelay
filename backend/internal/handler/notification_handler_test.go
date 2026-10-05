package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type notificationHTTPRepo struct {
	service.NotificationRepository
	userIDs                []int64
	filter                 service.NotificationListFilter
	workspaceID, projectID *int64
	markedID               int64
	requestContext         context.Context
}

func (r *notificationHTTPRepo) List(ctx context.Context, userID int64, f service.NotificationListFilter) ([]service.UserNotification, int64, error) {
	r.userIDs = append(r.userIDs, userID)
	r.filter = f
	r.requestContext = ctx
	return []service.UserNotification{{ID: 11, RecipientUserID: userID, Category: "budget", WorkspaceID: f.WorkspaceID, ProjectID: f.ProjectID}}, 31, nil
}
func (r *notificationHTTPRepo) UnreadCount(_ context.Context, userID int64, w, p *int64) (int64, error) {
	r.userIDs = append(r.userIDs, userID)
	r.workspaceID = w
	r.projectID = p
	return 3, nil
}
func (r *notificationHTTPRepo) MarkRead(_ context.Context, userID, id int64) error {
	r.userIDs = append(r.userIDs, userID)
	r.markedID = id
	if id == 99 {
		return infraerrors.NotFound("NOTIFICATION_NOT_FOUND", "notification not found")
	}
	return nil
}
func (r *notificationHTTPRepo) MarkAllRead(_ context.Context, userID int64, w, p *int64) (int64, error) {
	r.userIDs = append(r.userIDs, userID)
	r.workspaceID = w
	r.projectID = p
	return 3, nil
}

func notificationRouter(repo service.NotificationRepository, authenticated bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if authenticated {
		r.Use(func(c *gin.Context) { c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7}) })
	}
	h := NewNotificationHandler(service.NewNotificationCenterService(repo))
	r.GET("/notifications", h.List)
	r.GET("/notifications/unread-count", h.UnreadCount)
	r.POST("/notifications/:id/read", h.MarkRead)
	r.POST("/notifications/read-all", h.MarkAllRead)
	return r
}

func TestNotificationHTTPAlwaysUsesAuthenticatedRecipient(t *testing.T) {
	repo := &notificationHTTPRepo{}
	r := notificationRouter(repo, true)
	for _, tc := range []struct{ method, path string }{
		{"GET", "/notifications?recipient_user_id=99&workspace_id=11&project_id=13&category=budget&unread=false&page=2&page_size=10"},
		{"GET", "/notifications/unread-count?recipient_user_id=99&workspace_id=11&project_id=13"},
		{"POST", "/notifications/11/read?recipient_user_id=99"},
		{"POST", "/notifications/read-all?recipient_user_id=99&workspace_id=11&project_id=13"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"recipient_user_id":99}`))
		r.ServeHTTP(w, req)
		require.Equal(t, 200, w.Code, w.Body.String())
	}
	require.Equal(t, []int64{7, 7, 7, 7}, repo.userIDs)
	require.Equal(t, "budget", repo.filter.Category)
	require.NotNil(t, repo.filter.Unread)
	require.False(t, *repo.filter.Unread)
	require.Equal(t, int64(11), *repo.filter.WorkspaceID)
	require.Equal(t, int64(13), *repo.filter.ProjectID)
	require.Equal(t, 2, repo.filter.Page)
	require.Equal(t, 10, repo.filter.PageSize)
	require.Equal(t, int64(11), repo.markedID)
	require.Equal(t, int64(11), *repo.workspaceID)
	require.Equal(t, int64(13), *repo.projectID)
}

func TestNotificationHTTPPaginationMetadataMatchesBoundedQuery(t *testing.T) {
	for _, tc := range []struct {
		query string
		size  int
	}{{"", 20}, {"?page_size=500", 100}} {
		repo := &notificationHTTPRepo{}
		r := notificationRouter(repo, true)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/notifications"+tc.query, nil))
		require.Equal(t, 200, w.Code)
		var response struct {
			Data struct {
				Page     int                        `json:"page"`
				PageSize int                        `json:"page_size"`
				Total    int64                      `json:"total"`
				Items    []service.UserNotification `json:"items"`
			}
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		require.Equal(t, 1, response.Data.Page)
		require.Equal(t, tc.size, response.Data.PageSize)
		require.Equal(t, tc.size, repo.filter.PageSize)
		require.Nil(t, repo.filter.Unread)
		require.Equal(t, int64(31), response.Data.Total)
		require.Len(t, response.Data.Items, 1)
	}
}

func TestNotificationHTTPInvalidFiltersAndIDsAreRejected(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{"GET", "/notifications?unread=garbage"}, {"GET", "/notifications?workspace_id=-1"}, {"GET", "/notifications?project_id=bad"},
		{"GET", "/notifications/unread-count?workspace_id=0"}, {"GET", "/notifications/unread-count?project_id=bad"},
		{"POST", "/notifications/0/read"}, {"POST", "/notifications/bad/read"}, {"POST", "/notifications/read-all?project_id=-1"},
	} {
		repo := &notificationHTTPRepo{}
		r := notificationRouter(repo, true)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, 400, w.Code, tc.path)
		require.Empty(t, repo.userIDs, tc.path)
	}
}

func TestNotificationHTTPUnauthenticatedAndForeignID(t *testing.T) {
	for _, tc := range []struct{ method, path string }{{"GET", "/notifications"}, {"GET", "/notifications/unread-count"}, {"POST", "/notifications/11/read"}, {"POST", "/notifications/read-all"}} {
		repo := &notificationHTTPRepo{}
		r := notificationRouter(repo, false)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, http.StatusUnauthorized, w.Code, tc.path)
		require.Empty(t, repo.userIDs)
	}
	repo := &notificationHTTPRepo{}
	r := notificationRouter(repo, true)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/notifications/99/read", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "NOTIFICATION_NOT_FOUND")
}

func TestNotificationHTTPPropagatesRequestContext(t *testing.T) {
	type key struct{}
	repo := &notificationHTTPRepo{}
	r := notificationRouter(repo, true)
	req := httptest.NewRequest("GET", "/notifications", nil).WithContext(context.WithValue(context.Background(), key{}, "request-value"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code)
	require.Equal(t, "request-value", repo.requestContext.Value(key{}))
}
