package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

type workspaceWSBillingRepo struct{ service.UsageBillingRepository }

func (r workspaceWSBillingRepo) ApplyTenantUsage(ctx context.Context, cmd *service.UsageBillingCommand, _ *service.UsageLog) (*service.UsageBillingApplyResult, error) {
	return r.Apply(ctx, cmd)
}

func (workspaceWSBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	if cmd.WorkspaceID <= 0 || cmd.ProjectID <= 0 || cmd.BillingPrincipalUserID <= 0 || cmd.BudgetReservationID == "" {
		return nil, service.ErrBudgetReservationInvalid
	}
	return &service.UsageBillingApplyResult{Applied: true}, nil
}

func TestWorkspaceWSTwoTurnsRetainDifferentBudgetSnapshots(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			repo := &handlerTenantBudgetRepo{}
			result := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{firstPayload: `{"type":"response.create","model":"gpt-5.2"}`, secondPayload: `{"type":"response.create","model":"gpt-5.2"}`, tenant: &service.TenantContext{WorkspaceID: 12, ProjectID: 13, BillingPrincipalUserID: 1701}, tenantResolver: &workspaceWSGate{}, ingressMode: mode, budgetRepo: repo})
			require.Len(t, result.logs, 2)
			require.NotNil(t, result.logs[0].BudgetReservationID)
			require.NotNil(t, result.logs[1].BudgetReservationID)
			require.NotEqual(t, *result.logs[0].BudgetReservationID, *result.logs[1].BudgetReservationID)
			repo.mu.Lock()
			defer repo.mu.Unlock()
			require.Equal(t, 2, repo.reserved)
		})
	}
}
func TestWorkspaceWSSecondTurnBudgetDeniedBeforeForward(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			repo := &handlerTenantBudgetRepo{rejectAfter: 1}
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{firstPayload: `{"type":"response.create","model":"gpt-5.2"}`, secondPayload: `{"type":"response.create","model":"gpt-5.2"}`, tenant: &service.TenantContext{WorkspaceID: 12, ProjectID: 13, BillingPrincipalUserID: 1701}, tenantResolver: &workspaceWSGate{}, ingressMode: mode, budgetRepo: repo, secondTurnCloseExpected: true, closeReason: "PROJECT_BUDGET_EXCEEDED"})
		})
	}
}
func TestWorkspaceModelCatalogIntersection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/models", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{Tenant: &service.TenantContext{AllowedModels: []string{"allowed"}}})
		writeModelsListResponse(c, []gin.H{{"id": "allowed", "owner": "upstream"}, {"id": "denied"}})
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/models", nil))
	require.Equal(t, 200, w.Code)
	require.Contains(t, w.Body.String(), "allowed")
	require.NotContains(t, w.Body.String(), "denied")
	require.Contains(t, w.Body.String(), "upstream")
}

type workspaceWSGate struct {
	archived     atomic.Bool
	payerChanged atomic.Bool
	models       []string
}

func (g *workspaceWSGate) ResolveTenant(_ context.Context, k *service.APIKey) (*service.TenantContext, error) {
	if g.archived.Load() {
		return nil, service.ErrWorkspaceConflict
	}
	k.Status = service.StatusActive
	payer := int64(1701)
	if g.payerChanged.Load() {
		payer = 999
	}
	return &service.TenantContext{WorkspaceID: 12, ProjectID: 13, BillingPrincipalUserID: payer, AllowedModels: g.models}, nil
}
func TestWorkspaceWSFirstFrameProjectModelDenied(t *testing.T) {
	gate := &workspaceWSGate{models: []string{"gpt-allowed"}}
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{firstPayload: `{"type":"response.create","model":"gpt-other"}`, tenant: &service.TenantContext{WorkspaceID: 12, ProjectID: 13, BillingPrincipalUserID: 1701, AllowedModels: []string{"gpt-allowed"}}, tenantResolver: gate, firstFrameCloseExpected: true})
}
func TestWorkspaceWSArchivedBetweenTurnsDenied(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModeDedicated, service.OpenAIWSIngressModePassthrough} {
		t.Run(mode, func(t *testing.T) {
			gate := &workspaceWSGate{}
			runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{firstPayload: `{"type":"response.create","model":"gpt-allowed"}`, secondPayload: `{"type":"response.create","model":"gpt-allowed"}`, tenant: &service.TenantContext{WorkspaceID: 12, ProjectID: 13, BillingPrincipalUserID: 1701}, tenantResolver: gate, ingressMode: mode, closeReason: "tenant access denied", secondTurnCloseExpected: true, afterFirstUpstreamRequest: func(*service.ChannelService) error { gate.archived.Store(true); return nil }})
		})
	}
}
