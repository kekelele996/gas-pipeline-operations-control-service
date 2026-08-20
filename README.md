# 天然气长输管网运行控制服务

一个用 Go 标准库实现的天然气长输管网运行控制服务（B2B），覆盖管网拓扑档案、SCADA 遥测采集与告警、贸易计量结算、托运商合同与容量分配、检修作业许可、运行调度指令、事件/事故管理、泄漏检测分析、审计日志与通知推送。所有数据保存在内存 store + 互斥锁中，无任何第三方依赖。

## 运行

```bash
go run ./cmd/server
```

服务默认监听 `:18090`，启动后：

- `GET http://localhost:18090/health` → `{"status":"ok"}`（健康检查）
- `GET http://localhost:18090/api/summary` → 运维看板聚合数据（JSON）
- `http://localhost:18090/` → 运维看板页面（静态 HTML/CSS/JS）

首次启动会自动 seed 演示数据（1 条主干、5 座站场、2 台压缩机、4 个阀门、16 个测点、2 个计量点、1 份托运商合同）。

## 测试

```bash
go test ./...           # 全部测试
go test -race ./...     # 含竞态检测（关键并发包：scada / nomination / metering）
go build ./...          # 构建
```

## 配置

所有配置项有默认值，可用环境变量覆盖：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `GPC_HTTP_ADDR` | `:18090` | 监听地址 |
| `GPC_SITE_CODE` | `GPL-CC-01` | 站点代码 |
| `GPC_SITE_NAME` | `Northern Gas Pipeline Control Center` | 站点名称 |
| `GPC_SCADA_BUFFER` | `200` | 每测点环形缓冲容量 |
| `GPC_NOTIFY_FAIL_RATE` | `0.0` | 通知模拟失败率（0-1） |
| `GPC_AUDIT_RETENTION` | `5000` | 审计日志保留条数 |
| `GPC_LEAK_DROP` | `0.15` | 泄漏压降率阈值（MPa/min） |
| `GPC_LEAK_IMBALANCE` | `0.05` | 泄漏平衡差阈值（分数） |

完整列表见 `internal/config/config.go`。

## 目录结构

```
cmd/server/main.go              入口：加载配置、构建依赖、注册路由、seed 演示数据
internal/config/                配置加载（端口、阈值），带默认值
internal/platform/              公共工具：错误类型、ID 生成、时钟、JSON、校验、状态机
internal/network/               管网拓扑：Segment/Station/Compressor/Valve/Point（含状态机）
internal/scada/                 SCADA 遥测：环形缓冲、坏值过滤、阈值/变化率告警
internal/metering/              贸易计量：温度折算、日累计、结算单状态机
internal/contract/              托运商合同：容量预留/释放（原子，按合同加锁）
internal/nomination/             申报单：提交/确认/执行/取消（与合同容量联动）
internal/permit/                检修作业许可：审批（冲突校验）/开工/完工/过期扫描
internal/dispatch/              运行调度指令：创建/下发/执行（联动设备状态）/撤销
internal/incident/              事件/事故：上报/确认/处置项/关闭（联动告警恢复）
internal/leakdetect/            泄漏检测：压降率回归 + 上下游平衡差分析
internal/audit/                 审计日志：追加写、按 target/action 查询
internal/notify/                通知推送：入队/批量推送/失败重试（指数退避）
internal/httpapi/               HTTP 路由、中间件（日志/recover/CORS）、各领域 handler
internal/seed/                  演示数据
web/                            运维看板（index.html / app.js / style.css）
runtime_smoke.json              运行冒烟配置
```

## 架构要点

- **纯标准库**：仅用 `net/http`、`encoding/json`、`sync`、`context`、`time` 等，无第三方依赖。
- **内存 store + 互斥锁**：每个领域包的 store 自带锁；读方法返回深拷贝，不泄漏内部引用。
- **显式状态机**：所有生命周期（压缩机、阀门、申报、合同、许可、调度、事件、计量结算、告警、通知）用 `map[State][]State` 转换表 + `CanTransition`/`MustTransition`，非法转换返回 `ErrState`。
- **错误映射**：`platform.Error` 携带分类（`ErrNotFound`/`ErrConflict`/`ErrInvalid`/`ErrState`），HTTP 层统一映射到 404/409/400/500。
- **可注入时钟**：`platform.Clock` 接口便于测试确定性运行（`FakeClock`）。
- **并发安全**：SCADA 摄入、合同容量预留、计量日累计均按包级/合同级锁串行化关键写路径，`go test -race` 全绿。

## 关键 API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | 健康检查 |
| GET | `/api/summary` | 运维看板聚合 |
| GET | `/api/segments` | 管段列表 |
| GET | `/api/segments/{id}/devices` | 管段下设备 |
| POST | `/api/compressors/{id}/state` | 切换压缩机状态 |
| POST | `/api/telemetry` | 批量写入遥测（`{readings:[...]}`） |
| GET | `/api/alarms?active=true` | 告警查询 |
| POST | `/api/metering/readings` | 记录计量读数 |
| POST | `/api/metering/settle` | 生成日结算单 |
| POST | `/api/nominations/{id}/submit` | 提交申报（占用合同容量） |
| POST | `/api/permits/{id}/approve` | 审批许可（校验冲突） |
| POST | `/api/orders/{id}/execute` | 执行调度（联动设备） |
| POST | `/api/incidents/{id}/close` | 关闭事件（需处置项完成） |
| GET | `/api/leaks/analyze/{segmentId}` | 泄漏分析 |
