#!/usr/bin/env bash
# Shared paths for this local, isolated demo; never load deploy/.env.
set -euo pipefail
INVOICE_DEMO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INVOICE_DEMO_ROOT="$(cd "$INVOICE_DEMO_DIR/../.." && pwd)"
command -v python3 >/dev/null || { echo '缺少依赖：python3' >&2; exit 1; }
INVOICE_DEMO_STATE_DIR="$(python3 - "${INVOICE_DEMO_STATE_DIR:-$HOME/.local/state/modurelay-invoice-demo}" <<'PY'
from pathlib import Path
import sys
print(Path(sys.argv[1]).expanduser().resolve())
PY
)"
export INVOICE_DEMO_STATE_DIR

demo_compose() {
  docker compose --project-directory "$INVOICE_DEMO_DIR" \
    --project-name modurelay-invoice-demo --file "$INVOICE_DEMO_DIR/compose.yml" "$@"
}

demo_process() {
  python3 "$INVOICE_DEMO_DIR/process.py" "$1" "$2" \
    "$INVOICE_DEMO_STATE_DIR" "$INVOICE_DEMO_ROOT"
}

# One lifecycle command at a time, including restart's nested stop/start calls.
demo_lifecycle_lock() {
  if [[ "${INVOICE_DEMO_LOCK_HELD:-}" != "$INVOICE_DEMO_STATE_DIR" ]]; then
    exec python3 "$INVOICE_DEMO_DIR/process.py" lock "$INVOICE_DEMO_STATE_DIR" "$0" "$@"
  fi
}

demo_require() {
  local program
  for program in "$@"; do
    command -v "$program" >/dev/null || { echo "缺少依赖：$program" >&2; return 1; }
  done
}

demo_free_port() {
  python3 - "$1" <<'PY'
import socket, sys
with socket.socket() as probe:
    probe.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    try:
        probe.bind(('127.0.0.1', int(sys.argv[1])))
    except OSError:
        raise SystemExit(f'端口 {sys.argv[1]} 已被其他服务占用；请先确认进程，脚本不会自动停止其他服务。')
PY
}
