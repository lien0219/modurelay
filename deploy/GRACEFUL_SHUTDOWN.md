# 请求排空与优雅退出

收到 `SIGTERM` 或 `SIGINT` 后，应用先让 `GET /ready` 返回 503，等待负载均衡摘流传播，再停止接纳新的业务请求。已接纳的 HTTP/SSE 请求继续执行，WebSocket 也纳入跟踪。排空完成便提前进入清理，不会固定等待十五分钟。

`GET /health` 保留存活检查语义；不要用它判断是否应该继续分配新请求。`/ready` 是本实例的流量准入状态，不是数据库、Redis 或上游服务的全链路可用性检查。没有公开的排空写接口；由进程信号触发退出。

健康及监控请求不计入业务请求指标，但仍纳入内部退出等待；它们经过的中间件和指标查询也可能使用数据库。强制取消后必须等待处理函数真正返回，才可关闭这些依赖。

## 预算

| YAML 配置（`server` 下） | 环境变量 | 默认秒数 | 允许范围 |
| --- | --- | ---: | ---: |
| `drain_delay_seconds` | `SERVER_DRAIN_DELAY_SECONDS` | 5 | 0–300 |
| `shutdown_timeout_seconds` | `SERVER_SHUTDOWN_TIMEOUT_SECONDS` | 900 | 1–86400 |
| `force_close_timeout_seconds` | `SERVER_FORCE_CLOSE_TIMEOUT_SECONDS` | 10 | 1–300 |
| `cleanup_timeout_seconds` | `SERVER_CLEANUP_TIMEOUT_SECONDS` | 30 | 1–300 |

四个阶段使用独立预算：摘流传播 → 请求排空 → 超时取消后等待 handler 收尾 → 队列及资源清理。默认最大合计为 **945 秒**，部署层使用 **960 秒（16 分钟）**，留出 15 秒运行时余量。修改应用预算时，必须同步提高外层停止宽限；外层先 `SIGKILL` 会绕过后续计费和日志收尾。

摘流传播窗口内仍可接纳尚未更新路由的请求，随后关闭新业务准入。`SERVER_DRAIN_DELAY_SECONDS=0` 适合无代理的本地试验；有代理时，应按实际探针间隔、失败阈值、超时和路由传播延迟设置，不应只看一次 HTTP 探测耗时。

正常退出先等待请求，停止后台任务生产者，再依次排空用量/错误/审计队列、刷新缓存与配额、停止系统日志写入器，最后关闭 Redis 和数据库。阶段超时或停止失败时，不继续关闭仍可能被使用的底层资源；进程以非零状态退出。达到预算上限时会记录未完成状态并结束等待，不能保证剩余内存队列全部落库。进程崩溃、宿主机故障、OOM 和 `SIGKILL` 不在优雅退出保证内。

## 流式请求边界

- HTTP/SSE 在预算内可以自然结束；已有请求不会转移到另一实例。流已经返回 200 后仍可能中途断开，应检查协议终止事件及客户端读错误。
- Responses WebSocket 在当前生成轮次结束、终止事件发送和用量提交后关闭并提示客户端重连；空闲会话不应无限阻止退出。客户端需处理重连，不能假定长连接永久有效。
- 其它长 WebSocket 会话等待自然结束或达到排空预算，再强制取消。单纯调用 Go `http.Server.Shutdown` 不会等待已升级连接，因此应用单独跟踪它们。
- 超过配置预算的任意长流仍可能被中断。增大预算提高完成机会，无法提供无限等待或跨进程无缝续流。不要盲目重试可能已经执行或计费的请求。

## 部署

Compose 各应用服务默认 `stop_grace_period: ${APP_STOP_GRACE_PERIOD:-16m}`。容器健康检查继续使用 `/health`；代理使用 `/ready`。生产脚本的单容器 `--force-recreate` 会先停止旧容器再启动新容器，优雅排空能保护已有请求，但它仍有新请求不可用窗口。需要持续接流时，应部署多个可用实例，并确认更新期间有足够剩余容量。

示例 Caddy 上游探针已使用 `/ready`，检查周期 2 秒、超时 1 秒，默认单次失败即可摘流；这些时间小于默认 5 秒传播窗口，但实际代理和网络传播仍需现场验证。保留 SSE 自动刷新及原有非 SSE 压缩行为。

systemd 示例使用 `TimeoutStopSec=960`。通过 unit override 修改 `SERVER_*` 时，同步修改 `TimeoutStopSec`，再执行 `systemctl daemon-reload`。Apple container 脚本对应用使用 `.env` 中的 `APPLE_CONTAINER_STOP_TIMEOUT_SECONDS`（默认 960），PostgreSQL/Redis 仍各等待 30 秒；脚本先停止应用，再停止依赖。

Kubernetes 可将 [graceful-shutdown-patch.yaml](kubernetes/graceful-shutdown-patch.yaml) 合并到已有 Deployment。先按实际名称修改 Deployment 和 container 名称、监听端口，保留已有镜像、Secret、存储和资源配置。示例设置：

- readiness `/ready`，liveness `/health`。
- `terminationGracePeriodSeconds: 960`；滚动更新 `maxUnavailable: 0`、`maxSurge: 1`，需要集群有额外容量。
- 不添加重复 sleep 的 `preStop`。Pod 终止时 EndpointSlice 已将其标记为 terminating、ready=false，应用仍需等待路由更新并完成请求、计费和日志收尾。已有 `preStop` 的耗时也占用整个 Pod 宽限，合并前应一并计入预算。

Kubernetes 宽限不是对崩溃/节点故障的保证，也不能替代网关/Ingress 自身的连接和请求超时。参阅 [Pod 终止流程](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination-flow) 与 [Go Shutdown 语义](https://pkg.go.dev/net/http#Server.Shutdown)。

## 观测与验收

退出期间以本实例标准输出中的阶段日志为主要依据，检查排空开始、活跃 HTTP/WS/轮次数、超时和强制关闭、清理结果及总耗时。系统日志写入器停止后的最终日志只能依赖标准输出/journal/容器日志采集。

`GET /metrics/instance` 沿用 `OPS_METRICS_ENABLED` 与 `Authorization: Bearer <OPS_METRICS_TOKEN>`，提供本实例排空及活跃请求状态。应按 Pod/实例直接抓取，并保留 terminating Pod 为目标；通过 Service 随机抓取一个 Pod 会漏掉正在退出的实例，监听器关闭后则无法继续抓取。现有 `GET /metrics` 仍返回集群聚合值，不能把每个 Pod 的返回值再次 `sum`；口径见 [CLUSTER_DEPLOYMENT.md](CLUSTER_DEPLOYMENT.md)。

本地验收应覆盖：短请求正常结束、超过旧 5 秒限制的 SSE 正常结束、超出新预算的强制取消、Responses WebSocket 当前轮次结束后断开、空闲 WS、退出期间新请求准入、用量/错误/日志队列收尾，以及 cleanup 超时。使用本地测试库核对真实用量和错误记录；进程替身测试不能证明数据库最终落库。

本次验证在本地进行；尚未连接生产机器验证 Kubernetes、Ingress/Caddy 路由传播或真实线上计费。上线前需在目标环境演练 SIGTERM、滚动更新和缩容，核对客户端终止事件与后台用量记录。
