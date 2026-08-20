# BUG_REPRO: 结算错误链断链导致 500/吞错（bug-003）

## Bug 是什么
`internal/metering` 对不存在的结算单/计量点用 `fmt.Errorf`（`%v` 语义）直接抛错，丢失 `platform.ErrNotFound`/`ErrInvalid` sentinel，`errors.Is` 失效；HTTP handler 把错误吞掉返回 200 空草稿或把未分类错误映射成 500。

## 如何触发
查询/确认/对账不存在的结算单日期，或对未知计量点录读数、空日期结算。可运行：

```bash
go test ./internal/metering -run '^TestMeteringGetSettlementMissingR003A$' -count=1
go test ./internal/httpapi -run '^TestMeteringSettlementHTTPStatusR003D$' -count=1
```

## 真实错误信息
- `TestMeteringGetSettlementMissingR003A`：`errors.Is(err, platform.ErrNotFound)` 为 false；
- HTTP GET `/api/metering/settlements/<missing>` 返回 500（应 404）；POST confirm/reconcile 返回 200 空草稿。
