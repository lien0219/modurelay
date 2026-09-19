//go:build unit

package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type smsHandlerSettingRepoStub struct {
	values map[string]string
}

var _ service.SettingRepository = (*smsHandlerSettingRepoStub)(nil)

func (s *smsHandlerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (s *smsHandlerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *smsHandlerSettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

func (s *smsHandlerSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}

func (s *smsHandlerSettingRepoStub) SetMultiple(context.Context, map[string]string) error { return nil }

func (s *smsHandlerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}

func (s *smsHandlerSettingRepoStub) Delete(context.Context, string) error { return nil }

func newSMSHandlerBatchTestContext(t *testing.T, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/sms/orders/batch", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Idempotency-Key", "batch-test")
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	return c, recorder
}

func TestSMSHandlerPurchaseBatchLimitReturnsBadRequest(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	settingRepo := &smsHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeySMSPricingSettings: `{"batch_purchase_limit":2}`,
	}}
	mock.ExpectQuery(`SELECT id FROM sms_orders WHERE user_id=\$1 AND idempotency_key=\$2`).
		WithArgs(int64(42), "batch-test-0").
		WillReturnError(sql.ErrNoRows)
	svc := service.NewSMSService(db, service.NewSettingService(settingRepo, nil), nil)
	h := NewSMSHandler(svc)
	c, recorder := newSMSHandlerBatchTestContext(t, gin.H{"items": []map[string]any{{}, {}, {}}})

	h.PurchaseBatch(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Code     int               `json:"code"`
		Message  string            `json:"message"`
		Reason   string            `json:"reason"`
		Metadata map[string]string `json:"metadata"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Equal(t, "batch size must be between 1 and 2", response.Message)
	require.Equal(t, "BATCH_PURCHASE_LIMIT_EXCEEDED", response.Reason)
	require.Equal(t, "2", response.Metadata["max"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSMSHandlerSettingsReturnsConfiguredBatchPurchaseLimit(t *testing.T) {
	settingRepo := &smsHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeySMSPricingSettings: `{"batch_purchase_limit":7}`,
	}}
	svc := service.NewSMSService(nil, service.NewSettingService(settingRepo, nil), nil)
	h := NewSMSHandler(svc)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/sms/settings", nil)

	h.Settings(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Code int `json:"code"`
		Data struct {
			BatchPurchaseLimit int `json:"batch_purchase_limit"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 0, response.Code)
	require.Equal(t, 7, response.Data.BatchPurchaseLimit)
}

func validSMSPricingUpdatePayload() map[string]any {
	return map[string]any{
		"cost_multiplier":                   1.3,
		"fixed_markup":                      0,
		"unknown_grade_multiplier":          1,
		"unknown_grade_fixed_markup":        0,
		"temporary_expiry_minutes":          10,
		"self_service_cancel_after_minutes": 1,
		"grade_multipliers":                 map[string]float64{},
		"grade_fixed_markups":               map[string]float64{},
	}
}

func newSMSAdminPricingUpdateContext(t *testing.T, payload any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/sms/pricing", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestSMSHandlerAdminPricingUpdatePreservesOmittedBatchPurchaseLimit(t *testing.T) {
	settingRepo := &smsHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeySMSPricingSettings: `{"batch_purchase_limit":7}`,
	}}
	h := NewSMSHandler(service.NewSMSService(nil, service.NewSettingService(settingRepo, nil), nil))
	c, recorder := newSMSAdminPricingUpdateContext(t, validSMSPricingUpdatePayload())

	h.AdminPricingUpdate(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var saved service.SMSPricingSettings
	require.NoError(t, json.Unmarshal([]byte(settingRepo.values[service.SettingKeySMSPricingSettings]), &saved))
	require.Equal(t, 7, saved.BatchPurchaseLimit)
}

func TestSMSHandlerAdminPricingUpdateRejectsExplicitInvalidBatchPurchaseLimit(t *testing.T) {
	settingRepo := &smsHandlerSettingRepoStub{values: map[string]string{
		service.SettingKeySMSPricingSettings: `{"batch_purchase_limit":7}`,
	}}
	h := NewSMSHandler(service.NewSMSService(nil, service.NewSettingService(settingRepo, nil), nil))
	payload := validSMSPricingUpdatePayload()
	payload["batch_purchase_limit"] = 0
	c, recorder := newSMSAdminPricingUpdateContext(t, payload)

	h.AdminPricingUpdate(c)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var current map[string]any
	require.NoError(t, json.Unmarshal([]byte(settingRepo.values[service.SettingKeySMSPricingSettings]), &current))
	require.Equal(t, float64(7), current["batch_purchase_limit"])
}
