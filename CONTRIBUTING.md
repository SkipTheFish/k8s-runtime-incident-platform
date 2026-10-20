# 贡献指南

提交前请阅读 `docs/SPEC.md`、`docs/PLAN.md` 和相关架构文档。一个提交只解决一个可描述问题；数据库 Schema 仅追加迁移，不改写已发布迁移。新行为必须有测试，配置/API/运维变化同步更新文档。

```bash
go vet ./...
go test ./...
go test -race ./...
pwsh -NoProfile -File hack/test-helm-static.ps1
cd runtime-console && npm ci && npm run typecheck && npm run build
```

涉及迁移、镜像或 Kubernetes 生命周期的变更还应运行对应 Docker/kind 测试。拉取请求应说明行为变化、验证证据、兼容性和已知限制。禁止提交 Secret、二进制、覆盖率、前端 `dist` 或 `deploy/tmp`，也不得删除测试或降低断言来绕过失败。

Go 文件必须通过 gofmt；错误向上返回且不记录凭据。异步处理必须支持 Context 取消、有限重试和可诊断失败。破坏性契约变更需要显式版本策略。
