# BUG_REPRO: 申报挂起态状态机/容量统计错位（bug-010）

## Bug 是什么
`internal/nomination` 新增挂起中间态后：转换表漏 `held→confirmed` 边；`ConfirmHeld` 返回旧状态；容量统计与合同日期列表漏算挂起态。

## 如何触发
把已提交申报挂起后确认、查看容量占用、查看合同日期列表。可运行：

```bash
go test ./internal/nomination -run '^TestNominationHeldThenConfirmR010A$' -count=1
go test ./internal/nomination -run '^TestNominationHeldCountsCapacityR010B$' -count=1
```

## 真实错误信息
- `TestNominationHeldThenConfirmR010A`：ConfirmHeld 报 `cannot transition "held" -> "confirmed"`；
- `TestNominationHeldCountsCapacityR010B`：挂起申报未计入已用容量。
