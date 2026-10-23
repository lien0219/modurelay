#!/usr/bin/env bash
set -euo pipefail
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"
demo_require python3 docker
demo_process status backend || true
demo_process status frontend || true
demo_compose ps
cat <<EOF

发票本地演示：
  登录：http://127.0.0.1:3000/login
  用户：http://127.0.0.1:3000/invoices
  管理员：http://127.0.0.1:3000/admin/invoices
  后端：http://127.0.0.1:8080

  管理员：invoice-admin@local.test
  正常用户：invoice-user@local.test（人民币实付 600.00 元）
  低充值用户：invoice-low@local.test（人民币实付 299.99 元）
  三个演示账号密码均为：InvoiceDemo!2026
  管理员首次登录需在页面阅读并确认现有管理承诺。

  配置、PID、日志：$INVOICE_DEMO_STATE_DIR
  查询数据：bash deploy/invoice-demo/db.sh
  停止环境：bash deploy/invoice-demo/stop.sh
EOF
