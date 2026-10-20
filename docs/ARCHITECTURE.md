# 架构说明

## 边界与原则

系统将外部观测信号转换为可解释、可重放的 Incident。PostgreSQL 是所有持久状态的最终事实来源；Kubernetes Informer 负责观察而不承担持久一致性；规则是确定性的，不引入概率模型或自动修复。

首版部署边界是单集群、单租户、单实例。该约束允许进程内工作队列和本地资源图，但去重、Incident 与投递状态仍落入 PostgreSQL，以便重启后重放。

## 组件与数据流

1. Kubernetes Informer 接收 Event 和资源变化；Prometheus 接入层接收 firing/resolved Alert。
2. Normalizer 生成稳定 Signal ID，限制可变字段大小并保留原始数据引用。
3. Typed RateLimitingQueue 提供有界重试；处理耗尽后写入 Dead Letter。
4. Resource Graph 以 ClusterID、GVK、Namespace、Name、UID 定位根资源。FailedScheduling 的孤立 Pod 仅在该规则中允许以自身为根。
5. Rule Registry 执行三类确定性规则，输出 Fingerprint、Evidence、匹配条件和规则版本。
6. 单个 PostgreSQL 事务创建或恢复 Incident，同时写 Timeline、CorrelationDecision 与 Outbox。
7. Dispatcher 以 At-Least-Once 语义投递 HMAC Webhook；Context API 聚合相同事实供控制台和 Agent 使用。

## 一致性与失败语义

- `signals`、`processed_events` 和数据库唯一约束提供跨重启去重。
- Incident 活跃 Fingerprint 的部分唯一索引防止并发重复创建。
- Timeline、Decision、Incident 状态和 Outbox 同事务提交。
- Webhook 成功前 Outbox 保持待处理；重试耗尽进入 Dead Letter，可经受控接口重放。
- `/livez` 仅表示进程存活；`/readyz` 聚合迁移、数据库、必要依赖和 Kubernetes Cache Sync。

## 生命周期与安全

`signal.NotifyContext` 创建根 Context。HTTP、Controller Manager、Informer、Worker、Dispatcher 与诊断服务共享取消信号；任一核心组件异常向上返回并取消其余组件。HTTP 使用有界优雅关闭。

Kubernetes 权限由 Helm ClusterRole 明确列举。Secret 仅经环境变量或 Kubernetes Secret 注入。Webhook 目标执行 scheme、DNS/IP 和 allowlist 校验，并使用时间戳、事件 ID 与 HMAC。pprof 使用独立内部监听地址和 Bearer Token，公开 Service 不注册诊断路由。API 单租户身份不是安全隔离边界，对外暴露时应由可信反向代理认证。

## 可观测性与非目标

Prometheus 指标覆盖 Signal、队列、关联、Incident、Outbox 和 Webhook。可选 OTLP HTTP Exporter 将 Signal 处理、Incident 事务和异步投递以 Trace Link 关联。

安全多租户、多集群、多副本、Leader Election、分布式消息系统、Exactly Once、自动根因分析/修复、手工 Merge/Split、NodeNotReady E2E、Service/EndpointSlice/Ingress 拓扑和 PostgreSQL HA 均不在首版范围。
