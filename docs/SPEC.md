# Kubernetes Runtime Incident Platform Specification

## 1. 文档目的

本文档定义 Kubernetes Runtime Incident Platform 展示版本的功能范围、核心设计和验收标准。

项目目标是将 Prometheus Alert、Kubernetes Event 和 Kubernetes 资源拓扑，通过确定性、可解释的规则聚合为结构化 Incident，并通过稳定的 REST API 和可靠 Webhook，向人工、工作流及 Agent 提供经过筛选的故障上下文。

功能只有同时具备明确输入、确定输出、失败行为、自动化测试和可观测指标时，才视为完成。仅存在代码、接口或示例清单，不视为已经交付。

## 2. 版本范围

### 2.1 首版目标

首版以最快完成一个可部署、可演示、可重复验收的完整项目为目标：

- 单 Kubernetes 集群；
- 单租户；
- 单应用实例；
- PostgreSQL 持久化；
- Prometheus Alert 与 Kubernetes Event 接入；
- 基于资源拓扑的确定性 Incident 关联；
- Incident 生命周期与时间线；
- Agent Context REST API；
- Outbox 驱动的可靠 Webhook；
- ImagePullBackOff、CrashLoopBackOff、FailedScheduling 三类 E2E 场景；
- Docker Compose 本地依赖环境；
- Helm 与 kind 一键演示。

### 2.2 首版非目标

以下功能不纳入首版验收：

- 多租户安全隔离；
- 多集群统一控制面；
- 应用多副本和 Leader Election；
- 分布式消息队列；
- Exactly Once 投递；
- 自动根因分析；
- Agent 任意执行 kubectl 或 Shell；
- 自动修复、审批和回滚；
- 替代 Prometheus、Loki、Tempo 或 Grafana；
- 完整 On-call 排班和工单系统；
- 金融级容灾与跨地域高可用。

这些能力可以进入 Roadmap，但不得阻塞首版发布。

## 3. 核心验收场景

在全新的 kind 集群中执行一条命令安装系统并注入三类故障。系统必须将重复的 Prometheus Alert 和 Kubernetes Event 聚合为幂等、可解释、具有完整生命周期的 Incident，并通过版本化 Context API 和可靠 Webhook 暴露给外部消费者。

必须完成以下场景：

1. ImagePullBackOff / ErrImagePull；
2. CrashLoopBackOff；
3. FailedScheduling。

NodeNotReady 作为后续增强场景，不阻塞首版。原因是稳定制造 Node 故障通常需要多节点测试集群，展示成本较高。

## 4. 系统边界

```text
Prometheus Alert          Kubernetes Event
        |                        |
        +-----------+------------+
                    v
          Signal Normalization
                    v
 Informer + RateLimitingQueue + Worker
                    v
       Deduplication and Persistence
                    v
          Kubernetes Resource Graph
                    v
           Correlation Rule Engine
                    v
       Incident FSM + Timeline + Evidence
                    v
          PostgreSQL Transaction
                    v
        Context API + Outbox Webhook
```

Kubernetes API 用于保存策略和期望状态；PostgreSQL 用于保存高频 Signal、Incident、Timeline、关联决策和投递状态。不得将大量历史事件写入 CRD 或 etcd。

## 5. 领域模型

### 5.1 Signal

所有输入统一转换为 Signal：

```go
type Signal struct {
    ID              string
    ClusterID       string
    Source          SignalSource
    Type            SignalType
    Reason          string
    Namespace       string
    Resource        ObjectReference
    OwnerReferences []ObjectReference
    Labels          map[string]string
    Message         string
    OccurredAt      time.Time
    ObservedAt      time.Time
}
```

要求：

- Signal ID 必须稳定，重复投递不得生成新记录；
- 资源身份必须包含 ClusterID、GVK、Namespace、Name 和 UID；
- `OccurredAt` 表示源事件时间，`ObservedAt` 表示平台接收时间；
- 保存必要的原始引用，不在主表保存无限大小的原始对象。

### 5.2 Incident

Incident 至少包含：

- ID；
- Fingerprint；
- Title；
- Category；
- Severity；
- Status；
- RootResource；
- StartedAt、UpdatedAt、ResolvedAt；
- ReopenCount；
- Version；
- CorrelationPolicy 与 PolicyGeneration。

### 5.3 Evidence 与 CorrelationDecision

每次关联必须记录：

- 输入 Signal ID；
- 命中的规则名称和版本；
- 解析出的根资源；
- 生成的 Fingerprint；
- 时间窗口；
- 命中的条件；
- 关联原因；
- 决策时间。

系统必须能够回答“为什么这些 Signal 属于同一个 Incident”。

## 6. 数据库、配置与部署

### 6.1 数据库迁移

- 使用 Goose 或 golang-migrate 管理版本化 SQL；
- 删除 `main.go` 中的内嵌建表逻辑；
- 修复 `duration string` 等非法类型；
- 统一现有重复且不一致的 Incident Schema；
- 迁移失败时服务不得进入 Ready；
- CI 使用真实 PostgreSQL 15 从空库验证全部迁移；
- 重复执行迁移必须幂等。

至少包含以下数据模型：

- signals；
- incidents；
- incident_signals；
- timeline_entries；
- runtime_resources；
- resource_edges；
- correlation_decisions；
- processed_events；
- outbox_events；
- webhook_deliveries。

### 6.2 配置

提供 `.env.example`，至少包含：

```text
APP_PORT
POSTGRES_DSN
REDIS_ADDR
PROMETHEUS_URL
LOKI_URL
TEMPO_URL
CLUSTER_ID
KUBERNETES_ENABLED
WEBHOOK_WORKERS
```

要求：

- 不允许在代码中硬编码数据源地址；
- 启动时校验必填配置；
- Secret 不得打印到日志；
- README 中说明本地与集群内配置方式。

### 6.3 Docker Compose

本地 Compose 至少包含 PostgreSQL、Redis、Prometheus、Loki、Tempo、Backend 和 Frontend。

要求：

- 只保留一个 PostgreSQL；
- 所有服务配置 Healthcheck；
- 使用持久化 Volume；
- 固定或约束镜像版本；
- 后端和前端具有 Dockerfile；
- `docker compose up -d` 后 `/readyz` 可用。

### 6.4 Helm Chart

至少提供：

- Deployment；
- Service；
- ServiceAccount；
- RBAC；
- ConfigMap 和 Secret 引用；
- CRD；
- Startup、Liveness、Readiness Probe；
- Resources；
- SecurityContext；
- 可选 ServiceMonitor。

必须通过：

```bash
helm lint
helm template
helm install
helm upgrade
helm uninstall
```

### 6.5 一键演示

`make demo` 必须：

1. 检查 Docker、kind、kubectl 和 Helm；
2. 创建固定名称的 kind 集群；
3. 安装必要依赖；
4. 构建并加载镜像；
5. 安装项目；
6. 等待服务 Ready；
7. 应用演示策略；
8. 注入故障；
9. 等待 Incident；
10. 打印 API 与 UI 访问方式。

提供幂等的 `make demo-clean`。

### 6.6 仓库规范

- 删除 Git 中约 90 MB 的 `server` 二进制；
- 不提交构建产物和无用途的 coverage 文件；
- README 将 React/Tailwind 修正为实际使用的 Vue 技术栈；
- 明确已实现功能、已验证场景和已知限制；
- 提供 LICENSE、CONTRIBUTING、SECURITY 和架构文档。

## 7. client-go 可靠事件管线

### 7.1 监听范围

首版监听：

- core/v1 Events；
- Pods；
- Nodes；
- Deployments；
- ReplicaSets；
- StatefulSets；
- DaemonSets；
- Jobs。

工作负载资源用于构建资源图，Warning Event 用于生成故障 Signal。

### 7.2 处理流程

- 使用 SharedInformerFactory 与本地 Cache；
- 等待 Cache Sync 成功后再进入 Ready；
- 正确处理 Add、Update、Delete 与 DeletedFinalStateUnknown Tombstone；
- Informer Handler 仅执行轻量标准化和入队；
- 使用 Typed RateLimitingQueue；
- Worker 数可配置；
- 处理失败采用指数退避；
- 达到最大重试次数后写入 Dead Letter；
- 暴露队列深度、重试次数和失败数量指标；
- Context 取消后停止接收新任务并优雅退出。

### 7.3 验收标准

- 同一 Event 重复输入 100 次，只持久化一个 Signal；
- 前两次处理失败、第三次成功时，最终只生成一个 Incident；
- 超过最大重试次数后可查询 Dead Letter；
- 服务终止后在规定时间内优雅退出；
- `go test -race ./...` 无数据竞争。

## 8. Kubernetes 资源拓扑

首版至少维护：

```text
Pod -> ReplicaSet -> Deployment
Pod -> StatefulSet
Pod -> DaemonSet
Pod -> Job
Pod -> Node
```

要求：

- 使用 UID 区分同名重建资源；
- 使用 Informer Cache 和 Indexer，避免每条 Signal 查询 API Server；
- 保存资源和边的更新时间；
- 定期清理已删除且超过保留期的节点；
- Signal 可以解析到稳定的 RootResource；
- 资源图查询失败时保留 Signal，并进行有界重试，不得静默丢弃。

Service、EndpointSlice 和 Ingress 拓扑作为后续增强，不阻塞首版。

## 9. Correlation Rule Engine

### 9.1 设计原则

- 规则确定、可解释、可测试；
- 大模型不参与 Incident 身份和核心关联决策；
- 不为只有一个实现的组件创建无意义接口；
- 规则注册、匹配、Fingerprint 和证据输出保持清晰边界。

建议接口：

```go
type CorrelationRule interface {
    Name() string
    Version() string
    Match(context.Context, Signal, ResourceGraph) (MatchResult, error)
    Fingerprint(context.Context, Signal, ResourceGraph) (string, error)
}
```

### 9.2 CorrelationPolicy CRD

CRD 至少支持：

- enabled；
- sources；
- eventReasons；
- namespaces；
- groupBy；
- window；
- reopenWindow；
- severity；
- priority；
- explain 配置。

Admission Webhook 必须拒绝：

- 空 groupBy；
- 超出允许范围的时间窗口；
- 不支持的字段；
- 同优先级且可能冲突的规则；
- 无法编译的模板。

### 9.3 冲突规则

- 一个 Signal 只能有一个主 Incident；
- 高优先级规则优先；
- 同优先级多规则匹配视为配置错误；
- Incident 记录首次关联时的 PolicyGeneration；
- 无规则匹配的 Signal 保留为 Unmatched，不自动创建 Incident。

### 9.4 首版故障规则

每个规则必须定义输入 Signal、根资源解析、Fingerprint、时间窗口、合并条件、拆分条件、恢复条件和解释字段。

首版实现：

1. ImagePullBackOff / ErrImagePull；
2. CrashLoopBackOff；
3. FailedScheduling。

### 9.5 规则模拟

提供：

```http
POST /api/v1/correlation-policies/{name}/simulate
```

该接口使用历史或上传 Signal 进行 Dry Run，不修改真实 Incident，并返回输入数量、模拟 Incident、未匹配 Signal 和关联解释。

## 10. PostgreSQL 事务、幂等与 Incident FSM

### 10.1 状态

首版支持：

```text
Open -> Acknowledged -> Resolved
Open -> Resolved
Resolved -> Reopened
Reopened -> Acknowledged
Reopened -> Resolved
```

`Mitigating`、`Suppressed`、手工 Merge 和 Split 进入后续版本。

### 10.2 幂等与事务

一次 Signal 处理必须在单个事务内完成：

```text
插入 Signal
-> 创建或查找 Incident
-> 建立 Incident-Signal 关系
-> 更新 Incident Version
-> 追加 Timeline
-> 保存 CorrelationDecision
-> 写入 Outbox
```

要求：

- 使用唯一约束保证相同 Fingerprint 最多一个活跃 Incident；
- 使用行锁或乐观锁处理并发更新；
- 重复状态迁移具有幂等语义；
- 非法状态迁移返回稳定错误码；
- 支持可配置 Reopen Window；
- 数据库死锁和可重试错误进行有限重试；
- 所有事务具有超时。

验收标准：50 个并发任务处理相同 Fingerprint 时，最终只有一个活跃 Incident，Signal 关联完整，Timeline 和 Outbox 不重复。

## 11. Incident Context API

至少提供：

```text
GET  /api/v1/incidents
GET  /api/v1/incidents/{id}
GET  /api/v1/incidents/{id}/timeline
GET  /api/v1/incidents/{id}/context
GET  /api/v1/incidents/{id}/correlation
POST /api/v1/incidents/{id}/acknowledge
POST /api/v1/correlation-policies/{name}/simulate
```

要求：

- 提供 OpenAPI；
- Context Schema 独立版本化；
- 支持分页和时间范围；
- 限制 Evidence 和日志片段数量；
- 默认执行敏感字段脱敏；
- 返回 Incident Revision 或 ETag；
- 使用统一错误结构和错误码；
- Context 包含 Incident、RootResource、AffectedResources、Signals、Timeline、Evidence、CorrelationDecision 和原始数据链接。

首版 API 为单租户，不接受客户端随意指定 TenantID。

## 12. Outbox 与 Agent Webhook

### 12.1 投递语义

Webhook 提供 At-Least-Once 交付。消费者必须使用 `eventID` 幂等处理，系统不承诺 Exactly Once。

事件类型至少包含：

- incident.created；
- incident.updated；
- incident.acknowledged；
- incident.reopened；
- incident.resolved。

### 12.2 可靠性要求

- Outbox 与 Incident 变更处于同一数据库事务；
- 独立 Dispatcher 投递；
- HTTP 请求具有超时；
- 使用指数退避和随机抖动；
- 支持 Retry-After；
- 达到最大次数后进入 Dead Letter；
- 支持手工重放；
- 相同事件重试时 eventID 不变；
- 使用 HMAC-SHA256 签名；
- 使用时间戳限制重放；
- 防止访问本机、元数据服务和未授权内网地址；
- 限制 Payload 大小。

首版使用单 Dispatcher，不实现多副本竞争消费。

## 13. 单实例可靠性

为了最快完成可展示版本，首版不实现完整高可用集群，只验收以下可靠性：

- 所有持久状态保存于 PostgreSQL，不依赖进程内 map 作为最终事实；
- 服务重启后可以继续处理未完成 Outbox；
- Informer 重连或事件重放不会重复创建 Incident；
- HTTP Server、Informer、Worker 和 Dispatcher 使用统一 Context 优雅关闭；
- 组件错误不得在子 goroutine 中直接 `log.Fatal`；
- `/livez` 只检查进程存活；
- `/readyz` 检查数据库、Informer Cache Sync 和核心依赖；
- 滚动重启后已有 Incident、Timeline 和投递状态不丢失。

多副本、Leader Election、分布式锁和 PostgreSQL HA 进入后续版本。

## 14. 可观测性

### 14.1 日志

- 使用结构化日志；
- 日志包含 requestID、signalID、incidentID、policy 和 resource UID；
- 不记录 Token、密码和完整 Secret；
- 删除大量无意义的“初始化成功”日志。

### 14.2 指标

至少暴露：

```text
signals_received_total
signals_deduplicated_total
signals_failed_total
signals_unmatched_total
event_queue_depth
event_queue_retries_total
correlation_duration_seconds
incidents_created_total
incidents_reopened_total
incidents_resolved_total
outbox_pending
webhook_delivery_total
webhook_delivery_duration_seconds
```

### 14.3 Trace 与诊断

- 核心链路接入 OpenTelemetry；
- 支持从接收 Signal 追踪到 Incident 和 Outbox；
- 提供仅内部访问的 pprof；
- 提供 `/livez`、`/readyz` 和 `/metrics`。

## 15. 安全边界

首版为单租户部署：

- 不宣称安全多租户；
- 不使用客户端 `X-Tenant-ID` 作为安全边界；
- Kubernetes ServiceAccount 采用最小权限；
- API 使用静态 Token 或反向代理鉴权，具体方式在部署文档说明；
- Webhook Secret 通过 Kubernetes Secret 注入；
- 容器使用非 root 用户和只读根文件系统；
- Agent 接口首版只读，除 acknowledge 外不提供集群写操作；
- 所有人工状态变更记录操作者、时间和原因。

## 16. 性能与容量验收

### 16.1 目标规模

最终性能测试采用以下逻辑规模：

```text
500 Nodes
10,000 Pods
100,000 Kubernetes Events/天
2,000 活跃 Alert
10,000 Incident/天
```

### 16.2 测试方法

该目标通过合成资源拓扑与历史 Signal 回放测试，不要求真的创建 500 个 Kubernetes Node。kind E2E 只验证真实 Kubernetes 行为，容量测试验证业务处理能力。

压测必须记录：

- Signal 接收吞吐；
- P50、P95、P99 关联延迟；
- 队列最大深度；
- 积压恢复时间；
- CPU 和内存峰值；
- PostgreSQL 连接数和慢查询；
- 每个 Signal 的平均数据库操作数；
- Webhook 投递延迟；
- 去重比例与 Incident 生成数量。

首轮压测用于建立基线。不得在没有实测数据时预先声称具体 QPS 或延迟。发现容量不足时，应保留报告、瓶颈分析和优化前后对比。

## 17. 测试体系

### 17.1 Unit Test

覆盖：

- Signal ID 和 Fingerprint；
- 时间字段标准化；
- 三类 Correlation Rule；
- Incident 状态迁移；
- 去重与限流；
- Webhook HMAC；
- 重试退避；
- CorrelationDecision。

核心领域包目标覆盖率不低于 80%，但以行为覆盖而非数字为最终标准。

### 17.2 Race Test

CI 执行：

```bash
go test -race ./...
```

重点覆盖 Event Pipeline、Worker、Rule Registry 和 Dispatcher。

### 17.3 Benchmark

至少提供：

- BenchmarkSignalFingerprint；
- BenchmarkRuleEvaluation；
- BenchmarkIncidentCorrelation；
- BenchmarkEventNormalization。

### 17.4 envtest

验证：

- CRD 安装；
- AlertPolicy 生成 PrometheusRule；
- ObservabilityTarget 生成 ServiceMonitor；
- ObserveTenant Reconciler 能够运行；
- CorrelationPolicy 校验和 Status；
- Finalizer；
- OwnerReference；
- Generation 与 ObservedGeneration。

### 17.5 kind E2E

每个场景必须验证：

```text
故障产生
-> Signal 被接收
-> Incident 被创建
-> 重复 Signal 不产生重复 Incident
-> Timeline 存在正确证据
-> CorrelationDecision 可以解释关联原因
-> Context API 返回结构化上下文
-> Webhook 成功投递
-> 故障恢复
-> Incident 自动 Resolved
```

## 18. CI 验收

Pull Request CI 至少执行：

```text
gofmt check
go vet
go test
go test -race
frontend npm ci
frontend typecheck/build
database migration test
helm lint
docker build
```

kind E2E 可在主分支、Release 或带标签的 Pull Request 中执行，避免阻塞每次小改动。

## 19. 发布验收清单

首版发布必须同时满足：

- 全新环境可以通过文档完成安装；
- `make demo` 和 `make demo-clean` 可重复执行；
- 三类 E2E 全部通过；
- 重复和并发 Signal 不产生重复 Incident；
- 服务重启后数据和 Outbox 不丢失；
- Context API 有稳定 Schema 和示例；
- Webhook 重试、签名和 Dead Letter 可演示；
- 核心指标和健康检查可用；
- CI 全绿；
- README 明确限制为单集群、单租户、单实例；
- 性能测试覆盖约定的目标规模并输出报告；
- 已知限制记录在文档中。

## 20. 开发里程碑

### 20.1 提交历史与 GitHub 呈现约束

本次升级必须按照可独立构建、可独立验证、可安全回滚的粒度拆分为 77 个中文 commit。现有 2026 年 5 月的提交历史保持不变，本约束只适用于本次升级产生的 commit。

升级提交的时间范围固定为：

```text
首个升级 commit：2026-08-21
最后升级 commit：2026-10-08
时区：Asia/Shanghai（UTC+08:00）
实施周期：7 周
提交总数：77
```

各周提交数量允许根据实现复杂度自然波动，不得机械地固定为每周 11 个，但总数必须保持为 77 个。基准分布为：

```text
第 1 周： 9 个
第 2 周：12 个
第 3 周：10 个
第 4 周：13 个
第 5 周：11 个
第 6 周：10 个
第 7 周：12 个
```

最终迁移或推送至 GitHub 后，仓库 Commit 页面必须保留上述历史日期：

- 每个升级 commit 的 Author Date 与 Committer Date 使用同一个计划时间；
- commit 时间按实现依赖单调递增，不使用完全相同的时间戳批量提交；
- 每周分散在多个自然日，每日原则上为 1—3 个 commit；
- 远端仓库创建时间和实际推送时间可以晚于 commit 时间，不得在导入过程中覆盖 Git 元数据；
- 通过 GitHub Importer、mirror push 或普通 Git push 迁移时必须保留 commit 对象及其历史；
- GitHub Commit 页面中，本次升级记录应从 2026-08-21 开始，到 2026-10-08 结束；
- 发布前使用 `git log --format=fuller --reverse` 核对 Author Date、Committer Date、顺序和提交数量，并在 GitHub Commit 页面复核结果。

提交标题统一使用中文结构化格式，例如：

```text
功能（关联）：实现 CrashLoopBackOff 规则
测试（事件）：验证并发指纹幂等性
文档（性能）：发布首轮实测容量基线
```

### Milestone 1：可运行基础

- 修复迁移、配置和 Compose；
- 清理仓库；
- Dockerfile、Helm 和 CI；
- kind 一键安装；
- 注册现有 Controller。

### Milestone 2：可靠 Signal Pipeline

- Informer、Queue、Worker；
- Signal 标准化和幂等入库；
- 资源拓扑；
- 优雅关闭和指标。

### Milestone 3：Incident Core

- 三类关联规则；
- CorrelationPolicy；
- Incident FSM；
- PostgreSQL 事务与唯一约束；
- Timeline 和 CorrelationDecision。

### Milestone 4：外部集成

- Context API；
- Outbox；
- HMAC Webhook；
- 重试、Dead Letter 和重放。

### Milestone 5：质量验收

- Unit、Race、Benchmark；
- envtest；
- kind E2E；
- 容量回放；
- Demo、README 和发布文档。

## 21. 后续 Roadmap

- 多副本和 Leader Election；
- 多集群；
- 安全多租户；
- NodeNotReady 根因聚合；
- Service/EndpointSlice/Ingress 拓扑；
- 手工 Merge、Split 和 False Positive；
- MCP Server；
- Runbook、审批、Dry Run、自动修复和回滚；
- 历史规则回放与关联准确率评估；
- PostgreSQL HA 和灾难恢复。
