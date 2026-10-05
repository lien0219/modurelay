package handler

import (
	"net/http/httptest"
	"reflect"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMachineIdentityIsRetainedByModerationAndPromptAuditInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", nil)
	serviceAccountID := int64(31)
	apiKey := &service.APIKey{ID: 9, User: &service.User{ID: 99, Email: "creator@example.test"}, ServiceAccountID: &serviceAccountID}
	subject := middleware2.AuthSubject{PrincipalType: service.PrincipalTypeServiceAccount, ServiceAccountID: serviceAccountID, BillingUserID: 7}

	moderation := buildContentModerationInput(c, apiKey, subject, service.ContentModerationProtocolOpenAIResponses, "gpt-test", nil)
	moderationField := reflect.ValueOf(moderation).FieldByName("ServiceAccountID")
	require.True(t, moderationField.IsValid(), "moderation input must carry service_account_id")
	require.Equal(t, int64(31), moderationField.Int())
	require.Zero(t, moderation.UserID)
	require.Empty(t, moderation.UserEmail, "machine attribution must not couple to the creator email")

	auditRequest := buildSecurityAuditRequest(c, apiKey, subject, "openai_responses", "gpt-test", nil, "http")
	auditField := reflect.ValueOf(auditRequest).FieldByName("ServiceAccountID")
	require.True(t, auditField.IsValid(), "prompt audit request must carry service_account_id")
	require.Equal(t, int64(31), auditField.Int())
	require.Zero(t, auditRequest.UserID)
	require.Empty(t, auditRequest.UserEmail, "machine attribution must not couple to the creator email")
}

func TestMachineCyberPolicyEvidenceDoesNotAttributeCreator(t *testing.T) {
	meta := cyberPolicyOpsErrorMeta{UserID: 99, ServiceAccountID: 31, APIKeyID: 9}
	for _, entry := range []*service.OpsInsertErrorLogInput{
		buildCyberPolicyOpsErrorEntry(meta, &service.CyberPolicyMark{}),
		buildCyberSessionBlockedOpsEntry(meta),
	} {
		require.Nil(t, entry.UserID)
		require.EqualValues(t, 31, *entry.ServiceAccountID)
		require.EqualValues(t, 9, *entry.APIKeyID)
	}
}
