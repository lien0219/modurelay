# Service Account 最终人工验收 Runbook

本轮完成自动验证；以下 A–P 真实上游/总体验收状态为 **NOT RUN**，留待后续业务
功能总体验收执行。逐项保留状态码、请求/Usage/Event ID、归属和金额证据，不保存
raw Credential、Authorization 或 Webhook Secret。

## 准备

建立隔离 ACME / Production：Alice 为 Owner/Billing Principal，Bob 为 Developer。
另建 Workspace B / Project B 及独立用户。准备明确余额、Project/Workspace Budget，
和已授权、已启用、有价格的 GPT、Claude、Gemini、Seedance、Grok、compatible video
分组。记录调用前的余额、quota、budget。使用真实启用的完整模型 API ID，不能用
models 列表存在代替账户启用证据。

Panel 管理使用人类登录。API 前缀：
`/api/v1/workspaces/{workspace_id}/projects/{project_id}/service-accounts`。
调用 Credential 由安全环境变量提供，不粘贴到证据或终端输出。Webhook 接收端
校验 raw bytes 的 HMAC，按 Event ID 幂等。

## A. Create Service Account

Bob 创建 `backend-api`，确认 active、Project=Production、Workspace=ACME、creator=Bob。
列表/详情一致，更新名称/描述成功。重复 slug 返回 409，另一 Project 可同 slug。
逐角色验证：Owner/Admin/Developer 可管理，Billing/Viewer 可读 SA/Usage，但不能读
Credential 元数据或 mutation。

## B. Create Credential

创建 `backend-prod-01`，记录 ID，安全保存一次性 Secret。关闭并刷新后列表只显示
masked 后四位，任何详情、管理员页面和 Usage 关联对象都没有 Secret/digest。
同 Idempotency-Key 重试返回 409 `SERVICE_ACCOUNT_SECRET_CONSUMED` 和原 ID，Secret
不重放；跨作用域重复幂等键不能取得别的租户数据。

## C. GPT

机器 Credential 分别执行同步、streaming、Responses 续接、适用 WebSocket 和图片
调用。核对 SA、Workspace、Project、Key、付款方及 Reservation；机器 Usage 的
human User 为空，费用扣 Alice。轮换后同 SA Key 可续接 Response，其他 SA/User 拒绝。
验证模型、Group、ACL、quota、到期限制未扩大。适用时验证 Live Sideband/关闭，
Redis 重载后仍保存机器身份；Live 为 0 金额 budget receipt 和 0 费用 telemetry，
没有时长收费。

## D. Claude / Gemini / Composite

Claude 同步/stream、Gemini native/兼容入口、Composite 模型路由重复 C 的归属校验。
核对公开模型和 resolved platform。伪造 body/query/header 的 Workspace B/Project B/
SA B 不能改变身份，归属只来自 authenticated Credential。

## E. Seedance / Grok / compatible video

创建真实视频，记录 task ID。确认 Budget reserve 及 SA/Key/租户/付款方 snapshot。
完成后检查 Usage、钱包和预算 finalize。重复 status/content poll、后台 recovery 只能
结算一次。分别覆盖 Seedance、Grok、AIStarsLab 或已配置 compatible video；无授权
上游的子项明确 NOT RUN。

## F. Bob leaves

Alice 移除 Bob Membership。原 Credential 仍成功，扣当前付款方；Bob 失去所有 SA/
Credential 管理权限，也不继续收到普通 SA 通知。隔离数据中按原 User delete 流程
处理 creator，确认不影响机器归属或凭据。

## G. Disable SA

先暖凭据缓存，再 disable。提交后原 Key 立即拒绝，无 Provider 新调用，不等待 TTL。
UI 显示 Parent disabled，Key 自身状态仍可 active；审计/disabled Event/通知属于
正确租户，actor 为管理人类。

## H. Re-enable

Enable 后仅未 revoked/expired 且满足限制的 Key 恢复。revoked 不恢复，expired
仍拒绝。暖缓存再验；历史 Usage 的 SA ID、次数和金额不改变。

## I. Rotate

A active，Rotate 新建 B，仅 B Secret 显示一次。A 的 Secret/状态/限制/计数保持；
B 继承限制和有效期，计数归零。A/B 都能调用，业务切 B 后 revoke A：A 立即拒绝，
B 成功。rotation Event 含 old/new ID，无 Secret。并发 create/rotate 和禁用/到期
Credential 恢复不能绕过 active 上限。

## J. Billing / Budget / Platform quota

Bob 创建，Alice 付款：Bob 余额不变，Alice 只扣实际费用。Balance 模式消耗 Alice
平台 quota，Subscription 保持原豁免。分别验证 Workspace/Project hard budget 拒绝，
Reservation Actor 是 SA。先 accepted async task，再在允许变更的隔离租户切 payer
到 Carol：旧任务归 Alice，新请求归 Carol。人类 Project Key 并发/RPM/Fast 用户策略
仍匹配原 human actor，不因付款方不同而变化。

## K. Workspace Suspend

平台管理员 suspend，原 Key 和普通 SA/Credential mutation 立即拒绝，授权历史读
保留。平台审计和租户事件/通知存在。恢复 Workspace 仅恢复其他状态仍有效的 Key。

## L. Project Archive

独立 Project archive，Key 立即拒绝；SA/Credential 创建和 rotate 拒绝，授权历史
读取保留。错误 Project path 不能管理原 SA。不要为验收强改 archived 状态或删除
历史义务。

## M. Cross Tenant

Workspace B 用户知道 ACME 的 Project/SA/Credential ID，逐项请求 list/get/update/
disable/enable、Credential list/create/update/rotate/revoke、Usage、Audit。全部隐藏
或拒绝，不能读元数据、Secret 或金额。同 Workspace 错误 Project path 也拒绝。
Global Admin 与 Workspace Admin 分别检查，租户 Admin 不能成为平台 Admin。

## N. Secret Leakage / Events

Key SQL 只保存机器查找摘要；日志、Audit、Domain Event/outbox、通知、Webhook body、
localStorage、持久化 store、URL 中都没有 raw Secret，摘要也不经元数据 API 返回。
生成八种真实事件，核对 human actor 和 SA/Key scope。注入 outbox 写失败时业务
mutation 回滚。验证 Webhook HMAC、retry、Event ID 去重、租户隔离及无递归事件。

## O. FinOps

SA breakdown 和同窗口已结算 Usage 的 Requests、Spend、Tokens、Platforms、Models
一致。filter 同时影响 overview 和详情分页；旧数据为 Human / Legacy。切 Project
清筛选，跨租户 SA filter 拒绝。rename/disable 不改变历史 ID/金额，不靠前端全量
下载聚合。

## P. Async Disable

创建 accepted Seedance/Grok task，立即 disable SA 或 revoke Key。新请求拒绝；
后台仍完成原 SA/Key/payer/Reservation snapshot 结算，钱包、quota、budget 各一次。
覆盖 pending billing recovery、重复完成通知、异步图片和 Batch Image。断开客户端
不丢已接受义务，无法确定 Provider 是否接受时不得当作确定失败释放实际费用。

## 记录与完成标准

每项记录 PASS / FAIL / NOT RUN、证据链接、请求/Usage/Event ID 和失败原因。
所需真实上游和 A–P 生命周期全部验证后才能宣称最终总体验收完成。
自动过期提醒 Producer 为 **DEFERRED**，预留事件名不代表已运行提醒。
设计、安全边界与迁移说明见 [SERVICE_ACCOUNTS.md](SERVICE_ACCOUNTS.md)。
