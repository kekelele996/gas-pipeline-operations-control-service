# BUG_REPRO: 通知批量推送滞留/取消泄漏（bug-002）

## Bug 是什么
`internal/notify` 的批量推送（`Service.PushBatch`）用 worker 池并发投递：
- `wg.Add(1)` 写在 worker goroutine 内部，与 `wg.Wait()` 并发（`-race` 报 WaitGroup 误用）；
- 取消路径提前返回且不关闭任务 channel，worker 永远阻塞在 `for range` 上，goroutine 泄漏；
- 投递尝试耗尽后漏更新终态，通知一直停在排队/重试里；
- `Due` 忽略 `NextRetryAt`，重试中的通知在退避窗口结束前被反复尝试。

## 如何触发
批量入队通知后执行 `PushBatch`（可注入取消、`FailureRate=1` 模拟全失败），随后检查：
- 每条通知是否到达 sent/failed 终态；
- 取消后 goroutine 数是否回落；
- 重试中的通知是否在退避窗口内被再次尝试。可运行：

```bash
go test -race ./internal/notify -run '^TestNotifyBatchAllDeliveredR002A$' -count=1
go test -race ./internal/notify -run '^TestNotifyBatchCancelNoLeakR002B$' -count=1
go test -race ./internal/notify -run '^TestNotifyBatchWorkerRaceR002C$' -count=1
```

## 真实错误信息
- `WARNING: DATA RACE`（`WaitGroup.Add` 与 `Wait` 并发）；
- `TestNotifyBatchAllDeliveredR002A`：`notification ... still in queued after attempts exhausted`；
- `TestNotifyBatchCancelNoLeakR002B`：`goroutines leaked: before=N after=N+4`。
