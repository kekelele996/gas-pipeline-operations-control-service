# BUG_REPRO: 缺省配置下限值写入 nil map panic（bug-004）

## Bug 是什么
缺省配置（`config.Default()`）下 `LimitProvider` 为 typed-nil，`SegmentLimits`/`StationLimits` 返回 nil map；`network` 的多个写入点（段/站场/阀门/压缩机限值、批量录入）直接向 nil map 赋值；同时 `network.NewStore` 未初始化内部 `limits` map，`Put*Limit` 也写入 nil map。

## 如何触发
使用默认配置启动后，对任意管段/站场/阀门/压缩机调用限值录入（HTTP `POST /api/segments/{id}/limits` 或 service 调用）。可运行：

```bash
go test ./internal/network -run '^TestNetworkRecordLimitNoPanicR004B$' -count=1
go test ./internal/network -run '^TestNetworkStoreLimitInitR004D$' -count=1
```

## 真实错误信息
```text
panic: assignment to entry in nil map [recovered, repanicked]
gas-pipeline-operations-control-service/internal/network.(*Service).RecordOperatingLimit(...)
	internal/network/service.go:530 +0xa4
```
