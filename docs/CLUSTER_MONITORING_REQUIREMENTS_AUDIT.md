# 集群监控与告警需求核对及开发结果

更新日期：2026-10-23。范围：当前工作区源码、迁移、配置、前端与本轮实际测试，包含未提交及新增文件；对照《集群监控告警_最终需求表》32 项需求。

**代码完成 31 项；REL-01 的目标环境完整发布、灰度/回滚和签收尚待完成。** 本次复核没有保留已确认但未实现的软件项。原 CR-01～CR-09 均已修复，见[符合性复核](CLUSTER_MONITORING_CONFORMANCE_REVIEW.md)。

“代码完成”表示约定功能和回归已实现；“本地通过”只覆盖归档的本地场景。两者都不代表客户生产已部署或整项验收标准已签收。按本地证据分类为 **28 项通过、4 项部分通过**（MON-08、MON-10、MON-19、REL-01）；生产条件、规模及参数确认另列。

## 逐项完成状态

下表“代码完成”均指已完成（代码）；REL-01 的待完成工作属于发布验收。所有需求的负责人和客户签收结果仍需项目组登记。

| 编号 / 优先级 | 原需求 | 代码状态 | 本地验证 | 当前结果与主要源码 |
| --- | --- | --- | --- | --- |
| MON-01 / P0 | 单实例 / 集群统一监控 | 代码完成 | 通过 | 自动识别单实例/Kubernetes；保留原 Ops；K8s 专有项 N/A；修复 embed 实例抓取路由。 [ops_monitoring.go](../backend/internal/config/ops_monitoring.go)；[embed_on.go](../backend/internal/web/embed_on.go) |
| MON-02 / P0 | Pod CPU 使用量、Limit 与百分比 | 代码完成 | 通过 | 完整有效 CPU Limit（普通/init/常驻 sidecar/Pod-level/overhead/已生效 resize）；UID、新鲜度、无上限口径。 [ops_monitoring_limits.go](../backend/internal/service/ops_monitoring_limits.go)；[ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go) |
| MON-03 / P0 | Pod 内存使用量、Limit 与百分比 | 代码完成 | 通过 | working set、bytes/MiB、有效内存 Limit、使用率和历史；无上限与未知分开。 [ops_monitoring_limits.go](../backend/internal/service/ops_monitoring_limits.go)；[ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go) |
| MON-04 / P0 | Service QPS / RPM | 代码完成 | 通过 | 逐实例入站/完成 Counter 后按 Service 聚合；nginx/normalized 外部入口适配。 [registry.go](../backend/internal/pkg/httpmetrics/registry.go)；[ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go) |
| MON-05 / P0 | Service P95 / P99 与 TTFT | 代码完成 | 通过 | 跨真实协议成功 TTFT；元数据不计首输出；HTTP 总耗时与 TTFT 的 P95/P99 分列。 [ops_stream_first_output.go](../backend/internal/service/ops_stream_first_output.go)；[ops_ttft_protocol_test.go](../backend/internal/handler/ops_ttft_protocol_test.go) |
| MON-06 / P0 | 4xx / 5xx 数量、错误率 | 代码完成 | 通过 | 应用、应用拒绝、外部代理分别统计 4xx/5xx/429 数量和率；支持排除 429、状态码筛选。 [ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go)；[monitoringFilters.ts](../frontend/src/views/admin/ops/monitoringFilters.ts) |
| MON-07 / P0 | HTTP Counter / Histogram 采集 | 代码完成 | 通过 | 低基数 Counter/Histogram、入站/完成、探针排除、成功 TTFT、实例 token 鉴权。 [registry.go](../backend/internal/pkg/httpmetrics/registry.go)；[metrics.go](../backend/internal/server/routes/metrics.go) |
| MON-08 / P1 | 全部核心指标历史折线图 | 代码完成 | 部分通过 | 历史、区间/粒度、联动游标、放大、阈值、同期对比、断线、明细；30d 部分查询未达本地 2s 参考。 [OpsMonitoringPanel.vue](../frontend/src/views/admin/ops/components/OpsMonitoringPanel.vue)；[benchmark-history.py](../deploy/monitoring/benchmark-history.py) |
| MON-09 / P1 | 集群资源筛选及下钻 | 代码完成 | 通过 | 资源层级、UID、HTTP 维度与区间联动；分享链接使用 UTC ISO，避免跨时区偏移。 [OpsMonitoringPanel.vue](../frontend/src/views/admin/ops/components/OpsMonitoringPanel.vue)；[monitoringFilters.ts](../frontend/src/views/admin/ops/monitoringFilters.ts) |
| MON-10 / P1 | Pod 存活、重启、OOM 历史 | 代码完成 | 部分通过 | Ready/Pending/CrashLoop、重启、OOM 状态与窗口估算、UID Events 归档；本机 cAdvisor OOM Counter 不增待目标环境复验。 [ops_monitoring_events.go](../backend/internal/service/ops_monitoring_events.go)；[ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go) |
| MON-11 / P0 | 保留 PostgreSQL / Redis 监控 | 代码完成 | 通过 | 保留 PG/Redis 原 Ops、健康、应用池和历史；新增容量单列，无云 SDK 依赖。 [ops_cluster_metrics.go](../backend/internal/service/ops_cluster_metrics.go)；[OpsDashboard.vue](../frontend/src/views/admin/ops/OpsDashboard.vue) |
| MON-12 / P1 | 跨 Pod 集中日志及 Request ID | 代码完成 | 通过 | JSON stdout/共享库日志、落库开关、跨实例 request_id；Loki/Fluent Bit Compose/K8s 模板。 [logging](../deploy/logging/)；[central-logs-test.py](../deploy/tests/central-logs-test.py) |
| MON-13 / P1 | Prometheus / Kubernetes 通用监控数据源 | 代码完成 | 通过 | 固定 Prometheus Query API、15s 抓取、认证、超时与降级；K8s 标准只读 Events。 [ops_monitoring_provider.go](../backend/internal/service/ops_monitoring_provider.go)；[prometheus.cluster.yml](../deploy/prometheus.cluster.yml) |
| MON-14 / P1 | 指标聚合、去重及缺失口径 | 代码完成 | 通过 | Counter/Gauge 分路、去重、UID 时间对齐、限额加权；CrashLoop 仅凭新鲜健康证据恢复零。 [ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go)；[monitoring-promql](../deploy/tests/fixtures/monitoring-promql/) |
| MON-15 / P0 | 计费一致性监控与告警 | 代码完成 | 通过 | 失败意图、证据核对队列、人工审计、Usage Log 对账；固定原行投影与后台汇总账本，观测缺口 Unknown。 [ops_billing_operations.go](../backend/internal/repository/ops_billing_operations.go)；[ops_billing_reconciliation.go](../backend/internal/repository/ops_billing_reconciliation.go) |
| MON-16 / P0 | 监控系统自身健康告警 | 代码完成 | 通过 | 心跳/Unknown、独立 Prometheus/Alertmanager/watchdog 栈、断链与持久化、财务观测缺口规则。 [monitoring](../deploy/monitoring/)；[prometheus.monitoring.rules.yml](../deploy/prometheus.monitoring.rules.yml) |
| MON-17 / P1 | HPA 与集群容量监控 | 代码完成 | 通过 | HPA 副本/上限/CPU及内存目标；Deployment 汇总；四类 K8s 对象 Events。 [ops_monitoring_events.go](../backend/internal/service/ops_monitoring_events.go)；[ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go) |
| MON-18 / P1 | PostgreSQL / Redis 连接池与容量告警 | 代码完成 | 通过 | PG client backend 槽；Redis 按节点与热点比例；主/锁/只读三类池、等待量、历史和告警。 [ops_database_capacity.go](../backend/internal/service/ops_database_capacity.go)；[dbpoolstats](../backend/internal/pkg/dbpoolstats/) |
| MON-19 / P1 | 监控历史保留与降采样 | 代码完成 | 部分通过 | 完整 Thanos 栈；raw31d/5m90d/1h365d；真实历史查询、raw 删除、5m/1h 块生成；性能边界见下文。 [thanos-retention.compose.yml](../deploy/monitoring/thanos-retention.compose.yml)；[README.md](../deploy/monitoring/README.md) |
| ALT-01 / P0 | 完整自定义告警规则 | 代码完成 | 通过 | 规则完整 CRUD 与窗口/持续/冷却/资源/通知目标；严格校验可支持维度，历史不支持规则显示 invalid_rule；规则 revision 隔离配置变更，不误当故障恢复。 [ops_alert_validation.go](../backend/internal/service/ops_alert_validation.go)；[ops_alert_regressions_test.go](../backend/internal/service/ops_alert_regressions_test.go) |
| ALT-02 / P0 | P95 / P99 延迟告警 | 代码完成 | 通过 | P95/P99 与 TTFT P95/P99 规则、持续阈值、升级恢复；TTFT 自身有效样本门槛与完整协议采样。 [ops_alert_resource_evaluator.go](../backend/internal/service/ops_alert_resource_evaluator.go)；[ops_ttft_protocol_test.go](../backend/internal/handler/ops_ttft_protocol_test.go) |
| ALT-03 / P0 | 最低请求量门槛 | 代码完成 | 通过 | 入站 qps/rpm/request_count 使用同窗口入站请求数；完成与 TTFT 各用自身样本；低样本不触发、不误恢复并打断持续计时。 [ops_monitoring_alerts.go](../backend/internal/service/ops_monitoring_alerts.go)；[ops_alert_regressions_test.go](../backend/internal/service/ops_alert_regressions_test.go) |
| ALT-04 / P1 | 告警关联去重和聚合 | 代码完成 | 通过 | 规则版本/窗口/资源指纹、事故关联和唯一约束；并发幂等入队、最后一项恢复关闭事故，新一轮故障创建新事故。 [ops_repo_alerts.go](../backend/internal/repository/ops_repo_alerts.go)；[ops_alert_revision_postgres_test.go](../backend/internal/repository/ops_alert_revision_postgres_test.go) |
| ALT-05 / P1 | 告警升级与恢复通知 | 代码完成 | 通过 | 静默期记录状态；到期且故障持续时补首次通知，持久唯一键防重复；升级/恢复状态与通知意图原子写入，CrashLoop 健康零可解除。 [ops_alert_delivery_service.go](../backend/internal/service/ops_alert_delivery_service.go)；[ops_alert_revision_postgres_test.go](../backend/internal/repository/ops_alert_revision_postgres_test.go) |
| ALT-06 / P1 | 运维邮件默认中文 | 代码完成 | 通过 | 默认 zh-CN、显式 en-US、时区；保留自定义模板/收件人偏好及旧定时报表模板。 [ops_alert_delivery_network_test.go](../backend/internal/service/ops_alert_delivery_network_test.go)；[CLUSTER_ALERT_SEMANTICS.md](../docs/CLUSTER_ALERT_SEMANTICS.md) |
| ALT-07 / P1 | 告警评估状态持久化 | 代码完成 | 通过 | rule/resource、last evaluated、持续计数、event/incident/outbox 持久化；重复间隔不累加，规则修订隔离旧状态。缺采样期间不虚构持续故障时间。 [ops_alert_resource_evaluator.go](../backend/internal/service/ops_alert_resource_evaluator.go)；[ops_alert_revision_postgres_test.go](../backend/internal/repository/ops_alert_revision_postgres_test.go) |
| ALT-08 / P1 | Webhook 告警通知 | 代码完成 | 通过 | 独立投递 worker、多 URL/测试发送、HMAC、稳定幂等键、持久重试、HTTPS/DNS 固定与 SSRF 防护。 [ops_alert_delivery_service.go](../backend/internal/service/ops_alert_delivery_service.go)；[ops_alert_delivery_network_test.go](../backend/internal/service/ops_alert_delivery_network_test.go) |
| ALT-09 / P2 | Pod 级资源告警 | 代码完成 | 通过 | Pod 与 Deployment CPU/内存/重启/OOM 的独立指标与告警，完整有效 Limit、UID隔离、资源静默和恢复状态。 [ops_monitoring_alerts.go](../backend/internal/service/ops_monitoring_alerts.go)；[ops_monitoring_promql.go](../backend/internal/service/ops_monitoring_promql.go) |
| ALT-10 / P2 | 4xx / 5xx 独立分类告警 | 代码完成 | 通过 | 4xx/5xx 分类、429 排除、错误数量/率与请求门槛；应用/入口拒绝/外部入口来源分开，支持的协议/路由/状态筛选进入实际查询。 [ops_monitoring_alerts.go](../backend/internal/service/ops_monitoring_alerts.go)；[ops_alert_validation.go](../backend/internal/service/ops_alert_validation.go) |
| ALT-11 / P2 | 邮件失败持久化重试及状态追踪 | 代码完成 | 通过 | 持久投递任务、租约领取、退避、失败终态、人工重试与尝试历史。 [ops_repo_alert_deliveries.go](../backend/internal/repository/ops_repo_alert_deliveries.go)；[ops_alert_delivery_network_test.go](../backend/internal/service/ops_alert_delivery_network_test.go) |
| REL-01 / P0 | 双模式完整核心验收 | 待现场签收 | 部分通过 | 本地脚本和隔离 Linux 运行包已验证；客户目标部署、完整发布/回滚与最终签收尚未执行。 [cluster-monitoring-local-test.sh](../deploy/tests/cluster-monitoring-local-test.sh)；[CLUSTER_MONITORING_KUBERNETES_ACCEPTANCE.md](../deploy/tests/CLUSTER_MONITORING_KUBERNETES_ACCEPTANCE.md) |
| REL-02 / P1 | 监控 RBAC 与数据源安全 | 代码完成 | 通过 | 新 API 继承 AdminAuth；固定 PromQL/标签白名单与查询上限；密钥配置、URL/DNS 防护、只读 Events Role。 [ops_monitoring_routes_test.go](../backend/internal/server/routes/ops_monitoring_routes_test.go)；[metrics.go](../backend/internal/server/routes/metrics.go) |

## 本轮验证证据

| 验证范围 | 实际结果 | 证据 |
| --- | --- | --- |
| 最终后端 / 前端回归 | 迁移 280 后 6 个 Go 包通过：9,866 个顶层 pass、9,520 个子测试 pass、31 个顶层 skip、0 fail；环境条件型跳过保留原结果，真实 PG/Redis 专项另列。前端 435 个文件、3,241 个测试及 Vue/InfiniteCanvas 构建通过；不将不同套件和重复测试相加为唯一用例总数。 | [最终回归摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/final-regression-summary.json) |
| Prometheus 官方引擎 | v0.315.1：156 个固定查询和 5 个 watchdog 表达式解析通过，24 个语义场景通过；包含恢复零、失联、UID 与资源聚合。解析数量不算业务用例总数。 | [实跑日志](../outputs/cluster-monitoring-review-20261023/test-evidence/monitoring-promql-final.log) |
| Docker/Linux 运行与 API | 32 项检查通过：初始化/登录、旧 Ops、模式能力、鉴权、真实 HTTP 流量、数据源失联 Unknown 与恢复。测试镜像来自当前 Go 源码并嵌入完整 Vue + InfiniteCanvas 构建产物；没有宣称完整生产 Dockerfile 发布镜像已验证。 | [运行摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/docker-implementation-smoke-summary.json) |
| 浏览器 | 1440px / 390px、明暗主题无页面横向溢出；键盘筛选可达，未记录页面异常。 | [浏览器记录](../outputs/cluster-monitoring-review-20261023/test-evidence/ui-browser-verification.json) |
| 最终账务与迁移 | 迁移 280 后真实 PostgreSQL 15 个顶层用例通过，0 失败/跳过；含生产迁移链、回滚、重复请求、缺口、投影隔离、物理删除 Unknown、并发核对。 | [最终 PostgreSQL 摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/postgres-projection-isolation-summary.json) |
| 账务独立复核 | 观测表慢写 400ms、三个观测表分别锁住时，原金额事务在 100ms 驱动/服务端期限内提交，金额仅变更一次；原 P1 关闭。 | [独立复核](../outputs/cluster-monitoring-review-20261023/test-evidence/ledger-independent-review.json) |
| TTFT 与容量 | 真实协议/原始流回归；PG client=1、总 activity=9、limit=100；真实 Redis 三主节点热点和失联 Unknown。 | [财务/容量汇总](../outputs/cluster-monitoring-review-20261023/test-evidence/financial-capacity-delivery-summary.json)、[TTFT](../outputs/cluster-monitoring-review-20261023/test-evidence/stream-raw-ttft-regression-summary.json)、[Redis](../outputs/cluster-monitoring-review-20261023/test-evidence/redis-cluster-capacity-real-summary.json) |
| 告警与真实本地通知 | 规则/资源静默到期补首次通知、重启幂等、版本隔离、低样本与缺采样；本地 SMTP 收取中文触发/恢复，TLS Webhook 覆盖 429/500/200/超时、签名及稳定幂等键。未向用户真实收件人发送。 | [告警回归](../backend/internal/service/ops_alert_regressions_test.go)、[真实 PG outbox](../backend/internal/repository/ops_alert_revision_postgres_test.go)、[网络投递回归](../backend/internal/service/ops_alert_delivery_network_test.go)、[具名实际通过记录](../outputs/cluster-monitoring-review-20261023/test-evidence/final-regression-summary.json) |
| 独立监控栈 / 存储 | 实际 Prometheus、Sidecar、对象存储、Store、Query、Compactor、Alertmanager 与 watchdog 链路通过；40d/400d raw 块删除、5d 保留；真实生成 resolution=300000/3600000ms 的 5m/1h 块。 | [可复验部署](../deploy/monitoring/README.md)、[实际验证 JSON](../deploy/monitoring/evidence/local-validation-2026-10-23.json) |
| k3s 独立集群 | 本地 Docker Desktop Linux/arm64、k3d v5.9.0、Kubernetes v1.35.5+k3s1，使用较早阶段镜像：33 断言中 32 通过，唯一未过为实际 OOM 后 cAdvisor OOM Counter 仍为 0；OOMKilled/重启及资源、UID、流量、告警场景通过。 | [k3s 实测](../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/k3s-acceptance.json)、[验收规范](../deploy/tests/CLUSTER_MONITORING_KUBERNETES_ACCEPTANCE.md) |
| kind 独立集群 | kind v0.33.0 / Kubernetes v1.35.8，独立集群和数据库，最终镜像 `06875c9425b1…` 含完整前端与迁移 280；最终代码环境回归以本轮为准：33 断言中 32 通过，唯一未过同为 cAdvisor OOM Counter 不增；实际 OOMKilled/重启、同 UID CrashLoop 恢复、失联保持和告警解除通过。k3s 较早阶段证据不算最终代码全量回归；两种发行版共用本机 Linuxkit 内核，不代表两个客户云环境。 | [kind 实测](../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/kind-acceptance.json)、[验收规范](../deploy/tests/CLUSTER_MONITORING_KUBERNETES_ACCEPTANCE.md) |

## 必须保留的验收边界

- **资源与 OOM（MON-02/03/10/17、ALT-09）**：目标集群须提供标准只读 Pod/Events 和相应 exporter 数据；实际资源 Limit、resize 与标签需要对账。OOM Counter 为抓取窗口估算增量，依赖 cAdvisor 支持，不能作为精确事件审计。当前两个本地发行版共享 Linuxkit 内核，实际 OOMKilled/重启发生而 Counter 不增；不能把它记为 Counter 验收通过，也不能以零值证明没有 OOM。
- **长历史（MON-08/19）**：31 天、57 序列、2,544,480 样本的真实 TSDB 基线，5 次计时；7d 五类查询 p95 为 0.264～0.593s，30d 的错误率/P95/P99 查询 p95 分别为 3.659/3.772/2.949s，未达到本轮自定的 2s 参考。生产并发、序列规模和响应 SLO 尚未约定。raw31d、5m90d、1h365d 可配置；已验证实际降采样产物与 raw 过期删除，未声称验证了 90/365 天长期运行及过期行为。查询 step 不等于存储降采样，降采样也不保证更省磁盘。
- **账务（MON-15）**：迁移 280 使用原账户行固定大小累计金额与 revision，后台只读账户并写汇总账本；这是账户核对，非每笔金额事件的完整审计日志。逐请求幂等/意图证据独立保留，核对任务不自动重放扣费。未知、观测缺口、缺失账户不报健康零；物理创建后又在两次观察之间删除的账户不具备逐事件完整覆盖。观测表锁住或慢写不阻断原交易的结论，不扩展为普通业务失败或已被驱动取消的事务也能提交。迁移 279/280 涉及源表锁和初始化更新，须按真实账户规模安排升级；进程在缺口刷盘前崩溃的窗口需独立指标与日志补充观察。
- **来源与通知（MON-04/06、ALT-05/08/11）**：外部 Ingress/LB 需要实际接入 nginx 或 normalized 适配，不同层请求量不相加。缺失/低样本重置持续证据但不把 firing 误恢复；间隔达到两个完整评估周期也重置。SMTP/通知采用至少一次投递，远端成功而本地确认前退出可能重复；Webhook 接收侧按幂等键去重。公网证书、SMTP 投递及真实接收侧须现场验收。详见[告警语义](CLUSTER_ALERT_SEMANTICS.md)。
- **日志、自监控与权限（MON-12/16、REL-02）**：部署方选定共享库日志策略或 Loki 模板、留存与容量；watchdog 生产接收端应处于独立故障域，本机容器不能覆盖整台宿主机断电。生产 Secret、RBAC、网络策略、对象存储和备份由部署方配置并复核。

## 待完成的交付任务

REL-01 尚需在指定客户环境完成普通 Linux 独立主机、目标云/自建 Kubernetes 的发布验收，覆盖真实业务流量、迁移、灰度、告警通知与回滚，并填写签收结果。两种本地 Kubernetes 发行版及 Linux 容器验证提供可复验依据，不替代客户云环境验收。

交付包中的实现、[本地测试入口](../deploy/tests/cluster-monitoring-local-test.sh)、[Kubernetes 验收规范](../deploy/tests/CLUSTER_MONITORING_KUBERNETES_ACCEPTANCE.md)、[部署说明](../deploy/CLUSTER_MONITORING.md)和已归档结果可直接用于现场复验。上述验收边界不再登记成“功能尚未开发”，也不能从任务清单中省略。
