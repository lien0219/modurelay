#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("apply_usage_detail_privacy.py")
text = path.read_text(encoding="utf-8")

replacements = [
    (
        '''replace_once(
    "backend/internal/service/setting_parse.go",
    '\\t\\tSettingKeyAllowUserViewErrorRequests:                 "false",',
    '\\t\\tSettingKeyAllowUserViewErrorRequests:                 "false",\\n'
    '\\t\\tSettingKeyUsageDetailShowUnitPrices:                  "true",\\n'
    '\\t\\tSettingKeyUsageDetailShowRateMultiplier:              "true",\\n'
    '\\t\\tSettingKeyUsageDetailShowOriginalCost:                "true",',
)
''',
        '''regex_once(
    "backend/internal/service/setting_parse.go",
    r'(?m)^(?P<indent>\\s*)SettingKeyAllowUserViewErrorRequests:\\s*"false",\\s*$',
    r'\\g<indent>SettingKeyAllowUserViewErrorRequests: "false",\\n'
    r'\\g<indent>SettingKeyUsageDetailShowUnitPrices: "true",\\n'
    r'\\g<indent>SettingKeyUsageDetailShowRateMultiplier: "true",\\n'
    r'\\g<indent>SettingKeyUsageDetailShowOriginalCost: "true",',
)
''',
        "setting_parse default replacement",
    ),
    (
        '''replace_once(
    "frontend/src/types/index.ts",
    ''' + "'''" + '''  input_cost: number
  output_cost: number
  cache_creation_cost: number
  cache_read_cost: number
  total_cost: number
  actual_cost: number
  rate_multiplier: number
''' + "'''" + ''',
    ''' + "'''" + '''  input_cost?: number | null
  output_cost?: number | null
  cache_creation_cost?: number | null
  cache_read_cost?: number | null
  total_cost?: number | null
  actual_cost: number
  rate_multiplier?: number | null
''' + "'''" + ''',
)
''',
        "",
        "UsageLog cost type widening",
    ),
    (
        '''replace_once(
    "frontend/src/types/index.ts",
    '  image_input_cost: number\\n  image_output_tokens: number\\n  image_output_cost: number\\n',
    '  image_input_cost?: number | null\\n  image_output_tokens: number\\n  image_output_cost?: number | null\\n',
)
''',
        "",
        "UsageLog image cost type widening",
    ),
    (
        '''replace_once(
    "frontend/src/utils/billingMode.ts",
    '  total_cost: number\\n',
    '  total_cost?: number | null\\n',
)
''',
        "",
        "billingMode total cost type widening",
    ),
]

for old, new, label in replacements:
    if old not in text:
        raise SystemExit(f"expected {label} block was not found")
    text = text.replace(old, new, 1)

path.write_text(text, encoding="utf-8")
print("usage privacy patch fixer applied")
