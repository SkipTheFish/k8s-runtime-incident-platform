# 运维与故障排查

## 部署前检查

- 运行环境需要 OCI Runtime、PostgreSQL 15+；Kubernetes 部署还需 Helm、kubectl 和 Prometheus Operator CRD。
- 创建独立数据库与最小权限账号，DSN 通过 Secret 注入。
- 设置唯一且稳定的 `CLUSTER_ID`；启用 Kubernetes 后不要在重装时随意改变。
- Admission Webhook 证书必须包含 Service DNS SAN；Agent Webhook Secret 应是随机高熵值。

## 健康与观测

```bash
curl --fail http://127.0.0.1:8080/livez
curl --fail http://127.0.0.1:8080/readyz
curl --fail http://127.0.0.1:8080/metrics
```

`/livez` 失败表示进程或 HTTP 路径不可用。`/readyz` 非 200 时读取组件状态，依次检查迁移、PostgreSQL、Redis、Informer Cache Sync 与核心依赖。依赖故障不应令 `/livez` 失败。

诊断接口默认关闭。启用时使用内部地址和 Secret 注入 Token，不要把诊断端口加入公开 Service：

```bash
curl -H "Authorization: Bearer $DIAGNOSTICS_TOKEN" http://127.0.0.1:6060/debug/pprof/
```

## 常见故障

### Backend 不 Ready

检查脱敏后的配置/迁移日志、Pod 到 PostgreSQL/Redis 的连通性、迁移版本；启用 Kubernetes 时检查 ServiceAccount RBAC、必需 CRD、API Server 连通性和 Cache Sync。

### 没有生成 Incident

确认目标 Event 原因、`signals_received_total`/`signals_unmatched_total`/`signals_failed_total`、队列与 Dead Letter。检查根资源 UID 和 Cluster ID，不要只按名称判断。孤立 Pod 仅在 FailedScheduling 规则中合法。

### Webhook 未送达

检查 `outbox_pending`、投递指标、目标 allowlist 和网络可达性；对照事件 ID 验证接收端幂等及 HMAC 时间窗口。修复后使用人工重放接口，不直接改数据库。

### Controller 或 Admission Webhook 失败

确认 CRD 已安装、ClusterRole 对四类 observability 主资源具有 `get/list/watch/patch/update`，CA Bundle 与证书 SAN 匹配，并收集 Controller 日志、Webhook Configuration 和 Events。

## 升级、回滚与验收

升级前备份 PostgreSQL，在副本数据验证向前迁移；执行 `helm upgrade --install ... --wait --timeout 5m` 后检查 Ready 和三类冒烟场景。应用只能回滚到兼容当前 Schema 的镜像，迁移不自动 Down。

```bash
pwsh -NoProfile -File hack/test-k3s.ps1 -Context <context> -Namespace runtime-system
pwsh -NoProfile -File hack/e2e-kind.ps1
```

所有等待均有超时；失败证据位于 `deploy/tmp/e2e`，不得提交。
