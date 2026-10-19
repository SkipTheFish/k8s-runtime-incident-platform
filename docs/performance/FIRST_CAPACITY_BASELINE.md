# 首轮容量基线

本报告记录阶段 10.9 合成领域回放工具的首次可复现实测。它衡量 Signal 归一化与确定性规则评估，不包含网络、PostgreSQL 事务、Kubernetes API、Informer 或真实 Webhook，因此不能直接解释为生产 QPS 或 SLO。

## 固定环境

- 时间：2026-10-16（Asia/Shanghai）
- 操作系统/架构：Windows amd64
- Go：1.26.9
- CPU：13th Gen Intel(R) Core(TM) i5-13500HX
- Worker：8
- Profile：`full`
- 逻辑规模：500 Nodes、10,000 Pods、100,000 Signals、2,000 active alerts、10,000 Incident 指纹基数

## 复现命令

```powershell
go test -race ./cmd/replay-benchmark
go run ./cmd/replay-benchmark `
  -profile full `
  -workers 8 `
  -output deploy/tmp/replay-full.json
```

原始、未修饰的输出保存在 [replay-full-windows-amd64-2026-10-16.json](./replay-full-windows-amd64-2026-10-16.json)。微基准的独立首轮结果保存在 [domain-benchmark-baseline.txt](./domain-benchmark-baseline.txt)。

## 实测结果

| 指标 | 结果 |
| --- | ---: |
| Signal 数量 | 100,000 |
| 唯一 Incident 指纹 | 10,000 |
| 重复 Signal | 90,000（90%） |
| 领域回放吞吐 | 710,666.90 signals/s |
| 端到端队列延迟 P50 | 5,303 µs |
| 端到端队列延迟 P95 | 8,031 µs |
| 端到端队列延迟 P99 | 10,687 µs |
| 最大队列深度 | 4,096 |
| 生产结束后积压清空 | 4 ms |
| 总墙钟时间 | 140 ms |
| Go 总分配量 | 219,922,344 bytes |
| 采样堆内存峰值 | 27,308,936 bytes |

这里的延迟从进入进程内队列开始计时，包含排队和规则评估，但不包含数据库或网络。吞吐同样只表示本机内存回放上限。

## 瓶颈与结论

生产速度曾填满容量为 4,096 的工作队列，P99 主要反映排队等待。100,000 个 Signal 产生约 220 MB 累计 Go 分配，说明后续优化应优先关注每次规则评估的临时 map、JSON fingerprint 和字符串分配。当前 8 Worker 在该固定环境中能在生产结束后 4 ms 清空剩余积压。

本轮没有进行优化前后对比；这是首次基线，而不是性能承诺。

## 尚未测量

以下 SPEC 指标不能由合成工具诚实获得，必须在 Linux/Kubernetes 发布环境运行全链路压测后补充：

- 容器 CPU 峰值；
- PostgreSQL 连接数、慢查询和每 Signal 平均数据库操作数；
- Outbox 到真实接收端的 Webhook 投递延迟；
- Kubernetes API/Informer、网络和磁盘对吞吐及延迟的影响；
- 系统积压下的数据库恢复时间。

在这些结果补齐前，本报告仅证明领域回放工具可承载指定逻辑规模，不应据此声明生产容量门禁已通过。
