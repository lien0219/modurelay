# 隔离 Kubernetes 监控验收

此流程创建专用测试库和测试 Pod，读取真实 Kubernetes、kube-state-metrics、cAdvisor、Prometheus 与应用 API。它不能代替托管云集群验收、生产容量测试或邮件服务交付验收。不要在现有业务 namespace 上执行。

## 固定环境与隔离

本轮固定组合：k3d v5.9.0、K3s v1.35.5-k3s1、kube-state-metrics v2.20.0、Prometheus v3.5.0。应用镜像必须来自待交付源码并包含前端 embed 构建；记录实际 image ID。K3s 官方 ARM64 airgap 包 SHA256：`0bf33db3b0595166f1c36a6780ebd789343f50f23a4d61157519ada4d002919d`。

使用独立 kubeconfig，集群名以 `modurelay-monitoring-` 开头。不要更新默认 context，不要复用用户数据库，不要全局 prune。主机必须有足够空间；本轮设置 4 GiB 节点内存，磁盘低于 5 GiB 时停止新增环境。

```sh
k3d cluster create modurelay-monitoring-k3s \
  --servers 1 --agents 0 --servers-memory 4g \
  --image rancher/k3s:v1.35.5-k3s1 --no-lb \
  --api-port 127.0.0.1:65050 \
  --k3s-arg '--disable=traefik,servicelb@server:0' \
  --kubeconfig-update-default=false --kubeconfig-switch-context=false
k3d kubeconfig get modurelay-monitoring-k3s > /tmp/modurelay-monitoring-kubeconfig
chmod 600 /tmp/modurelay-monitoring-kubeconfig
```

端口需事先确认空闲。k3d v5.9.0 的 `--api-port 127.0.0.1:0` 在本轮生成了端口为 0 的 kubeconfig；若使用动态映射，必须从 `docker port <专用节点> 6443/tcp` 获取实际端口后仅修正专用 kubeconfig。

先从官方镜像仓库拉取并记录 digest，再导入，避免 Pod 启动依赖外网。若 registry 超时，保留失败记录；可使用官方 release airgap 包，校验 SHA256 后挂载到 `/var/lib/rancher/k3s/agent/images/`，K3s 自动导入。不能把导入成功当成业务验收通过。

```sh
k3d image import -c modurelay-monitoring-k3s \
  modurelay-monitoring-implementation:20261023 postgres:18.1-alpine3.23 \
  redis:7-alpine prom/prometheus:v3.5.0 \
  registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.20.0
python3 deploy/tests/cluster-monitoring-kubernetes-prepare.py \
  --artifacts-dir /tmp/modurelay-monitoring-live
python3 deploy/tests/cluster-monitoring-kubernetes-fixtures.py \
  --artifacts-dir /tmp/modurelay-monitoring-live
kubectl --kubeconfig /tmp/modurelay-monitoring-kubeconfig apply \
  -f /tmp/modurelay-monitoring-live/stack.json
kubectl --kubeconfig /tmp/modurelay-monitoring-kubeconfig apply \
  -f /tmp/modurelay-monitoring-live/fixtures.json
```

生成的凭据和含 Secret 的 manifest 权限为 0600。不要复制到仓库、工单或交付日志。所有应用容器通过同一 Secret 共享 JWT，数据库仅属于此 namespace。应用每次修改迁移内容后，使用新测试库，不能修改已有 migration checksum。

## 现场检查

为专用应用和 Prometheus 创建两个显式 IPv4 loopback 端口转发。不要使用已占用端口：一个端口可能只在 IPv6 绑定成功，而 IPv4 请求意外落入现有服务。

```sh
kubectl --kubeconfig /tmp/modurelay-monitoring-kubeconfig -n monitoring-acceptance \
  port-forward --address 127.0.0.1 service/modurelay-metrics 18080:8080
kubectl --kubeconfig /tmp/modurelay-monitoring-kubeconfig -n monitoring-acceptance \
  port-forward --address 127.0.0.1 service/prometheus 19090:9090
python3 deploy/tests/cluster-monitoring-kubernetes-probe.py \
  --artifacts-dir /tmp/modurelay-monitoring-live --stage baseline
```

探针接受 `--app-url`、`--prometheus-url`、`--timeout`、`--namespace` 和 `--cluster-label`。每阶段将真实响应保存在 `live-<stage>.json`，任何断言失败退出码为 1。

- `baseline`：真实抓取健康、CPU/内存、普通/初始化/sidecar/Pod-level/overhead有效 Limit、无 Limit 的 NotApplicable、HPA目标值、CrashLoop。
- `alert-create`、`alert-fired`：创建只在隔离库生效的 CrashLoop 规则并等待真实告警。规则禁用邮件，不会向他人发送消息。
- 将 HPA `spec.minReplicas` 改为 3 后执行 `scale`：验证 Deployment 就绪副本和三个独立实例抓取目标，以及 Deployment/HPA Events 归档。
- 将隔离 kube-state-metrics Deployment 缩到 0 后执行 `unknown`、`alert-held`：抓取/状态缺失不得恢复告警；随后恢复其副本到 1。
- 在 `crash-control` ConfigMap 添加 `healthy: "true"`，等待投影与容器退避结束后执行 `recovered`、`alert-resolved`：同 UID 应恢复到可信零值并解除告警。
- 删除并按原 manifest 重建 `ordinary` Pod 后执行 `uid`：旧 UID 无当前值，新 UID 有正确 Limit。
- `oom`：周期性内存耗尽负载验证 OOM last state、重启增量、cAdvisor OOM counter。窗口增量是 Prometheus 采样估算，不是精确事件审计。

测试负载会故意 crash/OOM，必须限定在专用集群。保留原始失败输出和修复后复验结果，不能删除失败后只展示成功。归档 `kubectl get pods -o json`、Events、Prometheus targets、镜像 ID 和探针 JSON 时检查不含凭据。完成后只删除本流程创建的专用集群/镜像/临时文件；禁止全局清理 Docker 或修改用户默认 kubeconfig。

新装测试管理员可能受到合规确认门控。自动化测试应在**专用本地测试库**使用明确标注 `disposable-automated-test-fixture` 的测试状态；它不代表操作人员接受任何真实部署条款，不得复制到真实环境。若复用本轮的探针，需先按仓库 `admin_compliance` 单元测试设置该本地 fixture，否则 API 423 是预期门控而非监控故障。生产管理员必须自行完成真实流程。

本轮实际发现 embed 构建的 `/metrics/instance` 曾被 SPA middleware 返回 HTML，已经增加后端路由豁免及 embed 回归。验收必须检查无令牌为 401、正确令牌为 Prometheus text/plain，并检查三个独立目标均 `up=1`，不能只检查 HTTP 200。

`traffic --kubeconfig <专用文件>` 阶段从隔离 Redis 测试容器经过 Kubernetes Service 发出 90 个成功请求，比较三个实例的总 Counter 增量，应恰好为 90。该阶段应单独运行，避免其他探针同时登录（登录也是合法 POST 流量）；脚本先等待一次抓取让自身登录进入基线。它证明计数守恒与实例去重，不代表负载容量达标。

专用库 fixture 可使用带仅测试用户校验的脚本：

```sh
kubectl --kubeconfig /tmp/modurelay-monitoring-kubeconfig -n monitoring-acceptance \
  exec -i deployment/postgres -- psql -U audit -d audit -v ON_ERROR_STOP=1 \
  < deploy/tests/cluster-monitoring-kubernetes-test-admin.sql
```

## 第二发行版 kind

本轮第二环境为 kind v0.33.0，固定节点镜像 `kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0`。先归档并删除本流程拥有的 k3d 集群，再创建 `modurelay-monitoring-kind`，指定独立 `--kubeconfig`；使用 `kind load docker-image --name modurelay-monitoring-kind` 导入应用及依赖。准备脚本传 `--cluster-label acceptance-kind`，探针也传相同标签。第二环境使用全新凭据、全新 Postgres 数据目录。

kind 不默认安装 metrics-server。官方 v0.8.1 `components.yaml` SHA256 为 `4a672c4891902573a3ff753cece5de1bf1f55dd053403dfec39df9d1636b7ff1`，镜像 digest 为 `sha256:b2d2efaf5ac3b366ed0f839d2412a2c4279d4fc2a2a733f12c52133faed36c41`。本地 kind 测试可添加 `--kubelet-insecure-tls` 适配临时 kubelet 证书，此设置只用于该一次性集群，不是生产配置建议。通过 `kubectl top nodes` 确认 resource metrics API 实际可用。

两个本地发行版仍共享 Docker Desktop/Linuxkit 宿主内核，不能视为两个独立托管云平台，也不能排除共享内核造成的 OOM exporter 局限。

## 2026-10-23 实测结果

| 隔离环境 | 断言结果 | 应用镜像 | 结果范围 |
| --- | --- | --- | --- |
| k3d 5.9.0 / K3s 1.35.5+k3s1 | 32 / 33 通过 | `sha256:ae68896d0b501df74d5133f2f9dbaa827a3d7e3a84abc3cb720c390db5934653` | 已修复 embed 指标路由；早于迁移 280 与最终 cgroup OOM 聚合修复 |
| kind 0.33.0 / Kubernetes 1.35.8 | 32 / 33 通过 | `sha256:06875c9425b160484d01d4109074f839500469a85724d0414d7ba91355dd2f10` | 最终源码、生产前端 embed、迁移 277–280 与最终 OOM 聚合实现 |

两套环境均实际通过普通/初始化/sidecar/Pod-level/overhead Limit、无 Limit 的 NotApplicable、CPU/内存、HPA 目标、1→3 副本、三个实例 90 次请求计数守恒、Pod UID 重建、Deployment/HPA Events 归档，以及 CrashLoop 告警触发→采集失联保持→同 UID 可信零值恢复→解除告警。kind 还单独记录 `/metrics/instance` 无令牌 401、有效令牌 Prometheus text/plain 200、HTTP 维度筛选 API、迁移和资源 metrics API。

两套唯一未通过断言为 `real_oom_counter_increase`：真实负载产生了 OOMKilled 和重启增量，但同一 Docker Desktop/Linuxkit 宿主上的 cAdvisor `container_oom_events_total` 始终为 0。因此不能将 OOM 窗口 Counter 验收标为通过；`pod_oom` 的最近终止状态与 `pod_oom_count` 的窗口增量应继续分开描述。两个本地发行版也不代表客户托管云签收。

可交付的脱敏证据位于 [Kubernetes 验收归档](../../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/manifest.json)，逐项真实响应见 [K3s 断言](../../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/k3s-acceptance.json) 和 [kind 断言](../../outputs/cluster-monitoring-review-20261023/test-evidence/kubernetes/kind-acceptance.json)。归档包含原始失败/重试记录、选取的 Pod 状态和 Events、目标健康、OOM 原始 Counter；剔除 Secret、kubeconfig、Pod spec/env/annotations，并核对不含生成的测试凭据。临时集群销毁后，这些文件仍可用于复核；再次运行时应生成新的独立证据。
