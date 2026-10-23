# 发票业务本地演示

在仓库根目录执行。需要 Docker Desktop、Go、Node.js、pnpm、Python 3。

```bash
git branch --show-current
bash deploy/invoice-demo/start.sh
```

前端：<http://127.0.0.1:3000/login>，后端：<http://127.0.0.1:8080>。
后端未运行时，启动脚本构建当前分支的 Go 后端并启动 Vite；后端启动时自动核对数据库迁移，首次创建账号和插入演示数据。
它不会运行全量测试或生产前端构建；修改 Vue 代码后由 Vite 自动更新，修改 Go 代码后运行 `restart.sh`。
从旧版发票演示升级时，也应运行 `restart.sh`，让新后端执行 `281_invoice_documents.sql`；已有申请和数据保留。

| 角色 | 登录邮箱 | 初始人民币实付充值 | 用途 |
| --- | --- | ---: | --- |
| 管理员 | `invoice-admin@local.test` | 0 | 导航开关、审核、上传 PDF、财务导出 |
| 正常用户 | `invoice-user@local.test` | 600.00 元 | 申请、详情、撤销、预览和下载已开票 PDF |
| 低充值用户 | `invoice-low@local.test` | 299.99 元 | 验证 300 元门槛 |

三个本地演示账号密码均为 **`InvoiceDemo!2026`**。
管理员首次登录按现有页面阅读并确认管理承诺，脚本不替管理员写确认记录。

## 走一遍业务

1. 用正常用户登录，打开「发票申请」或 `/invoices`，应看到可开票额度 **600 元**。
2. 申请 **300 元**普通发票，填写企业抬头、税号、接收邮箱（如 `finance@example.test`）和联系方式。
   可以选填期望开票日期，实际出票日期以财务处理结果为准。
   在列表里查看详情，确认抬头等资料；此时可用额度为 **300 元**。
3. 可以先撤销一笔待处理申请，确认额度恢复 **600 元**，再重新申请。
4. 用另一个浏览器会话或无痕窗口登录管理员，进入 `/admin/invoices`。
   可在详情里核对用户余额与充值记录，然后将申请标记「处理中」。
   管理员选择 PDF 后点击「上传 PDF」，再填写测试发票号码、标记「已开票」并保存处理结果；上传文件本身不会改变申请状态，缺 PDF 时后端会拒绝完成。
   列表支持跨页勾选多条申请；导出时可选择「选中的申请」或「当前筛选下全部申请」，并选择需要的列。
   导出结果为可用 Excel 打开的 CSV，可核对金额、抬头、税号、联系方式和期望开票日期等资料；单次最多 50,000 条，超出时须缩小筛选范围。
5. 回到用户会话刷新，点击记录查看已开票状态、票号和管理员说明，预览及下载 PDF。
   上传测试文件：[sample-invoice.pdf](sample-invoice.pdf)，仓库路径为 `deploy/invoice-demo/sample-invoice.pdf`，文件明确标有 SAMPLE / NOT A TAX INVOICE。
6. 使用低充值用户登录，应显示人民币有效充值 **299.99 元**，不能提交申请。
7. 管理员关闭「开放发票申请」后，用户刷新页面，导航消失；直接访问 `/invoices`
   仍可查看历史记录，新的申请被禁止。测试后可重新开启。

也可以驳回另一笔申请并填写原因，确认用户详情能看到原因，且申请占用额度被释放。
PDF 单文件最多 10 MiB；管理员登录后可预览，申请本人在「已开票」后可预览和下载。
待处理、处理中的 PDF 可以替换；已开票且已有 PDF 的记录不能替换文件。历史已开票但缺 PDF 的记录可补传一次。

额度示例：累计实付充值 300 元且已开票 300 元，再充值 200 元，剩余可开票额只有 200 元，不能申请 500 元；
由于单次最低 300 元，也不能申请 200 元。再充值 100 元后才可申请新的 300 元。

这里的票号、支付订单和企业信息都是本地测试数据。流程不连接开票机构，也没有配置真实支付渠道或 SMTP。
人民币实付额度与美元钱包余额分别计算；低充值账号还带有外币、未付款和订阅对照订单，它们不会增加人民币可开票额度。

## 插入与查看数据

```bash
# 只补缺失的演示账号/订单。不会重复充值、重置申请、覆盖处理结果或改回管理员设置。
bash deploy/invoice-demo/seed.sh

# 一次查看：迁移、功能开关、账号、充值订单、人民币额度、申请资料和 PDF 元数据。
bash deploy/invoice-demo/db.sh

# 自定义只读查询。
bash deploy/invoice-demo/db.sh query "SELECT application_no, amount_cents, status, invoice_number FROM invoice_applications ORDER BY id;"

# 进入只读 psql，使用 \q 退出。
bash deploy/invoice-demo/db.sh shell
```

数据定义在 [seed.sql](seed.sql)，查询定义在 [inspect.sql](inspect.sql)。
SQL 和脚本仅连接专用的 `invoice_demo` 数据库与角色。

PDF 存在独立的 `invoice_documents` 数据表中，列表只读取文件元数据；这样同一数据库下的多实例
都能访问票据，本地演示也不需要额外配置对象存储。查看元数据可运行：

```bash
bash deploy/invoice-demo/db.sh query "SELECT invoice_id, filename, size_bytes, sha256, uploaded_at FROM invoice_documents;"
```

## 启停与日志

```bash
bash deploy/invoice-demo/status.sh
bash deploy/invoice-demo/restart.sh     # 重新构建后端；保留全部演示数据
bash deploy/invoice-demo/stop.sh        # 停止两个进程与专用容器；保留数据卷
bash deploy/invoice-demo/start.sh       # 继续原流程

tail -f "$HOME/.local/state/modurelay-invoice-demo/backend.log"
tail -f "$HOME/.local/state/modurelay-invoice-demo/frontend.log"
```

配置、PID 和日志默认在 `~/.local/state/modurelay-invoice-demo`，不写入仓库。
需要改目录时，在每次命令前设置同一个 `INVOICE_DEMO_STATE_DIR`。
Docker 项目固定为 `modurelay-invoice-demo`，PostgreSQL 端口 `55433`，Redis 端口 `56380`；
数据库数据存放在该项目的 Docker volume 中。

所有端口只监听本机 `127.0.0.1`。脚本遇到被占用的应用端口会停止并说明，
停止脚本只处理自己记录且进程身份一致的进程，不按端口批量结束进程。

## 这次 main 合并与迁移的核对方式

- 用 Git 比较分支文件，发现发票与监控各有一份 `271_*.sql`。
- 查看 `backend/internal/repository/migrations_runner.go`：`schema_migrations.filename`
  是主键；执行器按完整文件名排序，逐个检查 SHA256，已执行文件校验一致就跳过。
  数字前缀不是迁移的唯一键，因此两份 271 可以分别执行。
- 保留两份迁移的文件名和内容，没有改动已执行迁移、改校验和或人工跳过迁移。
  在新 PostgreSQL 中执行合并后的全部迁移，并跑了 8 项真实数据库发票流程测试。
- Git 合并本身不足一秒；之后的前端全量测试约 103 秒、数据库集成测试约 33 秒，
  还有 Go 编译和前端构建，所以等待主要发生在验证阶段。这份本地启动脚本不重复这些全量检查。
