#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
demo_require docker go pnpm node python3
demo_lifecycle_lock "$@"
docker info >/dev/null
mkdir -p "$INVOICE_DEMO_STATE_DIR/bin" "$INVOICE_DEMO_STATE_DIR/data"
chmod 700 "$INVOICE_DEMO_STATE_DIR" "$INVOICE_DEMO_STATE_DIR/data"

if ! demo_process status backend >/dev/null; then demo_free_port 8080; fi
if ! demo_process status frontend >/dev/null; then demo_free_port 3000; fi
demo_compose up -d --wait --wait-timeout 90

if ! demo_process status backend >/dev/null; then
  echo '构建本分支后端（使用 Go 缓存；首次可能下载工具链/依赖）…'
  (cd "$INVOICE_DEMO_ROOT/backend" && go build -o "$INVOICE_DEMO_STATE_DIR/bin/invoice-demo-server" ./cmd/server)
  demo_process start backend
else
  echo '后端已在运行；修改 Go 代码后可运行 restart.sh。'
fi
demo_process wait backend
bash "$INVOICE_DEMO_DIR/seed.sh"

if ! demo_process status frontend >/dev/null; then
  (cd "$INVOICE_DEMO_ROOT/frontend" && pnpm install --frozen-lockfile)
  demo_process start frontend
fi
demo_process wait frontend
bash "$INVOICE_DEMO_DIR/status.sh"
