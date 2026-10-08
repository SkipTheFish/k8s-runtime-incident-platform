# 阶段九环境验收清单

以下实现与静态校验已纳入仓库，但必须在具备 Docker 引擎、kind、kubectl、Helm、OpenSSL 和镜像拉取能力的环境中完成最终验收。

## 待执行

1. `docker build -t runtime-incident-platform-backend:local .`，确认非 root 运行和 `/livez` 健康检查。
2. `docker build -t runtime-incident-platform-frontend:local runtime-console`，确认静态资源、SPA 回退和 `/api` 代理。
3. `docker compose --env-file .env.example -f deploy/docker-compose.yml up --build`，等待完整服务栈健康并访问前后端。
4. `pwsh -NoProfile -File hack/test-helm-kind.ps1`，验证 Helm install、upgrade、uninstall、Pod Ready、诊断收集、CRD 保留和重复执行。
5. 连续执行两次 `make demo`，确认初始化幂等；验证三类故障均形成 Incident、时间线与签名 Webhook 投递。
6. 连续执行两次 `make demo-clean`，确认只删除 `krip-demo` 集群和 `deploy/tmp/demo` 生成物。

上述检查未执行前，阶段九的“真实环境门禁”状态为待验证，不应声明生产就绪。
