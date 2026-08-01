#!/usr/bin/env python3
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
path = root / "backend/internal/server/api_contract_test.go"
text = path.read_text(encoding="utf-8")

pattern = re.compile(
    r'(?m)^(?P<indent>\s*)"allow_user_view_error_requests": false(?P<trailing>,?)\s*$'
)


def replacement(match: re.Match[str]) -> str:
    indent = match.group("indent")
    return (
        f'{indent}"allow_user_view_error_requests": false,\n'
        f'{indent}"usage_detail_show_unit_prices": true,\n'
        f'{indent}"usage_detail_show_rate_multiplier": true,\n'
        f'{indent}"usage_detail_show_original_cost": true'
    )

updated, count = pattern.subn(replacement, text)
if count != 2:
    raise SystemExit(f"expected exactly 2 admin settings contract anchors, found {count}")

path.write_text(updated, encoding="utf-8")
print("updated 2 admin settings API contract expectations")
