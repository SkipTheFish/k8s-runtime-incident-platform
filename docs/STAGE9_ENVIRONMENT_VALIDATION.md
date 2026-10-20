# 阶段九环境验收清单

以下实现与静态校验已纳入仓库，但必须在具备 Docker 引擎、kind、kubectl、Helm、OpenSSL 和镜像拉取能力的环境中完成最终验收。

## 自动化环境待执行

1. `docker build -t runtime-incident-platform-backend:local .`，确认非 root 运行和 `/livez` 健康检查。
2. `docker build -t runtime-incident-platform-frontend:local runtime-console`，确认静态资源、SPA 回退和 `/api` 代理。
3. `docker compose --env-file .env.example -f deploy/docker-compose.yml up --build`，等待完整服务栈健康并访问前后端。
4. `pwsh -NoProfile -File hack/test-helm-kind.ps1`，验证 Helm install、upgrade、uninstall、Pod Ready、诊断收集、CRD 保留和重复执行。
5. 连续执行两次 `make demo`，确认初始化幂等；验证三类故障均形成 Incident、时间线与签名 Webhook 投递。
6. 连续执行两次 `make demo-clean`，确认只删除 `krip-demo` 集群和 `deploy/tmp/demo` 生成物。

## 三节点 k3s 等价验收

实际部署可使用三节点 k3s 代替 Windows kind。安装时必须启用 `config.kubernetesEnabled=true` 和 Admission Webhook，并确保集群已提供 Prometheus Operator 的 `PrometheusRule`、`ServiceMonitor` CRD。部署完成后执行：

```powershell
pwsh -NoProfile -File hack/test-k3s.ps1 -Context <kube-context> -Namespace runtime-system -Release krip
```

该脚本检查 Pod Ready、`/livez`、`/readyz`、主资源/status RBAC、三类 Incident、Context 完整性和带签名头的 Webhook 成功投递。

2026-10-16 已在三节点 k3s 集群按上述入口完成实际验收，并在修复 FailedScheduling 孤立 Pod 根资源解析后复测通过。已观察到三类 Incident、完整 Context/Decision/Timeline、签名 Webhook 以及恢复后的 Resolved 状态。

该结果关闭阶段九真实业务闭环门禁，但不等价于 GitHub Actions 的 kind 流水线证据。上方 Docker Compose、kind 幂等安装/清理仍纳入阶段十发布清单，未完成前不声明生产就绪。
