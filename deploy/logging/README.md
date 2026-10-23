# 可选集中日志

应用继续输出 JSON stdout；Kubernetes 可由现有 Loki/ELK/OpenSearch/云日志采集器接收，不依赖云 SDK。记录内保留 request_id、instance、service、cluster、namespace、pod、pod_uid（后四项在相应环境配置后有值）。request_id 和 pod_uid 是查询字段，不作为高基数 Loki 索引标签。

## Docker / Linux 文件采集

启用 `LOG_FORMAT=json`、`LOG_OUTPUT_TO_STDOUT=true`、`LOG_OUTPUT_TO_FILE=true`，设置 `LOG_OUTPUT_FILE_PATH` 到单独日志目录。多进程各用不同文件，采集器只读挂载目录。设置 `LOG_DIRECTORY` 为该目录的绝对路径后执行：

```sh
LOG_DIRECTORY=/absolute/path/to/logs docker compose -p modurelay-logs -f deploy/logging/compose.yml up -d
```

无需挂载 Docker socket。Fluent Bit 持久化游标和有界磁盘重试缓冲；Loki 持久卷独立于应用/Pod，源文件删除不删除已接收日志。默认保留 30 天，由 Compactor 删除；删除延迟 2 小时，不是到期即时删除。生产对象存储、认证、备份按运维部署要求替换；此单机文件系统模板用于自建小规模/验收，不是高可用承诺。接现有 Loki 时设置 Fluent Bit `Host/Port/TLS`，不要把密钥放日志或 Git。

查询跨实例关联：`{job="modurelay"} | json | request_id="<request-id>"`。按 namespace/cluster/service 标签缩小范围，再用 `pod_uid` 检索重建前的记录。仅保留固定基础标签；禁止将用户密钥、用户 ID、Request ID、完整 URL 用作索引标签。

## Kubernetes stdout

使用节点标准 CRI 日志采集器，解析 CRI 外层后以 JSON 解析应用行。可复用 Fluent Bit 官方 Kubernetes DaemonSet，配置 `[FILTER] kubernetes` 的 `Merge_Log On`，Loki 输出 `Line_Format json`，仅选择 namespace/service/cluster 等有限标签。服务账号仅授予读 Pods/Namespaces；日志目录只读。应用 Downward API 见 `deploy/kubernetes/monitoring-env-patch.yaml`。无需更改应用镜像或调用云日志 SDK。

## 现有共享数据库路径

在 Ops → 系统日志 → 运行时配置中启用“持久化访问日志”，则各实例会将访问日志写入同一 PostgreSQL，按 host/request_id 集中查询并使用既有保留清理配置。此写入是有界异步尽力而为，页面显示队列丢弃/写入失败计数；入口拒绝采样记录可能不入库，因此需要完整 stdout 时使用外部采集。未启用时，页面明确显示访问日志能力受限，系统警告/错误日志仍可查询。启用共享库不意味着已连接 Loki，外部 Loki 查询使用其 API/Grafana。

本地取证脚本：`python3 deploy/tests/central-logs-test.py`。仅生成合成日志，在独立项目中验证两个实例同 request_id 可查、敏感字段不引入、删除源日志后仍可查，随后移除测试容器和卷。

配置依据：[Loki filesystem example](https://grafana.com/docs/loki/latest/configure/examples/configuration-examples/)、[Loki retention](https://grafana.com/docs/loki/latest/operations/storage/retention/)、[Fluent Bit Loki output](https://docs.fluentbit.io/manual/data-pipeline/outputs/loki)。
