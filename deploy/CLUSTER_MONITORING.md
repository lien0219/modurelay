# 集群监控部署与验收

本文对应 `集群监控告警_最终需求表.xlsx`，代码与最新验证见 [需求核对表](../docs/CLUSTER_MONITORING_REQUIREMENTS_AUDIT.md)。代码完成、本地复验和目标部署环境验收分别记录；生产放行以实际部署环境和指定版本的证据为准。

## 数据源与模式

默认 `OPS_MONITORING_MODE=auto`，通过 `KUBERNETES_SERVICE_HOST` 识别 Kubernetes；也可明确设置 `standalone` 或 `kubernetes`。未配置 Prometheus 时原 Ops 数据库/Redis 监控继续工作，新增 HTTP 历史显示 Unknown，单实例的 Pod/HPA 数据显示 NotApplicable。

```dotenv
OPS_METRICS_ENABLED=true
# 从 Secret 注入，不写进仓库
OPS_METRICS_TOKEN=<scrape-token>
OPS_MONITORING_MODE=kubernetes
OPS_MONITORING_CLUSTER_NAME=modurelay
OPS_MONITORING_PROMETHEUS_URL=http://prometheus.monitoring.svc.cluster.local:9090
OPS_MONITORING_BEARER_TOKEN=<optional-query-token>
OPS_MONITORING_TIMEOUT_SECONDS=8
OPS_MONITORING_MAX_QUERY_RANGE_HOURS=744
OPS_ALERT_WEBHOOK_SECRET=<shared-random-secret-at-least-32-bytes>
```

Prometheus URL 仅由部署配置提供；API 不接受自定义 URL/PromQL。URL 可使用受控私网和 localhost，禁止云 metadata/link-local 地址，DNS 解析后固定目标 IP，禁止重定向和继承系统代理。Webhook 仅允许公开 HTTPS 443，另有更严格的内网地址过滤。Query token、HMAC secret 不会返回 UI/API。

- Docker/Linux：[独立抓取配置](prometheus.standalone.yml)。Linux 将目标改为进程地址；容器网络将目标改为实际服务名。
- Kubernetes：[双抓取配置](prometheus.cluster.yml)、[metrics Service](kubernetes/metrics-service.yaml)、[环境变量补丁](kubernetes/monitoring-env-patch.yaml)。逐 Pod Counter 必须通过 endpoint/Pod 发现，不能抓一个负载均衡 Service 代表全部 Pod。
- `cluster` 标签须在应用、kube-state-metrics、cAdvisor、scheduler 指标中保持一致。应用需要 `namespace/service/pod/uid/instance`；`uid` 使用 Pod UID，不可使用 Pod 名代替。
- 每 15 秒抓取。标准 Kubernetes 数据链包括 kubelet/cAdvisor、kube-state-metrics。开启本文后半段的标准 Pods API 资源采样，以完整覆盖 init、sidecar、Pod-level、overhead 和已生效 resize；scheduler 可作为经权威快照校验的优先来源。未开启该能力时的 `regular_container_sum` 只代表常规容器限额合计，不能宣称完整有效 Limit。
- ServiceMonitor/PodMonitor 可等价替换普通 scrape，但必须保留相同 job 和资源标签，且确保 Prometheus HA 副本在 Query 层按 replica label 去重。

## 指标口径

`/metrics` 是共享集群 Gauge（例如数据库记录的完成业务请求、心跳资源聚合），只抓一个逻辑目标；不能跨 Pod 求和，也不能对其 QPS Gauge 使用 rate。

`/metrics/instance` 是实例专属 Counter/Histogram 和进程池统计：

| 指标 | 口径 |
| --- | --- |
| `modurelay_http_requests_started_total` | 进入 HTTP 处理链的请求，尚未完成也计数 |
| `modurelay_http_requests_total` | HTTP 处理完成的请求，包含状态码；和业务结算日志不是同一口径 |
| `modurelay_http_request_duration_seconds_*` | 完整 HTTP/SSE 请求耗时 Histogram |
| `modurelay_http_ttft_seconds_*` | 有有效首输出、状态 2xx/3xx 的首 Token 耗时；失败无输出不补零 |
| `modurelay_db_pool_wait_total` / `modurelay_db_pool_wait_duration_seconds_total` | 当前进程 PostgreSQL 应用池累计等待次数/秒数 |
| `modurelay_redis_pool_timeout_total` | 当前进程 Redis 连接池累计超时 |
| `modurelay_database_pool_*{pool}` | primary / advisory_lock / read_replica 三类进程池的连接数、预算及等待；固定标签，不暴露地址 |
| `modurelay_billing_observation_*` | 本实例财务监控漏记总数、尚未持久化数量、最近失败时间；用于独立缺口告警 |
| `modurelay_ops_job_last_success_timestamp_seconds`（位于 `/metrics`） | 监控采集/告警评估成功时间，用于外部 watchdog |

HTTP 标签仅使用固定 method/source/status/route/protocol 枚举；无用户 ID、请求 ID、原始 URL、模型名或密钥。Service/Pod 来自 scrape 标签。内部探针与 metrics 自身排除。Counter 先逐实例 rate/increase，再聚合；Histogram 聚合 bucket 后计算分位数，不平均 Pod P99。

`source=app`、`source=ingress`、`source=external_ingress` 分开查询。这里 ingress 表示**应用入口鉴权/限流拒绝**；外部代理/LB 通过 nginx 或 normalized adapter 接入，配置和维度见后文。这些源有重叠，不能相加为互不重复请求。

CPU/内存显示用量、Limit 和百分比；不以宿主机容量冒充 Pod Limit。Limit 缺失或多容器不完整时百分比 Unknown；单实例 Pod 字段 NotApplicable。Pod UID 区分同名重建，rate 跨新 Pod 创建时间的窗口不归到新 UID。限额加权 CPU 使用相同的有用量及限额 UID 集合，并在标签中标明该范围。

## 事件、日志与容量

开启 `OPS_MONITORING_KUBERNETES_EVENTS_ENABLED=true` 后，使用 Pod ServiceAccount token、CA 和标准 Kubernetes API，只读采集 Events，写入共享 PostgreSQL；按 event UID 去重并记录 involved object UID，保留 Pod 删除后的历史。默认只读当前 namespace。`OPS_MONITORING_EVENTS_NAMESPACE=*` 需要显式跨 namespace 授权，默认不授予。样例 [Role/RoleBinding](kubernetes/monitoring-events-rbac.yaml) 只允许当前 namespace 的 events get/list。默认保留 30 天，可用 `OPS_MONITORING_EVENTS_RETENTION_DAYS` 设置。

`pod_oom` 是最近终止原因为 OOMKilled 的容器数量状态，不是选定窗口内的 OOM 次数；Pod Events 用于历史溯源。HPA 目标利用率及 HPA/Deployment 对象扩缩容事件尚未接入归档。

原 Ops 系统日志通过共享数据库跨 Pod 查询，保留 host/request_id；HTTP access log 的默认落库策略未改变。JSON stdout 可接 Loki/ELK/OpenSearch 等标准采集器，部署补丁注入 namespace/pod/service。完整访问日志持久保留、Pod 删除后查询能力依赖所选集中日志后端或显式开启 Ops access log 落库，未配置时不可宣称已具备。

PostgreSQL/Redis **实例容量**与**每个应用进程连接池**分开：PostgreSQL 使用标准 `pg_stat_activity` 的 `client backend` 与 `max_connections`；Redis 使用 `INFO clients`。权限不允许时显示 Unknown。Redis Cluster 逐主节点执行标准查询，保留节点明细及最繁忙节点比例；节点失联显示 Unknown，不将各节点上限相加伪装成单实例容量。主连接、锁连接和只读副本应用池分别登记，等待 Counter 通过实例 scrape 汇总，不依赖云接口。

## 告警和投递

后台仍使用管理员权限。规则可按固定指标及资源标签配置窗口、持续时长、冷却、最小请求量、错误数门槛、429 排除、升级阈值和静默时间；不支持的标签组合在服务端拒绝。数据不足/失败不触发也不恢复，持续超阈值计数中断，已有事件继续保留。

事件/资源状态和通知 outbox 存 PostgreSQL。通知意图与事件转换在同一事务写入；租约领取、失败退避、发送结果和人工重试入口均持久化。Webhook body 中 `transition` 为 firing/upgrade/resolved，签名规则：

```text
X-ModuRelay-Timestamp: Unix 秒
X-ModuRelay-Signature: sha256=hex(HMAC-SHA256(secret, timestamp + "." + 原始body))
Idempotency-Key: ops-alert-<delivery-id>
```

接收方验证时间窗口和 HMAC，并按 Idempotency-Key 去重。远端成功后本实例在确认落库前退出时会重投，这是至少一次投递；SMTP 本身没有事务性幂等确认，极端崩溃窗口可能产生重复邮件，不能承诺 exactly-once。失败累计到上限后可在事件投递详情人工重试。不要把 secret 放在 Webhook URL query 中。

导入 [独立 Prometheus 告警规则](prometheus.monitoring.rules.yml)，并为 Prometheus/Alertmanager 配置外部接收端。它监测 scrape 连续 2 分钟失败、目标消失、collector/evaluator 停止。`ModuRelayMonitoringWatchdog` 必须接外部心跳服务才能发现整个 Prometheus 退出；仅在应用内查询 Prometheus 无法覆盖通知系统自身全部故障。

## 计费一致性

新结算在原有幂等 INSERT 内一并保存预期 Usage Log 金额和余额/订阅扣费金额，随原事务原子提交/回滚。不增加监控回调或改动金额计算。旧历史及图片冻结/释放没有可验证证据时保持 NULL，不回填虚构金额。

独立 collector 从初始 24h 回溯开始，以持久 checkpoint 每批最多 10000 条推进；最近 2 分钟暂缓，未恢复问题持续复查。未追平、数据库失败或快照超过 3 分钟都返回 Unknown。每条异常含 settlement_id/request_id、精确金额与首次/最近发现/恢复时间，不含密钥。指标统计全部未恢复问题，不因时间窗口移动自动“恢复”。

存在未恢复异常的结算证据暂不归档，确保后续 Usage Log 补齐仍可复核；确认恢复后，下一轮正常清理可将证据归档。页面明确区分未恢复异常与已恢复历史。

迁移 278 增加提交前独立意图、结算失败记录和持久证据核对 outbox；统一扣费及批图 reserve/capture/release 均接入。已进入统一扣费的异步媒体同样覆盖；尚未进入计费阶段的媒体轮询业务队列不冒充财务补偿队列。核对可将缺少结算证据的项转入 `manual_required`，人工处理后必须填写说明重新核对；该动作不会再次扣款。

迁移 279 建立账户基线；必须一并应用迁移 **280**。最终实现只在账户已持有的原行内维护固定大小的预期余额/用量与 revision，业务事务不写账本观测表。后台读取原行后独立生成版本区间汇总流水并核对实际金额，观测表锁等待或慢写不会阻断扣费。窗口归零、充值、冻结/扣费/释放均覆盖；物理删除、历史覆盖缺口和采样失败保持 Unknown，不伪造完整证据。这是账户汇总核对，逐请求结算证据仍由 dedup/intent 保留，不宣称完整逐笔账务审计。部署前应评估 279/280 的基线迁移与账户表锁窗口。

意图与结果观察每次默认最多 50ms（`OPS_BILLING_OBSERVATION_TIMEOUT_MS`，上限 200ms），一次正常结算最多增加两次预算；失败后熔断 30s。观察失败不让扣费失败，也不报告健康零值：每实例心跳持久化漏记累计数，UI 展示未确认缺口及审计说明。确认只覆盖已看到的累计值，后续新缺口再次使状态 Unknown。硬退出前尚未 flush 的内存缺口由实例 Counter、结构化日志和独立 watchdog 补充观察；不承诺在所有基础设施同时故障时仍获得完整审计。

## 历史留存

Query API 默认限制 31 天、每条 11000 点、总 100000 点、最多 200 条序列、8 个并发请求，按区间选择 15s/60s/5m/15m 查询粒度。查询粒度只影响返回点数，不代表 TSDB 已做存储降采样。

纯 Prometheus 明确设置 `--storage.tsdb.retention.time=31d` 与按容量规划的 `--storage.tsdb.retention.size`；超期清理由 Prometheus 负责。分层存储使用 [完整可选栈](monitoring/thanos-retention.compose.yml)，包含 Prometheus、Sidecar、Store、Query、Compactor、对象存储及独立告警接收。raw 31d、5m 90d、1h 365d 可调整，云对象存储有单独 override；初始化、验证及部署参数见 [监控栈说明](monitoring/README.md)。单 bucket 只运行一个 compactor。

Thanos 在数据足够老后才生成 5m/1h 层；保留期不能短到在下一级降采样前删掉源数据。Query 层设置 replica label 和自动分辨率选择；raw 保留至少 31d，以保留本需求 7d/30d 内短窗口 Histogram/Counter 查询的精度。降采样的目的主要是长期查询，不能当成磁盘必然变小的承诺。来源：[Thanos Compactor](https://thanos.io/tip/components/compact.md/)、[Thanos Query](https://thanos.io/tip/components/query.md/)。

本地真实 TSDB 已导入 31 天、57 条序列、2,544,480 个样本并测量 7d/30d 固定查询；结果保存于交付证据目录。该基线不能代替目标集群规模及并发 SLO。生产 SLO、指标规模、对象存储容量和留存期限由部署方确认；不把查询 step 称为存储降采样。

## 可复现本地验证

```sh
bash deploy/tests/cluster-monitoring-local-test.sh
# 官方 Prometheus parser/engine 合约校验；首次下载独立测试模块依赖
bash deploy/tests/monitoring-promql-contract-test.sh
cd frontend
pnpm test:run
pnpm build
```

本机可不用 Docker 运行专用 PostgreSQL SQL 验证；只连接自己新建的临时实例：

```sh
MONITORING_PG_DIR=$(mktemp -d /tmp/modurelay-monitoring-pg.XXXXXX)
initdb -D "$MONITORING_PG_DIR/data" -A trust --no-locale -E UTF8
pg_ctl -D "$MONITORING_PG_DIR/data" -l "$MONITORING_PG_DIR/server.log" \
  -o "-k $MONITORING_PG_DIR -h 127.0.0.1 -p 55473" start
cd backend
MODURELAY_MONITORING_TEST_DSN="host=$MONITORING_PG_DIR port=55473 dbname=postgres sslmode=disable" \
  go test -tags=unit ./internal/repository -run 'TestMonitoring.*Postgres' -count=1
pg_ctl -D "$MONITORING_PG_DIR/data" stop
```

这些测试各自创建并清理专用 schema；完整迁移测试还会创建并删除随机命名的独立数据库，需要临时 PostgreSQL 用户有 CREATEDB 权限，并验证首次迁移及重启幂等。测试没有配置 DSN 时会明确跳过；跳过不能计作数据库验收通过。

## 环境放行与复验范围

| 环境 | 必须提供的证据 | 当前结论 |
| --- | --- | --- |
| Docker Compose 单实例 | 实际 scrape、原 Ops/业务回归、故障恢复、账务对账 | 已有隔离 Docker/Linux 实测，最新结果见交付证据 |
| Linux 独立主机 | 进程采集、无 K8s 降级、PG/Redis、历史与告警 | Linux arm64 容器进程已测；客户主机服务管理及发布回滚需现场验收 |
| 托管 Kubernetes | 1 Pod/≥3 Pod、Limit 变更、流量/分位数对账、OOM/HPA/事件、切主、网络故障 | 没有客户托管集群凭据，保留现场验收项 |
| 自建 Kubernetes | 同上，独立配置与证据 | 隔离 k3s 与 kind 已实测 Limit、三副本流量、UID、事件及故障/恢复；本机 cAdvisor OOM Counter 不增，不能计为该项通过。结果见专用脚本及需求核对表，不替代客户托管集群验收 |
| 通知接收端 | SMTP 失败恢复、Webhook 200/429/500/超时及签名、持久重试 | 本地真实协议接收与真实 PostgreSQL 状态测试分别记录；客户公网目标仍需投递验收 |
| TSDB | 7d/30d数据、retention/compaction、SLO负载报告 | 本地栈及真实历史查询已有证据；生产规模及并发 SLO 待现场确认 |

不得自动对生产环境进行 OOM、SIGKILL、扩缩容、账务故障注入或切流。先在专用验收环境执行，再保存时间、版本、资源 UID、PromQL 结果、日志和截图；14 项 P0 全部具备证据后再放行。

## 2026-10-23 补齐后的监控契约

### 完整生效 Limit 采样

先部署迁移 277，再为应用 ServiceAccount 授予当前 namespace 的 `pods get/list`（见现有 monitoring-events-rbac.yaml），显式设置 `OPS_MONITORING_KUBERNETES_RESOURCES_ENABLED=true`。该开关与 Events 独立。现有 leader collector 在后台分页读取标准 Pods API，将当前 UID 的有效 CPU/内存限额写入共享数据库快照；`/metrics` 只读快照，不发 Kubernetes API 请求。失败心跳、超过两分钟的快照、无权限、无法解释的 in-place resize 都不会伪造成可用限额。

计算覆盖常规容器合计、按顺序执行的 init 最大值、`restartPolicy: Always` sidecar 累加、Pod-level resources 覆盖和 overhead。存在已生效 status.resources 时使用它；resize 未提供已生效资源时该资源为 Unknown。scheduler 数值与权威快照一致时优先使用 scheduler；否则使用 Kubernetes API 计算值，`limit_source` 明确标示。计算依据为 [Kubernetes resource helpers](https://github.com/kubernetes/component-helpers/blob/master/resource/helpers.go)。未开启此采样时保留旧版 scheduler / regular-container fallback，其能力边界仍需在交付中标明。

采样明确确认未设上限时，Limit/百分比对应点返回 `state=NotApplicable,value=null`，标签 `limit_state=unlimited`；未采集到、过期或解析不确定返回 Unknown。用量仍独立可见。Prometheus 保存这些快照的历史，因此 Limit 变更前后不会使用最新 Limit 重算旧用量。

### 外部入口适配及维度

已有 `source=ingress` 保持应用入口拒绝语义，新增 `source=external_ingress` 与它及 `app` 分离，禁止直接相加当作互斥流量。配置 `OPS_MONITORING_EXTERNAL_INGRESS_ADAPTER=nginx` 和 `OPS_MONITORING_EXTERNAL_INGRESS_JOB=<实际scrape job>` 后，固定查询读取 `nginx_ingress_controller_requests` 与 `nginx_ingress_controller_request_duration_seconds`，将标准 `status` 归一化为 HTTP 统计语义。入口 scrape 需配置一致的 `cluster` 标签，并保留标准 `namespace/service/instance` 标签。参考 [ingress-nginx 官方指标](https://kubernetes.github.io/ingress-nginx/user-guide/monitoring/)。

其他 LB/代理可选择 `normalized`，由其 exporter 或 recording rules 输出以下固定契约：

- `modurelay_external_http_requests_total{cluster,namespace,service,instance,method,status_code,route,protocol}`：每实例请求 Counter。
- `modurelay_external_http_request_duration_seconds_bucket/_count/_sum`：完整请求耗时 Histogram；bucket 保留 `le`。
- `up{cluster,job,instance}`：与该 exporter 匹配的真实 scrape 健康。

不能从没有相应原始维度的 LB 指标臆造 route/protocol。官方 Query API 新增 `protocol=http|sse|websocket`、低基数 `route`（含 gemini）及精确 `status_code=100..599`；Histogram 不支持状态码筛选，入站计数不支持尚未确定的响应协议/状态码。nginx adapter 没有应用 route/protocol、后端 Pod UID 或 Deployment 维度，相关过滤明确拒绝；normalized adapter 可使用已按上述契约真实提供的 route/protocol。

### OOM、资源事件与扩容

`pod_oom` / `deployment_oom` 保留“最后一次终止原因为 OOM 的容器数”状态。新增 `pod_oom_count` / `deployment_oom_count` 使用 cAdvisor `container_oom_events_total` 的窗口 `increase`；它表示 kernel OOM Counter 的采样增量，边界外推可能为小数，不能宣传为逐事件精确审计次数。没有 exporter Counter 就返回 Unknown，不用 LastTermination 状态冒充次数。

Events 现在归档 Pod、Deployment、ReplicaSet 和 HorizontalPodAutoscaler，新增 `object_kind/object_name/object_uid`，兼容旧 `pod/pod_uid` 查询。新 API 可按对象类型、名称、UID 筛选，Pod 同名重建仍按 UID 隔离。新增 HPA CPU/内存目标利用率查询使用 kube-state-metrics `kube_horizontalpodautoscaler_spec_target_metric`（实验性标准指标）；采集器未启用时返回 Unknown。参考 [KSM HPA 指标](https://github.com/kubernetes/kube-state-metrics/blob/main/docs/metrics/workload/horizontalpodautoscaler-metrics.md)。
