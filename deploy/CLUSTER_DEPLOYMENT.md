# 多实例、Redis Cluster 与 PostgreSQL 读写分离

默认配置仍为单节点 Redis、单个 PostgreSQL 主库；不设置新变量即可沿用单实例部署。应用 Pod 数量与 Redis 模式相互独立：多个 Pod 也可以共享单节点 Redis。

## 环境变量

```dotenv
# 所有 Pod 使用同一个 PostgreSQL 可写主库入口
DATABASE_HOST=postgres-primary.database.svc.cluster.local
DATABASE_PORT=5432
DATABASE_USER=sub2api
DATABASE_DBNAME=sub2api

# 可选：报表副本（可填指向多个只读实例的 Service）
DATABASE_READ_REPLICA_ENABLED=true
DATABASE_READ_REPLICA_HOST=postgres-replicas.database.svc.cluster.local
DATABASE_READ_REPLICA_PORT=5432
DATABASE_READ_REPLICA_MAX_OPEN_CONNS=64
DATABASE_READ_REPLICA_MAX_IDLE_CONNS=16
DATABASE_READ_REPLICA_CONNECT_TIMEOUT_SECONDS=3

# Redis Cluster；不使用 Cluster 时设为 standalone 并继续用 REDIS_HOST/PORT
REDIS_MODE=cluster
REDIS_CLUSTER_ADDRS=redis-0.redis.database.svc.cluster.local:6379,redis-1.redis.database.svc.cluster.local:6379,redis-2.redis.database.svc.cluster.local:6379
REDIS_DB=0

# 可选：集群级 Prometheus 抓取接口
OPS_METRICS_ENABLED=true
```

密码和 `OPS_METRICS_TOKEN` 通过 Kubernetes Secret 注入。所有应用 Pod 使用相同的 `JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 和功能相关加密密钥。完整选项见 [.env.example](.env.example) 和 [config.example.yaml](config.example.yaml)；Compose 各部署文件均透传新配置。仅运行应用、连接外部数据库/Redis 时使用 `docker-compose.standalone.yml`。

副本的 `USER/PASSWORD/DBNAME/SSLMODE` 留空时继承主库，`PORT=0` 时继承主库端口。可单独设置只读数据库用户。副本连接启用 `default_transaction_read_only=on`；副本不可用时，报表回退主库并退避 30 秒后重试。主库与副本有独立连接池，容量预算按「Pod 数 × 每 Pod 连接上限」计算。每个 Pod 另有最多 16 条连接、2 条空闲连接的主库任务锁连接池，避免后台持锁占满业务池；数据库连接预算需计入这一部分。主库入口必须只路由到可写节点，数据库高可用切换由基础设施提供。

主库连接用于 session advisory lock，必须直连 PostgreSQL 或使用保持会话的连接代理；不要把 `DATABASE_HOST` 指向 PgBouncer transaction/statement pooling 入口，代理切换后端会话会破坏锁的归属。迁移锁同样需要这一前提。

Redis ACL、密码、TLS 继续使用 `REDIS_USERNAME`、`REDIS_PASSWORD`、`REDIS_ENABLE_TLS`。Cluster 只支持 DB 0，种子节点可以只有一个，但应用必须能访问 Redis 发现到的所有节点地址，不能只暴露一个随机转发到 Redis 节点的普通 Service。连接池参数在 Cluster 中按节点计算，应相应降低原单节点环境里过大的 pool/idle 配置。计费、锁和限流不从 Redis 副本读取。

## 一致性边界

| 数据路径 | 读取位置 | 写入位置 |
| --- | --- | --- |
| 余额准入（资格检查、在途预留、固定费用） | 主库单列余额查询 | 主库 |
| 订阅额度、API Key 额度、鉴权状态、调度状态 | 主库及现有共享缓存 | 主库 |
| 扣费、退款、充值、图片费用冻结、幂等校验、事务 | 主库 | 主库 |
| 用量 dashboard/trend 报表 | 启用时优先副本 | 主库 |
| Ops 实时 QPS/TPS、告警和后台聚合任务 | 主库 | 主库 |

没有按 SQL 是否以 `SELECT` 开头来自动分流。所有普通 Ent 客户端及注入的 `*sql.DB` 始终指向主库，仅明确允许的报表方法使用副本。报表可能暂时滞后；报表结果不参与扣费判定。

扣费保留主库事务、数据库唯一幂等键和原子金额更新。余额缓存失效通过同一 Lua 操作更新跨 Pod 回填版本并删除余额键；收到成功响应才算确认，之前的异步回填无法再覆盖该版本。失效最多尝试三次，每次独立超时，失败后本实例保留该用户的待确认状态，后续访问先重试，未确认时拒绝该用户的余额准入；这个用户的失效错误不触发影响其他用户的全局熔断。

所有实例的余额准入都独立读取主库当前余额，不信任 Redis 残留值、不合并跨请求的余额读取。这样即使失效失败的实例崩溃，其他实例也不会继续按旧缓存余额放行。主库读失败时返回计费暂不可用；主库连接容量规划需计入这些单列查询。已提交的扣费不会因缓存失效失败被标记为未扣费，实际费用记录仍保留。既有按请求结束后的实际用量结算逻辑保持不变；流式请求在途费用估算、是否允许赊欠等仍由既有计费策略决定，这些策略与主从复制延迟无关。

## 指标抓取与后台口径

业务 QPS/TPS 原本就来自共享数据库中的请求记录；现在继续保持这个全局口径，不能把每个 Pod 返回的全局值再次相加。TPS 表示 tokens/second，包含输入、输出和缓存 token，不是 PostgreSQL transactions/second。

每个 Pod 每 15 秒发布一次 Redis runtime 心跳，45 秒没有更新的实例自动剔除。后台系统资源读数聚合活跃 Pod：连接池、内存、goroutine 相加，CPU 利用率取资源样本均值。CPU/内存延续现有 cgroup 优先、宿主机回退口径；建议为 Pod 配置 CPU/内存限制。无容器限额时按宿主机 boot ID 去重，避免多 Pod 重复累加同一宿主机容量；这不代表 Kubernetes 所有节点的完整监控。

`GET /metrics` 默认关闭，开启后必须携带 `Authorization: Bearer <OPS_METRICS_TOKEN>`。任意 Pod 都返回集群聚合结果；无法取得共享指标时返回 503，不把单 Pod 的值冒充集群总量。接口与业务 HTTP 共用端口，参考 [metrics Service](kubernetes/metrics-service.yaml) 和 [Prometheus 配置](prometheus.cluster.yml)。将 Service selector 改成实际应用 Pod 的标签，并限制其网络访问范围。

主要指标：

| 指标 | 含义 |
| --- | --- |
| `modurelay_cluster_qps` | 最近 60 秒已完成请求数 / 60 |
| `modurelay_cluster_tps` | 最近 60 秒 token 数 / 60 |
| `modurelay_cluster_instances` | 心跳仍有效的实例数 |
| `modurelay_cluster_db_connections_active` | 应用主库连接池的活跃连接总数 |
| `modurelay_cluster_redis_connections` | 活跃 Pod 到所有 Redis 节点的连接总数 |
| `modurelay_cluster_memory_used_bytes` | 活跃实例报告的内存用量总和 |

这些都是 gauge。QPS/TPS 按请求完成并落库后统计，不代表尚未完成的流式请求速率。抓取一个 Service 逻辑目标即可；若使用 PodMonitor 抓取每个 Pod，相同集群序列应以 `max` 去重，不能 `sum`，也不应对 QPS gauge 再用 `rate()`。

## 多实例审计与运行要求

- Redis 操作已适配跨槽读取、所有主分片扫描及多键 Lua 的 hash tag；同一原子操作涉及的键固定在同一槽。Live 并发控制、scheduler fencing、图片任务队列、平台配额及 dirty set 各自保持一个一致性域，需按对应分片容量规划，不能把它们拆到多个槽后声称仍然原子。其他缓存仍可跨分片分布。
- 新 Pod 启动仅清理过期并发占位，不再删除其他活跃 Pod 的占位和共享等待数。
- 常规定时任务、Ops 聚合/清理及计划账号测试使用主库 advisory lock，避免各 Pod 对 Redis 可达性不同导致选出两个 leader。简单运行模式也可多 Pod 部署，不再作为跳过清理任务锁的依据。
- Ops 分钟采集在锁内检查时间桶，避免多个 Pod 错峰重复写入同一分钟。
- 空库自动初始化的管理员检查与创建在一个主库事务锁内执行，避免多 Pod 同时创建初始管理员。
- 连接池、WebSocket 连接、进程 worker 队列和本地快速缓存仍属于各 Pod。扩容不能迁移正在执行的请求；发布时应留足优雅退出时间。文件结果、备份和插件文件若需被任意 Pod 访问，应配置对象存储或共享持久卷；节点本地目录不会自动复制。
- Grok 团队/模型软退避、上游传输熔断及 OAuth provider 刷新速率门控仍是进程内优化；它们不参与数据库扣费幂等，不作为集群全局配额保证。上游 provider 的限速预算要结合副本数配置。进程内异步用量队列也没有因此变成持久消息队列，强杀 Pod 的故障恢复需单独验证。
- 所有 Pod 必须连接同一套共享数据库/Redis，并使用相同配置与程序版本。不同环境不要共用相同 Redis 键空间。

从旧版本升级时，先排空旧 Pod 的请求和后台任务，再切换到新版本。旧版本的 Redis/DB 锁选择方式与新版本不同，不应长期混跑；Redis 单节点切换 Cluster 时还涉及键槽和缓存数据迁移，应在维护窗口内完成。除下述等待队列 v2 键外，单节点模式保留既有 Redis 键名。

### 等待队列 v2 升级

用户与账号等待队列分别使用 `concurrency:wait:v2:{userID}` 和 `wait:account:v2:{accountID}` 有序集合；每次入队有唯一等待者 ID 和独立到期时间。新请求不会延长其他等待者的有效期，迟到或重复释放也不会删除新的等待者。等待租约时长覆盖用户默认等待时间、sticky/fallback 等待超时中的最大值，并保留秒级时钟的边界余量。

旧键 `concurrency:wait:{userID}`、`wait:account:{accountID}` 仍保持整数类型。新版本把其正计数纳入准入及负载读取，但不修改数值、不延长 TTL；旧实例停止写入后，残留计数会自然过期。不要在升级时直接删除仍有旧等待者的键。

旧版本无法识别 v2 成员，因此两代实例同时接受新排队请求时不能保证统一等待上限。切换前先停止向旧 Pod 分发新请求，等待其排队及在途请求退出并停止旧实例，再由新版本接流量。若旧实例被强制停止，旧整数残留可暂时使新版本更保守，最长等待旧键剩余 TTL 后恢复；新版本后续流量不会延长这个窗口。Redis Cluster 下两代键沿用相同的并发域 hash tag。

## 验证

后端单元测试包含：单节点默认配置、Cluster 配置校验、真实 Redis Cluster 跨槽行为、两个实例的并发占位、余额过期回填、主副本读取隔离、指标双 Pod 聚合/过期/鉴权和后台锁互斥。真实 PostgreSQL 集成测试使用独立 Docker 容器验证多仓库并发扣费的幂等性。运行前确保 Docker 可用：

```sh
cd backend
go test -tags=unit ./...
go test -tags=integration ./internal/repository -run 'UsageBilling'
go test -tags=integration ./internal/setup -run TestBootstrapAdminConcurrentPods
go test ./internal/repository -run TestRedisClusterAtomicCaches -count=1
```

真实 Redis Cluster 测试需要本机 `redis-server`、`redis-cli`，自动启动并销毁三个临时主节点；缺少二进制时会明确跳过。测试部署环境应另外验证 Redis 主节点故障切换、PostgreSQL 人为复制延迟、Pod 滚动退出和生产流量级别的压测。
