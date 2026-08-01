#!/usr/bin/env python3
from __future__ import annotations

import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def read(rel: str) -> str:
    return (ROOT / rel).read_text(encoding="utf-8")


def write(rel: str, content: str) -> None:
    path = ROOT / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")


def replace_once(rel: str, old: str, new: str) -> None:
    text = read(rel)
    count = text.count(old)
    if count != 1:
        raise RuntimeError(f"{rel}: expected exactly one anchor, found {count}: {old[:120]!r}")
    write(rel, text.replace(old, new, 1))


def replace_n(rel: str, old: str, new: str, expected: int) -> None:
    text = read(rel)
    count = text.count(old)
    if count != expected:
        raise RuntimeError(f"{rel}: expected {expected} anchors, found {count}: {old[:120]!r}")
    write(rel, text.replace(old, new))


def regex_once(rel: str, pattern: str, replacement: str, flags: int = 0) -> None:
    text = read(rel)
    updated, count = re.subn(pattern, replacement, text, count=1, flags=flags)
    if count != 1:
        raise RuntimeError(f"{rel}: regex expected one match, found {count}: {pattern!r}")
    write(rel, updated)


# ---------------------------------------------------------------------------
# Backend: persisted settings and public/admin views
# ---------------------------------------------------------------------------
replace_once(
    "backend/internal/service/domain_constants.go",
    '// SettingKeyAllowUserViewErrorRequests controls whether end users can view\n// their own failed requests on the usage page. Default false (opt-in).\nconst SettingKeyAllowUserViewErrorRequests = "allow_user_view_error_requests"',
    '// SettingKeyAllowUserViewErrorRequests controls whether end users can view\n// their own failed requests on the usage page. Default false (opt-in).\nconst SettingKeyAllowUserViewErrorRequests = "allow_user_view_error_requests"\n\n// User usage cost-detail visibility. These are opt-out settings so existing\n// installations preserve their current UI and API behavior after upgrading.\nconst (\n\tSettingKeyUsageDetailShowUnitPrices     = "usage_detail_show_unit_prices"\n\tSettingKeyUsageDetailShowRateMultiplier = "usage_detail_show_rate_multiplier"\n\tSettingKeyUsageDetailShowOriginalCost   = "usage_detail_show_original_cost"\n)',
)

replace_once(
    "backend/internal/service/setting_parse.go",
    '\t\tSettingKeyAllowUserViewErrorRequests:                 "false",',
    '\t\tSettingKeyAllowUserViewErrorRequests:                 "false",\n'
    '\t\tSettingKeyUsageDetailShowUnitPrices:                  "true",\n'
    '\t\tSettingKeyUsageDetailShowRateMultiplier:              "true",\n'
    '\t\tSettingKeyUsageDetailShowOriginalCost:                "true",',
)
replace_once(
    "backend/internal/service/setting_parse.go",
    '\tresult.AllowUserViewErrorRequests = settings[SettingKeyAllowUserViewErrorRequests] == "true" // default false',
    '\tresult.AllowUserViewErrorRequests = settings[SettingKeyAllowUserViewErrorRequests] == "true" // default false\n'
    '\tresult.UsageDetailShowUnitPrices = !isFalseSettingValue(settings[SettingKeyUsageDetailShowUnitPrices])\n'
    '\tresult.UsageDetailShowRateMultiplier = !isFalseSettingValue(settings[SettingKeyUsageDetailShowRateMultiplier])\n'
    '\tresult.UsageDetailShowOriginalCost = !isFalseSettingValue(settings[SettingKeyUsageDetailShowOriginalCost])',
)

replace_once(
    "backend/internal/service/settings_view.go",
    '\t// 允许终端用户在用量页查看自己的失败请求\n\tAllowUserViewErrorRequests bool\n}',
    '\t// 允许终端用户在用量页查看自己的失败请求\n\tAllowUserViewErrorRequests bool\n\n'
    '\t// 普通用户用量费用明细展示（管理员始终可见）\n'
    '\tUsageDetailShowUnitPrices     bool\n'
    '\tUsageDetailShowRateMultiplier bool\n'
    '\tUsageDetailShowOriginalCost   bool\n}',
)
replace_once(
    "backend/internal/service/settings_view.go",
    '\t// 允许终端用户在用量页查看自己的失败请求\n\tAllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`\n}',
    '\t// 允许终端用户在用量页查看自己的失败请求\n\tAllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`\n\n'
    '\tUsageDetailShowUnitPrices     bool `json:"usage_detail_show_unit_prices"`\n'
    '\tUsageDetailShowRateMultiplier bool `json:"usage_detail_show_rate_multiplier"`\n'
    '\tUsageDetailShowOriginalCost   bool `json:"usage_detail_show_original_cost"`\n}',
)

replace_once(
    "backend/internal/service/setting_update.go",
    '\tupdates[SettingKeyAllowUserViewErrorRequests] = strconv.FormatBool(settings.AllowUserViewErrorRequests)\n',
    '\tupdates[SettingKeyAllowUserViewErrorRequests] = strconv.FormatBool(settings.AllowUserViewErrorRequests)\n'
    '\tupdates[SettingKeyUsageDetailShowUnitPrices] = strconv.FormatBool(settings.UsageDetailShowUnitPrices)\n'
    '\tupdates[SettingKeyUsageDetailShowRateMultiplier] = strconv.FormatBool(settings.UsageDetailShowRateMultiplier)\n'
    '\tupdates[SettingKeyUsageDetailShowOriginalCost] = strconv.FormatBool(settings.UsageDetailShowOriginalCost)\n',
)

replace_once(
    "backend/internal/service/setting_public.go",
    '\t\tSettingKeyAllowUserViewErrorRequests,\n',
    '\t\tSettingKeyAllowUserViewErrorRequests,\n'
    '\t\tSettingKeyUsageDetailShowUnitPrices,\n'
    '\t\tSettingKeyUsageDetailShowRateMultiplier,\n'
    '\t\tSettingKeyUsageDetailShowOriginalCost,\n',
)
replace_once(
    "backend/internal/service/setting_public.go",
    '\t\tAllowUserViewErrorRequests: settings[SettingKeyAllowUserViewErrorRequests] == "true",\n',
    '\t\tAllowUserViewErrorRequests: settings[SettingKeyAllowUserViewErrorRequests] == "true",\n'
    '\t\tUsageDetailShowUnitPrices: !isFalseSettingValue(settings[SettingKeyUsageDetailShowUnitPrices]),\n'
    '\t\tUsageDetailShowRateMultiplier: !isFalseSettingValue(settings[SettingKeyUsageDetailShowRateMultiplier]),\n'
    '\t\tUsageDetailShowOriginalCost: !isFalseSettingValue(settings[SettingKeyUsageDetailShowOriginalCost]),\n',
)
replace_once(
    "backend/internal/service/setting_public.go",
    '\tAllowUserViewErrorRequests           bool `json:"allow_user_view_error_requests"`\n}',
    '\tAllowUserViewErrorRequests           bool `json:"allow_user_view_error_requests"`\n'
    '\tUsageDetailShowUnitPrices            bool `json:"usage_detail_show_unit_prices"`\n'
    '\tUsageDetailShowRateMultiplier        bool `json:"usage_detail_show_rate_multiplier"`\n'
    '\tUsageDetailShowOriginalCost          bool `json:"usage_detail_show_original_cost"`\n}',
)
replace_once(
    "backend/internal/service/setting_public.go",
    '\t\tAllowUserViewErrorRequests:           settings.AllowUserViewErrorRequests,\n',
    '\t\tAllowUserViewErrorRequests:           settings.AllowUserViewErrorRequests,\n'
    '\t\tUsageDetailShowUnitPrices:            settings.UsageDetailShowUnitPrices,\n'
    '\t\tUsageDetailShowRateMultiplier:        settings.UsageDetailShowRateMultiplier,\n'
    '\t\tUsageDetailShowOriginalCost:          settings.UsageDetailShowOriginalCost,\n',
)
replace_once(
    "backend/internal/service/setting_public.go",
    '''func (s *SettingService) IsUserErrorViewAllowed(ctx context.Context) bool {
\tvals, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyAllowUserViewErrorRequests})
\tif err != nil {
\t\tslog.Warn("failed to get allow_user_view_error_requests setting, defaulting to false", "error", err)
\t\treturn false
\t}
\treturn vals[SettingKeyAllowUserViewErrorRequests] == "true"
}
''',
    '''func (s *SettingService) IsUserErrorViewAllowed(ctx context.Context) bool {
\tvals, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyAllowUserViewErrorRequests})
\tif err != nil {
\t\tslog.Warn("failed to get allow_user_view_error_requests setting, defaulting to false", "error", err)
\t\treturn false
\t}
\treturn vals[SettingKeyAllowUserViewErrorRequests] == "true"
}

// UsageDetailVisibility controls which billing calculation details are exposed to
// ordinary users. It affects both rendering and the user usage API response.
type UsageDetailVisibility struct {
\tShowUnitPrices     bool
\tShowRateMultiplier bool
\tShowOriginalCost   bool
}

func defaultUsageDetailVisibility() UsageDetailVisibility {
\treturn UsageDetailVisibility{
\t\tShowUnitPrices:     true,
\t\tShowRateMultiplier: true,
\t\tShowOriginalCost:   true,
\t}
}

// GetUsageDetailVisibility reads all three switches in one query. It deliberately
// fails open to the historical behavior so a transient settings-store failure does
// not unexpectedly remove fields from an already-running production deployment.
func (s *SettingService) GetUsageDetailVisibility(ctx context.Context) UsageDetailVisibility {
\tdefaults := defaultUsageDetailVisibility()
\tif s == nil || s.settingRepo == nil {
\t\treturn defaults
\t}
\tvals, err := s.settingRepo.GetMultiple(ctx, []string{
\t\tSettingKeyUsageDetailShowUnitPrices,
\t\tSettingKeyUsageDetailShowRateMultiplier,
\t\tSettingKeyUsageDetailShowOriginalCost,
\t})
\tif err != nil {
\t\tslog.Warn("failed to get user usage detail visibility, defaulting to visible", "error", err)
\t\treturn defaults
\t}
\treturn UsageDetailVisibility{
\t\tShowUnitPrices:     !isFalseSettingValue(vals[SettingKeyUsageDetailShowUnitPrices]),
\t\tShowRateMultiplier: !isFalseSettingValue(vals[SettingKeyUsageDetailShowRateMultiplier]),
\t\tShowOriginalCost:   !isFalseSettingValue(vals[SettingKeyUsageDetailShowOriginalCost]),
\t}
}
''',
)

# DTOs used by admin and public settings APIs.
replace_n(
    "backend/internal/handler/dto/settings.go",
    '\tAllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`',
    '\tAllowUserViewErrorRequests bool `json:"allow_user_view_error_requests"`\n'
    '\tUsageDetailShowUnitPrices     bool `json:"usage_detail_show_unit_prices"`\n'
    '\tUsageDetailShowRateMultiplier bool `json:"usage_detail_show_rate_multiplier"`\n'
    '\tUsageDetailShowOriginalCost   bool `json:"usage_detail_show_original_cost"`',
    2,
)

replace_once(
    "backend/internal/handler/admin/setting_handler_update.go",
    '\tAllowUserViewErrorRequests *bool `json:"allow_user_view_error_requests"`\n}',
    '\tAllowUserViewErrorRequests *bool `json:"allow_user_view_error_requests"`\n'
    '\tUsageDetailShowUnitPrices     *bool `json:"usage_detail_show_unit_prices"`\n'
    '\tUsageDetailShowRateMultiplier *bool `json:"usage_detail_show_rate_multiplier"`\n'
    '\tUsageDetailShowOriginalCost   *bool `json:"usage_detail_show_original_cost"`\n}',
)
replace_once(
    "backend/internal/handler/admin/setting_handler_update.go",
    '''\t\tAllowUserViewErrorRequests: func() bool {
\t\t\tif req.AllowUserViewErrorRequests != nil {
\t\t\t\treturn *req.AllowUserViewErrorRequests
\t\t\t}
\t\t\treturn previousSettings.AllowUserViewErrorRequests
\t\t}(),
''',
    '''\t\tAllowUserViewErrorRequests: func() bool {
\t\t\tif req.AllowUserViewErrorRequests != nil {
\t\t\t\treturn *req.AllowUserViewErrorRequests
\t\t\t}
\t\t\treturn previousSettings.AllowUserViewErrorRequests
\t\t}(),
\t\tUsageDetailShowUnitPrices: func() bool {
\t\t\tif req.UsageDetailShowUnitPrices != nil {
\t\t\t\treturn *req.UsageDetailShowUnitPrices
\t\t\t}
\t\t\treturn previousSettings.UsageDetailShowUnitPrices
\t\t}(),
\t\tUsageDetailShowRateMultiplier: func() bool {
\t\t\tif req.UsageDetailShowRateMultiplier != nil {
\t\t\t\treturn *req.UsageDetailShowRateMultiplier
\t\t\t}
\t\t\treturn previousSettings.UsageDetailShowRateMultiplier
\t\t}(),
\t\tUsageDetailShowOriginalCost: func() bool {
\t\t\tif req.UsageDetailShowOriginalCost != nil {
\t\t\t\treturn *req.UsageDetailShowOriginalCost
\t\t\t}
\t\t\treturn previousSettings.UsageDetailShowOriginalCost
\t\t}(),
''',
)

replace_once(
    "backend/internal/handler/admin/setting_handler.go",
    '\t\tAllowUserViewErrorRequests: settings.AllowUserViewErrorRequests,\n',
    '\t\tAllowUserViewErrorRequests: settings.AllowUserViewErrorRequests,\n'
    '\t\tUsageDetailShowUnitPrices: settings.UsageDetailShowUnitPrices,\n'
    '\t\tUsageDetailShowRateMultiplier: settings.UsageDetailShowRateMultiplier,\n'
    '\t\tUsageDetailShowOriginalCost: settings.UsageDetailShowOriginalCost,\n',
)
replace_once(
    "backend/internal/handler/setting_handler.go",
    '\t\tAllowUserViewErrorRequests: settings.AllowUserViewErrorRequests,\n',
    '\t\tAllowUserViewErrorRequests: settings.AllowUserViewErrorRequests,\n'
    '\t\tUsageDetailShowUnitPrices: settings.UsageDetailShowUnitPrices,\n'
    '\t\tUsageDetailShowRateMultiplier: settings.UsageDetailShowRateMultiplier,\n'
    '\t\tUsageDetailShowOriginalCost: settings.UsageDetailShowOriginalCost,\n',
)

# ---------------------------------------------------------------------------
# Backend: strict redaction for ordinary-user usage endpoints
# ---------------------------------------------------------------------------
write(
    "backend/internal/handler/dto/usage_visibility.go",
    '''package dto

import (
\t"encoding/json"

\t"github.com/Wei-Shaw/sub2api/internal/service"
)

var userUsageUnitPriceFields = []string{
\t"input_cost",
\t"output_cost",
\t"cache_creation_cost",
\t"cache_read_cost",
\t"image_input_cost",
\t"image_output_cost",
}

// UsageLogFromServiceWithVisibility converts a service usage record and removes
// disabled billing-calculation fields before the payload reaches JSON encoding.
// Admin endpoints continue using UsageLogFromServiceAdmin and remain unaffected.
func UsageLogFromServiceWithVisibility(log *service.UsageLog, visibility service.UsageDetailVisibility) map[string]any {
\treturn RedactUsageLogForUser(UsageLogFromService(log), visibility)
}

// RedactUsageLogForUser performs response-level redaction rather than UI-only
// hiding. The omitted keys therefore cannot be recovered through DevTools.
func RedactUsageLogForUser(log *UsageLog, visibility service.UsageDetailVisibility) map[string]any {
\tif log == nil {
\t\treturn nil
\t}
\traw, err := json.Marshal(log)
\tif err != nil {
\t\treturn map[string]any{}
\t}
\tpayload := make(map[string]any)
\tif err := json.Unmarshal(raw, &payload); err != nil {
\t\treturn map[string]any{}
\t}

\tif !visibility.ShowUnitPrices {
\t\tfor _, field := range userUsageUnitPriceFields {
\t\t\tdelete(payload, field)
\t\t}
\t}
\tif !visibility.ShowRateMultiplier {
\t\tdelete(payload, "rate_multiplier")
\t\tredactNestedGroupRate(payload["group"])
\t\tif apiKey, ok := payload["api_key"].(map[string]any); ok {
\t\t\tredactNestedGroupRate(apiKey["group"])
\t\t}
\t}
\tif !visibility.ShowOriginalCost {
\t\tdelete(payload, "total_cost")
\t}
\treturn payload
}

func redactNestedGroupRate(value any) {
\tif group, ok := value.(map[string]any); ok {
\t\tdelete(group, "rate_multiplier")
\t}
}
''',
)
write(
    "backend/internal/handler/dto/usage_visibility_test.go",
    '''package dto

import (
\t"encoding/json"
\t"testing"

\t"github.com/Wei-Shaw/sub2api/internal/service"
\t"github.com/stretchr/testify/require"
)

func TestRedactUsageLogForUserOmitsDisabledBillingFields(t *testing.T) {
\tlog := &UsageLog{
\t\tID:                1,
\t\tInputCost:         1.25,
\t\tOutputCost:        2.5,
\t\tCacheCreationCost: 0.5,
\t\tCacheReadCost:     0.25,
\t\tImageInputCost:    0.75,
\t\tImageOutputCost:   1.75,
\t\tTotalCost:         7,
\t\tActualCost:        0.42,
\t\tRateMultiplier:    0.06,
\t}

\tpayload := RedactUsageLogForUser(log, service.UsageDetailVisibility{})
\traw, err := json.Marshal(payload)
\trequire.NoError(t, err)
\tencoded := string(raw)

\tfor _, key := range append(append([]string{}, userUsageUnitPriceFields...), "rate_multiplier", "total_cost") {
\t\trequire.NotContains(t, encoded, `"`+key+`"`)
\t}
\trequire.Contains(t, encoded, `"actual_cost":0.42`)
}

func TestRedactUsageLogForUserKeepsEnabledBillingFields(t *testing.T) {
\tlog := &UsageLog{
\t\tID:             1,
\t\tInputCost:      1.25,
\t\tOutputCost:     2.5,
\t\tTotalCost:      3.75,
\t\tActualCost:     0.225,
\t\tRateMultiplier: 0.06,
\t}

\tpayload := RedactUsageLogForUser(log, service.UsageDetailVisibility{
\t\tShowUnitPrices:     true,
\t\tShowRateMultiplier: true,
\t\tShowOriginalCost:   true,
\t})
\trequire.Equal(t, 1.25, payload["input_cost"])
\trequire.Equal(t, 2.5, payload["output_cost"])
\trequire.Equal(t, 3.75, payload["total_cost"])
\trequire.Equal(t, 0.06, payload["rate_multiplier"])
}
''',
)

replace_once(
    "backend/internal/handler/usage_handler.go",
    '''\tout := make([]dto.UsageLog, 0, len(records))
\tfor i := range records {
\t\tout = append(out, *dto.UsageLogFromService(&records[i]))
\t}
\tresponse.Paginated(c, out, result.Total, page, pageSize)
''',
    '''\tvisibility := service.UsageDetailVisibility{
\t\tShowUnitPrices:     true,
\t\tShowRateMultiplier: true,
\t\tShowOriginalCost:   true,
\t}
\tif h.settingService != nil {
\t\tvisibility = h.settingService.GetUsageDetailVisibility(c.Request.Context())
\t}
\tout := make([]map[string]any, 0, len(records))
\tfor i := range records {
\t\tout = append(out, dto.UsageLogFromServiceWithVisibility(&records[i], visibility))
\t}
\tresponse.Paginated(c, out, result.Total, page, pageSize)
''',
)
replace_once(
    "backend/internal/handler/usage_handler.go",
    '\tresponse.Success(c, dto.UsageLogFromService(record))\n',
    '''\tvisibility := service.UsageDetailVisibility{
\t\tShowUnitPrices:     true,
\t\tShowRateMultiplier: true,
\t\tShowOriginalCost:   true,
\t}
\tif h.settingService != nil {
\t\tvisibility = h.settingService.GetUsageDetailVisibility(c.Request.Context())
\t}
\tresponse.Success(c, dto.UsageLogFromServiceWithVisibility(record, visibility))
''',
)

# ---------------------------------------------------------------------------
# Frontend: types, public settings cache, admin controls and usage rendering
# ---------------------------------------------------------------------------
replace_once(
    "frontend/src/api/admin/settings.ts",
    '  // Allow user view error requests\n  allow_user_view_error_requests: boolean;\n}',
    '  // Allow user view error requests\n  allow_user_view_error_requests: boolean;\n'
    '  usage_detail_show_unit_prices: boolean;\n'
    '  usage_detail_show_rate_multiplier: boolean;\n'
    '  usage_detail_show_original_cost: boolean;\n}',
)
replace_once(
    "frontend/src/api/admin/settings.ts",
    '  allow_user_view_error_requests?: boolean;\n}',
    '  allow_user_view_error_requests?: boolean;\n'
    '  usage_detail_show_unit_prices?: boolean;\n'
    '  usage_detail_show_rate_multiplier?: boolean;\n'
    '  usage_detail_show_original_cost?: boolean;\n}',
)

replace_once(
    "frontend/src/types/index.ts",
    '  allow_user_view_error_requests?: boolean\n}',
    '  allow_user_view_error_requests?: boolean\n'
    '  usage_detail_show_unit_prices?: boolean\n'
    '  usage_detail_show_rate_multiplier?: boolean\n'
    '  usage_detail_show_original_cost?: boolean\n}',
)
replace_once(
    "frontend/src/types/index.ts",
    '''  input_cost: number
  output_cost: number
  cache_creation_cost: number
  cache_read_cost: number
  total_cost: number
  actual_cost: number
  rate_multiplier: number
''',
    '''  input_cost?: number | null
  output_cost?: number | null
  cache_creation_cost?: number | null
  cache_read_cost?: number | null
  total_cost?: number | null
  actual_cost: number
  rate_multiplier?: number | null
''',
)
replace_once(
    "frontend/src/types/index.ts",
    '  image_input_cost: number\n  image_output_tokens: number\n  image_output_cost: number\n',
    '  image_input_cost?: number | null\n  image_output_tokens: number\n  image_output_cost?: number | null\n',
)
replace_once(
    "frontend/src/utils/billingMode.ts",
    '  total_cost: number\n',
    '  total_cost?: number | null\n',
)

replace_once(
    "frontend/src/stores/app.ts",
    '  const backendModeEnabled = computed(() => cachedPublicSettings.value?.backend_mode_enabled ?? false)\n',
    '  const backendModeEnabled = computed(() => cachedPublicSettings.value?.backend_mode_enabled ?? false)\n'
    '  const usageDetailShowUnitPrices = computed(() => cachedPublicSettings.value?.usage_detail_show_unit_prices ?? true)\n'
    '  const usageDetailShowRateMultiplier = computed(() => cachedPublicSettings.value?.usage_detail_show_rate_multiplier ?? true)\n'
    '  const usageDetailShowOriginalCost = computed(() => cachedPublicSettings.value?.usage_detail_show_original_cost ?? true)\n',
)
replace_once(
    "frontend/src/stores/app.ts",
    '        allow_user_view_error_requests: false,\n',
    '        allow_user_view_error_requests: false,\n'
    '        usage_detail_show_unit_prices: true,\n'
    '        usage_detail_show_rate_multiplier: true,\n'
    '        usage_detail_show_original_cost: true,\n',
)
replace_once(
    "frontend/src/stores/app.ts",
    '    hasActiveToasts,\n    backendModeEnabled,\n',
    '    hasActiveToasts,\n'
    '    backendModeEnabled,\n'
    '    usageDetailShowUnitPrices,\n'
    '    usageDetailShowRateMultiplier,\n'
    '    usageDetailShowOriginalCost,\n',
)

usage_display_card = '''          <!-- User Usage Cost Detail Visibility -->
          <div class="card">
            <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ localText("用户费用明细展示", "User cost detail display") }}
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                {{ localText("控制普通用户在用量记录中看到的计费计算信息。管理员用量记录始终完整显示。", "Control billing-calculation details shown in ordinary users' usage records. Admin usage records always remain complete.") }}
              </p>
            </div>
            <div class="divide-y divide-gray-100 px-6 dark:divide-dark-700">
              <div class="flex items-center justify-between gap-6 py-4">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">
                    {{ localText("显示输入/输出单价", "Show input/output unit prices") }}
                  </label>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {{ localText("关闭后，同时隐藏费用拆分、缓存及图片单价，并从普通用户用量 API 响应中移除对应成本字段。", "When disabled, cost breakdown, cache and image unit prices are hidden and the related cost fields are removed from user usage API responses.") }}
                  </p>
                </div>
                <Toggle v-model="form.usage_detail_show_unit_prices" />
              </div>
              <div class="flex items-center justify-between gap-6 py-4">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">
                    {{ localText("显示计费倍率", "Show billing multiplier") }}
                  </label>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {{ localText("关闭后隐藏本次请求倍率，并从普通用户用量 API 响应中移除倍率字段。", "When disabled, the request multiplier is hidden and removed from user usage API responses.") }}
                  </p>
                </div>
                <Toggle v-model="form.usage_detail_show_rate_multiplier" />
              </div>
              <div class="flex items-center justify-between gap-6 py-4">
                <div>
                  <label class="font-medium text-gray-900 dark:text-white">
                    {{ localText("显示原始金额", "Show original cost") }}
                  </label>
                  <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                    {{ localText("关闭后仅保留用户实际扣费，并从普通用户用量 API 响应中移除倍率应用前的原始金额。", "When disabled, only the user-billed amount remains and the pre-multiplier original cost is removed from user usage API responses.") }}
                  </p>
                </div>
                <Toggle v-model="form.usage_detail_show_original_cost" />
              </div>
            </div>
            <div class="border-t border-amber-200 bg-amber-50 px-6 py-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300">
              {{ localText("严格脱敏已启用：关闭任一开关后，相关字段不仅在界面隐藏，也无法通过浏览器开发者工具从普通用户接口读取。", "Strict redaction is enforced: disabling a switch also removes the related fields from ordinary-user API responses, not just the UI.") }}
            </div>
          </div>

'''
replace_once(
    "frontend/src/views/admin/SettingsView.vue",
    '          <!-- Custom Menu Items -->\n',
    usage_display_card + '          <!-- Custom Menu Items -->\n',
)
replace_once(
    "frontend/src/views/admin/SettingsView.vue",
    '  // Allow user view error requests\n  allow_user_view_error_requests: false,\n});',
    '  // Allow user view error requests\n'
    '  allow_user_view_error_requests: false,\n'
    '  // User usage cost-detail visibility (opt-out for upgrade compatibility)\n'
    '  usage_detail_show_unit_prices: true,\n'
    '  usage_detail_show_rate_multiplier: true,\n'
    '  usage_detail_show_original_cost: true,\n'
    '});',
)
replace_once(
    "frontend/src/views/admin/SettingsView.vue",
    '      allow_user_view_error_requests: form.allow_user_view_error_requests,\n',
    '      allow_user_view_error_requests: form.allow_user_view_error_requests,\n'
    '      usage_detail_show_unit_prices: form.usage_detail_show_unit_prices,\n'
    '      usage_detail_show_rate_multiplier: form.usage_detail_show_rate_multiplier,\n'
    '      usage_detail_show_original_cost: form.usage_detail_show_original_cost,\n',
)

# UsageTable props and reactive visibility controls.
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '  showAccountBilling?: boolean\n  showUpstreamEndpoint?: boolean\n',
    '  showAccountBilling?: boolean\n'
    '  showUpstreamEndpoint?: boolean\n'
    '  showUnitPrices?: boolean\n'
    '  showRateMultiplier?: boolean\n'
    '  showOriginalCost?: boolean\n',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '  showAccountBilling: true,\n  showUpstreamEndpoint: true,\n  flat: false\n',
    '  showAccountBilling: true,\n'
    '  showUpstreamEndpoint: true,\n'
    '  showUnitPrices: true,\n'
    '  showRateMultiplier: true,\n'
    '  showOriginalCost: true,\n'
    '  flat: false\n',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'const showAccountBilling = props.showAccountBilling\nconst showUpstreamEndpoint = props.showUpstreamEndpoint\n',
    'const showAccountBilling = props.showAccountBilling\n'
    'const showUpstreamEndpoint = props.showUpstreamEndpoint\n'
    'const showUnitPrices = computed(() => props.showUnitPrices)\n'
    'const showRateMultiplier = computed(() => props.showRateMultiplier)\n'
    'const showOriginalCost = computed(() => props.showOriginalCost)\n',
)

replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '<div class="mb-2 border-b border-gray-700 pb-1.5">\n            <div class="text-xs font-semibold text-gray-300 mb-1">{{ t(\'usage.costDetails\') }}</div>',
    '<div v-if="showUnitPrices || showOriginalCost || (tooltipData && isImageUsage(tooltipData))" class="mb-2 border-b border-gray-700 pb-1.5">\n'
    '            <div class="text-xs font-semibold text-gray-300 mb-1">{{ t(\'usage.costDetails\') }}</div>',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'v-if="tooltipData && tooltipData.input_cost > 0"',
    'v-if="showUnitPrices && tooltipData && (tooltipData.input_cost ?? 0) > 0"',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '${{ tooltipData.input_cost.toFixed(6) }}',
    '${{ (tooltipData.input_cost ?? 0).toFixed(6) }}',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'v-if="tooltipData && hasImageInputCost(tooltipData)"',
    'v-if="showUnitPrices && tooltipData && hasImageInputCost(tooltipData)"',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '${{ tooltipData.image_input_cost.toFixed(6) }}',
    '${{ (tooltipData.image_input_cost ?? 0).toFixed(6) }}',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'v-if="tooltipData && tooltipData.output_cost > 0"',
    'v-if="showUnitPrices && tooltipData && (tooltipData.output_cost ?? 0) > 0"',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '${{ tooltipData.output_cost.toFixed(6) }}',
    '${{ (tooltipData.output_cost ?? 0).toFixed(6) }}',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'v-if="tooltipData && hasImageOutputCost(tooltipData)"',
    'v-if="showUnitPrices && tooltipData && hasImageOutputCost(tooltipData)"',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '${{ tooltipData.image_output_cost.toFixed(6) }}',
    '${{ (tooltipData.image_output_cost ?? 0).toFixed(6) }}',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '<template v-if="tooltipData && !isImageUsage(tooltipData) && (!tooltipData.billing_mode || tooltipData.billing_mode === BILLING_MODE_TOKEN)">',
    '<template v-if="showUnitPrices && tooltipData && !isImageUsage(tooltipData) && (!tooltipData.billing_mode || tooltipData.billing_mode === BILLING_MODE_TOKEN)">',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'formatTokenPricePerMillion(tooltipData.input_cost, textInputTokens(tooltipData))',
    'formatTokenPricePerMillion(tooltipData.input_cost ?? 0, textInputTokens(tooltipData))',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'tooltipData.output_cost > 0 && textOutputTokens(tooltipData) > 0',
    '(tooltipData.output_cost ?? 0) > 0 && textOutputTokens(tooltipData) > 0',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'formatTokenPricePerMillion(tooltipData.output_cost, textOutputTokens(tooltipData))',
    'formatTokenPricePerMillion(tooltipData.output_cost ?? 0, textOutputTokens(tooltipData))',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '<div class="flex items-center justify-between gap-4">\n                <span class="text-gray-400">{{ t(\'usage.imageUnitPrice\') }}</span>',
    '<div v-if="showUnitPrices" class="flex items-center justify-between gap-4">\n'
    '                <span class="text-gray-400">{{ t(\'usage.imageUnitPrice\') }}</span>',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '<div class="flex items-center justify-between gap-4">\n                <span class="text-gray-400">{{ t(\'usage.imageTotalPrice\') }}</span>',
    '<div v-if="showOriginalCost" class="flex items-center justify-between gap-4">\n'
    '                <span class="text-gray-400">{{ t(\'usage.imageTotalPrice\') }}</span>',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '<div v-else class="flex items-center justify-between gap-4">\n              <span class="text-gray-400">{{ t(\'usage.unitPrice\') }}</span>',
    '<div v-else-if="showUnitPrices" class="flex items-center justify-between gap-4">\n'
    '              <span class="text-gray-400">{{ t(\'usage.unitPrice\') }}</span>',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'v-if="tooltipData && tooltipData.cache_creation_cost > 0"',
    'v-if="showUnitPrices && tooltipData && (tooltipData.cache_creation_cost ?? 0) > 0"',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '${{ tooltipData.cache_creation_cost.toFixed(6) }}',
    '${{ (tooltipData.cache_creation_cost ?? 0).toFixed(6) }}',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    'v-if="tooltipData && tooltipData.cache_read_cost > 0"',
    'v-if="showUnitPrices && tooltipData && (tooltipData.cache_read_cost ?? 0) > 0"',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '${{ tooltipData.cache_read_cost.toFixed(6) }}',
    '${{ (tooltipData.cache_read_cost ?? 0).toFixed(6) }}',
)
replace_once(
    "frontend/src/components/admin/usage/UsageTable.vue",
    '''          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.rate') }}</span>
            <span class="font-semibold text-blue-400">{{ formatMultiplier(tooltipData?.rate_multiplier || 1) }}x</span>
          </div>
          <div class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.original') }}</span>
            <span class="font-medium text-white">${{ tooltipData?.total_cost?.toFixed(6) || '0.000000' }}</span>
          </div>
''',
    '''          <div v-if="showRateMultiplier" class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.rate') }}</span>
            <span class="font-semibold text-blue-400">{{ formatMultiplier(tooltipData?.rate_multiplier ?? 1) }}x</span>
          </div>
          <div v-if="showOriginalCost" class="flex items-center justify-between gap-6">
            <span class="text-gray-400">{{ t('usage.original') }}</span>
            <span class="font-medium text-white">${{ tooltipData?.total_cost?.toFixed(6) || '0.000000' }}</span>
          </div>
''',
)

replace_once(
    "frontend/src/views/user/UsageView.vue",
    '          :show-account-billing="false"\n          :show-upstream-endpoint="false"\n',
    '          :show-account-billing="false"\n'
    '          :show-upstream-endpoint="false"\n'
    '          :show-unit-prices="appStore.usageDetailShowUnitPrices"\n'
    '          :show-rate-multiplier="appStore.usageDetailShowRateMultiplier"\n'
    '          :show-original-cost="appStore.usageDetailShowOriginalCost"\n',
)

print("usage-detail privacy patch applied successfully")
