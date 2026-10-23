#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
demo_require python3 docker
demo_lifecycle_lock "$@"
if [[ $# -gt 1 || ( $# -eq 1 && "$1" != '--apps-only' ) ]]; then
  echo '用法：stop.sh [--apps-only]' >&2; exit 1
fi
demo_process stop frontend
demo_process stop backend
if [[ "${1:-}" != '--apps-only' ]]; then demo_compose stop; fi
echo '已停止演示进程；数据库卷和申请记录保留。'
