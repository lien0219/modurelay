# 可选独立监控、长期存储与心跳接收栈

此目录提供 Prometheus、Thanos Sidecar / Query / Store / Compactor、Alertmanager、持久化 Watchdog 接收端，以及隔离的本地 S3 兼容存储。默认应用抓取目标为空，不会发送真实邮件或外部 Webhook。镜像使用固定版本；本地 MinIO 额外固定镜像摘要。业务数据库不参与历史指标查询。

## 启动与接入

在仓库根目录运行：

```sh
python3 deploy/monitoring/bootstrap-monitoring.py
docker compose -f deploy/monitoring/thanos-retention.compose.yml --profile local-object-store up -d --build
```

`deploy/monitoring/secrets/` 已被 Git 忽略。初始化程序不覆盖已有文件、不输出凭据：父目录权限为 `0700`，文件为 `0644`，使 Compose 绑定挂载后的非 root 进程可读各自被授权的文件。应限制宿主机和 Docker 管理权限，不将该目录放到共享位置。正式部署可由秘密管理系统生成对应文件。

应用设置 `OPS_METRICS_ENABLED=true`、`OPS_METRICS_TOKEN`；把相同 token 写入 `secrets/app-metrics-token`，并配置两组抓取目标：

- 按 `application-targets.example.json` 填写 `secrets/application-targets.json`，列出每个实例的独立地址和稳定标签，用于抓取 `/metrics/instance` 的 Counter/Histogram。
- 按 `cluster-targets.example.json` 填写 `secrets/cluster-targets.json`，每个逻辑集群只配置一个地址（Kubernetes 可使用 Service 地址），用于抓取 `/metrics` 的共享 Gauge 和后台任务心跳。两组目标的 `cluster` 标签必须一致；缺少这一组目标会持续触发后台任务心跳缺失告警。

单实例部署的两组目标可使用相同地址；默认 `host.docker.internal:8080` 只是示例，需要根据真实地址修改。已有部署升级时，重新运行初始化命令以补齐 `cluster-targets.json`（不会覆盖已有凭据或目标），填写后重新创建 Prometheus 容器以挂载新文件。变更 token 后也需重新创建容器，目标文件由 file-SD 定期读取。

默认仅发布宿主回环端口：Prometheus `19090`、Thanos Query `10902`、Alertmanager `19093`、Watchdog `19094`。应用本地运行时可设置 `OPS_MONITORING_MODE=standalone` 和 `OPS_MONITORING_PROMETHEUS_URL=http://127.0.0.1:10902`；Kubernetes 使用 `kubernetes` 或 `auto` 模式。容器内应用应使用可达的内部网络地址或受控代理，不能把自身容器的 `127.0.0.1` 当作宿主。Query 提供 Prometheus API，配置中的 `OPS_MONITORING_BEARER_TOKEN` 用于需要鉴权的代理。

各组件使用项目独立命名卷，`docker compose down` 保留数据；只有明确放弃此项目数据时才加 `--volumes`。`volume-init` 仅初始化 Store/Compactor 卷根目录的 UID，长驻 Thanos 进程仍使用镜像非 root 用户。Sidecar 与 Prometheus 以相同 UID 访问 TSDB。

## 存储与留存

Prometheus 原始数据保留 `31d`，本地容量上限默认 `10GB`，容量与时间限制先到者生效。Sidecar 上传关闭的两小时块，Query 同时查询 Sidecar 和 Store。Compactor 配置原始 `31d`、5 分钟降采样 `90d`、1 小时降采样 `365d`；Query 开启自动降采样。持久化卷分别保存原始 TSDB、对象、Store 缓存、Compactor 工作数据和接收记录。

Sidecar 需要唯一外部标签、相同的 TSDB 最小/最大块时长；配置已设置两小时块，避免 Prometheus 本地压缩与 Sidecar 上传冲突。[Thanos Sidecar 文档](https://thanos.io/tip/components/sidecar.md/)

Compactor 在后台执行压缩、降采样与留存。生产配置保留默认删除延迟，超过保留期限不会立即物理删除；不要对同一个桶运行多个无分片 Compactor。[Thanos Compactor 文档](https://thanos.io/tip/components/compact.md/)

使用批准的云对象存储时，复制 `object-store.cloud.example.yml` 为私密配置，设置桶、区域、端点和工作负载身份后运行：

```sh
THANOS_OBJECT_STORE_CONFIG=/absolute/private/object-store.yml \
docker compose -f deploy/monitoring/thanos-retention.compose.yml \
  -f deploy/monitoring/thanos-cloud.override.yml up -d --build
```

云存储模式不启用 `local-object-store` profile。真实桶需预先创建，遵循最小权限、TLS、备份与容量策略。支持的提供方配置以 [Thanos 对象存储文档](https://thanos.io/tip/thanos/storage.md/) 为准。本地 MinIO 仅用于隔离验证，不替代客户生产对象存储的审批与可用性设计。

## 独立 Watchdog 与财务观测缺口

Prometheus 持续产生 `ModuRelayMonitoringWatchdog`，Alertmanager 每分钟发送，接收端默认在 180 秒没有收到 firing 心跳时令 `/healthz` 返回 `503`。普通告警、resolved 心跳和鉴权失败请求不能刷新计时。`/alerts` 与 `/events` 需要 `Authorization: Bearer <receiver-token>`；接收记录有界持久化，重启后继续计算过期时间。

正式环境应把 `watchdog-receiver.compose.yml` 部署到**不同主机或故障域**，用 TLS 反向代理开放 `/alerts`，通过独立运维探针检查 `/healthz`。复制 `stack-alertmanager.external.example.yml` 到私密路径，替换示例域名，将同一个接收 token 分发到双方，设置 `MONITORING_ALERTMANAGER_CONFIG` 后重新创建 Alertmanager。默认同宿主接收端只验证软件链路；宿主整体掉电仍需要外部探针发现。[Alertmanager Webhook 配置](https://prometheus.io/docs/alerting/latest/configuration/)

`billing-observation.rules.yml` 独立观察 `modurelay_billing_observation_unpersisted > 0` 或 `modurelay_billing_observation_missed_total` 的五分钟增量，持续一分钟触发 `ModuRelayBillingObservationGap`。这表示结算观测链路存在缺口，需人工核对；不会补扣款或修改账务。`last_failure_timestamp_seconds` 保留排查时间，业务服务仍以 Unknown 表达未核实状态。

## 可重复验证

```sh
python3 -m unittest discover -s deploy/monitoring -p test_watchdog_receiver.py
python3 deploy/monitoring/smoke-stack.py --output /tmp/monitoring-stack.json
python3 deploy/monitoring/smoke-retention.py --output /tmp/monitoring-retention.json
python3 deploy/monitoring/smoke-downsampling.py --output /tmp/monitoring-downsampling.json
python3 deploy/monitoring/benchmark-history.py --output /tmp/monitoring-history.json
docker run --rm --entrypoint /bin/promtool -v "$PWD/deploy/monitoring:/rules:ro" -w /rules prom/prometheus:v3.15.1 test rules billing-observation.test.yml
```

这些脚本使用自己的临时目录、Compose 项目或容器，退出时删除自己的测试卷/数据。不会停止其他容器，不向真实用户发送消息。磁盘剩余不超过 5 GiB 时拒绝继续生成数据。

2026-10-23 本地实测：Watchdog 两项单元测试通过；完整栈验证了实时查询、真实 TSDB 块上传 MinIO 后经 Store 查询、鉴权通知、重启持久化、采集/通知管道中断后的心跳过期。历史测试块未进入 Prometheus TSDB，排除了 Sidecar 代答。真实 Thanos Compactor 删除了 40 天、400 天前的原始样本块，保留 5 天前块。该删除测试使用私有 FILESYSTEM 桶和零删除延迟，只证明原始 31 天策略，不冒充 90/365 天降采样长期运行证据。

另使用 Prometheus 官方 BlockWriter 创建一条 Counter 的真实 TSDB 块（12 天跨度、3457 个样本），交给 Thanos Compactor 三轮运行，实际生成原始、5 分钟、1 小时数据块。读取落盘 `meta.json` 验证 `thanos.downsample.resolution` 分别为 `0`、`300000`、`3600000` 毫秒；降采样代码路径已实际执行，不仅是配置检查。该小规模验证不代表真实对象存储吞吐或 90/365 天到期删除已经验收。

历史查询使用生产代码固定 PromQL、31 天合成 Counter/Histogram、60 秒采样、57 条序列、2,544,480 个样本（输入约 568 MiB，TSDB 约 7.6 MiB）；每项预热一次后运行五次。7 天步长 300 秒、2017 点；30 天步长 900 秒、2881 点。下表单位为秒，p95 为五次运行中的最近秩统计量；少量重复只构成本机基线。

| 指标 | 7 天 p95 | 30 天 p95 |
| --- | ---: | ---: |
| HTTP 5xx 比例 | 0.395 | 3.659 |
| HTTP P95 | 0.287 | 3.772 |
| HTTP P99 | 0.593 | 2.949 |
| QPS | 0.348 | 1.241 |
| 请求量 | 0.264 | 0.997 |

30 天前三项未达到脚本自设的 2 秒本地目标。该基准直接查询 Prometheus 原始块；生产对象存储、Thanos 降采样、实际序列规模、并发和网络不同，需要在客户约定的规模和 SLO 下验收。它证明长窗口真实查询可运行并记录了性能限制，不代表生产性能已经达标。完整 p50/p95/max、逐次耗时、镜像版本和各项验证结果保存在 [本地证据 JSON](evidence/local-validation-2026-10-23.json)。
