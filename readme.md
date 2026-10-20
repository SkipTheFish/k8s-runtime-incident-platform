# Kubernetes Runtime Incident Platform

将 Kubernetes Runtime Event 与 Prometheus Alert 规范化、去重并确定性关联为可审计 Incident 的首版平台。后端使用 Go、client-go、controller-runtime 和 PostgreSQL；控制台使用 Vue 3、TypeScript、Vite、Pinia 与 Naive UI。

## 已实现能力

- Kubernetes Event 与 Prometheus Alert 统一 Signal、持久化去重、限速重试及 Dead Letter。
- 基于 Cluster ID、GVK、UID 与 OwnerReference 的资源拓扑。
- ImagePullBackOff/ErrImagePull、CrashLoopBackOff、FailedScheduling 三类确定性规则与恢复闭环。
- CorrelationPolicy、AlertPolicy、ObservabilityTarget、ObserveTenant CRD 与 Admission Webhook。
- Incident FSM、Timeline、Evidence、CorrelationDecision、版本化 Context API 和 ETag。
- 事务 Outbox、HMAC Webhook、Retry-After、人工重放与网络目标限制。
- `/livez`、`/readyz`、Prometheus 指标、可选 OTLP Trace 与受保护 pprof。
- Docker Compose、Helm、kind/k3s 演示和 Vue 运维控制台。

三类场景已在三节点 k3s 环境完成实际闭环验收；自动化 kind E2E 会在主分支、版本标签、手工触发或带 `e2e` 标签的拉取请求中运行。当前本地性能证据见 [首轮容量基线](docs/performance/FIRST_CAPACITY_BASELINE.md)。

## 架构

```text
Kubernetes Event / Prometheus Alert
                │
       normalize + durable dedupe
                │
      typed rate-limiting queue
                │
       resource graph resolution
                │
    deterministic correlation rules
                │
 Incident + Timeline + Decision + Outbox
                │                         │
       Context/REST API              signed Webhook
                │
          Vue console
```

组件、一致性和生命周期设计见 [架构文档](docs/ARCHITECTURE.md)，API 契约见 [OpenAPI](docs/openapi.yaml)。

## 快速开始

前置工具：Go 版本以 `go.mod` 为准，Node.js 24、Docker Compose v2。复制 `.env.example` 后按环境修改，不要提交真实 Secret。

```bash
docker compose -f deploy/docker-compose.yml up -d --build
curl --fail http://127.0.0.1:8080/livez
curl --fail http://127.0.0.1:8080/readyz
```

控制台默认地址为 `http://127.0.0.1:8081`。停止并保留数据：

```bash
docker compose -f deploy/docker-compose.yml down
```

### Kubernetes / Helm

集群必须已有 Prometheus Operator 的 `PrometheusRule` 和 `ServiceMonitor` CRD，并准备 PostgreSQL DSN Secret：

```bash
kubectl create namespace runtime-system
kubectl -n runtime-system create secret generic runtime-incident-platform-database \
  --from-literal=dsn='postgres://USER:PASSWORD@HOST:5432/DB?sslmode=require'
helm upgrade --install krip deploy/helm/runtime-incident-platform \
  --namespace runtime-system --set config.clusterId=my-cluster \
  --set config.kubernetesEnabled=true --wait --timeout 5m
```

生产式安装还应启用 Admission Webhook TLS 和签名 Webhook Secret；完整准备、验证及回滚命令见 [运维手册](docs/OPERATIONS.md)。三节点 k3s 验收入口见 [阶段九环境验证](docs/STAGE9_ENVIRONMENT_VALIDATION.md)。

## 开发与验证

```bash
go build ./...
go vet ./...
go test ./...
go test -race ./...
pwsh -NoProfile -File hack/test-helm-static.ps1
cd runtime-console
npm ci
npm run typecheck
npm run build
```

真实 PostgreSQL 迁移测试需要运行中的 Docker daemon：`pwsh -NoProfile -File hack/test-migrations.ps1`。kind 全链路验收：`pwsh -NoProfile -File hack/e2e-kind.ps1`。

## 明确限制

首版仅支持单集群、单租户、单 Backend 实例。它不提供安全多租户隔离、多集群汇聚、Leader Election、分布式队列、Exactly Once、自动根因分析、自动修复、Incident Merge/Split、NodeNotReady E2E、扩展网络拓扑或 PostgreSQL HA。Webhook 为 At-Least-Once，接收端必须按事件 ID 幂等。

项目状态与“生产就绪”不是同义词：发布前仍须完成目标环境的 CI、kind/k3s E2E、镜像摘要记录和发布清单。安全问题请按 [SECURITY.md](SECURITY.md) 私下报告；参与开发请阅读 [CONTRIBUTING.md](CONTRIBUTING.md)。

## License

本项目采用 [Apache License 2.0](LICENSE)。
