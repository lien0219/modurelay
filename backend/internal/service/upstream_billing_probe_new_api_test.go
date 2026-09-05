package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type newAPITestEncryptor struct{}

func (newAPITestEncryptor) Encrypt(plaintext string) (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

func (newAPITestEncryptor) Decrypt(ciphertext string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	return string(decoded), err
}

func configureNewAPITestToken(
	t *testing.T,
	svc *UpstreamBillingProbeService,
	account *Account,
	token string,
) {
	t.Helper()
	svc.secretEncryptor = newAPITestEncryptor{}
	svc.secretKeyConfigured = true
	stored, err := encryptNewAPIUserAccessToken(svc.secretEncryptor, true, token)
	require.NoError(t, err)
	account.Credentials[NewAPIUserAccessTokenCredentialKey] = stored
}

func newAPIProbeTestResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newAPIProbeTestAccount(id int64) *Account {
	rate := 0.25
	return &Account{
		ID:             id,
		Name:           "new-api-upstream",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Status:         StatusActive,
		Schedulable:    true,
		Concurrency:    2,
		RateMultiplier: &rate,
		Credentials: map[string]any{
			"api_key":                        "sk-new-api",
			"base_url":                       "https://new-api.example/v1",
			NewAPIUpstreamGroupCredentialKey: "vip",
		},
		Extra: map[string]any{
			UpstreamBillingProbeEnabledExtraKey:    true,
			UpstreamBillingRateSyncEnabledExtraKey: true,
		},
	}
}

func newAPIStatusTestBody(displayType string) string {
	return `{"success":true,"data":{"setup":true,"quota_per_unit":500000,"quota_display_type":"` + displayType + `","usd_exchange_rate":7.2,"custom_currency_exchange_rate":1.5}}`
}

func newAPILegacyStatusTestBody(displayInCurrency bool) string {
	return `{"success":true,"data":{"setup":true,"quota_per_unit":500000,"display_in_currency":` +
		map[bool]string{true: "true", false: "false"}[displayInCurrency] + `,"usd_exchange_rate":7.2}}`
}

func newAPIPricingTestBody() string {
	return `{"success":true,"group_ratio":{"vip":0.8,"default":1}}`
}

func newAPITokenUsageTestBody(granted, used, available float64, unlimited bool, expiresAt int64) string {
	return `{"code":true,"data":{"object":"token_usage","total_granted":` +
		formatNewAPITestNumber(granted) + `,"total_used":` + formatNewAPITestNumber(used) +
		`,"total_available":` + formatNewAPITestNumber(available) +
		`,"unlimited_quota":` + map[bool]string{true: "true", false: "false"}[unlimited] +
		`,"expires_at":` + formatNewAPITestInt(expiresAt) + `}}`
}

func formatNewAPITestNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatNewAPITestInt(value int64) string {
	return strconv.FormatInt(value, 10)
}

func TestNewAPIProbeUsesFiniteTokenQuotaBeforeWallet(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 5, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name               string
		availableQuota     float64
		wantRemaining      float64
		wantAutoUnschedule bool
	}{
		{name: "positive quota", availableQuota: 1_000_000, wantRemaining: 2},
		{name: "zero quota", availableQuota: 0, wantRemaining: 0, wantAutoUnschedule: true},
		{name: "negative quota", availableQuota: -500_000, wantRemaining: -1, wantAutoUnschedule: true},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newAPIProbeTestAccount(int64(300 + index))
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				newAPIProbeTestResponse(http.StatusNotFound, `{"error":"not found"}`),
				newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
				newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
				newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(5_000_000, 5_000_000-tt.availableQuota, tt.availableQuota, false, 0)),
			}}
			svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
			svc.now = func() time.Time { return fixedNow }

			snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

			require.NoError(t, err)
			require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Status)
			require.Equal(t, newAPIProviderName, snapshot.Data["provider"])
			require.Equal(t, "vip", snapshot.Data["selected_group"])
			require.Equal(t, true, snapshot.Data["group_selection_valid"])
			require.Equal(t, 0.8, snapshot.Data["resolved_rate_multiplier"])
			require.Equal(t, 0.8, *account.RateMultiplier)
			require.NotNil(t, snapshot.Balance)
			require.Equal(t, newAPIBalanceSourceToken, snapshot.Balance.Source)
			require.Equal(t, tt.wantRemaining, snapshot.Balance.Data["remaining"])
			require.Equal(t, false, snapshot.Balance.Data["unlimited"])
			require.Equal(t, tt.wantAutoUnschedule, snapshot.AutoUnschedulable)
			require.Len(t, upstream.requests, 4, "finite token quota must never query wallet endpoints")
			require.Equal(t, []string{
				"/v1/sub2api/billing",
				"/api/status",
				"/api/pricing",
				"/api/usage/token/",
			}, newAPIProbeRequestPaths(upstream.requests))
			require.Empty(t, upstream.requests[1].Header.Get("Authorization"))
			require.Empty(t, upstream.requests[2].Header.Get("Authorization"), "pricing only supports dashboard credentials, not relay API keys")
			require.Equal(t, "Bearer sk-new-api", upstream.requests[3].Header.Get("Authorization"))
			for _, request := range upstream.requests {
				require.True(t, HTTPUpstreamRedirectsDisabled(request.Context()))
				require.True(t, HTTPUpstreamResolvedIPPinningRequired(request.Context()))
			}
		})
	}
}

func TestNewAPIProbeUsesUserAccessTokenForUnlimitedTokenWallet(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 5, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name               string
		walletQuota        float64
		wantBalance        float64
		wantAutoUnschedule bool
	}{
		{name: "positive wallet", walletQuota: 4_100_000, wantBalance: 8.2},
		{name: "zero wallet", walletQuota: 0, wantBalance: 0, wantAutoUnschedule: true},
		{name: "negative wallet", walletQuota: -500_000, wantBalance: -1, wantAutoUnschedule: true},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newAPIProbeTestAccount(int64(305 + index))
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				newAPIProbeTestResponse(http.StatusNotFound, `{}`),
				newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"setup":true,"quota_per_unit":500000,"quota_display_type":"CNY","usd_exchange_rate":1}}`),
				newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
				newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
				newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"id":42,"quota":`+formatNewAPITestNumber(tt.walletQuota)+`}}`),
				newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"total":1,"items":[{"id":9,"key":"new-api"}]}}`),
			}}
			svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
			configureNewAPITestToken(t, svc, account, "persistent-user-pat")
			svc.now = func() time.Time { return fixedNow }

			snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

			require.NoError(t, err)
			require.NotNil(t, snapshot.Balance)
			require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
			require.Equal(t, newAPIBalanceSourceWallet, snapshot.Balance.Source)
			require.Equal(t, "wallet", snapshot.Balance.Data["mode"])
			require.Equal(t, "CNY", snapshot.Balance.Data["unit"])
			require.Equal(t, tt.wantBalance, snapshot.Balance.Data["balance"])
			require.Equal(t, tt.wantAutoUnschedule, snapshot.AutoUnschedulable)
			require.Equal(t, []string{
				"/v1/sub2api/billing",
				"/api/status",
				"/api/pricing",
				"/api/usage/token/",
				"/api/user/self",
				"/api/token/",
			}, newAPIProbeRequestPaths(upstream.requests))
			require.Equal(t, "Bearer sk-new-api", upstream.requests[3].Header.Get("Authorization"))
			require.Equal(t, "Bearer persistent-user-pat", upstream.requests[4].Header.Get("Authorization"))
			require.Equal(t, "Bearer persistent-user-pat", upstream.requests[5].Header.Get("Authorization"))
			require.Equal(t, "1", upstream.requests[5].URL.Query().Get("p"))
			require.Equal(t, "100", upstream.requests[5].URL.Query().Get("page_size"))
			require.Equal(t, true, snapshot.Balance.Data["ownership_verified"])
			encoded, marshalErr := json.Marshal(snapshot)
			require.NoError(t, marshalErr)
			require.NotContains(t, string(encoded), "persistent-user-pat")
		})
	}
}

func TestNewAPIProbeVerifiesMaskedTokenOwnership(t *testing.T) {
	account := newAPIProbeTestAccount(308)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"id":42,"quota":4100000}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"total":1,"items":[{"id":9,"key":"ne****pi"}]}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"key":"new-api"}}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
	configureNewAPITestToken(t, svc, account, "persistent-user-pat")

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, newAPIBalanceSourceWallet, snapshot.Balance.Source)
	require.Equal(t, true, snapshot.Balance.Data["ownership_verified"])
	require.Equal(t, []string{
		"/v1/sub2api/billing",
		"/api/status",
		"/api/pricing",
		"/api/usage/token/",
		"/api/user/self",
		"/api/token/",
		"/api/token/9/key",
	}, newAPIProbeRequestPaths(upstream.requests))
	require.Equal(t, http.MethodPost, upstream.requests[6].Method)
	require.Equal(t, "Bearer persistent-user-pat", upstream.requests[6].Header.Get("Authorization"))
}

func TestNewAPIProbeRetriesLegacyDashboardAuthentication(t *testing.T) {
	account := newAPIProbeTestAccount(309)
	account.Credentials[NewAPIUserIDCredentialKey] = int64(77)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
		newAPIProbeTestResponse(http.StatusUnauthorized, `{"success":false,"message":"unauthorized"}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"id":77,"quota":4100000}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"total":1,"items":[{"id":9,"key":"new-api"}]}}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
	configureNewAPITestToken(t, svc, account, "legacy-user-pat")

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, newAPIBalanceSourceWallet, snapshot.Balance.Source)
	require.Equal(t, "Bearer legacy-user-pat", upstream.requests[4].Header.Get("Authorization"))
	require.Empty(t, upstream.requests[4].Header.Get("New-Api-User"))
	for _, request := range upstream.requests[5:] {
		require.Equal(t, "legacy-user-pat", request.Header.Get("Authorization"))
		require.Equal(t, "77", request.Header.Get("New-Api-User"))
	}
}

func TestNewAPIProbeRejectsMismatchedWalletWithoutUnscheduling(t *testing.T) {
	account := newAPIProbeTestAccount(314)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"id":42,"quota":0}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"total":1,"items":[{"id":11,"key":"another-key"}]}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
	configureNewAPITestToken(t, svc, account, "wrong-account-pat")

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, newAPIBalanceSourceToken, snapshot.Balance.Source)
	require.Equal(t, true, snapshot.Balance.Data["unlimited"])
	require.Equal(t, newAPIWalletProbeStatusFailed, snapshot.Balance.Data["wallet_probe_status"])
	require.Equal(t, "token_owner_mismatch", snapshot.Balance.Data["wallet_probe_error"])
	require.False(t, snapshot.AutoUnschedulable)
	require.True(t, account.Schedulable)
}

func TestNewAPIProbeRejectsConfiguredUserIDMismatch(t *testing.T) {
	account := newAPIProbeTestAccount(317)
	account.Credentials[NewAPIUserIDCredentialKey] = int64(77)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"id":42,"quota":0}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
	configureNewAPITestToken(t, svc, account, "user-pat")

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.Equal(t, newAPIBalanceSourceToken, snapshot.Balance.Source)
	require.Equal(t, "user_mismatch", snapshot.Balance.Data["wallet_probe_error"])
	require.False(t, snapshot.AutoUnschedulable)
	require.Len(t, upstream.requests, 6, "a mismatched user ID must fail before token ownership requests")
}

func TestNewAPIDashboardAuthenticationOverridesAccountHeaders(t *testing.T) {
	account := newAPIProbeTestAccount(315)
	account.Credentials[credKeyHeaderOverrideEnabled] = true
	account.Credentials[credKeyHeaderOverrides] = map[string]any{
		"Authorization": "Bearer attacker",
		"New-Api-User":  "999",
	}
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{resp: newAPIProbeTestResponse(http.StatusOK, `{}`)}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

	result := svc.doNewAPIDashboardProbeRequest(
		context.Background(), http.MethodGet, account, "https://new-api.example", "/api/user/self",
		newAPIDashboardAuth{accessToken: "trusted-pat", userID: 42, legacy: true},
		"", HTTPUpstreamProfileOpenAI, nil, upstreamBalanceProbeMaxBodyBytes, time.Now(),
	)

	require.Empty(t, result.reason)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "trusted-pat", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "42", upstream.requests[0].Header.Get("New-Api-User"))
}

func TestNewAPIUserAccessTokenStorageFailsClosed(t *testing.T) {
	account := newAPIProbeTestAccount(316)

	plaintextService := &UpstreamBillingProbeService{
		secretEncryptor:     newAPITestEncryptor{},
		secretKeyConfigured: true,
	}
	account.Credentials[NewAPIUserAccessTokenCredentialKey] = "legacy-plaintext-pat"
	token, failure := plaintextService.newAPIUserAccessTokenFromAccount(account)
	require.Empty(t, token)
	require.Equal(t, "credential_reentry_required", failure.reason)

	account.Credentials[NewAPIUserAccessTokenCredentialKey] = newAPIUserAccessTokenCiphertextPrefix +
		base64.StdEncoding.EncodeToString([]byte("encrypted-pat"))
	unconfiguredService := &UpstreamBillingProbeService{secretEncryptor: newAPITestEncryptor{}}
	token, failure = unconfiguredService.newAPIUserAccessTokenFromAccount(account)
	require.Empty(t, token)
	require.Equal(t, "encryption_key_unavailable", failure.reason)

	account.Credentials[NewAPIUserAccessTokenCredentialKey] = newAPIUserAccessTokenCiphertextPrefix + "not-base64"
	token, failure = plaintextService.newAPIUserAccessTokenFromAccount(account)
	require.Empty(t, token)
	require.Equal(t, "credential_decrypt_failed", failure.reason)
}

func TestNewAPIProbeFallsBackFromUnlimitedTokenToWallet(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 5, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		displayType string
		wantBalance float64
	}{
		{displayType: "USD", wantBalance: 75},
		{displayType: "CNY", wantBalance: 75},
		{displayType: "TOKENS", wantBalance: 75},
		{displayType: "CUSTOM", wantBalance: 112.5},
	}

	for index, tt := range tests {
		t.Run(tt.displayType, func(t *testing.T) {
			account := newAPIProbeTestAccount(int64(310 + index))
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				newAPIProbeTestResponse(http.StatusNotFound, `{"error":"not found"}`),
				newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody(tt.displayType)),
				newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
				newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
				newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100,"hard_limit_usd":100,"system_hard_limit_usd":100}`),
				newAPIProbeTestResponse(http.StatusOK, `{"object":"list","total_usage":2500}`),
			}}
			svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
			svc.now = func() time.Time { return fixedNow }

			snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

			require.NoError(t, err)
			require.NotNil(t, snapshot.Balance)
			require.Equal(t, newAPIBalanceSourceWallet, snapshot.Balance.Source)
			require.Equal(t, tt.displayType, snapshot.Balance.Data["unit"])
			require.Equal(t, tt.wantBalance, snapshot.Balance.Data["balance"])
			require.Equal(t, true, snapshot.Balance.Data["is_valid"])
			require.False(t, snapshot.AutoUnschedulable)
			require.Equal(t, []string{
				"/v1/sub2api/billing",
				"/api/status",
				"/api/pricing",
				"/api/usage/token/",
				"/dashboard/billing/subscription",
				"/dashboard/billing/usage",
			}, newAPIProbeRequestPaths(upstream.requests))
			require.Empty(t, upstream.requests[2].Header.Get("Authorization"))
			for _, request := range upstream.requests[3:] {
				require.Equal(t, "Bearer sk-new-api", request.Header.Get("Authorization"))
			}
		})
	}
}

func TestNewAPIProbeFallsBackToWalletWhenTokenUsageIsUnsupported(t *testing.T) {
	account := newAPIProbeTestAccount(319)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPILegacyStatusTestBody(true)),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusNotFound, `{"error":"not found"}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100,"hard_limit_usd":100,"system_hard_limit_usd":100}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"list","total_usage":2500}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
	require.Equal(t, newAPIBalanceSourceWallet, snapshot.Balance.Source)
	require.Equal(t, "USD", snapshot.Balance.Data["unit"])
	require.Equal(t, 75.0, snapshot.Balance.Data["balance"])
	require.Equal(t, []string{
		"/v1/sub2api/billing",
		"/api/status",
		"/api/pricing",
		"/api/usage/token/",
		"/dashboard/billing/subscription",
		"/dashboard/billing/usage",
	}, newAPIProbeRequestPaths(upstream.requests))
}

func TestNewAPIProbeUsesUserWalletAfterLegacyUnlimitedSentinel(t *testing.T) {
	account := newAPIProbeTestAccount(351)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusNotFound, `{"error":"not found"}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"id":42,"quota":0}}`),
		newAPIProbeTestResponse(http.StatusOK, `{"success":true,"data":{"total":1,"items":[{"id":9,"key":"new-api"}]}}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
	configureNewAPITestToken(t, svc, account, "persistent-user-pat")

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
	require.Equal(t, newAPIBalanceSourceWallet, snapshot.Balance.Source)
	require.Equal(t, 0.0, snapshot.Balance.Data["balance"])
	require.Equal(t, true, snapshot.Balance.Data["ownership_verified"])
	require.True(t, snapshot.AutoUnschedulable)
	require.Equal(t, []string{
		"/v1/sub2api/billing",
		"/api/status",
		"/api/pricing",
		"/api/usage/token/",
		"/dashboard/billing/subscription",
		"/api/user/self",
		"/api/token/",
	}, newAPIProbeRequestPaths(upstream.requests))
	require.Equal(t, "Bearer sk-new-api", upstream.requests[4].Header.Get("Authorization"))
	require.Equal(t, "Bearer persistent-user-pat", upstream.requests[5].Header.Get("Authorization"))
	require.Equal(t, "Bearer persistent-user-pat", upstream.requests[6].Header.Get("Authorization"))
}

func TestNewAPIProbeKeepsLegacyUnlimitedSentinelWithoutUserToken(t *testing.T) {
	account := newAPIProbeTestAccount(352)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusNotFound, `{"error":"not found"}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
	require.Equal(t, newAPIBalanceSourceToken, snapshot.Balance.Source)
	require.Equal(t, true, snapshot.Balance.Data["unlimited"])
	require.Equal(t, true, snapshot.Balance.Data["is_valid"])
	require.Equal(t, newAPIWalletProbeStatusNotConfigured, snapshot.Balance.Data["wallet_probe_status"])
	require.False(t, snapshot.AutoUnschedulable)
	require.Len(t, upstream.requests, 5)
}

func TestNewAPIProbeKeepsUnlimitedWhenWalletCannotBeConfirmed(t *testing.T) {
	tests := []struct {
		name                 string
		subscriptionResponse *http.Response
	}{
		{
			name: "unlimited dashboard sentinel",
			subscriptionResponse: newAPIProbeTestResponse(http.StatusOK,
				`{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000}`),
		},
		{
			name:                 "dashboard authorization failure",
			subscriptionResponse: newAPIProbeTestResponse(http.StatusForbidden, `{"error":"forbidden"}`),
		},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newAPIProbeTestAccount(int64(320 + index))
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				newAPIProbeTestResponse(http.StatusNotFound, `{}`),
				newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
				newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
				newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
				tt.subscriptionResponse,
			}}
			svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

			snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

			require.NoError(t, err)
			require.NotNil(t, snapshot.Balance)
			require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
			require.Equal(t, newAPIBalanceSourceToken, snapshot.Balance.Source)
			require.Equal(t, true, snapshot.Balance.Data["unlimited"])
			require.Equal(t, newAPIWalletProbeStatusNotConfigured, snapshot.Balance.Data["wallet_probe_status"])
			require.False(t, snapshot.AutoUnschedulable)
			require.Len(t, upstream.requests, 5, "ambiguous wallet state must not request usage or create a finite balance")
		})
	}
}

func TestNewAPIProbeReportsUserWalletFailureWithoutDisabling(t *testing.T) {
	account := newAPIProbeTestAccount(329)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, newAPIPricingTestBody()),
		newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(0, 0, 0, true, 0)),
		newAPIProbeTestResponse(http.StatusUnauthorized, `{"success":false,"message":"unauthorized"}`),
		newAPIProbeTestResponse(http.StatusOK, `{"object":"billing_subscription","soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000}`),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})
	configureNewAPITestToken(t, svc, account, "expired-user-pat")

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.NotNil(t, snapshot.Balance)
	require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
	require.Equal(t, newAPIBalanceSourceToken, snapshot.Balance.Source)
	require.Equal(t, true, snapshot.Balance.Data["unlimited"])
	require.Equal(t, newAPIWalletProbeStatusFailed, snapshot.Balance.Data["wallet_probe_status"])
	require.Equal(t, "unauthorized", snapshot.Balance.Data["wallet_probe_error"])
	require.Equal(t, http.StatusUnauthorized, snapshot.Balance.Data["wallet_probe_http_status"])
	require.False(t, snapshot.AutoUnschedulable)
	require.Equal(t, []string{
		"/v1/sub2api/billing",
		"/api/status",
		"/api/pricing",
		"/api/usage/token/",
		"/api/user/self",
		"/dashboard/billing/subscription",
	}, newAPIProbeRequestPaths(upstream.requests))
}

func TestNewAPIProbeContainsGroupFailureAndRejectsInvalidSelection(t *testing.T) {
	account := newAPIProbeTestAccount(330)
	account.Credentials[NewAPIUpstreamGroupCredentialKey] = "removed"
	initialRate := *account.RateMultiplier
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		newAPIProbeTestResponse(http.StatusNotFound, `{}`),
		newAPIProbeTestResponse(http.StatusOK, newAPIStatusTestBody("USD")),
		newAPIProbeTestResponse(http.StatusOK, strings.Repeat("x", newAPIGroupProbeMaxBodyBytes+1)),
		newAPIProbeTestResponse(http.StatusOK, newAPITokenUsageTestBody(1_000_000, 500_000, 500_000, false, 0)),
	}}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

	snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

	require.NoError(t, err)
	require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Status)
	require.Equal(t, UpstreamBillingProbeStatusFailed, snapshot.Data["groups_status"])
	require.Equal(t, "response_too_large", snapshot.Data["groups_error"])
	require.Equal(t, false, snapshot.Data["group_selection_valid"])
	require.NotContains(t, snapshot.Data, "resolved_rate_multiplier")
	require.Equal(t, initialRate, *account.RateMultiplier)
	require.Equal(t, UpstreamBillingProbeStatusOK, snapshot.Balance.Status)
}

func TestNewAPIProbeDoesNotIdentifyInvalidOrOversizedStatus(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "invalid schema", body: `{"success":true,"data":{"quota_per_unit":500000}}`},
		{name: "oversized response", body: strings.Repeat("x", upstreamBillingProbeMaxBodyBytes+1)},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := newAPIProbeTestAccount(int64(340 + index))
			repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				newAPIProbeTestResponse(http.StatusNotFound, `{}`),
				newAPIProbeTestResponse(http.StatusOK, tt.body),
				newAPIProbeTestResponse(http.StatusNotFound, `{}`),
			}}
			svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

			snapshot, err := svc.ProbeAccount(context.Background(), account.ID)

			require.NoError(t, err)
			require.Equal(t, UpstreamBillingProbeStatusUnsupported, snapshot.Status)
			require.Equal(t, []string{"/v1/sub2api/billing", "/api/status", "/v1/usage"}, newAPIProbeRequestPaths(upstream.requests))
		})
	}
}

func TestNewAPIParsersAndURLBuilderFailClosed(t *testing.T) {
	_, err := parseNewAPITokenUsage([]byte(`{"code":true,"data":{"object":"token_usage","total_granted":1,"total_used":0,"total_available":1,"expires_at":0}}`))
	require.Error(t, err)

	_, err = parseNewAPIUserWallet([]byte(`{"success":true,"data":{"quota":"invalid"}}`), &newAPIStatusConfig{
		QuotaPerUnit: 500_000, QuotaDisplayType: "USD", USDExchangeRate: 1,
	})
	require.Error(t, err)

	_, err = normalizeNewAPIUserAccessToken("valid\r\ninjected")
	require.ErrorIs(t, err, ErrNewAPIUserAccessTokenInvalid)

	_, err = newAPITokenBalanceData(&newAPITokenUsage{TotalGranted: -1, TotalUsed: 0, TotalAvailable: -1}, &newAPIStatusConfig{
		QuotaPerUnit: 500_000, QuotaDisplayType: "USD", USDExchangeRate: 7.2,
	}, time.Now())
	require.Error(t, err)

	_, err = parseNewAPIUpstreamGroups(&newAPIHTTPResult{
		statusCode: http.StatusOK,
		body:       []byte(`{"success":true,"group_ratio":{"hostile":101}}`),
	})
	require.Error(t, err)

	_, err = parseNewAPIUpstreamGroups(&newAPIHTTPResult{
		statusCode: http.StatusOK,
		body:       []byte(`{"success":true,"group_ratio":{" padded ":1}}`),
	})
	require.Error(t, err)

	legacyCurrency, err := parseNewAPIStatus([]byte(newAPILegacyStatusTestBody(true)))
	require.NoError(t, err)
	require.Equal(t, "USD", legacyCurrency.QuotaDisplayType)
	legacyTokens, err := parseNewAPIStatus([]byte(newAPILegacyStatusTestBody(false)))
	require.NoError(t, err)
	require.Equal(t, "TOKENS", legacyTokens.QuotaDisplayType)

	probeURL, err := buildNewAPIEndpointURL("https://new-api.example/gateway/api/v1?secret=discard", "/api/status")
	require.NoError(t, err)
	require.Equal(t, "https://new-api.example/gateway/api/status", probeURL)
}

func TestNewAPIRequestUsesProbeClockForRetryAfter(t *testing.T) {
	fixedNow := time.Date(2026, time.September, 5, 8, 0, 0, 0, time.UTC)
	account := newAPIProbeTestAccount(350)
	repo := &upstreamBillingProbeAccountRepo{accounts: map[int64]*Account{account.ID: account}}
	response := newAPIProbeTestResponse(http.StatusTooManyRequests, `{}`)
	response.Header.Set("Retry-After", fixedNow.Add(10*time.Minute).Format(http.TimeFormat))
	upstream := &httpUpstreamRecorder{resp: response}
	svc := newUpstreamBillingProbeTestService(repo, upstream, &upstreamBillingProbeSettingRepo{})

	result := svc.doNewAPIProbeRequest(
		context.Background(), account, "https://new-api.example", "/api/usage/token/",
		"sk-new-api", "", HTTPUpstreamProfileOpenAI, nil, upstreamBalanceProbeMaxBodyBytes, fixedNow,
	)

	require.Equal(t, 10*time.Minute, result.retryAfter)
}

func newAPIProbeRequestPaths(requests []*http.Request) []string {
	paths := make([]string, 0, len(requests))
	for _, request := range requests {
		paths = append(paths, request.URL.Path)
	}
	return paths
}
