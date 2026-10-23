#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
demo_require python3
demo_lifecycle_lock "$@"
bash "$INVOICE_DEMO_DIR/stop.sh" --apps-only
bash "$INVOICE_DEMO_DIR/start.sh"
