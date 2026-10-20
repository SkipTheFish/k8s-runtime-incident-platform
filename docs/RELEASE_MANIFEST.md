# 首版发布清单

## 版本身份

| 项目 | 值 | 状态 |
| --- | --- | --- |
| 目标版本 | `v0.1.0` | 候选，尚未打 tag |
| Helm Chart | `runtime-incident-platform` `0.1.0` | 已固定 |
| Chart appVersion | `0.1.0` | 已固定 |
| 数据库迁移 | `00011_outbox_replay_audit.sql` | 最新版本 |
| Backend 镜像 | repository/tag 待发布流水线确定 | 阻塞 |
| Backend digest | `sha256:<pending>` | 阻塞 |
| Frontend 镜像 | repository/tag 待发布流水线确定 | 阻塞 |
| Frontend digest | `sha256:<pending>` | 阻塞 |
| Git commit | 以发布候选 HEAD 为准 | 打 tag 前记录 |

镜像必须按不可变 digest 部署和记录；不得用本地 image ID、可变 `latest` 或未经 Registry 拉取验证的值代替。

## 证据索引

- 规范与计划：[SPEC](SPEC.md)、[PLAN](PLAN.md)
- 自动化门禁：[PR CI](../.github/workflows/ci.yml)、[kind E2E](../.github/workflows/kind-e2e.yml)
- 三类场景：`hack/e2e-image-pull.ps1`、`hack/e2e-crash-loop.ps1`、`hack/e2e-failed-scheduling.ps1`
- 集群验收：[阶段九环境验证](STAGE9_ENVIRONMENT_VALIDATION.md)
- 性能：[首轮容量基线](performance/FIRST_CAPACITY_BASELINE.md)及同目录原始输出
- API：[OpenAPI](openapi.yaml)
- 安装与排障：[README](../readme.md)、[运维手册](OPERATIONS.md)
- 最终判断：[发布验收清单](ACCEPTANCE.md)

## 已知限制

单集群、单租户、单 Backend 实例；无 Leader Election、分布式队列、Exactly Once、安全多租户、多集群、自动根因分析/修复、Merge/Split、NodeNotReady E2E、扩展网络拓扑或 PostgreSQL HA。Outbox 是 At-Least-Once。当前容量报告不包含 PostgreSQL、网络和真实 Webhook 成本。

## 打 tag 条件

1. `docs/ACCEPTANCE.md` 没有未勾选项。
2. 两个 CI Workflow 对候选 SHA 全绿。
3. 发布镜像已推送并从 Registry 按 digest 拉取验证。
4. 本表写入候选 SHA、镜像名称与 digest。
5. `pwsh -NoProfile -File hack/test-release-evidence.ps1 -RequireComplete` 通过。

在满足条件前不创建 `v0.1.0` tag 或 GitHub Release。
