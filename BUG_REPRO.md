# BUG_REPRO: 遥测历史快照串改 + data race（bug-001）

## Bug 是什么
`internal/scada` 的历史读取路径把内部环形缓冲切片直接返回给调用方（引用逃逸），且读路径不加锁遍历共享切片。并发写入读数时：
- `go test -race` 报 data race（`ringBuffer.push` 写 与 历史读取 并发）；
- 同一份已返回的历史快照会在后续写入后被串改（内容跟着新读数变）。

## 如何触发
对同一测点并发执行：多个 goroutine 写读数（`Service.Ingest`）+ 读取历史（`Service.History` / `Store.History`），并断言早先拿到的快照保持稳定。可运行：

```bash
go test -race ./internal/scada -run '^TestScadaSnapshotDetachedR001A$' -count=1
go test -race ./internal/scada -run '^TestScadaConcurrentHistoryStableR001C$' -count=1
```

## 真实错误信息
```text
==================
WARNING: DATA RACE
Write at 0x00c00015e288 by goroutine 12:
  gas-pipeline-operations-control-service/internal/scada.(*ringBuffer).push()
      internal/scada/store.go:32 +0x38c
  gas-pipeline-operations-control-service/internal/scada.(*Store).AppendReading()
      internal/scada/store.go:134 +0x320
  gas-pipeline-operations-control-service/internal/scada.(*Service).ingestOne()
      internal/scada/service.go:150 +0x4b0
Previous read at 0x00c00015e288 by goroutine 11:
  ... race001_test.go:101
```
