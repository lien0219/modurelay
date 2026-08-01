#!/usr/bin/env python3
from pathlib import Path

path = Path(__file__).with_name("apply_usage_detail_privacy.py")
text = path.read_text(encoding="utf-8")

old = '''replace_once(
    "backend/internal/service/setting_parse.go",
    '\\t\\tSettingKeyAllowUserViewErrorRequests:                 "false",',
    '\\t\\tSettingKeyAllowUserViewErrorRequests:                 "false",\\n'
    '\\t\\tSettingKeyUsageDetailShowUnitPrices:                  "true",\\n'
    '\\t\\tSettingKeyUsageDetailShowRateMultiplier:              "true",\\n'
    '\\t\\tSettingKeyUsageDetailShowOriginalCost:                "true",',
)
'''
new = '''regex_once(
    "backend/internal/service/setting_parse.go",
    r'(?m)^(?P<indent>\\s*)SettingKeyAllowUserViewErrorRequests:\\s*"false",\\s*$',
    r'\\g<indent>SettingKeyAllowUserViewErrorRequests: "false",\\n'
    r'\\g<indent>SettingKeyUsageDetailShowUnitPrices: "true",\\n'
    r'\\g<indent>SettingKeyUsageDetailShowRateMultiplier: "true",\\n'
    r'\\g<indent>SettingKeyUsageDetailShowOriginalCost: "true",',
)
'''

if old not in text:
    raise SystemExit("expected setting_parse default replacement block was not found")

path.write_text(text.replace(old, new, 1), encoding="utf-8")
print("usage privacy patch fixer applied")
