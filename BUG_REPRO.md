# BUG_REPRO: 审计日志保留/切片别名（bug-006）

## Bug 是什么
`internal/audit` 保留淘汰从尾部截断（丢新留旧）；`All`/`Query`/`Recent` 直接返回内部切片共享底层数组，调用方修改会污染日志。

## 如何触发
写入超过保留上限的审计条目后查询；或获取 All/Query/Recent 结果后修改返回切片再查询。可运行：

```bash
go test ./internal/audit -run '^TestAuditRetentionKeepsNewestR006A$' -count=1
go test ./internal/audit -run '^TestAuditQueryIsolatedR006B$' -count=1
```

## 真实错误信息
- `TestAuditRetentionKeepsNewestR006A`：最旧条目仍留在日志里；
- `TestAuditQueryIsolatedR006B`：修改查询结果后日志条目被污染。
