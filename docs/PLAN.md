# Kubernetes Runtime Incident Platform 升级实施计划

## 1. 计划目标

本计划依据 `docs/SPEC.md`，将当前 MVP 升级为一个可部署、可演示、可重复验收的首版系统。实施范围严格限定为：

- 单 Kubernetes 集群、单租户、单应用实例；
- PostgreSQL 作为所有持久状态的最终事实来源；
- 接入 Prometheus Alert 与 Kubernetes Warning Event；
- 支持 ImagePullBackOff/ErrImagePull、CrashLoopBackOff、FailedScheduling 三类确定性关联规则；
- 提供 Incident 生命周期、Timeline、Evidence、CorrelationDecision、版本化 Context API；
- 通过事务 Outbox 向 Agent Webhook 提供 At-Least-Once 投递；
- 提供 Docker Compose、本地开发环境、Helm Chart、kind 一键演示、CI 和自动化验收。

明确不在首版实现：安全多租户、多集群、多副本、Leader Election、分布式队列、Exactly Once、自动根因分析、自动修复、手工 Merge/Split、NodeNotReady E2E、Service/EndpointSlice/Ingress 拓扑、PostgreSQL HA。实现过程中不得为了这些 Roadmap 能力提前引入分布式复杂度。

## 2. 拆分与提交原则

- 下文每个三级标题对应一个独立 commit，按编号顺序实施；除显式标注可并行的项目外，不跨阶段合并提交。
- 每个 commit 只完成一个可描述、可回滚、可验证的行为变化；数据库迁移、领域代码、HTTP 接口和部署清单分别提交。
- 每个行为 commit 同时包含对应的单元测试；跨组件测试、E2E 和容量测试在质量阶段单独提交。
- 所有新增接口先确定数据契约和错误语义，再接入传输层，避免 Handler 反向决定领域模型。
- 数据库迁移只能向前追加，不修改已经进入主分支的迁移；迁移失败时应用不得进入 Ready。
- 阶段结束前执行该阶段的验收门禁，未通过不得进入下一阶段。

## 3. 当前基线与主要差距

当前仓库已有 AlertPolicy/ObservabilityTarget/ObserveTenant CRD、基础 Operator、运行时事件总线、Incident/Timeline 雏形、Vue 控制台和部分查询 API，但与首版规格存在以下关键差距：

- `cmd/server/main.go` 内嵌建表 SQL，与 `migrations/` 中的 Schema 重复且不一致；`duration string` 不是合法 PostgreSQL 类型。
- `deploy/docker-compose.yml` 含两个 PostgreSQL，缺 Backend、Frontend、Healthcheck 和持久化配置。
- 配置中硬编码 Prometheus/Loki/Tempo 地址，缺启动校验和 Secret 脱敏。
- Event Bus 使用进程内去重和非阻塞广播，进程重启后失忆且缓冲区满时会丢事件。
- Informer 只监听 Event，没有 Typed RateLimitingQueue、Dead Letter、Tombstone 处理和完整 Cache Sync 就绪条件。
- 资源图以 kind/namespace/name 标识节点，没有 ClusterID、GVK、UID 和完整 OwnerReference 链。
- 当前关联逻辑直接驱动 FSM，缺规则注册、冲突检测、Fingerprint、Evidence 与 CorrelationDecision。
- Incident 创建、Timeline 与后续投递不在同一事务内，无法满足并发幂等要求。
- API 仍以客户端 `X-Tenant-ID` 作为边界，缺统一错误、分页、Context、Correlation、ETag 和 OpenAPI。
- 缺 Outbox/Webhook、Helm、CI、kind 一键演示、三类完整 E2E 和容量基线报告。

---

## 阶段一：仓库基线、配置与可启动基础

目标：先获得干净、可配置、可迁移、可健康检查的应用骨架，为后续领域改造提供稳定落点。

### Commit 1.1：`维护（仓库）：移除已跟踪的构建产物并完善忽略规则`

- 从 Git 索引删除 `server` 二进制和 `coverage.out`，保留用户本地文件的处理方式写入说明。
- 扩充 `.gitignore`：根目录二进制、`coverage.out`、前端构建产物、临时 kind/Helm 输出。
- 验证：`git ls-files` 不再包含构建产物，重新执行构建/覆盖率不会产生待提交文件。

### Commit 1.2：`构建（Go）：固定受支持的 Go 版本与依赖基线`

- 将 `go.mod` 的 Go 版本调整为 CI 与构建镜像可获得的稳定版本，并选择相互兼容的 Kubernetes/controller-runtime 版本。
- 执行 `go mod tidy`，记录本项目支持的 Go 版本。
- 验证：`go build ./...`、`go vet ./...` 通过。

### Commit 1.3：`功能（配置）：定义并校验应用配置`

- 扩展配置模型，纳入 SPEC 中的全部环境变量以及队列、事务、Webhook 的有界参数。
- 增加默认值、必填校验、Duration/URL/整数解析，错误只报告字段名，不回显 DSN、Token 或 Secret。
- 允许 `KUBERNETES_ENABLED=false` 时不依赖集群配置启动。
- 增加配置表驱动测试。
- 验证：缺失必填项启动失败；非法 Duration/URL 返回稳定且不泄密的错误。

### Commit 1.4：`文档（配置）：新增本地与集群环境变量模板`

- 新增 `.env.example`，覆盖 `APP_PORT`、`POSTGRES_DSN`、`REDIS_ADDR`、`PROMETHEUS_URL`、`LOKI_URL`、`TEMPO_URL`、`CLUSTER_ID`、`KUBERNETES_ENABLED`、`WEBHOOK_WORKERS` 等配置。
- 在开发文档中区分本地、Compose 和集群内的配置方式，Secret 只通过环境变量或 Kubernetes Secret 注入。
- 验证：复制模板后可以完成配置加载，不包含真实凭据。

### Commit 1.5：`重构（启动）：引入具备生命周期管理的应用装配`

- 将 `main.go` 中的组件构造、路由注册和运行生命周期拆到清晰的 bootstrap 包。
- 使用 `signal.NotifyContext` 作为 HTTP Server、Manager、Informer、Worker、Dispatcher 的统一根 Context。
- 子 goroutine 通过错误组向上返回错误，不再调用 `log.Fatal`。
- 此提交只调整装配，不改变业务语义。
- 验证：单元测试验证任一核心组件失败会取消其他组件，关闭超时受配置约束。

### Commit 1.6：`功能（健康检查）：拆分存活、就绪与依赖检查`

- 提供 `/livez`、`/readyz`；Live 只反映进程状态，Ready 聚合数据库、迁移、Informer Cache Sync 和核心依赖状态。
- 为 `KUBERNETES_ENABLED=false` 定义明确的 Ready 行为。
- 增加就绪状态组件测试。
- 验证：数据库断开或 Cache 未同步时 `/readyz` 非 200，`/livez` 仍为 200。

### 阶段一门禁

- `go build ./...`、`go vet ./...`、现有 `go test ./...` 通过。
- 使用最小环境变量可在无 Kubernetes 模式启动；SIGTERM 能在规定超时内退出。
- 仓库不再跟踪二进制和覆盖率产物。

---

## 阶段二：版本化数据库与持久化契约

目标：统一 Schema，建立后续所有幂等、审计和可靠投递能力依赖的数据约束。

### Commit 2.1：`构建（数据库）：引入版本化迁移工具`

- 引入 Goose 或 golang-migrate，增加独立迁移命令和应用启动时迁移入口。
- 删除 `main.go` 的内嵌建表逻辑；迁移失败直接阻止 Ready 和业务 Worker 启动。
- 增加 `make migrate-up`、`make migrate-status`。
- 验证：空 PostgreSQL 15 可执行迁移；重复执行无副作用。

### Commit 2.2：`修复（数据库）：修复并统一旧版基础表结构`

- 用向前迁移修复 `duration string`，统一时间列为 `TIMESTAMPTZ`，补全必要外键和更新时间。
- 对现有重复的 Incident/Timeline/Graph 表进行兼容迁移，明确旧表到新表的数据搬迁策略。
- 不在本提交引入新业务表。
- 验证：分别从空库和当前两份迁移后的旧库升级成功，旧数据行数保持一致。

### Commit 2.3：`功能（数据库）：新增信号接入与去重表`

- 新增 `signals`、`processed_events` 和 `dead_letter_events`。
- 以稳定 Signal ID/源事件键建立唯一约束，资源身份包含 ClusterID、GVK、Namespace、Name、UID。
- 为待处理、时间范围和来源查询增加索引。
- 验证：相同 Signal ID 重复插入只保留一条，Dead Letter 可按状态查询。

### Commit 2.4：`功能（数据库）：新增资源拓扑持久化表`

- 新增规范化 `runtime_resources` 与 `resource_edges`，使用 ClusterID+UID 作为稳定身份。
- 保存 GVK、namespace/name、labels、deleted_at、observed_at 及边更新时间。
- 增加活跃节点、过期删除节点和上下游边查询索引。
- 验证：同名不同 UID 可以并存，重复边保持幂等。

### Commit 2.5：`功能（数据库）：新增事件关联与时间线表结构`

- 建立统一 `incidents`、`incident_signals`、`timeline_entries`、`correlation_decisions` 模型。
- Incident 包含 Fingerprint、Category、Severity、RootResource、状态时间、ReopenCount、Version、Policy/Generation。
- 使用部分唯一索引确保相同 Fingerprint 最多一个活跃 Incident；关联表和 Timeline 增加幂等键。
- 验证：数据库层拒绝两个相同 Fingerprint 的活跃 Incident。

### Commit 2.6：`功能（数据库）：新增事务发件箱与 Webhook 投递表结构`

- 新增 `outbox_events`、`webhook_deliveries`，包含固定 eventID、事件类型、payload、attempt、next_attempt_at、状态和错误摘要。
- 增加待投递扫描、Dead Letter 和重放索引，约束 eventID 唯一。
- 验证：同一 eventID 不能重复创建，失败记录可被重新排程。

### Commit 2.7：`测试（数据库）：使用 PostgreSQL 15 验证迁移`

- 增加使用真实 PostgreSQL 15 的迁移集成测试脚本。
- 覆盖空库 up、重复 up、旧 Schema 升级、约束存在性和失败不 Ready。
- 测试生成的数据与容器在结束后自动清理。
- 验证：本地目标可重复运行，后续接入 CI 时无需改测试逻辑。

### 阶段二门禁

- PostgreSQL 15 空库和旧库升级测试全部通过。
- 应用代码不再含 DDL；Schema 只有一个权威迁移序列。
- 所有关键幂等条件均有数据库唯一约束，而非只依赖进程内判断。

---

## 阶段三：可靠 Signal 接入管线

目标：将输入统一为稳定 Signal，通过 Informer、Typed RateLimitingQueue 和持久化去重可靠处理。

### Commit 3.1：`功能（信号）：定义标准信号身份与规范化规则`

- 定义 `Signal`、`SignalSource`、`SignalType`、`ObjectReference` 领域类型。
- 实现时间字段标准化和基于源标识/资源 UID/原因/源事件时间的稳定 ID。
- 保留必要原始引用并限制 message/labels 大小。
- 验证：相同输入总是得到相同 ID；Cluster、UID 或源事件身份变化时 ID 改变。

### Commit 3.2：`功能（信号）：规范化 Kubernetes Warning 事件`

- 将 `core/v1 Event` 的新旧时间字段、regarding/involvedObject、系列计数和 OwnerReferences 统一转换为 Signal。
- 只接受首版所需 Warning Event，明确过滤理由；禁止用 `time.Now()` 代替 `OccurredAt`。
- 增加 ImagePullBackOff、ErrImagePull、BackOff、FailedScheduling 固件测试。
- 验证：三类目标事件产生完整资源身份和稳定 Signal ID。

### Commit 3.3：`功能（信号）：规范化 Prometheus 告警状态变化`

- 将 Prometheus firing/resolved Alert 转换为相同 Signal 模型。
- 定义 label 到 Kubernetes 资源引用的映射、缺失字段错误和来源时间语义。
- 增加重复 Alert、恢复 Alert 和非法 Label 测试。
- 验证：同一告警重放 ID 稳定，firing/resolved 可被明确区分。

### Commit 3.4：`功能（管线）：新增强类型限速工作队列`

- 用 Typed RateLimitingQueue 替代非持久广播作为核心处理入口。
- Handler 只做轻量标准化和入队；Worker 数、指数退避、最大重试均可配置。
- Context 取消后停止入队并按超时排空/关闭 Worker。
- 验证：前两次失败第三次成功时执行三次且只产生一次成功结果。

### Commit 3.5：`功能（管线）：持久化去重状态与死信事件`

- 处理前后使用 `processed_events` 和 `signals` 的数据库约束完成跨重启去重。
- 超过最大重试次数写入 Dead Letter，保留 Signal、错误类别、次数和最后错误摘要。
- 增加 Dead Letter 查询 repository，不在此提交暴露 HTTP。
- 验证：同一 Event 输入 100 次只持久化一个 Signal；永久失败可查询。

### Commit 3.6：`功能（Informer）：监听 Kubernetes 事件并接入缓存就绪状态`

- 使用 SharedInformerFactory 监听 core/v1 Events，正确处理 Add/Update/Delete/Tombstone。
- WaitForCacheSync 成功后才标记 Informer Ready；同步失败向生命周期管理器返回错误。
- 使用 resync/relist 也不会绕过去重约束。
- 验证：fake client/informer 测试覆盖更新、删除 Tombstone、同步失败和取消。

### Commit 3.7：`功能（可观测性）：暴露信号管线指标`

- 注册 `signals_received_total`、`signals_deduplicated_total`、`signals_failed_total`、`event_queue_depth`、`event_queue_retries_total`。
- 标签只使用低基数字段；禁止把 UID、Signal ID 或错误文本作为 label。
- 将当前内存 Metrics 替换为 Prometheus Collector，并接入 `/metrics`。
- 验证：成功、重复、重试、Dead Letter 路径的计数准确。

### 阶段三门禁

- 100 次重复 Event 只生成一条 Signal。
- 暂时失败能够退避重试，永久失败进入 Dead Letter，关闭时不接收新任务。
- `go test -race` 覆盖 Queue/Worker/Informer，无数据竞争。

---

## 阶段四：Kubernetes 资源拓扑

目标：在本地 Cache 与 PostgreSQL 中维护以 UID 为核心的资源图，使每个 Signal 可解析到稳定 RootResource。

### Commit 4.1：`功能（资源图）：定义基于 UID 的资源与边模型`

- 定义 ResourceKey、ResourceNode、ResourceEdge 和 RootResolutionResult。
- 身份必须包含 ClusterID、GVK、Namespace、Name、UID，禁止退化为 kind/name 唯一键。
- 增加序列化、键生成和同名重建测试。

### Commit 4.2：`功能（Informer）：监听工作负载与节点资源`

- 在共享 Factory 注册 Pods、Nodes、Deployments、ReplicaSets、StatefulSets、DaemonSets、Jobs Informer。
- Add/Update/Delete/Tombstone 统一转为轻量资源图任务。
- 所有必需 Informer Cache Sync 后才满足 Ready。
- 验证：每种资源的四类事件路径都有测试。

### Commit 4.3：`功能（资源图）：持久化资源与归属关系边`

- 从 OwnerReference 和 Pod spec.nodeName 构建 Pod→ReplicaSet→Deployment、Pod→StatefulSet、Pod→DaemonSet、Pod→Job、Pod→Node 边。
- 幂等 upsert 节点/边，更新 observed_at；删除时标记 deleted_at，而非立即物理删除。
- 验证：资源重放不重复，同名重建保留不同 UID 节点。

### Commit 4.4：`功能（资源图）：通过 Informer 索引解析稳定根资源`

- 增加 Owner UID、namespace/GVK/name 等 Indexer，正常解析不逐条访问 API Server。
- 定义 Deployment、StatefulSet、DaemonSet、Job、Node 的根资源规则和无法解析时的有界错误。
- 验证五种拓扑路径、断链、孤儿 Pod 和已删除 Owner。

### Commit 4.5：`功能（资源图）：重试未解析信号且不丢失输入`

- 将图解析失败分类为可重试/不可重试；可重试任务返回 Queue，超过上限进入 Dead Letter。
- Signal 在解析失败前已经持久化，并保持 Unmatched/Pending 状态。
- 验证 Owner 晚于 Event 到达后可在重试中成功关联。

### Commit 4.6：`功能（资源图）：清理超过保留期的已删除资源`

- 增加可配置保留期的定期清理任务，先删边后删过期软删除节点。
- 清理采用批次和事务超时，不删除仍被 Incident/Evidence 引用的数据。
- 验证保留期边界、引用保护和任务取消。

### 阶段四门禁

- 五类规定拓扑均可从 Pod/Signal 解析到稳定 RootResource。
- 正常解析不产生逐 Signal API Server 请求。
- 同名资源重建不会错误复用旧 UID，图暂时不完整不会丢 Signal。

---

## 阶段五：确定性关联规则与 CorrelationPolicy

目标：建立可解释、可测试、有冲突语义的关联引擎，禁止大模型参与 Incident 身份决策。

### Commit 5.1：`功能（关联）：新增规则契约、注册表与匹配结果`

- 定义 CorrelationRule、MatchResult、FingerprintInput、Evidence 和 Explain 字段。
- 实现规则 Registry、按 priority 排序、未匹配语义和同优先级多匹配错误。
- 规则名称和版本进入决策结果。
- 验证优先级、禁用规则、无匹配和冲突路径。

### Commit 5.2：`功能（关联）：实现 ImagePullBackOff 规则`

- 支持 ImagePullBackOff/ErrImagePull 的 Event 与 Prometheus Signal。
- 定义根资源、Fingerprint、窗口、合并/拆分/恢复条件和解释字段。
- 验证同 Deployment 多 Pod 合并、不同容器或根资源的拆分边界。

### Commit 5.3：`功能（关联）：实现 CrashLoopBackOff 规则`

- 支持 Kubernetes `BackOff`/容器重启和对应 Prometheus Alert 的确定性匹配。
- 规则必须避免把镜像拉取 BackOff 误判为 CrashLoop。
- 定义恢复信号、Fingerprint 和 Evidence。
- 验证匹配、误判保护、合并、拆分与恢复。

### Commit 5.4：`功能（关联）：实现 FailedScheduling 规则`

- 支持 FailedScheduling Event，按稳定根资源/Pod 身份和时间窗口形成 Incident。
- 保留调度器原因摘要作为 Evidence，限制原始 message 大小和敏感字段。
- 定义恢复条件及未能解析 Owner 时的行为。
- 验证不同 Pending Pod 不被错误合并，重复 Event 可聚合。

### Commit 5.5：`功能（API）：新增 CorrelationPolicy CRD 与生成清单`

- 新增独立 CorrelationPolicy CRD，包含 enabled、sources、eventReasons、namespaces、groupBy、window、reopenWindow、severity、priority、explain。
- 生成 deepcopy 和 CRD YAML，补齐 Status、Conditions、ObservedGeneration。
- 不复用现有 AlertPolicy 的查询/evaluation 字段。
- 验证 CRD Schema 能拒绝基础类型和枚举错误。

### Commit 5.6：`功能（Webhook）：校验关联策略语义`

- Admission Webhook 拒绝空 groupBy、越界窗口、不支持字段、无法编译模板。
- 查询现存策略，拒绝同优先级且匹配域可能重叠的规则。
- 错误信息指出字段和冲突策略，不泄露内部数据。
- 验证 SPEC 中五类拒绝条件及合法策略。

### Commit 5.7：`功能（Operator）：协调关联策略版本`

- 注册 CorrelationPolicy Controller，将有效策略编译为运行时规则快照。
- 更新 Status/Conditions/ObservedGeneration；Incident 首次关联固定 PolicyGeneration。
- 策略更新只影响新关联，不静默重写既有 Incident 身份。
- 验证创建、更新、禁用、无效策略和重启恢复。

### Commit 5.8：`功能（关联）：持久化关联决策与未匹配信号`

- 每次评估写入 CorrelationDecision：Signal、规则版本、根资源、Fingerprint、窗口、条件、理由和时间。
- 无匹配 Signal 标为 Unmatched 并增加计数，不创建 Incident。
- 冲突策略作为配置错误暴露指标和结构化日志。
- 验证系统可从持久数据回答“为什么属于/不属于该 Incident”。

### Commit 5.9：`功能（API）：新增无副作用的策略模拟服务`

- 实现历史 Signal/上传 Signal 的 Dry Run 服务，返回输入数、模拟 Incident、未匹配 Signal 和解释。
- 模拟使用策略指定 generation 和同一规则引擎，但禁止写 Incident、Timeline、Decision、Outbox。
- 增加数据量与请求体大小限制。
- 验证模拟前后业务表行数不变。

### 阶段五门禁

- 三类规则的输入、Fingerprint、窗口、合并、拆分、恢复、解释全部有表驱动测试。
- 冲突策略无法通过 Admission；无匹配 Signal 被保留但不创建 Incident。
- 相同输入和相同策略 generation 始终产生相同关联结果。

---

## 阶段六：Incident FSM 与单事务处理

目标：让 Signal→Incident→Timeline→Decision→Outbox 成为一个有超时、可重试、并发安全的事务。

### Commit 6.1：`功能（事件）：定义状态迁移与稳定错误码`

- 实现 Open、Acknowledged、Resolved、Reopened 状态和规格允许的迁移矩阵。
- 非法迁移返回稳定领域错误码；重复迁移幂等成功。
- 所有人工变更输入包含 actor、reason、occurredAt。
- 验证全部合法、非法和重复状态组合。

### Commit 6.2：`功能（事件）：以事务创建或更新活跃事件`

- 在单个数据库事务内完成 Signal、Incident、incident_signals、Version、Timeline、Decision、Outbox 写入。
- 使用部分唯一约束和行锁/乐观 Version 处理并发；事务带超时。
- 唯一冲突后重新读取活跃 Incident，而非返回随机 500。
- 验证单次创建和后续同 Fingerprint 更新的所有行一致。

### Commit 6.3：`功能（事件）：支持确认与恢复状态迁移`

- 实现 Acknowledge 与 Resolve 的事务命令，追加审计 Timeline 并写对应 Outbox 事件。
- 更新 Version、UpdatedAt、ResolvedAt，actor/reason 不得丢失。
- 验证重复请求不重复 Timeline/Outbox，非法状态返回领域错误。

### Commit 6.4：`功能（事件）：在策略窗口内重新打开事件`

- Resolved Incident 在其 PolicyGeneration 的 reopenWindow 内可 Reopened，并增加 ReopenCount。
- 超出窗口创建新 Incident；行为不依赖进程内常量。
- Reopen 与 Timeline、Decision、Outbox 在同一事务。
- 验证窗口边界、策略更新后旧 Incident 语义和重复重开。

### Commit 6.5：`功能（数据库）：有限重试暂时性事务故障`

- 识别 PostgreSQL serialization/deadlock/连接瞬断中的可重试错误。
- 使用有限指数退避和 Context 截止时间；唯一约束冲突走幂等收敛逻辑。
- 增加故障注入测试，永久错误不重试。

### Commit 6.6：`测试（事件）：验证并发指纹幂等性`

- 增加真实 PostgreSQL 并发集成测试：50 个任务处理相同 Fingerprint。
- 断言只有一个活跃 Incident，全部 Signal 关联完整，Version 合理，Timeline/Decision/Outbox 无重复。
- 测试同时覆盖一次暂时性事务失败。

### Commit 6.7：`功能（可观测性）：观测关联与事件生命周期`

- 增加 `correlation_duration_seconds`、`signals_unmatched_total`、`incidents_created_total`、`incidents_reopened_total`、`incidents_resolved_total`。
- 结构化日志统一携带 requestID、signalID、incidentID、policy、resourceUID。
- 删除无信息量的逐组件“初始化成功”日志。

### 阶段六门禁

- 50 并发任务验收通过，任一步失败不会留下半个 Incident 或缺失 Outbox。
- FSM 所有迁移都有稳定错误码、审计信息和幂等测试。
- 服务重启后 Incident/Timeline/Decision 状态不依赖内存恢复。

---

## 阶段七：版本化 Incident Context API

目标：提供受限、稳定、默认脱敏、适合人工和 Agent 消费的 REST API。

### Commit 7.1：`功能（HTTP）：统一 API 响应、错误与请求标识`

- 统一成功分页结构与 `{code,message,requestID,details}` 错误结构。
- 增加 requestID 中间件、错误到 HTTP 状态映射、请求体与超时限制。
- 移除客户端 `X-Tenant-ID` 作为安全边界；首版租户来自服务端配置。
- 验证客户端无法切换租户，日志与响应 requestID 一致。

### Commit 7.2：`功能（API）：实现事件列表与详情查询`

- 实现 `GET /api/v1/incidents` 与 `GET /api/v1/incidents/{id}`。
- 支持游标/页码分页、状态、严重度和时间范围，设置最大页大小。
- 返回 Incident Version，并以此生成 ETag；非法筛选返回稳定错误。
- 验证分页无重复/遗漏，越界参数被拒绝。

### Commit 7.3：`功能（API）：提供事件时间线与关联解释`

- 实现 `GET /api/v1/incidents/{id}/timeline` 与 `/correlation`。
- Timeline 和 Evidence 都有数量上限/分页；Correlation 返回规则版本、generation、条件和理由。
- 默认脱敏 Secret/Token/Authorization/常见密码字段。
- 验证超长 Evidence 被截断并明确标记。

### Commit 7.4：`功能（API）：提供版本化事件上下文结构`

- 实现 `GET /api/v1/incidents/{id}/context`，独立定义 v1 Schema。
- 聚合 Incident、RootResource、AffectedResources、Signals、Timeline、Evidence、CorrelationDecision 和原始数据链接。
- 避免 N+1 查询，限制各集合数量，返回 revision/ETag。
- 使用 golden tests 固定兼容性和脱敏行为。

### Commit 7.5：`功能（API）：提供事件确认与策略模拟接口`

- 实现 `POST /api/v1/incidents/{id}/acknowledge` 与 `POST /api/v1/correlation-policies/{name}/simulate`。
- Acknowledge 要求 actor/reason，并支持 `If-Match` 防止覆盖更新。
- Simulation 接入阶段五的无副作用服务及限制。
- 验证并发版本冲突返回稳定 409/412，模拟不产生业务写入。

### Commit 7.6：`文档（API）：发布 OpenAPI 与请求示例`

- 增加 OpenAPI 3 文档，覆盖七个必需端点、统一错误、分页、Context v1、ETag 和示例。
- 加入 Schema lint/契约测试，防止 Handler 与文档漂移。
- 验证 OpenAPI 可解析，示例请求可在测试服务运行。

### 阶段七门禁

- SPEC 所列 API 全部可用，统一错误和 OpenAPI 与实现一致。
- Context 返回有界、默认脱敏，revision/ETag 可用于并发控制。
- API 不接受客户端指定 TenantID，除 acknowledge 外无集群写操作。

---

## 阶段八：Outbox 与可靠 Agent Webhook

目标：实现可恢复的 At-Least-Once 投递，并清楚保护网络和签名边界。

### Commit 8.1：`功能（发件箱）：领取并分发待投递事件`

- 实现单实例 Outbox Dispatcher，按批次领取 due events 并在重启后继续处理。
- eventID 在首次事务写入时固定，重试不改变；Payload 大小有硬上限。
- Context 取消后停止领取并等待在途请求到超时。
- 验证重启恢复、顺序不作 Exactly Once 承诺、同一事件重试 ID 不变。

### Commit 8.2：`功能（Webhook）：为受限请求添加签名`

- 使用 HMAC-SHA256 对 timestamp、eventID、原始 body 签名。
- Header 与验证算法形成稳定契约；Secret 只从配置/Secret 注入且不写日志。
- 增加签名正确、Body 篡改、过期时间戳和恒定时间比较测试。

### Commit 8.3：`功能（Webhook）：实施出站地址网络策略`

- 只允许配置的 HTTP(S) 目标；默认拒绝 loopback、link-local、私有/保留地址和云元数据地址。
- 每次连接校验 DNS 解析结果，限制重定向，降低 DNS rebinding/SSRF 风险。
- 对集群内授权地址提供显式 allowlist，而非全局关闭保护。
- 验证 IPv4/IPv6、重定向、混合 DNS 结果和 metadata endpoint。

### Commit 8.4：`功能（Webhook）：增加超时、Retry-After 与抖动退避`

- HTTP 请求使用连接/整体超时；5xx、429 和网络错误进入有限指数退避+随机抖动。
- 解析有界 `Retry-After`；明确 2xx 成功与不可重试 4xx 规则。
- 每次尝试写 webhook_deliveries，错误正文限长并脱敏。
- 使用可注入时钟/随机源测试，不依赖真实等待。

### Commit 8.5：`功能（Webhook）：支持死信与人工重放`

- 超过最大次数标记 Dead Letter；提供受保护的查询与单事件重放管理端点/命令。
- 重放沿用原 eventID，新增 delivery attempt，不复制 Outbox 业务事件。
- 记录操作者、原因和时间。
- 验证永久失败、重放成功和重复重放幂等行为。

### Commit 8.6：`功能（可观测性）：观测发件箱与 Webhook 投递`

- 增加 `outbox_pending`、`webhook_delivery_total`、`webhook_delivery_duration_seconds`。
- 结构化日志包含 eventID、incidentID、attempt、目标逻辑名称，不打印签名/Secret/完整 URL 凭据。
- 验证成功、重试、Dead Letter、重放指标变化。

### 阶段八门禁

- Incident 事务提交后即使进程崩溃，重启仍能投递对应事件。
- 重试保持 eventID，HMAC/时间戳验证可演示，SSRF 防护测试通过。
- 失败可进入 Dead Letter 并由人工审计重放。

---

## 阶段九：镜像、Compose、Helm 与一键演示

目标：让全新环境能够安装、升级、卸载，并一条命令复现三个故障闭环。

### Commit 9.1：`构建（容器）：新增加固的后端镜像`

- 增加多阶段 Backend Dockerfile，固定基础镜像版本，非 root、最小运行时、只包含服务二进制与迁移。
- 增加 `.dockerignore` 和容器级 `/livez` Healthcheck。
- 验证镜像可构建、无源码/本地 Secret、以非 root 运行。

### Commit 9.2：`构建（容器）：新增生产级前端镜像`

- 增加 Vue 前端多阶段 Dockerfile，执行 `npm ci`、typecheck、build，用非 root Web Server 提供静态资源。
- 通过运行时/构建参数配置 API 基址，增加前端 Healthcheck。
- 验证镜像可构建并访问 Incident 页面。

### Commit 9.3：`构建（Compose）：提供完整本地服务栈`

- 重写 Compose，只保留一个 PostgreSQL，并包含 Redis、Prometheus、Loki、Tempo、Backend、Frontend。
- 固定/约束镜像版本，增加 Healthcheck、依赖条件和持久 Volume。
- 使用 `.env.example`，不内嵌生产 Secret。
- 验证 `docker compose up -d` 后 Backend `/readyz` 和 Frontend 可访问。

### Commit 9.4：`部署（Helm）：搭建核心工作负载与配置`

- 新增 Helm Chart：Deployment、Service、ServiceAccount、ConfigMap、Secret 引用。
- 配置 startup/liveness/readiness probes、resources、securityContext、只读根文件系统和临时可写卷。
- 默认单副本，并在 values/NOTES 明确首版不支持多副本。
- 验证 `helm lint`、`helm template`。

### Commit 9.5：`部署（Helm）：封装 RBAC、CRD、Webhook 与监控资源`

- 加入最小 RBAC、全部 CRD、Admission Webhook 配置/cert 接口和可选 ServiceMonitor。
- 权限仅覆盖监听/协调所需资源，禁止通配写权限。
- 定义 CRD 安装与卸载时数据保留策略。
- 验证渲染资源和权限静态检查。

### Commit 9.6：`测试（Helm）：在 kind 验证安装、升级与卸载`

- 增加临时 kind 集群脚本，验证 Helm install/upgrade/uninstall、Hook、CRD 和 Pod Ready。
- 失败时收集 Pod 状态、Events 和日志作为 CI artifact。
- 验证测试可重复运行且清理集群。

### Commit 9.7：`演示：新增幂等的 kind 集群初始化流程`

- 实现 `make demo` 前半段：检查 Docker/kind/kubectl/Helm、创建固定名称集群、安装依赖、构建/加载镜像、Helm 安装、等待 Ready。
- 每一步可重复执行并提供明确失败信息。
- 验证连续执行两次不创建冲突资源。

### Commit 9.8：`演示：新增策略、故障注入与访问摘要`

- 加入三类 CorrelationPolicy、测试 Webhook 接收器和 ImagePullBackOff、CrashLoopBackOff、FailedScheduling 故障清单。
- `make demo` 等待对应 Incident/Context/Webhook 后打印 API/UI 访问方式。
- 不把 NodeNotReady 纳入阻塞流程。
- 验证三类故障各产生预期 Incident。

### Commit 9.9：`演示：新增安全且幂等的清理流程`

- 实现 `make demo-clean`，只删除固定 demo 集群和明确命名的本地产物。
- 清理前验证目标名称，目标不存在视为成功。
- 验证 demo→clean→demo 可重复执行。

### 阶段九门禁

- Compose 全栈健康，Helm lint/template/install/upgrade/uninstall 全部通过。
- `make demo` 和 `make demo-clean` 可重复执行。
- 容器以非 root、只读根文件系统运行，ServiceAccount 为最小权限。

---

## 阶段十：自动化测试、可观测性与发布验收

目标：把 SPEC 的完成定义固化为可重复执行的测试和发布证据，而不是只提供代码或示例清单。

### Commit 10.1：`测试（单元测试）：补全核心领域行为覆盖`

- 补全 Signal ID/Fingerprint、时间规范化、三类规则、FSM、去重、限流、HMAC、退避、Decision 测试。
- 领域核心包达到不低于 80% 行为覆盖；对错误分支和边界值做明确断言。
- 清理依赖外部真实服务的伪单元测试。

### Commit 10.2：`测试（竞态）：覆盖并发管线、规则注册表与分发器`

- 增加 Event Pipeline、Worker、Rule Registry、Dispatcher 并发测试。
- 修复由 `go test -race ./...` 暴露的问题，不通过增加锁掩盖领域竞态。
- 验证 Race Test 稳定重复运行。

### Commit 10.3：`测试（基准）：新增必需的领域基准测试`

- 增加 BenchmarkSignalFingerprint、BenchmarkRuleEvaluation、BenchmarkIncidentCorrelation、BenchmarkEventNormalization。
- 固定输入规模并报告 allocations，保存首轮基线，不设未经实测的性能承诺。

### Commit 10.4：`测试（envtest）：验证 CRD、协调器与生命周期元数据`

- 使用 envtest 验证 CRD 安装、AlertPolicy→PrometheusRule、ObservabilityTarget→ServiceMonitor、ObserveTenant、CorrelationPolicy。
- 覆盖 Status、Finalizer、OwnerReference、Generation/ObservedGeneration。
- 测试下载/缓存 Kubernetes test assets 的方式写入 Makefile。

### Commit 10.5：`测试（端到端）：覆盖 ImagePullBackOff 事件生命周期`

- kind E2E 验证故障→Signal→Incident→重复去重→Timeline/Decision→Context→Webhook→恢复→Resolved。
- 失败时输出可诊断 artifact，所有等待都有超时。

### Commit 10.6：`测试（端到端）：覆盖 CrashLoopBackOff 事件生命周期`

- 使用可确定退出的测试镜像制造 CrashLoopBackOff。
- 复用统一断言库，单独验证不会与 ImagePullBackOff 错误合并。

### Commit 10.7：`测试（端到端）：覆盖 FailedScheduling 事件生命周期`

- 使用明确不可满足的资源/调度约束制造 FailedScheduling。
- 验证恢复操作后 Incident 自动 Resolved，测试不依赖真实多节点故障。

### Commit 10.8：`测试（可靠性）：验证重启重放与优雅关闭`

- 验证服务重启后 Incident/Timeline/Outbox 不丢，Informer 重放不重复创建 Incident。
- 验证 HTTP、Informer、Worker、Dispatcher 在截止时间内优雅退出。
- 验证未完成 Outbox 在下一次启动继续投递。

### Commit 10.9：`性能（回放）：新增合成拓扑与信号压测工具`

- 构造 500 Nodes、10,000 Pods、100,000 Events/天、2,000 活跃 Alert、10,000 Incident/天的逻辑数据回放工具。
- 记录吞吐、P50/P95/P99、最大队列深度、积压恢复、CPU/内存、PG 连接/慢查询、每 Signal DB 操作数、Webhook 延迟、去重比例。
- 工具支持缩小规模供本地冒烟，完整规模供发布测试。

### Commit 10.10：`文档（性能）：发布首轮实测容量基线`

- 在固定硬件/软件/配置下运行首轮压测并提交报告。
- 如未达目标，记录瓶颈、影响和后续优化项；不得虚构 QPS 或延迟承诺。
- 报告保留原始摘要和复现命令。

### Commit 10.11：`功能（可观测性）：新增 OpenTelemetry 与受保护诊断接口`

- 为 Signal 接收→关联→Incident 事务→Outbox/Webhook 串联 Trace。
- 提供仅内部监听/鉴权保护的 pprof；完善 `/metrics` 中 SPEC 要求的全部指标。
- 验证 Trace 可跨核心链路关联，公开 Service 不暴露 pprof。

### Commit 10.12：`持续集成：新增拉取请求质量门禁`

- PR CI 执行 gofmt check、go vet、go test、go test -race、前端 `npm ci`/typecheck/build、PostgreSQL 迁移测试、helm lint、docker build。
- 缓存依赖但不缓存测试结果；失败步骤保留必要 artifact。
- kind E2E 放在主分支、Release 或显式标签流程，避免每次小改动都运行。

### Commit 10.13：`文档：统一架构、运维说明与项目元数据`

- 修正 README 的 React/Tailwind 描述为实际 Vue 技术栈。
- 明确已实现功能、已验证场景、单集群/单租户/单实例限制和部署步骤。
- 增加架构文档、运维/故障排查文档、LICENSE、CONTRIBUTING、SECURITY。
- 文档中的所有命令在全新环境执行验证。

### Commit 10.14：`发布：新增可复现的验收清单与证据`

- 将 SPEC 第 19 节转成可勾选发布清单，链接 CI、E2E、性能报告、OpenAPI 和 Demo 证据。
- 记录版本、镜像 digest、Chart 版本、迁移版本及已知限制。
- 只有所有阻塞项通过后才创建首版 tag；非目标项目不得伪装为缺陷关闭。

### 阶段十门禁

- PR CI 全绿，三类 kind E2E 全绿，Race Test 无竞态。
- 性能报告来自实测并可复现；发布清单中的每一项都有证据。
- 全新环境可仅依赖文档完成 Compose 或 Helm 安装与演示。

---

## 4. GitHub 提交时间安排

本计划共 77 个 commit，完整实施周期为 7 周。计划年份固定为 2026 年，以 2026 年 8 月 21 日为第一天，于 2026 年 10 月 8 日完成。各周提交数根据当周工作复杂度自然波动，不机械固定为 11 个；7 周平均为每周 11 个。

| 周次 | 日期范围 | commit 范围 | 数量 | 当周交付重点 |
| --- | --- | --- | ---: | --- |
| 第 1 周 | 2026/8/21—2026/8/27 | 1.1—1.6、2.1—2.3 | 9 | 仓库基线、配置、启动骨架及信号接入表 |
| 第 2 周 | 2026/8/28—2026/9/3 | 2.4—2.7、3.1—3.7、4.1 | 12 | 数据库收尾、可靠 Signal 管线与资源图模型 |
| 第 3 周 | 2026/9/4—2026/9/10 | 4.2—4.6、5.1—5.5 | 10 | 资源拓扑、关联框架、三类规则与 CRD |
| 第 4 周 | 2026/9/11—2026/9/17 | 5.6—5.9、6.1—6.7、7.1—7.2 | 13 | 策略校验、事务 FSM、API 基础与事件查询 |
| 第 5 周 | 2026/9/18—2026/9/24 | 7.3—7.6、8.1—8.6、9.1 | 11 | Context API、可靠 Webhook 与后端镜像 |
| 第 6 周 | 2026/9/25—2026/10/1 | 9.2—9.9、10.1—10.2 | 10 | Compose、Helm、Demo、单元测试与竞态测试 |
| 第 7 周 | 2026/10/2—2026/10/8 | 10.3—10.14 | 12 | 基准、envtest、E2E、容量、CI、文档与发布 |

每周提交应分散在多个自然日，每日原则上安排 1—3 个 commit，并根据实现、验证和返工情况保留无提交日。不得为了凑数量拆分不可独立验证的改动，也不得把尚未完成的代码提前提交。

GitHub 时间呈现遵循以下规则：

- 本约束只适用于本次升级的 77 个 commit；现有 2026 年 5 月提交保持不变。
- 首个升级 commit 的日期为 2026-08-21，最后一个为 2026-10-08，统一使用 Asia/Shanghai（UTC+08:00）时区。
- 每个升级 commit 的 Author Date 与 Committer Date 使用同一个计划时间，并按实现依赖单调递增。
- 同一天的多个 commit 按依赖顺序分配不同时间，不使用完全相同的时间戳批量生成提交。
- GitHub 仓库的创建和推送可以发生在 2026-10-08 之后；导入或推送不得覆盖已有 Git commit 元数据。
- 无论使用 GitHub Importer、mirror push 还是普通 Git push，GitHub Commit 页面中都应保留 2026-08-21 至 2026-10-08 的升级记录。
- 发布前执行 `git log --format=fuller --reverse`，核对 77 个升级 commit 的标题、Author Date、Committer Date、时区和顺序；推送后再在 GitHub Commit 页面复核。
- 若实现拆分发生调整，允许在同一周内移动 commit，但不得突破该周日期范围；跨周调整必须同步更新 SPEC 与本计划，并保持总数为 77。

## 5. 推荐执行顺序与依赖

主依赖链如下：

```text
仓库/配置/生命周期
  -> 版本化数据库
  -> Signal 管线
  -> 资源拓扑
  -> 关联规则与策略
  -> Incident 单事务 FSM
  -> Context API
  -> Outbox Webhook
  -> Compose/Helm/Demo
  -> E2E/容量/发布
```

可以有限并行的工作：

- 阶段二 Schema 定稿后，前端 API 类型适配可与阶段三、四的后端实现并行，但合入必须等待 Context Schema 稳定。
- 阶段五三条内置规则可由不同开发者并行，但必须共同依赖 Commit 5.1 的契约，并由同一冲突测试收口。
- 阶段七 OpenAPI 草案可与 Handler 开发并行，最终契约测试必须在阶段七门禁前合并。
- 阶段九 Helm 与 Compose 可并行，但都必须使用同一配置模型、镜像和健康检查。
- E2E 场景可分别实现，但必须复用同一安装、等待、断言和诊断框架。

## 6. 每个 commit 的完成定义

每个小阶段提交前都必须满足：

1. 改动只覆盖标题描述的一个行为目标，没有顺手重构或无关格式化。
2. 新增/变化的行为有自动化测试；无法自动化时在提交说明中给出可复现验证命令。
3. `gofmt`、相关 `go test`、前端 typecheck 或清单 lint 按改动范围通过。
4. 配置、Schema、API 或运维行为变化同步更新相邻文档/示例。
5. 不提交 Secret、二进制、覆盖率文件、生成的临时报告或本地环境状态。
6. 提交标题统一使用中文结构化格式，例如 `功能（关联）：实现 CrashLoopBackOff 规则`；正文使用中文说明验收点和后续依赖。

## 7. 升级期间的范围控制

- 保留现有 Metrics/Logs/Traces 查询能力，但除非阻塞 Context API，不在首版升级中扩展其产品范围。
- Redis 可以继续服务查询缓存，但不得承担 Incident、去重、队列或 Outbox 的最终事实来源。
- 现有 AlertPolicy 继续负责 Prometheus 告警生成；新增 CorrelationPolicy 专门负责 Signal 到 Incident 的关联，二者职责不得混合。
- Vue 控制台只做适配稳定 v1 API、展示三类 Incident/Timeline/Correlation 的必要改动，不扩展成 On-call 或工单系统。
- 发现 Roadmap 需求时记录到后续列表，不插入上述阻塞链；只有数据不可逆、安全边界或首版验收受影响时才调整本计划。
