# 安全策略

## 报告漏洞

请勿在公开 Issue 披露未修复漏洞。优先使用 GitHub Private vulnerability reporting；若不可用，请联系仓库维护者建立私密沟通渠道。

报告应包含受影响版本、复现步骤、影响、脱敏日志和建议缓解方式。删除 Token、DSN、Cookie、Webhook Secret、证书私钥及客户数据。维护者会确认收到并协调修复与披露时间。

## 支持范围与 Secret

安全修复面向主分支及最新发布版本。当前首版是单集群、单租户、单实例系统，不宣称安全多租户隔离。公开部署必须置于可信认证反向代理之后，并限制 RBAC、数据库权限、网络出口与诊断端口。

Secret 只能通过环境变量、编排平台 Secret 或外部 Secret 管理器注入。禁止提交真实凭据、在错误中回显 DSN/Token、把 pprof 暴露到公开 Service，或绕过 Webhook 目标和签名校验。
