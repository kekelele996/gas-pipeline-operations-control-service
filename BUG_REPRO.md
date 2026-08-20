# BUG_REPRO: 许可审批 defer 吞错 + 冲突检查过严（bug-007）

## Bug 是什么
`internal/permit` 的 Approve/Start/Complete/Cancel 用命名返回值 + defer 无条件把主错误覆盖为审计结果（nil），冲突/非法状态被吞掉返回成功；HTTP 审批 handler 也吞错返回 200；store 的 OpenForSegment 忽略窗口重叠，非重叠许可也被判冲突。

## 如何触发
审批与在途许可重叠的许可、重复审批、对非待审批许可开工/完工/取消；或审批同段非重叠窗口许可。可运行：

```bash
go test ./internal/permit -run '^TestPermitApproveConflictNotSwallowedR007A$' -count=1
go test ./internal/permit -run '^TestPermitNonOverlappingApproveR007G$' -count=1
```

## 真实错误信息
- 重叠审批返回 200 且状态仍为 pending（无冲突报错）；
- `TestPermitNonOverlappingApproveR007G`：非重叠许可审批失败。
