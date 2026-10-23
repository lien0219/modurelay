# 集群监控与告警需求符合性复核

更新日期：2026-10-23。复核范围为当前工作区（含未提交、新增文件）与 32 项原始需求；总体完成状态见[需求审计](CLUSTER_MONITORING_REQUIREMENTS_AUDIT.md)。

**原 CR-01～CR-07 以及补充发现的 CR-08、CR-09 均已修复并完成对应复验。** 下表保留原触发条件、最终处理和回归依据，供交付追溯；它们不再是待开发任务。独立收尾复核在账务观测、资源 Limit、CrashLoop 恢复及新增管理员 API 范围内，未发现新的可复现严重问题。该结论不替代客户生产验收。

## 缺陷关闭清单

| 编号 / 对应需求 | 原问题及影响 | 当前状态与修复 | 复验依据 |
| --- | --- | --- | --- |
| CR-01 / MON-05、ALT-02 | Anthropic/Gemini 等真实协议 Handler 未把成功首输出写入统一 TTFT 上下文，导致漏样本与慢请求漏报。 | **已修复**。统一成功首输出传递；原始流区分元数据、正文、工具和推理首输出，保持原 Responses 配置语义。 | [统一上下文](../backend/internal/service/ops_upstream_context.go)、[首输出判定](../backend/internal/service/ops_stream_first_output.go)；[实际协议 Handler 回归](../backend/internal/handler/ops_ttft_protocol_test.go) `TestMonitoringTTFTActualProtocolHandlers`；[原始流实跑摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/stream-raw-ttft-regression-summary.json)。 |
| CR-02 / ALT-05、ALT-09 | 静默期间首次 firing 仅保存事件；到期后持续故障不补首次通知。 | **已修复**。有效故障持续时补建首次通知；规则/资源静默、重启后重复评估均受持久 outbox 唯一键保护。 | [补发实现](../backend/internal/service/ops_alert_delivery_service.go) `ensureFiringNotification`；[评估器](../backend/internal/service/ops_alert_resource_evaluator.go)；[服务回归](../backend/internal/service/ops_alert_regressions_test.go) `TestSilenceExpiryEnqueuesFirstFiringOnceAcrossEvaluatorRestart`；[真实 PG 回归](../backend/internal/repository/ops_alert_revision_postgres_test.go) `TestMonitoringSilenceExpiryOutboxSupplementIsDurablePostgres`。 |
| CR-03 / MON-14、ALT-05 | CrashLoop 恢复后原因序列消失，空向量被当成缺失，告警无法解除。 | **已修复**。使用新鲜、同 UID 的身份、抓取与容器健康证据形成恢复零；抓取失败、init 状态未知和替换 UID 不补零。 | [查询实现](../backend/internal/service/ops_monitoring_promql.go) `pod_crash_loop` 与健康证据构造；[官方 Prometheus 引擎语义回归](../deploy/tests/fixtures/monitoring-promql/semantics_test.go)覆盖 Running 恢复、init/sidecar、采集失败、UID 替换；[执行入口](../deploy/tests/monitoring-promql-contract-test.sh)。 |
| CR-04 / ALT-03 | 入站 QPS/RPM 忽略最低请求量，低样本也推进或恢复告警。 | **已修复**。`qps_started/rpm_started/request_count_started` 使用相同过滤与窗口的入站计数；完成请求和 TTFT 分别使用对应样本。 | [计数查询](../backend/internal/service/ops_monitoring_alerts.go)；[回归](../backend/internal/service/ops_alert_regressions_test.go) `TestStartedMetricsRespectMinimumRequests`、`TestStartedAlertSamplesUseArrivalCounterAndMatchingWindow`，覆盖低样本不触发/不恢复。 |
| CR-05 / ALT-01、ALT-10 | 接受 `platform/group_id/region` 等未参与查询的业务条件，实际评估被扩大为全局。 | **已修复**。新增/修改明确拒绝不支持维度；历史规则保持原配置并显示 `invalid_rule`，停止查询及触发/恢复。可支持的资源、protocol/route/status_code 进入实际查询。 | [规则校验](../backend/internal/service/ops_alert_validation.go)、[历史规则评估保护](../backend/internal/service/ops_alert_resource_evaluator.go)；[回归](../backend/internal/service/ops_alert_regressions_test.go) `TestMonitoringBusinessDimensionsRejectedForNewAndStoredRules`。 |
| CR-06 / MON-09 | `datetime-local` 原字符串写入分享链接，接收者时区改变查询窗口。 | **已修复**。链接和 API 均保存 UTC ISO；输入控件才转换到本地显示。 | [前端实现](../frontend/src/views/admin/ops/components/OpsMonitoringPanel.vue) `syncQuery/queryBounds`；[前端回归](../frontend/src/views/admin/ops/components/__tests__/OpsMonitoringPanel.spec.ts)覆盖 UTC 链接加载与编辑后的绝对时间分享。 |
| CR-07 / MON-18 | PG `pg_stat_activity` 全量计数含后台进程，导致连接槽占用偏高。 | **已修复**。明确普通 client backend / max_connections 口径；后台进程不计入分子，保留预留客户端槽的上限含义。 | [查询实现](../backend/internal/service/ops_database_capacity.go) `postgresCapacitySQL`；[真实 PG 测试](../backend/internal/service/ops_database_capacity_test.go) `TestMonitoringDatabaseCapacityConnectionSlotsPostgres`；[实测摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/financial-capacity-delivery-summary.json) client=1、activity=9、limit=100。 |
| CR-08 / MON-01、MON-07 | 启用正式前端 embed 时 SPA 路由处理抢占 `/metrics/instance`，实例抓取返回页面而非指标。 | **已修复**。两条嵌入式前端处理路径均放行实例指标端点，继续由原指标路由执行 token 鉴权。 | [embed 实现](../backend/internal/web/embed_on.go) `shouldBypassEmbeddedFrontend`；[两个 Middleware/ServeEmbeddedFrontend 回归](../backend/internal/web/embed_test.go)；[实际 Linux embed/API 运行摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/docker-implementation-smoke-summary.json)。 |
| CR-09 / MON-15 | 原同步 ledger 触发器访问观测表；观测表锁住/慢写可耗尽请求期限并回滚原金额事务。 | **已修复**。迁移 280 改为原账户行固定大小 expected/revision 运算，不做观测表 I/O；后台仅读取账户投影并写汇总账本，不持有或更新账户行。 | [迁移 280](../backend/migrations/280_ops_account_projection_isolation.sql)、[后台核对](../backend/internal/repository/ops_billing_account_projection.go)；[真实 PG 隔离回归](../backend/internal/repository/ops_billing_operations_postgres_test.go) `TestMonitoringBillingProjectionIgnoresSlowAndLockedObserverPostgres`；[独立复验](../outputs/cluster-monitoring-review-20261023/test-evidence/ledger-independent-review.json)。 |

## 关闭结论的证据边界

[最终回归摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/final-regression-summary.json)记录迁移 280 后六个 Go 包结果：9,866 顶层通过、9,520 子测试通过、31 顶层条件型跳过、0 失败；前端 435 文件 / 3,241 测试及 Vue/InfiniteCanvas 构建通过。真实 PostgreSQL 专项单列，不把主套件的 skip 改成 pass。[官方 Prometheus 实跑日志](../outputs/cluster-monitoring-review-20261023/test-evidence/monitoring-promql-final.log)记录 156 固定查询、5 watchdog 表达式解析及 24 语义场景通过。最终摘要也包含两个实际本地 SMTP/TLS 用例的具名通过记录。不同套件、子测试和重复执行不合并为唯一通过总数。

CR-09 的原 P1 已关闭：独立复验分别锁住 `ops_account_ledger_entries`、`ops_account_ledger_heads`、`ops_billing_recording_gaps`，以及让观测 INSERT 延迟 400ms，原用户/订阅金额变更在 100ms 驱动与服务端期限内仍可提交，且每次只修改一次。修复依赖消除同步观测表访问，未用捕获 `query_canceled` 掩盖业务回滚。它保证观测表不可用不阻断原交易，不保证已被驱动主动取消或本身失败的业务事务仍能提交。

账本现为固定源行投影与后台 revision 区间汇总，**不是每笔金额事件的完整审计日志**。逐请求幂等与意图证据独立存在；核对/人工处理不执行存储的扣费命令。物理删除或观测缺口保持 Unknown；迁移 279/280 的源表锁与初始化工作仍需目标规模升级验收。[最终真实 PostgreSQL 摘要](../outputs/cluster-monitoring-review-20261023/test-evidence/postgres-projection-isolation-summary.json)包含 15 个通过用例，覆盖迁移链、观测故障、并发核对和删除后 Unknown。

告警修复还包含规则 revision/窗口指纹隔离、旧评估器写入保护与缺采样持续计时重置；不能将停机或未知区间当成持续超阈，也不能将 Unknown 当作健康恢复。详细契约见[告警语义](CLUSTER_ALERT_SEMANTICS.md)。真实本地 SMTP 和 TLS Webhook 用例覆盖触发/恢复、失败后重试、状态码/超时与签名；不构成客户公网网络或真实邮箱签收，SMTP 保持至少一次投递语义。

资源 Limit、CrashLoop 和新 API 的收尾复核关注了 UID/新鲜度、缺失误零、忽略过滤和管理员鉴权。[资源计算回归](../backend/internal/service/ops_monitoring_limits_test.go)、[官方引擎回归](../deploy/tests/fixtures/monitoring-promql/semantics_test.go)、[新增 API 鉴权回归](../backend/internal/server/routes/ops_monitoring_routes_test.go)覆盖这些路径。没有由本次复核确认但尚未修复的严重问题，不表示覆盖所有未知缺陷。

## 尚未签收的范围

缺陷关闭与 31 项代码完成不等于 32 项需求的生产 DoD 已全部通过。MON-08/19 的部分 30d 原始查询未达本地自定 2s 参考；MON-10 的本机 cAdvisor OOM Counter 不增，实际 OOMKilled/重启有证据但 Counter 未通过；REL-01 的完整目标环境发布、灰度与回滚仍待签收。[k3s](../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/k3s-acceptance.json) 与 [kind](../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/kind-acceptance.json) 均完成 33 项中 32 项检查，唯一未过均为 OOM Counter；实际 CrashLoop 恢复、失联保持和解除告警通过。k3s 为较早阶段镜像，最终代码环境复验以 kind 的 `06875c9425b1…` 完整前端/迁移 280 镜像为准，不将早期 k3s 记录标为最终全量回归。具体结果及部署边界统一以[需求审计](CLUSTER_MONITORING_REQUIREMENTS_AUDIT.md)为准。

本轮使用当前源码、完整 Vue + InfiniteCanvas 前端嵌入的 Linux 测试镜像；并未据此宣称完整生产 Dockerfile 发布镜像或客户云环境已经验收。
