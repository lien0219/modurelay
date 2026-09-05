package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	// NewAPIUpstreamGroupCredentialKey is a managed, non-secret credential
	// field. It is returned to the admin UI but cannot be changed through the
	// generic credentials map.
	NewAPIUpstreamGroupCredentialKey = "upstream_billing_new_api_group"
	// NewAPIUserAccessTokenCredentialKey stores the persistent personal access
	// token used only for New API dashboard wallet reads. It is never used for
	// relay traffic and is redacted from every account response and audit body.
	NewAPIUserAccessTokenCredentialKey = "upstream_billing_new_api_user_access_token"
	// NewAPIUserIDCredentialKey stores the optional numeric user identity needed
	// by legacy New API releases that require the New-Api-User header.
	NewAPIUserIDCredentialKey = "upstream_billing_new_api_user_id"

	newAPIProviderName = "new_api"
	// /api/pricing includes the complete public model catalog before the group
	// ratios. Keep the response bounded, but leave enough room for production
	// New API installations with large custom catalogs.
	newAPIGroupProbeMaxBodyBytes          = 2 * 1024 * 1024
	newAPIGroupMaxCount                   = 256
	newAPIGroupMaxNameRunes               = 128
	newAPIUnlimitedSubscriptionSentinel   = 100000000.0
	newAPIBalanceSourceToken              = "new_api_token"
	newAPIBalanceSourceWallet             = "new_api_wallet"
	newAPIWalletProbeStatusNotConfigured  = "not_configured"
	newAPIWalletProbeStatusFailed         = "failed"
	newAPIUserAccessTokenMaxBytes         = 4096
	newAPIUserAccessTokenCiphertextPrefix = "mrenc:v1:"
	newAPIUserIDMax                       = int64(math.MaxInt32)
	newAPITokenOwnershipPageSize          = 100
	newAPITokenOwnershipMaxPages          = 10
)

var (
	ErrNewAPIUpstreamGroupRequiresProbe = infraerrors.BadRequest(
		"NEW_API_UPSTREAM_GROUP_REQUIRES_PROBE",
		"probe and confirm a New API upstream before selecting its upstream group",
	)
	ErrNewAPIUpstreamGroupUnavailable = infraerrors.BadRequest(
		"NEW_API_UPSTREAM_GROUP_UNAVAILABLE",
		"the selected New API upstream group is not available in the latest probe",
	)
	ErrNewAPIUpstreamGroupReprobeRequired = infraerrors.BadRequest(
		"NEW_API_UPSTREAM_GROUP_REPROBE_REQUIRED",
		"the upstream identity changed; probe it again before selecting a New API upstream group",
	)
	ErrNewAPIUserAccessTokenRequiresProbe = infraerrors.BadRequest(
		"NEW_API_USER_ACCESS_TOKEN_REQUIRES_PROBE",
		"probe and confirm a New API upstream before configuring its user access token",
	)
	ErrNewAPIUserAccessTokenReprobeRequired = infraerrors.BadRequest(
		"NEW_API_USER_ACCESS_TOKEN_REPROBE_REQUIRED",
		"the upstream identity changed; probe it again before configuring a New API user access token",
	)
	ErrNewAPIUserAccessTokenInvalid = infraerrors.BadRequest(
		"NEW_API_USER_ACCESS_TOKEN_INVALID",
		"the New API user access token is invalid",
	)
	ErrNewAPIUserAccessTokenEncryptionKey = infraerrors.BadRequest(
		"NEW_API_USER_ACCESS_TOKEN_ENCRYPTION_KEY_NOT_CONFIGURED",
		"cannot store a New API user access token without a fixed TOTP_ENCRYPTION_KEY",
	)
	ErrNewAPIUserIDInvalid = infraerrors.BadRequest(
		"NEW_API_USER_ID_INVALID",
		"the New API user ID must be a positive integer or zero to clear it",
	)
	ErrNewAPIUserIDRequiresAccessToken = infraerrors.BadRequest(
		"NEW_API_USER_ID_REQUIRES_ACCESS_TOKEN",
		"a New API user ID can be configured only with a New API user access token",
	)
)

type newAPIWalletProbeFailure struct {
	reason     string
	httpStatus int
}

type newAPIStatusConfig struct {
	QuotaPerUnit               float64
	QuotaDisplayType           string
	USDExchangeRate            float64
	CustomCurrencyExchangeRate float64
}

type newAPIHTTPResult struct {
	body       []byte
	statusCode int
	retryAfter time.Duration
	reason     string
}

type newAPIDashboardAuth struct {
	accessToken string
	userID      int64
	legacy      bool
}

type newAPIUserWallet struct {
	data   map[string]any
	userID int64
}

func (s *UpstreamBillingProbeService) probeNewAPIUpstream(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	apiKey string,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	intervalMinutes int,
	now time.Time,
	forceBalance bool,
) (*UpstreamBillingProbeSnapshot, bool) {
	probeCtx, cancel := context.WithTimeout(ctx, upstreamBillingProbeRequestTimeout)
	defer cancel()

	statusResult := s.doNewAPIProbeRequest(
		probeCtx, account, normalizedBaseURL, "/api/status", "", proxyURL,
		profile, tlsProfile, upstreamBillingProbeMaxBodyBytes, now,
	)
	if statusResult.reason != "" || statusResult.statusCode < 200 || statusResult.statusCode >= 300 {
		return nil, false
	}
	statusConfig, err := parseNewAPIStatus(statusResult.body)
	if err != nil {
		return nil, false
	}

	data := map[string]any{
		"object":         "new_api.group_billing",
		"schema_version": 1,
		"billing_scope":  "token",
		"provider":       newAPIProviderName,
		"observed_at":    now.UTC().Format(time.RFC3339Nano),
	}

	groupsResult := s.doNewAPIProbeRequest(
		probeCtx, account, normalizedBaseURL, "/api/pricing", "", proxyURL,
		profile, tlsProfile, newAPIGroupProbeMaxBodyBytes, now,
	)
	groups, groupsErr := parseNewAPIUpstreamGroups(groupsResult)
	if groupsErr != nil {
		data["groups_status"] = UpstreamBillingProbeStatusFailed
		data["groups_error"] = groupsErr.Error()
	} else {
		data["groups_status"] = UpstreamBillingProbeStatusOK
		data["available_groups"] = groups
	}

	selectedGroup := NewAPIUpstreamGroupFromAccount(account)
	if selectedGroup != "" {
		data["selected_group"] = selectedGroup
		data["group_selection_valid"] = false
		for _, group := range groups {
			if group.Name != selectedGroup {
				continue
			}
			data["group_selection_valid"] = true
			data["group_rate_multiplier"] = group.RateMultiplier
			data["resolved_rate_multiplier"] = group.RateMultiplier
			data["effective_rate_multiplier"] = group.RateMultiplier
			data["peak_rate_enabled"] = false
			break
		}
	} else {
		data["group_selection_required"] = true
	}

	balance := s.probeNewAPIBalance(
		probeCtx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile,
		statusConfig, intervalMinutes, now, forceBalance,
	)
	snapshot := &UpstreamBillingProbeSnapshot{
		Status:        UpstreamBillingProbeStatusOK,
		Data:          data,
		Balance:       balance,
		ReceivedAt:    probeTimePtr(now),
		FreshUntil:    probeTimePtr(now.Add(2 * time.Duration(intervalMinutes) * time.Minute)),
		LastAttemptAt: now,
		NextProbeAt:   now.Add(nextProbeDelay(intervalMinutes, 0)),
		HTTPStatus:    statusResult.statusCode,
	}
	return snapshot, true
}

func (s *UpstreamBillingProbeService) probeNewAPIBalance(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	apiKey string,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	statusConfig *newAPIStatusConfig,
	intervalMinutes int,
	now time.Time,
	force bool,
) *UpstreamBalanceProbeSnapshot {
	var previous *UpstreamBalanceProbeSnapshot
	if snapshot := decodeUpstreamBillingProbeSnapshot(account.Extra); snapshot != nil {
		previous = snapshot.Balance
	}
	if !force && previous != nil &&
		(previous.Source == newAPIBalanceSourceToken || previous.Source == newAPIBalanceSourceWallet) &&
		!previous.NextProbeAt.IsZero() && now.Before(previous.NextProbeAt) {
		return previous
	}

	result := s.doNewAPIProbeRequest(
		ctx, account, normalizedBaseURL, "/api/usage/token/", apiKey, proxyURL,
		profile, tlsProfile, upstreamBalanceProbeMaxBodyBytes, now,
	)
	if result == nil || result.reason != "" || result.statusCode < 200 || result.statusCode >= 300 {
		// New API versions before /api/usage/token/ was introduced still expose
		// the token-authenticated dashboard wallet endpoints. Fall back only when
		// the token endpoint is explicitly unsupported; authorization, transport,
		// and server failures must remain visible instead of being masked.
		if result.reason == "" &&
			(result.statusCode == http.StatusNotFound || result.statusCode == http.StatusMethodNotAllowed) {
			wallet, unlimitedToken := s.probeNewAPIWalletBalance(
				ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile, statusConfig, now,
			)
			if wallet != nil {
				return newUpstreamBalanceProbeSuccess(
					wallet, newAPIBalanceSourceWallet, intervalMinutes, now, http.StatusOK,
				)
			}
			if unlimitedToken {
				wallet, userWalletFailure := s.probeConfiguredNewAPIUserWalletBalance(
					ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile,
					statusConfig, now,
				)
				if wallet != nil {
					wallet["is_valid"] = true
					return newUpstreamBalanceProbeSuccess(
						wallet, newAPIBalanceSourceWallet, intervalMinutes, now, http.StatusOK,
					)
				}
				return newUpstreamBalanceProbeSuccess(
					newAPIUnlimitedTokenBalanceData(account, statusConfig, true, userWalletFailure),
					newAPIBalanceSourceToken, intervalMinutes, now, http.StatusOK,
				)
			}
		}
		failure := newUpstreamBalanceProbeFailure(
			previous, intervalMinutes, now, result.statusCode,
			newAPIResultFailureReason(result), result.retryAfter,
		)
		failure.Source = newAPIBalanceSourceToken
		return failure
	}

	token, err := parseNewAPITokenUsage(result.body)
	if err != nil {
		failure := newUpstreamBalanceProbeFailure(
			previous, intervalMinutes, now, result.statusCode, "invalid_response", result.retryAfter,
		)
		failure.Source = newAPIBalanceSourceToken
		return failure
	}
	if !token.Unlimited {
		data, err := newAPITokenBalanceData(token, statusConfig, now)
		if err != nil {
			failure := newUpstreamBalanceProbeFailure(
				previous, intervalMinutes, now, result.statusCode, "invalid_response", result.retryAfter,
			)
			failure.Source = newAPIBalanceSourceToken
			return failure
		}
		return newUpstreamBalanceProbeSuccess(
			data, newAPIBalanceSourceToken, intervalMinutes, now, result.statusCode,
		)
	}

	wallet, userWalletFailure := s.probeConfiguredNewAPIUserWalletBalance(
		ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile, statusConfig, now,
	)
	if wallet != nil {
		wallet["is_valid"] = token.isValidAt(now)
		return newUpstreamBalanceProbeSuccess(
			wallet, newAPIBalanceSourceWallet, intervalMinutes, now, http.StatusOK,
		)
	}

	// Older New API installations can expose the account wallet through the
	// token-authenticated compatibility endpoints when token-stat display is
	// disabled. Keep this fallback for compatibility, but reject the documented
	// 100000000 unlimited-token sentinel as a wallet observation.
	if wallet, _ := s.probeNewAPIWalletBalance(
		ctx, account, normalizedBaseURL, apiKey, proxyURL, profile, tlsProfile, statusConfig, now,
	); wallet != nil {
		wallet["is_valid"] = token.isValidAt(now)
		return newUpstreamBalanceProbeSuccess(
			wallet, newAPIBalanceSourceWallet, intervalMinutes, now, http.StatusOK,
		)
	}

	// An unlimited token is a successful, authoritative token observation. If
	// the wallet endpoint is disabled, token-scoped, or otherwise ambiguous,
	// preserve that fact instead of manufacturing a finite balance that could
	// pause scheduling incorrectly.
	data := newAPIUnlimitedTokenBalanceData(account, statusConfig, token.isValidAt(now), userWalletFailure)
	return newUpstreamBalanceProbeSuccess(
		data, newAPIBalanceSourceToken, intervalMinutes, now, result.statusCode,
	)
}

func (s *UpstreamBillingProbeService) probeConfiguredNewAPIUserWalletBalance(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	relayAPIKey string,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	statusConfig *newAPIStatusConfig,
	now time.Time,
) (map[string]any, *newAPIWalletProbeFailure) {
	userAccessToken, failure := s.newAPIUserAccessTokenFromAccount(account)
	if userAccessToken == "" {
		return nil, failure
	}
	return s.probeNewAPIUserWalletBalance(
		ctx, account, normalizedBaseURL, relayAPIKey, userAccessToken, NewAPIUserIDFromAccount(account),
		proxyURL, profile, tlsProfile, statusConfig, now,
	)
}

func newAPIUnlimitedTokenBalanceData(
	account *Account,
	statusConfig *newAPIStatusConfig,
	isValid bool,
	userWalletFailure *newAPIWalletProbeFailure,
) map[string]any {
	data := map[string]any{
		"mode":      "key_quota",
		"unit":      statusConfig.balanceUnit(),
		"unlimited": true,
		"is_valid":  isValid,
	}
	if !NewAPIUserAccessTokenConfigured(account) {
		data["wallet_probe_status"] = newAPIWalletProbeStatusNotConfigured
	} else {
		data["wallet_probe_status"] = newAPIWalletProbeStatusFailed
		if userWalletFailure != nil {
			data["wallet_probe_error"] = userWalletFailure.reason
			if userWalletFailure.httpStatus > 0 {
				data["wallet_probe_http_status"] = userWalletFailure.httpStatus
			}
		}
	}
	return data
}

type newAPITokenUsage struct {
	TotalGranted   float64
	TotalUsed      float64
	TotalAvailable float64
	Unlimited      bool
	ExpiresAt      int64
}

func parseNewAPITokenUsage(body []byte) (*newAPITokenUsage, error) {
	var response struct {
		Code *bool `json:"code"`
		Data *struct {
			Object         string   `json:"object"`
			TotalGranted   *float64 `json:"total_granted"`
			TotalUsed      *float64 `json:"total_used"`
			TotalAvailable *float64 `json:"total_available"`
			UnlimitedQuota *bool    `json:"unlimited_quota"`
			ExpiresAt      *int64   `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Code == nil || !*response.Code || response.Data == nil ||
		response.Data.Object != "token_usage" || response.Data.TotalGranted == nil ||
		response.Data.TotalUsed == nil || response.Data.TotalAvailable == nil ||
		response.Data.UnlimitedQuota == nil || response.Data.ExpiresAt == nil {
		return nil, errors.New("unexpected New API token usage schema")
	}
	for _, value := range []float64{
		*response.Data.TotalGranted, *response.Data.TotalUsed, *response.Data.TotalAvailable,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("invalid New API token quota")
		}
	}
	return &newAPITokenUsage{
		TotalGranted:   *response.Data.TotalGranted,
		TotalUsed:      *response.Data.TotalUsed,
		TotalAvailable: *response.Data.TotalAvailable,
		Unlimited:      *response.Data.UnlimitedQuota,
		ExpiresAt:      *response.Data.ExpiresAt,
	}, nil
}

func (t *newAPITokenUsage) isValidAt(now time.Time) bool {
	return t != nil && (t.ExpiresAt <= 0 || t.ExpiresAt > now.Unix())
}

func newAPITokenBalanceData(token *newAPITokenUsage, status *newAPIStatusConfig, now time.Time) (map[string]any, error) {
	if token == nil || status == nil || token.Unlimited {
		return nil, errors.New("finite New API token usage required")
	}
	if token.TotalGranted < 0 || token.TotalUsed < 0 {
		return nil, errors.New("invalid New API token quota")
	}
	limit, unit, ok := status.convertQuota(token.TotalGranted)
	if !ok {
		return nil, errors.New("invalid New API quota conversion")
	}
	used, _, ok := status.convertQuota(token.TotalUsed)
	if !ok {
		return nil, errors.New("invalid New API quota conversion")
	}
	remaining, _, ok := status.convertQuota(token.TotalAvailable)
	if !ok {
		return nil, errors.New("invalid New API quota conversion")
	}
	return map[string]any{
		"mode":      "key_quota",
		"unit":      unit,
		"limit":     limit,
		"used":      used,
		"remaining": remaining,
		"unlimited": false,
		"is_valid":  token.isValidAt(now),
	}, nil
}

func (s *UpstreamBillingProbeService) probeNewAPIUserWalletBalance(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	relayAPIKey string,
	userAccessToken string,
	configuredUserID int64,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	statusConfig *newAPIStatusConfig,
	now time.Time,
) (map[string]any, *newAPIWalletProbeFailure) {
	authModes := []newAPIDashboardAuth{{accessToken: userAccessToken}}
	if configuredUserID > 0 {
		authModes = append(authModes, newAPIDashboardAuth{
			accessToken: userAccessToken,
			userID:      configuredUserID,
			legacy:      true,
		})
	}

	var lastFailure *newAPIWalletProbeFailure
	for index, auth := range authModes {
		result := s.doNewAPIDashboardProbeRequest(
			ctx, http.MethodGet, account, normalizedBaseURL, "/api/user/self", auth,
			proxyURL, profile, tlsProfile, upstreamBalanceProbeMaxBodyBytes, now,
		)
		if result == nil || result.reason != "" || result.statusCode < 200 || result.statusCode >= 300 {
			httpStatus := 0
			if result != nil {
				httpStatus = result.statusCode
			}
			lastFailure = &newAPIWalletProbeFailure{
				reason:     newAPIResultFailureReason(result),
				httpStatus: httpStatus,
			}
			if index == 0 && len(authModes) > 1 && shouldRetryLegacyNewAPIAuth(result) {
				continue
			}
			return nil, lastFailure
		}

		wallet, err := parseNewAPIUserWallet(result.body, statusConfig)
		if err != nil {
			lastFailure = &newAPIWalletProbeFailure{reason: "invalid_response", httpStatus: result.statusCode}
			if index == 0 && len(authModes) > 1 {
				continue
			}
			return nil, lastFailure
		}
		if configuredUserID > 0 && wallet.userID != configuredUserID {
			return nil, &newAPIWalletProbeFailure{reason: "user_mismatch", httpStatus: result.statusCode}
		}

		ownershipFailure := s.verifyNewAPIRelayTokenOwnership(
			ctx, account, normalizedBaseURL, relayAPIKey, auth, proxyURL, profile, tlsProfile, now,
		)
		if ownershipFailure != nil {
			return nil, ownershipFailure
		}
		wallet.data["ownership_verified"] = true
		return wallet.data, nil
	}
	return nil, lastFailure
}

func parseNewAPIUserWallet(body []byte, statusConfig *newAPIStatusConfig) (*newAPIUserWallet, error) {
	var response struct {
		Success *bool `json:"success"`
		Data    *struct {
			ID    *int64   `json:"id"`
			Quota *float64 `json:"quota"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Success == nil || !*response.Success || response.Data == nil ||
		response.Data.ID == nil || *response.Data.ID <= 0 ||
		response.Data.Quota == nil || !finiteNumber(*response.Data.Quota) {
		return nil, errors.New("unexpected New API user wallet schema")
	}
	balance, unit, ok := statusConfig.convertQuota(*response.Data.Quota)
	if !ok {
		return nil, errors.New("invalid New API wallet conversion")
	}
	return &newAPIUserWallet{
		userID: *response.Data.ID,
		data: map[string]any{
			"mode":      "wallet",
			"unit":      unit,
			"balance":   balance,
			"remaining": balance,
			"unlimited": false,
		},
	}, nil
}

type newAPITokenOwnershipItem struct {
	ID  int64  `json:"id"`
	Key string `json:"key"`
}

func (s *UpstreamBillingProbeService) verifyNewAPIRelayTokenOwnership(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	relayAPIKey string,
	auth newAPIDashboardAuth,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	now time.Time,
) *newAPIWalletProbeFailure {
	expectedKey := normalizeNewAPIRelayTokenKey(relayAPIKey)
	if expectedKey == "" {
		return &newAPIWalletProbeFailure{reason: "token_owner_mismatch"}
	}
	expectedMaskedKey := maskNewAPITokenKey(expectedKey)

	for page := 1; page <= newAPITokenOwnershipMaxPages; page++ {
		query := url.Values{}
		query.Set("p", strconv.Itoa(page))
		query.Set("page_size", strconv.Itoa(newAPITokenOwnershipPageSize))
		result := s.doNewAPIDashboardProbeRequest(
			ctx, http.MethodGet, account, normalizedBaseURL, "/api/token/?"+query.Encode(), auth,
			proxyURL, profile, tlsProfile, upstreamBalanceProbeMaxBodyBytes, now,
		)
		if result == nil || result.reason != "" || result.statusCode < 200 || result.statusCode >= 300 {
			return newAPIWalletProbeFailureFromResult(result)
		}
		items, total, err := parseNewAPITokenOwnershipPage(result.body)
		if err != nil {
			return &newAPIWalletProbeFailure{reason: "ownership_invalid_response", httpStatus: result.statusCode}
		}
		for _, item := range items {
			candidate := normalizeNewAPIRelayTokenKey(item.Key)
			switch {
			case candidate == "":
				continue
			case !strings.Contains(candidate, "*"):
				if constantTimeStringEqual(candidate, expectedKey) {
					return nil
				}
			case item.ID > 0 && candidate == expectedMaskedKey:
				fullKeyResult := s.doNewAPIDashboardProbeRequest(
					ctx, http.MethodPost, account, normalizedBaseURL,
					"/api/token/"+strconv.FormatInt(item.ID, 10)+"/key", auth,
					proxyURL, profile, tlsProfile, upstreamBalanceProbeMaxBodyBytes, now,
				)
				if fullKeyResult == nil || fullKeyResult.reason != "" ||
					fullKeyResult.statusCode < 200 || fullKeyResult.statusCode >= 300 {
					failure := newAPIWalletProbeFailureFromResult(fullKeyResult)
					failure.reason = "token_ownership_unverified"
					return failure
				}
				fullKey, parseErr := parseNewAPIFullTokenKey(fullKeyResult.body)
				if parseErr != nil {
					return &newAPIWalletProbeFailure{
						reason: "ownership_invalid_response", httpStatus: fullKeyResult.statusCode,
					}
				}
				if constantTimeStringEqual(normalizeNewAPIRelayTokenKey(fullKey), expectedKey) {
					return nil
				}
			}
		}

		if total <= page*newAPITokenOwnershipPageSize || len(items) < newAPITokenOwnershipPageSize {
			return &newAPIWalletProbeFailure{reason: "token_owner_mismatch", httpStatus: result.statusCode}
		}
	}
	return &newAPIWalletProbeFailure{reason: "token_ownership_unverified"}
}

func parseNewAPITokenOwnershipPage(body []byte) ([]newAPITokenOwnershipItem, int, error) {
	var response struct {
		Success *bool `json:"success"`
		Data    *struct {
			Total int                        `json:"total"`
			Items []newAPITokenOwnershipItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, 0, err
	}
	if response.Success == nil || !*response.Success || response.Data == nil ||
		response.Data.Total < 0 || len(response.Data.Items) > newAPITokenOwnershipPageSize {
		return nil, 0, errors.New("unexpected New API token list schema")
	}
	return response.Data.Items, response.Data.Total, nil
}

func parseNewAPIFullTokenKey(body []byte) (string, error) {
	var response struct {
		Success *bool `json:"success"`
		Data    *struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}
	if response.Success == nil || !*response.Success || response.Data == nil ||
		normalizeNewAPIRelayTokenKey(response.Data.Key) == "" {
		return "", errors.New("unexpected New API token key schema")
	}
	return response.Data.Key, nil
}

func normalizeNewAPIRelayTokenKey(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "sk-")
}

func maskNewAPITokenKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return strings.Repeat("*", len(key))
	}
	if len(key) <= 8 {
		return key[:2] + "****" + key[len(key)-2:]
	}
	return key[:4] + "**********" + key[len(key)-4:]
}

func constantTimeStringEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func newAPIWalletProbeFailureFromResult(result *newAPIHTTPResult) *newAPIWalletProbeFailure {
	httpStatus := 0
	if result != nil {
		httpStatus = result.statusCode
	}
	return &newAPIWalletProbeFailure{
		reason:     newAPIResultFailureReason(result),
		httpStatus: httpStatus,
	}
}

func shouldRetryLegacyNewAPIAuth(result *newAPIHTTPResult) bool {
	return result != nil && result.reason == "" &&
		(result.statusCode == http.StatusUnauthorized || result.statusCode == http.StatusForbidden ||
			(result.statusCode >= 200 && result.statusCode < 300))
}

// probeNewAPIWalletBalance reads New API's token-authenticated compatibility
// endpoints. A nil wallet with unlimitedToken=true is an authenticated,
// recognized unlimited-token sentinel, not a transport or schema failure.
func (s *UpstreamBillingProbeService) probeNewAPIWalletBalance(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	apiKey string,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	statusConfig *newAPIStatusConfig,
	now time.Time,
) (map[string]any, bool) {
	subscriptionResult := s.doNewAPIProbeRequest(
		ctx, account, normalizedBaseURL, "/dashboard/billing/subscription", apiKey,
		proxyURL, profile, tlsProfile, upstreamBalanceProbeMaxBodyBytes, now,
	)
	if subscriptionResult.reason != "" || subscriptionResult.statusCode < 200 || subscriptionResult.statusCode >= 300 {
		return nil, false
	}
	var subscription struct {
		Object             string   `json:"object"`
		SoftLimitUSD       *float64 `json:"soft_limit_usd"`
		HardLimitUSD       *float64 `json:"hard_limit_usd"`
		SystemHardLimitUSD *float64 `json:"system_hard_limit_usd"`
	}
	if err := json.Unmarshal(subscriptionResult.body, &subscription); err != nil ||
		subscription.Object != "billing_subscription" || subscription.SoftLimitUSD == nil ||
		subscription.HardLimitUSD == nil || subscription.SystemHardLimitUSD == nil ||
		!finiteNumber(*subscription.SoftLimitUSD) || !finiteNumber(*subscription.HardLimitUSD) ||
		!finiteNumber(*subscription.SystemHardLimitUSD) {
		return nil, false
	}
	if equalBillingMultiplier(*subscription.HardLimitUSD, newAPIUnlimitedSubscriptionSentinel) {
		return nil, true
	}

	usageResult := s.doNewAPIProbeRequest(
		ctx, account, normalizedBaseURL, "/dashboard/billing/usage", apiKey,
		proxyURL, profile, tlsProfile, upstreamBalanceProbeMaxBodyBytes, now,
	)
	if usageResult.reason != "" || usageResult.statusCode < 200 || usageResult.statusCode >= 300 {
		return nil, false
	}
	var usage struct {
		Object     string   `json:"object"`
		TotalUsage *float64 `json:"total_usage"`
	}
	if err := json.Unmarshal(usageResult.body, &usage); err != nil ||
		usage.Object != "list" || usage.TotalUsage == nil || !finiteNumber(*usage.TotalUsage) {
		return nil, false
	}
	// New API keeps the historical *_usd field names, but currently returns
	// CNY and TOKENS dashboard amounts in the configured display unit. CUSTOM
	// still follows the default USD branch and therefore needs one conversion.
	reportedBalance := *subscription.HardLimitUSD - *usage.TotalUsage/100
	balance, unit, ok := statusConfig.convertDashboardBalance(reportedBalance)
	if !ok {
		return nil, false
	}
	return map[string]any{
		"mode":      "wallet",
		"unit":      unit,
		"balance":   balance,
		"remaining": balance,
		"unlimited": false,
	}, false
}

func parseNewAPIStatus(body []byte) (*newAPIStatusConfig, error) {
	var response struct {
		Success *bool `json:"success"`
		Data    *struct {
			Setup                      *bool    `json:"setup"`
			QuotaPerUnit               *float64 `json:"quota_per_unit"`
			QuotaDisplayType           string   `json:"quota_display_type"`
			DisplayInCurrency          *bool    `json:"display_in_currency"`
			USDExchangeRate            *float64 `json:"usd_exchange_rate"`
			CustomCurrencyExchangeRate *float64 `json:"custom_currency_exchange_rate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Success == nil || !*response.Success || response.Data == nil ||
		response.Data.Setup == nil || response.Data.QuotaPerUnit == nil ||
		response.Data.USDExchangeRate == nil ||
		!finitePositive(*response.Data.QuotaPerUnit) || !finitePositive(*response.Data.USDExchangeRate) {
		return nil, errors.New("unexpected New API status schema")
	}
	displayType := strings.ToUpper(strings.TrimSpace(response.Data.QuotaDisplayType))
	if displayType == "" && response.Data.DisplayInCurrency != nil {
		if *response.Data.DisplayInCurrency {
			displayType = "USD"
		} else {
			displayType = "TOKENS"
		}
	}
	if displayType != "USD" && displayType != "CNY" && displayType != "TOKENS" && displayType != "CUSTOM" {
		return nil, errors.New("unsupported New API quota display type")
	}
	customRate := 1.0
	if response.Data.CustomCurrencyExchangeRate != nil {
		customRate = *response.Data.CustomCurrencyExchangeRate
	}
	if displayType == "CUSTOM" && !finitePositive(customRate) {
		return nil, errors.New("invalid New API custom currency rate")
	}
	return &newAPIStatusConfig{
		QuotaPerUnit:               *response.Data.QuotaPerUnit,
		QuotaDisplayType:           displayType,
		USDExchangeRate:            *response.Data.USDExchangeRate,
		CustomCurrencyExchangeRate: customRate,
	}, nil
}

func (c *newAPIStatusConfig) convertQuota(value float64) (float64, string, bool) {
	if c == nil || !finiteNumber(value) || !finitePositive(c.QuotaPerUnit) {
		return 0, "", false
	}
	converted := value
	switch c.QuotaDisplayType {
	case "USD":
		converted = value / c.QuotaPerUnit
	case "CNY":
		converted = value / c.QuotaPerUnit * c.USDExchangeRate
	case "TOKENS":
	case "CUSTOM":
		converted = value / c.QuotaPerUnit * c.CustomCurrencyExchangeRate
	default:
		return 0, "", false
	}
	return converted, c.balanceUnit(), finiteNumber(converted)
}

func (c *newAPIStatusConfig) balanceUnit() string {
	if c == nil {
		return ""
	}
	return c.QuotaDisplayType
}

func (c *newAPIStatusConfig) convertDashboardBalance(value float64) (float64, string, bool) {
	if c == nil || !finiteNumber(value) {
		return 0, "", false
	}
	converted := value
	switch c.QuotaDisplayType {
	case "USD", "CNY", "TOKENS":
	case "CUSTOM":
		converted = value * c.CustomCurrencyExchangeRate
	default:
		return 0, "", false
	}
	return converted, c.balanceUnit(), finiteNumber(converted)
}

func parseNewAPIUpstreamGroups(result *newAPIHTTPResult) ([]NewAPIUpstreamGroup, error) {
	if result == nil {
		return nil, errors.New("empty_response")
	}
	if result.reason != "" {
		return nil, errors.New(result.reason)
	}
	if result.statusCode < 200 || result.statusCode >= 300 {
		if result.statusCode == http.StatusUnauthorized || result.statusCode == http.StatusForbidden {
			return nil, errors.New("unauthorized")
		}
		return nil, errors.New("http_error")
	}
	var response struct {
		Success    *bool              `json:"success"`
		GroupRatio map[string]float64 `json:"group_ratio"`
	}
	if err := json.Unmarshal(result.body, &response); err != nil {
		return nil, errors.New("invalid_response")
	}
	if response.Success == nil || !*response.Success || response.GroupRatio == nil ||
		len(response.GroupRatio) > newAPIGroupMaxCount {
		return nil, errors.New("invalid_response")
	}
	groups := make([]NewAPIUpstreamGroup, 0, len(response.GroupRatio))
	for rawName, multiplier := range response.GroupRatio {
		name, ok := normalizeNewAPIUpstreamGroupName(rawName)
		if !ok || name != rawName || !finiteNumber(multiplier) || multiplier < 0 || multiplier > upstreamBillingRateSyncMaxMultiplier {
			return nil, errors.New("invalid_response")
		}
		groups = append(groups, NewAPIUpstreamGroup{Name: name, RateMultiplier: multiplier})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	return groups, nil
}

func normalizeNewAPIUpstreamGroupName(value string) (string, bool) {
	name := strings.TrimSpace(value)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > newAPIGroupMaxNameRunes {
		return "", false
	}
	for _, char := range name {
		if unicode.IsControl(char) {
			return "", false
		}
	}
	return name, true
}

func NewAPIUpstreamGroupFromAccount(account *Account) string {
	if account == nil || account.Credentials == nil {
		return ""
	}
	value, _ := account.Credentials[NewAPIUpstreamGroupCredentialKey].(string)
	name, ok := normalizeNewAPIUpstreamGroupName(value)
	if !ok {
		return ""
	}
	return name
}

func NewAPIUserAccessTokenConfigured(account *Account) bool {
	if account == nil || account.Credentials == nil {
		return false
	}
	value, _ := account.Credentials[NewAPIUserAccessTokenCredentialKey].(string)
	return strings.TrimSpace(value) != ""
}

func NewAPIUserIDFromAccount(account *Account) int64 {
	if account == nil {
		return 0
	}
	userID := account.GetCredentialAsInt64(NewAPIUserIDCredentialKey)
	if userID <= 0 || userID > newAPIUserIDMax {
		return 0
	}
	return userID
}

func encryptNewAPIUserAccessToken(
	encryptor SecretEncryptor,
	encryptionKeyConfigured bool,
	plaintext string,
) (string, error) {
	if !encryptionKeyConfigured || encryptor == nil {
		return "", ErrNewAPIUserAccessTokenEncryptionKey
	}
	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("encrypt New API user access token: %w", err)
	}
	return newAPIUserAccessTokenCiphertextPrefix + ciphertext, nil
}

func (s *UpstreamBillingProbeService) newAPIUserAccessTokenFromAccount(
	account *Account,
) (string, *newAPIWalletProbeFailure) {
	if !NewAPIUserAccessTokenConfigured(account) {
		return "", nil
	}
	stored, _ := account.Credentials[NewAPIUserAccessTokenCredentialKey].(string)
	if !strings.HasPrefix(stored, newAPIUserAccessTokenCiphertextPrefix) {
		return "", &newAPIWalletProbeFailure{reason: "credential_reentry_required"}
	}
	if s == nil || !s.secretKeyConfigured || s.secretEncryptor == nil {
		return "", &newAPIWalletProbeFailure{reason: "encryption_key_unavailable"}
	}
	plaintext, err := s.secretEncryptor.Decrypt(strings.TrimPrefix(stored, newAPIUserAccessTokenCiphertextPrefix))
	if err != nil {
		return "", &newAPIWalletProbeFailure{reason: "credential_decrypt_failed"}
	}
	token, err := normalizeNewAPIUserAccessToken(plaintext)
	if err != nil || token == "" {
		return "", &newAPIWalletProbeFailure{reason: "credential_invalid"}
	}
	return token, nil
}

func normalizeNewAPIUserAccessToken(value string) (string, error) {
	token := strings.TrimSpace(value)
	if token == "" {
		return "", nil
	}
	if len(token) > newAPIUserAccessTokenMaxBytes || !utf8.ValidString(token) {
		return "", ErrNewAPIUserAccessTokenInvalid
	}
	for _, char := range token {
		// New API personal access tokens are opaque printable bearer tokens.
		// Reject whitespace, controls and non-ASCII input so a stored value can
		// never inject or split an outbound Authorization header.
		if char <= 0x20 || char >= 0x7f {
			return "", ErrNewAPIUserAccessTokenInvalid
		}
	}
	return token, nil
}

func isConfirmedNewAPIProbe(snapshot *UpstreamBillingProbeSnapshot) bool {
	return snapshot != nil && snapshot.Status == UpstreamBillingProbeStatusOK &&
		snapshot.Data != nil && snapshot.Data["provider"] == newAPIProviderName
}

func validateNewAPIUpstreamGroupSelection(snapshot *UpstreamBillingProbeSnapshot, requested string) error {
	if !isConfirmedNewAPIProbe(snapshot) ||
		snapshot.Data["groups_status"] != UpstreamBillingProbeStatusOK {
		return ErrNewAPIUpstreamGroupRequiresProbe
	}
	rawGroups, exists := snapshot.Data["available_groups"]
	if !exists {
		return ErrNewAPIUpstreamGroupRequiresProbe
	}
	groups, err := decodeNewAPIUpstreamGroups(rawGroups)
	if err != nil {
		return ErrNewAPIUpstreamGroupRequiresProbe
	}
	for _, group := range groups {
		if group.Name == requested {
			return nil
		}
	}
	return ErrNewAPIUpstreamGroupUnavailable
}

func decodeNewAPIUpstreamGroups(value any) ([]NewAPIUpstreamGroup, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var groups []NewAPIUpstreamGroup
	if err := json.Unmarshal(raw, &groups); err != nil || len(groups) > newAPIGroupMaxCount {
		return nil, errors.New("invalid New API group list")
	}
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		name, ok := normalizeNewAPIUpstreamGroupName(group.Name)
		if !ok || name != group.Name || !finiteNumber(group.RateMultiplier) ||
			group.RateMultiplier < 0 || group.RateMultiplier > upstreamBillingRateSyncMaxMultiplier {
			return nil, errors.New("invalid New API group list")
		}
		if _, exists := seen[name]; exists {
			return nil, errors.New("invalid New API group list")
		}
		seen[name] = struct{}{}
	}
	return groups, nil
}

func (s *UpstreamBillingProbeService) doNewAPIProbeRequest(
	ctx context.Context,
	account *Account,
	normalizedBaseURL string,
	endpoint string,
	apiKey string,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	maxBodyBytes int64,
	now time.Time,
) *newAPIHTTPResult {
	return s.doNewAPIRequest(
		ctx, http.MethodGet, account, normalizedBaseURL, endpoint, apiKey, nil,
		proxyURL, profile, tlsProfile, maxBodyBytes, now,
	)
}

func (s *UpstreamBillingProbeService) doNewAPIDashboardProbeRequest(
	ctx context.Context,
	method string,
	account *Account,
	normalizedBaseURL string,
	endpoint string,
	auth newAPIDashboardAuth,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	maxBodyBytes int64,
	now time.Time,
) *newAPIHTTPResult {
	return s.doNewAPIRequest(
		ctx, method, account, normalizedBaseURL, endpoint, "", &auth,
		proxyURL, profile, tlsProfile, maxBodyBytes, now,
	)
}

func (s *UpstreamBillingProbeService) doNewAPIRequest(
	ctx context.Context,
	method string,
	account *Account,
	normalizedBaseURL string,
	endpoint string,
	apiKey string,
	dashboardAuth *newAPIDashboardAuth,
	proxyURL string,
	profile HTTPUpstreamProfile,
	tlsProfile *tlsfingerprint.Profile,
	maxBodyBytes int64,
	now time.Time,
) *newAPIHTTPResult {
	probeURL, err := buildNewAPIEndpointURL(normalizedBaseURL, endpoint)
	if err != nil {
		return &newAPIHTTPResult{reason: "request_build_failed"}
	}
	req, err := http.NewRequestWithContext(ctx, method, probeURL, nil)
	if err != nil {
		return &newAPIHTTPResult{reason: "request_build_failed"}
	}
	reqCtx := WithHTTPUpstreamProfile(req.Context(), profile)
	reqCtx = WithHTTPUpstreamResolvedIPPinning(reqCtx)
	req = req.WithContext(WithHTTPUpstreamRedirectsDisabled(reqCtx))
	req.Header.Set("Accept", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	account.ApplyHeaderOverrides(req.Header)
	if dashboardAuth != nil {
		deleteHeaderCaseInsensitive(req.Header, "Authorization")
		deleteHeaderCaseInsensitive(req.Header, "New-Api-User")
		if dashboardAuth.legacy {
			req.Header.Set("Authorization", dashboardAuth.accessToken)
			req.Header.Set("New-Api-User", strconv.FormatInt(dashboardAuth.userID, 10))
		} else {
			req.Header.Set("Authorization", "Bearer "+dashboardAuth.accessToken)
		}
	}

	resp, err := s.accountTestService.httpUpstream.DoWithTLS(
		req, proxyURL, account.ID, account.Concurrency, tlsProfile,
	)
	if err != nil {
		reason := upstreamBillingProbeRequestFailureReason(err)
		logUpstreamBillingProbeRequestFailure(account.ID, req, proxyURL != "", reason, err)
		return &newAPIHTTPResult{reason: reason}
	}
	if resp == nil || resp.Body == nil {
		return &newAPIHTTPResult{reason: "empty_response"}
	}
	defer func() { _ = resp.Body.Close() }()
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	result := &newAPIHTTPResult{
		statusCode: resp.StatusCode,
		retryAfter: retryAfter(resp.Header, now),
	}
	if readErr != nil {
		result.reason = "response_read_failed"
		return result
	}
	if int64(len(body)) > maxBodyBytes {
		result.reason = "response_too_large"
		return result
	}
	result.body = body
	return result
}

func buildNewAPIEndpointURL(baseURL string, endpoint string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid New API base URL")
	}
	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil || parsedEndpoint.IsAbs() || parsedEndpoint.Host != "" || parsedEndpoint.Fragment != "" {
		return "", fmt.Errorf("invalid New API endpoint")
	}
	path := strings.TrimRight(parsed.Path, "/")
	lowerPath := strings.ToLower(path)
	for _, suffix := range []string{"/api/v1", "/v1"} {
		if strings.HasSuffix(lowerPath, suffix) {
			path = path[:len(path)-len(suffix)]
			break
		}
	}
	parsed.Path = strings.TrimRight(path, "/") + "/" + strings.TrimLeft(parsedEndpoint.Path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = parsedEndpoint.RawQuery
	parsed.Fragment = ""
	return parsed.String(), nil
}

func deleteHeaderCaseInsensitive(header http.Header, name string) {
	for existing := range header {
		if strings.EqualFold(existing, name) {
			delete(header, existing)
		}
	}
}

func newAPIResultFailureReason(result *newAPIHTTPResult) string {
	if result == nil {
		return "empty_response"
	}
	if result.reason != "" {
		return result.reason
	}
	if result.statusCode == http.StatusUnauthorized || result.statusCode == http.StatusForbidden {
		return "unauthorized"
	}
	if result.statusCode == http.StatusNotFound || result.statusCode == http.StatusMethodNotAllowed {
		return "unsupported"
	}
	return "http_error"
}

func finiteNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func finitePositive(value float64) bool {
	return finiteNumber(value) && value > 0
}
