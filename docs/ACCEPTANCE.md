# 首版发布验收清单

本清单逐项映射 `SPEC.md` 第 19 节。`[x]` 只表示已有可定位证据；`[ ]` 是发布阻塞项。代码合入不等于发布通过。

## 阻塞清单

- [ ] 在全新环境仅按 [README](../readme.md) 和 [运维手册](OPERATIONS.md) 完成一次 Compose 或 Helm 安装。
- [ ] 在可用 Docker/kind 环境连续执行 `make demo`、`make demo-clean`、`make demo`，保存输出。
- [ ] GitHub Actions 的 [PR 质量门禁](../.github/workflows/ci.yml) 全绿。
- [ ] GitHub Actions 的 [kind E2E](../.github/workflows/kind-e2e.yml) 三类场景全绿并保存失败/成功证据。
- [x] 三节点 k3s 已验证 ImagePullBackOff、CrashLoopBackOff、FailedScheduling 的创建、去重、Context、签名 Webhook 与自动恢复；入口和覆盖项见 [阶段九环境验收](STAGE9_ENVIRONMENT_VALIDATION.md)。
- [x] 重复和并发 Signal 不产生重复 Incident；证据为领域并发测试、事务集成测试与 `go test -race ./...`。
- [x] 服务重启后 Incident、Timeline、Outbox 可重放且不重复；证据为阶段 10.8 可靠性测试。
- [x] Context API 使用 `incident-context/v1`，具有示例、ETag 与 [OpenAPI](openapi.yaml)。
- [x] Webhook 重试、HMAC、Retry-After、Dead Letter 与人工重放有自动化测试和演示接收器。
- [x] SPEC 指标、`/livez`、`/readyz`、OTLP Trace 与受保护诊断接口已实现并有单元测试。
- [x] [README](../readme.md) 明确单集群、单租户、单实例限制。
- [x] 合成回放覆盖 500 Nodes、10,000 Pods、100,000 Signals、2,000 active alerts 与 10,000 指纹，并发布 [首轮容量基线](performance/FIRST_CAPACITY_BASELINE.md)。
- [ ] 在 Linux/Kubernetes 环境补测数据库、CPU、慢查询和真实 Webhook 延迟；当前报告明确不宣称生产容量。
- [x] 已知限制记录于 README、架构文档、本清单和 [发布清单](RELEASE_MANIFEST.md)。

## 发布决定

状态：**不可创建首版 tag**。

原因：全新环境安装、GitHub CI、kind E2E 以及全链路容量指标仍缺可归档证据。三节点 k3s 验收证明核心业务闭环可用，但不替代上述明确门禁。全部未勾选项关闭后，重新运行 `hack/test-release-evidence.ps1 -RequireComplete`，记录镜像 digest，再创建 tag。
