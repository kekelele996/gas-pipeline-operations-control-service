# BUG_REPRO: 调度指令 ctx 丢弃/取消不传播（bug-005）

## Bug 是什么
`internal/dispatch` 在设备操作路径丢弃请求 ctx：`resolveSegment`/`checkActionAllowed`/`apply` 都改成 `context.Background()`，`Issue`/`Execute` 无 `ctx.Err()` 快速失败。取消/超时的请求仍会继续下发设备动作，前一个请求的状态还可能影响后续。

## 如何触发
用已取消或已超时的 ctx 调用 Create/Issue/Execute（或 HTTP 执行携带取消请求 ctx）。可运行：

```bash
go test ./internal/dispatch -run '^TestDispatchCreateCancelR005C$' -count=1
go test ./internal/dispatch -run '^TestDispatchExecuteCancelR005A$' -count=1
```

## 真实错误信息
- `TestDispatchCreateCancelR005C`：取消 ctx 下 Create 仍成功创建指令；
- `TestDispatchExecuteCancelR005A`：ctx 超时后设备操作仍完成并把指令标成 executed。
