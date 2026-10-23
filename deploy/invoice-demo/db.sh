#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
demo_require docker
case "${1:-inspect}" in
  inspect)
    demo_compose exec -T postgres psql -X -U invoice_demo -d invoice_demo \
      -v ON_ERROR_STOP=1 < "$INVOICE_DEMO_DIR/inspect.sql"
    ;;
  query)
    [[ $# -eq 2 ]] || { echo '用法：db.sh query "SELECT ..."' >&2; exit 1; }
    demo_compose exec -T -e 'PGOPTIONS=-c default_transaction_read_only=on' postgres \
      psql -X -U invoice_demo -d invoice_demo -v ON_ERROR_STOP=1 -c "$2"
    ;;
  shell)
    demo_compose exec -e 'PGOPTIONS=-c default_transaction_read_only=on' postgres \
      psql -X -U invoice_demo -d invoice_demo -v ON_ERROR_STOP=1
    ;;
  *) echo '用法：db.sh [inspect | query "SELECT ..." | shell]' >&2; exit 1 ;;
esac
