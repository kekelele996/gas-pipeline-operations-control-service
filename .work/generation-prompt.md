# 天然气长输管网运行控制服务（Go 全后端 + 前端运维页）

请生成一个完整的 Go 1.23+ 项目，模块名 `gas-pipeline-operations-control-service`，**只使用 Go 标准库**（net/http、encoding/json、sync、context、time 等），不引入任何第三方依赖，全部业务数据保存在**内存 store + 互斥锁**中。目标是交付一个可运行、可测试、结构清晰的 B2B 管网运行控制服务，代码量要大（非测试 .go 文件不少于 60 个，非测试代码不少于 6000 行），每个包的逻辑要真实、有业务深度，禁止写占位空壳。

## 业务背景

某天然气长输管网运营公司需要一套「管网运行控制服务」，覆盖：管网拓扑档案、SCADA 遥测采集与告警、贸易计量结算、托运商合同与容量分配、检修作业许可、运行调度指令、事件/事故管理、泄漏检测分析、审计日志与通知推送。服务以 HTTP JSON API 对外，另提供一个简单的运维看板页面（web/，纯静态 HTML/CSS/JS，由 Go 进程直接托管）。

## 总体要求

1. 所有数据模型独立成文件；每个领域包包含 `model.go`（数据模型与状态常量）、`store.go`（内存存储，带互斥锁，注意返回拷贝不要泄漏内部引用）、`service.go`（业务逻辑）、必要的 `errors.go`/`types.go`/`state.go`。
2. 每个领域包都要有**单元测试** `_test.go`（覆盖正常路径、边界、并发安全），测试要真实调用 service/store。
3. 状态机用显式转换表（map[State][]State）+ `CanTransition`/`Transition` 方法实现，转换非法返回错误。
4. HTTP 入口在 `cmd/server`，路由注册在 `internal/httpapi`，所有 handler 通过真实 service 调用；提供 `/health` 健康检查。
5. 提供一个 `runtime_smoke.json`（mode: service, start 命令, ready_url=/health, port）。
6. 根目录 README.md 说明项目结构、运行与测试命令。
7. 前端 `web/` 一个 index.html + 静态资源，页面能展示管网概览（可调 `/api/summary`）。
8. 代码注释用中文或英文均可，但要干净；不要写「TODO 占位」式空实现。

## 目录与包规划（每包职责、关键文件、关键状态机）

```
cmd/server/main.go              入口：加载配置、构建依赖、注册路由、监听端口、seed 演示数据
internal/config/config.go       配置加载（端口、站点代码、超时、阈值），带默认值
internal/platform/errors.go     统一错误类型（ErrNotFound/ErrConflict/ErrInvalid/ErrState），Is 支持
internal/platform/id.go         生成短 ID（时间戳+随机）
internal/platform/clock.go      可注入时钟接口（Now()），便于测试
internal/platform/json.go       JSON 读写 helper
internal/platform/validate.go   通用校验（非空、枚举、正数）
internal/network/model.go       管网拓扑：Segment(管段)、Station(站场)、Compressor(压缩机)、Valve(阀门)、Point(测点)
internal/network/store.go       拓扑存储（按类型索引，带锁，返回深拷贝）
internal/network/service.go     拓扑查询/维护：沿线管段列表、站点所属管段、压缩机启停状态变更（状态机：运行/停机/检修）
internal/scada/model.go         遥测点定义、读数快照、告警规则、告警记录
internal/scada/store.go         读数环形缓冲（按测点保留最近 N 条）、告警存储
internal/scada/service.go       遥测写入（带限幅/坏值过滤）、批量写入、告警评估（阈值/变化率）、告警查询
internal/metering/model.go      计量点、流量原始值、折算系数、日累计、贸易计量结算单
internal/metering/store.go      计量点与日累计存储
internal/metering/service.go    流量折算（标准体积）、日累计滚动、按日结算（生成结算单，状态机：草稿/已确认/已对账）
internal/nomination/model.go    托运商申报单（Nomination）：状态机 草稿/已提交/已确认/已执行/已取消
internal/nomination/store.go    申报存储
internal/nomination/service.go  申报提交、容量校验（与合同剩余容量比较）、确认、执行扣减、取消
internal/contract/model.go      托运商合同：合同量、已用容量、有效期、状态
internal/contract/store.go      合同存储
internal/contract/service.go    合同创建、剩余容量查询、容量占用/释放（与 nomination 联动）
internal/permit/model.go        检修作业许可（Permit）：状态机 草稿/待审批/已批准/作业中/已完成/已取消/已过期
internal/permit/store.go        许可存储
internal/permit/service.go      许可申请、审批（校验是否与在途调度/事件冲突）、开工、完工、取消、过期扫描
internal/dispatch/model.go      运行调度指令（Order）：状态机 待下发/已下发/已执行/已撤销
internal/dispatch/store.go      指令存储
internal/dispatch/service.go    指令创建（校验阀门/压缩机状态）、下发、执行（联动设备状态）、撤销
internal/incident/model.go      事件/事故（Incident）：状态机 待确认/处置中/已关闭
internal/incident/store.go      事件存储
internal/incident/service.go    事件上报（联动告警）、指派、处置、关闭（校验关联处置项完成）
internal/leakdetect/model.go    泄漏分析：压力异常段、判定结果、泄漏告警
internal/leakdetect/service.go  按管段聚合遥测，压力下降率/平衡差分析，产出泄漏判定
internal/audit/model.go         审计日志条目
internal/audit/store.go         审计存储（追加写）
internal/audit/service.go       记录操作（谁、何时、对什么、结果），查询
internal/notify/model.go        通知/告警推送：接收人、渠道、状态（待推送/已推送/失败/重试中）
internal/notify/store.go        通知存储
internal/notify/service.go      通知入队、批量推送（模拟）、失败重试、状态回查
internal/httpapi/router.go      路由注册（标准库 ServeMux）
internal/httpapi/server.go      HTTP Server 组装、中间件（日志/恢复/recover）
internal/httpapi/network.go     /api/segments /api/stations /api/compressors/{id}/state ...
internal/httpapi/scada.go       /api/telemetry  POST 批量写入 /api/alarms GET
internal/httpapi/metering.go    /api/metering/daily /api/metering/settle ...
internal/httpapi/nomination.go  /api/nominations ... 
internal/httpapi/permit.go      /api/permits ...
internal/httpapi/dispatch.go    /api/orders ...
internal/httpapi/incident.go    /api/incidents ...
internal/httpapi/summary.go     /api/summary 运维看板聚合
internal/seed/seed.go           演示数据（管段/站场/合同/计量点/测点等）
web/index.html                  运维看板（纯静态，fetch /api/summary 渲染）
web/app.js
web/style.css
```

## 各领域包的详细业务要求（务必实现到位）

### internal/scada
- 测点 Point：id、名称、类型（压力/流量/温度）、单位、所属管段、上下限、变化率上限、是否启用。
- 读数 Reading：point_id、value、ts。
- 写入 service.Ingest(ctx, readings)：过滤坏值（NaN、超出物理范围）、限幅、存环形缓冲（每点最近 200 条）、对启用告警的测点做阈值/变化率评估，产生 Alarm。
- 并发安全：Ingest 可被多个 goroutine 同时调用（内部 store 要加锁且不泄漏引用）；提供 `History(pointID)` 返回深拷贝快照。
- 告警 Alarm：id、point_id、级别、消息、时间、状态（活动/已确认/已恢复）。
- 测试要覆盖：并发写入+历史快照稳定性（race 下不 panic、快照不被后续写入污染）、阈值告警、变化率告警、坏值过滤。

### internal/metering
- 计量点 Meter：id、名称、所在管段、单位、压力/温度基准、折算系数表（按温度档位）。
- 折算：原始体积 → 标准体积 = raw × factor(temperature)。
- 日累计：DailyTotal{meter_id, date, raw_volume, std_volume, reading_count, first_ts, last_ts}；每日第一条读数为基数，后续读数累加差值。
- 结算单 Settlement：id、日期、状态机（草稿/已确认/已对账）、条目（每个计量点的累计量）、由 service.Settle 生成。
- service 方法：RecordReading、GetDailyTotal、SettleDaily（生成/更新结算单）、Confirm、Reconcile。
- 测试：跨天累计正确、结算单状态流转、并发读数不丢累计。

### internal/nomination + internal/contract（联动）
- Contract：id、托运商、合同量（m³/日）、已用容量、有效期起止、状态（生效/暂停/到期）。
- Nomination：id、合同 id、日期、申报量、状态机（草稿/已提交/已确认/已执行/已取消）。
- service.Submit：校验申报量 ≤ 合同剩余容量（remaining = 合同量 - 已用），通过则占用容量；确认后状态流转；执行时扣减日已用容量；取消释放容量。
- 并发：同一合同多条申报同时提交不能超容量（用合同级锁或原子容量预留）。
- 测试：容量边界、并发提交不超量、取消释放、状态非法流转拒绝。

### internal/permit
- Permit：id、管段、作业类型、作业窗口（起止时间）、申请人、审批人、状态机（草稿/待审批/已批准/作业中/已完成/已取消/已过期）。
- 审批校验：作业窗口不能与同管段在途事件冲突、不能与已下发未执行的调度指令冲突。
- 开工：作业中；完工：已完成（联动 audit 记录）；取消：已取消；过期扫描：超过窗口结束未开工 → 已过期。
- 测试：正常全流程、冲突拒绝、过期扫描。

### internal/dispatch
- Order：id、类型（开阀/关阀/启压缩机/停压缩机/提量/降量）、目标设备 id、状态机（待下发/已下发/已执行/已撤销）。
- 创建校验：目标设备存在；下发校验：设备当前状态允许（如关阀指令要求阀门非关闭）。
- 执行：联动 network.Compressor/Valve 状态变更。
- 测试：非法目标、重复执行拒绝、撤销。

### internal/incident
- Incident：id、管段/设备、级别、描述、关联告警 id、处置项列表（每个处置项有完成标志）、状态机（待确认/处置中/已关闭）。
- 上报：可携带关联 alarm；确认：待确认→处置中；处置项全部完成才能关闭。
- 测试：未完成处置项不能关闭、关联告警自动恢复。

### internal/leakdetect
- 输入：某管段各测点最近 N 条压力读数。分析：计算压力下降速率、上下游平衡差。
- 判定：下降率超阈值 或 平衡差超阈值 → LeakAlert{segment_id, severity, evidence}。
- 测试：正常波动不误报、持续下降触发。

### internal/network
- Segment/Station/Compressor/Valve 档案；Compressor 状态机（运行/停机/检修）；Valve 状态（开/关/故障）。
- service：按管段查站点与设备、压缩机状态切换、阀门状态切换（联动 dispatch 执行）。

### internal/audit
- 记录 AuditEntry{id, actor, action, target_type, target_id, detail, ts}；追加式 store；查询按 target/action 过滤。

### internal/notify
- Notification{id, recipient, channel, subject, body, status(待推送/已推送/失败/重试中), attempt, next_retry_at}。
- service.Enqueue、PushBatch（模拟发送：成功或按配置失败率失败）、RetryFailed（把失败的置为重试中并按 backoff 重推）。
- 测试：批量推送状态流转、失败重试。

### internal/httpapi
- 中间件：请求日志、panic recover（500）、简单 CORS。
- 所有 handler 解析 JSON body 校验后调 service，错误映射：ErrNotFound→404、ErrConflict/ErrInvalid/ErrState→400/409、其它→500。
- /api/summary：聚合管段数、站点数、活动告警数、当日累计量、未关闭事件数、进行中许可数、待推送通知数，供看板使用。

## 测试要求
- 每个领域包至少 1 个 `_test.go`，关键并发包（scada、nomination、metering）要有并发测试（构造多个 goroutine + 同步屏障，跑 `go test -race` 不 panic、结果正确）。
- 测试名称要有意义，如 `TestScadaConcurrentIngestSnapshotStable`、`TestNominationConcurrentCapacityNoOverbook`。

## 交付检查
- `go build ./...` 必须通过；`go test ./...` 全绿。
- 运行 `go run ./cmd/server` 后监听 :18090，`/health` 返回 200，`/api/summary` 返回 JSON。
