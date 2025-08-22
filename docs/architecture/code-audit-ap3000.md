# 代码审计与业务规范（AP3000 对齐）

本报告基于现有代码与《AP3000-设备与服务器通信协议》对齐进行审计，给出业务规范、无效代码清理建议与改进计划。

## 一、API ↔ 协议映射核验
- 充电控制
  - `POST /api/v1/charging/start` / `stop` → 指令 `0x82`
  - 实现：`internal/adapter/http/gateway_handlers.go` → `DeviceGateway.SendChargingCommandWithParams`
  - 字段校验：端口 1-based→0-based，订单号≤16字节，余额4字节小端，value 2字节小端
- 设备定位
  - `POST /api/v1/device/locate` → 指令 `0x96`
  - 实现：`DeviceGateway.SendLocationCommand`
- 状态/详情
  - `GET /api/v1/device/{id}/status|detail` → `TCPManager` 单一数据源
- 统一构包与发送
  - 构包：`pkg/protocol/dny_packet.go`（小端、校验）
  - 发送：`pkg/network/tcp_writer.go`（重试/写超时）

## 二、关键业务规范
- DeviceID 标准化：`utils.DeviceIDProcessor.SmartConvertDeviceID`
- PhysicalID 一致性：发送前以解析值为准，必要时回写 `Device.PhysicalID`
- 消息ID：`pkg.Protocol.GetNextMessageID()`，避免重复
- 发送节流：同设备命令≥0.5s（建议在 `TCPWriter`/`DeviceGateway` 分设备节流）
- 日志：结构化日志，记录 deviceID/physicalID/msgID/cmd/dataHex/packetHex/packetLen

## 三、无效/冗余代码与改进建议
- 控制台打印（建议移除或替换为结构化日志）
  - 已处理：`pkg/gateway/device_gateway.go`（移除 `fmt.Printf`）
  - 待处理（建议保留在测试用例内，业务路径移除）：
    - `pkg/core/tcp_manager.go`（多处 `fmt.Printf` 调试输出）
    - `pkg/protocol/dny_packet.go`（GetHeadLen/Checksum 错误打印）
    - `internal/ports/tcp_server.go`（错误打印）
    - `internal/infrastructure/zinx_server/handlers/sim_card_handler.go`（调试打印）
    - `internal/infrastructure/logger/improved_logger.go`（初始化期打印，可迁移为 logger.Warn/Info）
  - 说明：测试目录 `internal/..._test.go`、`test/*.go` 可保留 `fmt.Printf`
- 重复协议解析/工具函数
  - 建议：分散的十六进制/校验/判断方法统一移动到 `pkg/utils`，避免重复
- 协议解析职责
  - 已对齐：`DNYPacket` 只做基础识别与分包，完整解析移至拦截器
- 注释掉的大块旧实现
  - 建议：清理历史注释代码块，保留 Git 历史即可

## 四、风险与回归建议
- 替换日志需回归：
  - 设备上线/下线、心跳、充电指令全链路联调
  - Hex 与校验日志保留到 `debug` 或可配置开关
- 节流策略引入后：压测同设备高频控制，验证稳定性

## 五、执行清单（建议分支执行）
- 批量替换业务路径 `fmt.Printf` → `logger.WithFields(...).Level(...)`
- 提取/复用工具函数到 `pkg/utils`
- 在 `TCPWriter`/`DeviceGateway` 增加 per-device 节流（≥0.5s）
- 为 `0x82/0x96` 发送增加超时度量与失败重试统计指标

## 六、相关链接
- 协议映射说明：`docs/architecture/ap3000-mapping.md`
- 数据流图：`docs/architecture/data-flow-diagram.md`
- 协议文档：`docs/协议/AP3000-设备与服务器通信协议.md`
