#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
demo_require docker
demo_compose exec -T postgres psql -X -U invoice_demo -d invoice_demo \
  -v ON_ERROR_STOP=1 < "$INVOICE_DEMO_DIR/seed.sql"
