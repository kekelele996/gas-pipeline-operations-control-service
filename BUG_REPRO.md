# BUG_REPRO: 申报/合同容量错误链断链（bug-009）

## Bug 是什么
`internal/contract`/`internal/nomination` 用 `fmt.Errorf`（%v 语义）包装容量不足/状态/不存在错误，`errors.Is(ErrExhausted/ErrState/ErrNotFound)` 失效，HTTP 超容量提交回 500 而非 409。

## 如何触发
提交超容量申报、对非生效合同预留、提交/确认/执行非法状态申报、查/释放不存在合同。可运行：

```bash
go test ./internal/contract -run '^TestContractReserveExhaustedChainR009A$' -count=1
go test ./internal/nomination -run '^TestNominationSubmitOverCapacityR009D$' -count=1
```

## 真实错误信息
- `TestContractReserveExhaustedChainR009A`：`errors.Is(err, platform.ErrExhausted)` 为 false；
- HTTP 超容量提交返回 500（应 409）。
