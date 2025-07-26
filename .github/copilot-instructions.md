# Copilot Instructions for IOT-Zinx

## 项目架构与核心知识

- **六边形架构（端口与适配器）**：业务逻辑与技术实现彻底分离。核心目录：

  - `internal/domain/dny_protocol`：DNY 协议领域模型（如 `Message`，`DeviceRegisterData`，协议常量）
  - `internal/app`：业务服务实现（如 `DeviceService`，`UnifiedChargingService`）
  - `internal/ports`：系统边界接口（如 `tcp_server.go` TCP 入口）
  - `internal/adapter`：对接外部系统（如 HTTP API、Zinx、Redis）
  - `internal/infrastructure`：技术支撑（如日志、配置、存储）

- **协议统一处理**：DNY 协议数据流：

  - 入口：`pkg/protocol/dny_protocol_parser.go` 中的 `ParseDNYProtocolData()` 统一解析所有协议变体
  - 标准化：`*dny_protocol.Message` 统一消息格式
  - 解码器：`pkg/protocol/dny_decoder.go` DNY 协议拦截器处理 TCP 流分包
  - 数据包：`pkg/protocol/dny_packet.go` 基础数据包识别和分包
  - 处理器：每个命令有独立 Handler（如 `SwipeCardHandler` 0x02，`ChargeControlHandler` 0x82）
  - 适配器：新架构推荐用 `ProtocolDataAdapter` 替代传统 Handler

- **DataBus 事件驱动架构**：

  - 核心：`pkg/databus` 统一数据流转和事件发布
  - 适配器：`pkg/databus/adapters/` 包含 TCP、会话、协议桥接等集成组件
  - 集成器：`TCPDataBusIntegrator` 统一管理所有 TCP 相关适配器
  - 会话管理：`TCPSessionManager` 处理连接生命周期，`UnifiedSessionManager` 统一会话状态

- **TCP 连接生命周期**：

  - 建立：`ConnectionHooks.OnConnectionStart()` → 创建会话 → 等待 ICCID
  - 注册：ICCID → 设备注册（0x20）→ 绑定设备 ID → 激活连接
  - 心跳：标准心跳（0x01）、设备心跳（0x21）、功率心跳（0x26）
  - 断开：`OnConnectionStop()` → 清理会话 → 发布断开事件

- **命令模式与状态管理**：
  - 命令管理：`pkg/network/command_manager.go` 管理命令超时重发
  - 状态跟踪：充电状态、连接状态、设备状态分别由不同组件管理
  - 响应处理：每个命令处理器负责构建和发送响应

## 项目约定与开发习惯

- **工具函数查找顺序**：优先查 `pkg/` 再查 `internal/`，避免重复造轮子
- **协议开发模式**：
  - 命令常量：集中在 `pkg/constants/ap3000_commands.go`
  - 协议解析：使用 `pkg/protocol/dny_protocol_parser.go` 中的 `ParseDNYProtocolData()` 入口
  - 处理器模式：新开发用 `ProtocolDataAdapter`，遗留 Handler 逐步迁移
  - 响应构建：用 `dny_protocol.BuildXXXPacket()` 系列函数
- **DataBus 集成模式**：
  - 事件发布：使用 `dataBus.PublishDeviceData()` 发布设备数据变更
  - 适配器开发：继承 `TCPDataBusIntegrator` 或实现具体适配器接口
  - 会话管理：通过 `TCPSessionManager` 管理连接状态，避免直接操作连接属性
- **错误处理约定**：
  - 结构化日志：用 `logrus.WithFields()` 记录上下文信息
  - 命令失败：通过 `command_manager.ConfirmCommand()` 确认命令状态
  - 连接异常：通过 DataBus 事件通知，不直接断开连接
- **业务状态管理**：
  - 充电状态：优先使用 `EnhancedChargeControlHandler` 新架构
  - 设备状态：通过 `DeviceSession.UpdateStatus()` 更新
  - 连接状态：区分 TCP 连接状态和业务连接状态
- **测试与调试指南**：
  - 单元测试：`go test ./...`，重点测试协议解析和业务逻辑
  - 集成测试：使用 `cmd/device-simulator/` 模拟设备连接
  - 日志调试：查看 `logs/gateway-*.log`，关注 DNY 协议解析和事件流转
  - 性能调试：检查 DataBus 事件处理性能和会话管理效率

## AI 协作模式与工作流程

### 基本协作规则

你是 vscode IDE 的 AI 编程助手，用中文协助用户，面向专业程序员，交互简洁专业。

**核心原则**：

- 所有响应以 `[模式：X]` 标签开头，默认 `[模式：研究]`
- 变更前必须查现有实现，避免重复造轮子
- 多思考多检查，确保逻辑正确、上下文明确
- 重要决策/疑问主动用 `interactive_feedback` 征询用户

### 工作流程模式

#### 标准五阶段流程（推荐）

1. **`[模式：研究]`** - 理解需求，查阅相关文档代码
2. **`[模式：构思]`** - 提供多种方案选择（至少两种，含评估）
3. **`[模式：计划]`** - 细化步骤清单（文件/函数/逻辑/预期结果），用 `Context7` 查新库，完成后用 `interactive-feedback` 请求批准
4. **`[模式：执行]`** - 用户批准后执行，计划存入 `./issues/任务名.md`，关键节点用 `interactive-feedback` 反馈
5. **`[模式：评审]`** - 对照计划评估结果，报告问题建议，用 `interactive-feedback` 确认

#### 快速模式

**`[模式：快速]`** - 跳过标准流程，直接响应，完成后用 `interactive-feedback` 确认

### 项目特定约定

- 协议/事件变更等复杂操作，优先查阅 `docs/`、`pkg/`、`internal/`
- 计划/执行/评审结果建议写入 `issues/` 便于团队追踪
- 业务逻辑变更需关注协议兼容性与事件一致性
- 使用 MCP 服务：`interactive_feedback` 用户反馈，`Context7` 查询最新库文档

---

如需详细协议、架构、DataBus、日志等说明，请查阅：

- `docs/` 设计文档
- `pkg/databus/adapters/README.md` 适配器与事件流转
- `README.md` 项目总览与快速入门
