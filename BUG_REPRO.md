# BUG_REPRO: 事件升级态状态机错位（bug-008）

## Bug 是什么
`internal/incident` 新增升级中间态后：转换表漏 `escalated→closed` 边；`Escalate` 返回旧状态；开放列表/许可冲突检查漏算升级态；无处置项也可关闭。

## 如何触发
把处置中事件升级后尝试关闭、查看开放列表、走许可冲突检查；或直接关闭无处置项事件。可运行：

```bash
go test ./internal/incident -run '^TestIncidentEscalateThenCloseR008A$' -count=1
go test ./internal/incident -run '^TestIncidentEscalatedCountedOpenR008B$' -count=1
```

## 真实错误信息
- 升级后 Close 报 `illegal state transition: cannot transition "escalated" -> "closed"`；
- 开放列表/冲突检查缺失升级态事件。
