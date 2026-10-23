# 集群告警计时、规则修改与通知语义

本文件说明代码策略；真实集群、SMTP、公网 Webhook 故障注入仍需部署验收。

- **持续时间**：第一条有效超阈样本是计时起点。后续按评估间隔累计，需 `ceil(持续秒数 / 评估间隔秒数) + 1` 条连续有效样本。相同间隔内重试不累计；正常的 Leader 交接从数据库读取进度。
- **缺失与低样本**：查询失败、资源缺失、样本不足立即清零持续证据；距上次评估达到两个完整间隔也重置，不能把未观测的停机时间算作持续超阈。已有 firing 事件保持，不将 Unknown 或 InsufficientData 当作健康恢复。
- **规则修改**：窗口、持续时间、指标、阈值、比较符、冷却、最低样本、严重级别、启停或影响评估的过滤条件变化，递增 `evaluation_revision`。该版本的持续证据重新建立，旧事件保留并标为 `manual_resolved`，`resolution_reason=rule_reconfigured`，不发送虚假的“恢复正常”通知。并发中的旧评估器不能重新写入旧状态、创建旧版本事件，或修改新版本事件。
- **仅通知修改**：名称/说明、邮件开关、Webhook 目标、`silenced_until` 不改变评估版本和已有持续计数。资源级静默独立存储。
- **指纹与事故**：告警指纹包含规则 ID、评估版本、窗口和资源标识；Pod 使用 UID。数据库对每个规则/资源的 active 事件保持唯一。不同规则在同一资源上可关联同一事故，保留各自事件；最后一个 active 事件结束后事故关闭。
- **静默到期**：静默期间可记录 firing 事件而不产生投递任务。静默结束且有效样本仍超阈时，补建缺失的首次 firing 通知；持久 outbox 的事件/渠道/转换/目标唯一键使重复评估和进程重启不重复入队。已经恢复的事件不会补发过时 firing。配置从未发送目标的事件增加新目标后，可以向新目标发送当前 firing。
- **通知重试**：通知 worker 与评估 Leader 独立，通过数据库租约领取；失败退避重试，最终失败可人工重试。Webhook 使用稳定 `Idempotency-Key`，接收方应去重。SMTP 为至少一次投递，远端收取成功但本地回执前进程退出可能重复，不能承诺 exactly-once。
- **无效历史规则**：当前监控指标不支持 `platform/group_id/region`。新增/编辑时拒绝这些组合；历史规则保留原配置并显示 `invalid_rule`，停止查询与触发/恢复，等待管理员修正，不能自动放宽为全服务范围。
- **入站门槛**：`qps_started/rpm_started/request_count_started` 的 `min_requests` 使用同一评估窗口的入站 Counter 增量；完成请求量不能代替正在处理的入站请求量。
- **Deployment 告警**：使用 `deployment_cpu_usage_percent/deployment_memory_usage_percent/deployment_restarts/deployment_oom_count` 聚合指标。普通 Pod 指标中的 `deployment` 是所属工作负载过滤，各 Pod 仍按 UID 独立评估。
