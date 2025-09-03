# IoT-Zinx 数据流图（AP3000 对齐版）

本文档描述了IoT-Zinx系统修复后的完整数据流，展示了从TCP连接建立到API响应的完整链路。

## 系统概述

IoT-Zinx采用分层架构设计，实现了设备连接管理、协议解析、业务处理和API服务的完整功能。修复后的系统具有以下特点：

- **单一数据源**：Device结构作为设备信息的唯一来源
- **职责分离**：ConnectionSession管理连接级别数据，Device管理设备级别数据
- **并发安全**：所有数据更新都有适当的mutex保护
- **统一接口**：所有模块通过TCPManager访问数据

## 完整数据链路流程（涵盖 AP3000 要点）

### 1. TCP连接管理链路

```
TCP连接建立 → OnConnStart回调 → tcpManager.RegisterConnection() 
→ ConnectionSession创建 → 连接属性设置 → 连接状态：已连接
```

**关键组件**：
- `internal/ports/tcp_server.go` - TCP服务器配置和连接钩子
- `pkg/core/tcp_manager.go` - 连接注册和管理（单一数据源：connections/deviceGroups/deviceIndex）
- `internal/infrastructure/zinx_server/handlers/*` - 协议帧解析入口

**数据结构**：
```go
type ConnectionSession struct {
    SessionID       string
    ConnID          uint64
    RemoteAddr      string
    LastActivity    time.Time
    ConnectionState constants.ConnStatus
    // 只管理连接级别数据，不存储设备信息
}
```

### 2. ICCID处理链路

```
ICCID数据接收 → SimCardHandler验证 → 连接属性存储 
→ TCPManager同步 → DeviceGroup准备 → 等待设备注册
```

**关键组件**：
- `internal/infrastructure/zinx_server/handlers/sim_card_handler.go`
- ICCID格式验证（ITU-T E.118标准）
- 连接属性存储机制

**验证规则**：
- 固定长度20字节
- 必须以"89"开头
- 全部为十六进制字符

### 3. 设备注册链路

```
设备注册包接收 → DeviceRegisterHandler处理 → 设备ID解析 
→ PhysicalID转换 → Device创建 → DeviceGroup关联 
→ 设备索引映射(deviceID→iccid) → 注册成功响应
```

**关键组件**：
- `internal/infrastructure/zinx_server/handlers/device_register_handler.go`
- `pkg/utils/physical_id_helper.go` - PhysicalID转换工具
- `pkg/core/tcp_manager.go` - 设备注册管理

**数据结构**：
```go
type Device struct {
    DeviceID        string
    PhysicalID      uint32
    ICCID           string
    DeviceType      uint16
    Status          constants.DeviceStatus
    LastHeartbeat   time.Time
    Properties      map[string]interface{}
    mutex           sync.RWMutex  // 并发保护
}
```

### 4. 心跳处理链路

```
心跳包接收 → 各Handler处理 → 设备ID提取 
→ tcpManager.UpdateHeartbeat() → Device.LastHeartbeat更新 
→ 连接会话活动时间更新 → 设备状态：在线
```

**支持的心跳类型（对齐 AP3000）**：
- 标准心跳包 (0x01)
- 主机心跳包 (0x11) 
- 简化心跳包 (0x21)
- 功率心跳包 (0x06)
- 端口功率心跳包 (0x26)

**统一处理机制**：
所有心跳处理器都调用 `tcpManager.UpdateHeartbeat(deviceId)` 进行统一的心跳更新。

### 5. API数据获取链路

```
API请求 → 智能DeviceID处理 → tcpManager.GetDeviceByID() 
→ Device数据返回 → JSON序列化响应 → API响应返回
```

**智能DeviceID处理**：
- 支持十进制格式：`10644723`
- 支持6位十六进制：`A26CF3`
- 支持8位十六进制：`04A26CF3`

**API端点**：
- `GET /api/v1/device/{id}/status` - 设备状态查询（数据源：`core.TCPManager`）
- `GET /api/v1/device/{id}/detail` - 设备详情查询（数据源：`core.TCPManager`）
- `POST /api/v1/charging/start` - 开始充电 → 指令 0x82
- `POST /api/v1/charging/stop` - 停止充电 → 指令 0x82
- `POST /api/v1/device/locate` - 设备定位 → 指令 0x96

### 6. 充电控制链路

```
充电命令API → 设备查找验证 → PhysicalID验证 
→ DNY协议构建 → TCPWriter发送 → 设备响应处理 → 命令执行结果
```

**关键组件**：
- `pkg/gateway/send.go` - 统一发送入口（节流≥0.5s、消息ID）
- `pkg/gateway/control.go` - 业务命令封装（0x96 等）
- `pkg/protocol/unified_dny_builder.go` - DNY统一构建（小端、长度含校验）
- `pkg/network/unified_sender.go`/`pkg/network/tcp_writer.go` - 统一发送（RAW TCP、写超时、重试/退避）

## 数据存储架构

### 三层映射结构（来自 `core.TCPManager` 单一数据源）

```go
type TCPManager struct {
    connections  sync.Map // connID → *ConnectionSession
    deviceGroups sync.Map // iccid → *DeviceGroup  
    deviceIndex  sync.Map // deviceID → iccid
}
```

### 数据关系

1. **一对一关系**：一个TCP连接对应一个ConnectionSession
2. **一对多关系**：一个DeviceGroup包含多个Device，但只关联一个TCP连接
3. **索引映射**：deviceIndex提供从设备ID到ICCID的快速查找

### 并发安全保障

- **ConnectionSession**：使用sync.RWMutex保护连接级别数据
- **DeviceGroup**：使用sync.RWMutex保护设备组级别数据  
- **Device**：使用sync.RWMutex保护设备级别数据
- **TCPManager**：使用sync.Map提供线程安全的映射操作

## 关键设计原则

1. **单一职责**：每个组件只负责特定的功能领域
2. **数据一致性**：Device作为设备信息的单一来源
3. **并发安全**：所有共享数据都有适当的保护机制
4. **接口统一**：所有模块通过TCPManager访问数据
5. **错误处理**：完善的错误处理和日志记录机制；DNY包校验失败/长度不符/端口越界必须拒发

## 唯一发送链路（删除过时引用）

- 统一链路：HTTP API → DeviceGateway → UnifiedDNYBuilder → UnifiedSender/TCPWriter → RAW TCP → 设备
- 删除过时引用：原 `pkg/protocol/dny_packet.go` 作为基础类型保留，构包以 `unified_dny_builder.go` 为唯一来源；发送仅经 `unified_sender.go`/`tcp_writer.go`，禁止二次封装。

## 性能优化

1. **内存优化**：消除了重复数据存储，减少内存占用
2. **查找优化**：通过索引映射实现O(1)的设备查找
3. **并发优化**：使用读写锁减少锁竞争
4. **连接复用**：多个设备共享同一TCP连接

## 监控和诊断

系统提供了完善的监控和诊断功能：

- **连接状态监控**：实时跟踪连接状态变化
- **心跳监控**：监控设备心跳超时和离线检测
- **性能指标**：连接数、设备数、消息处理量等统计
- **日志记录**：详细的操作日志和错误日志
- **健康检查**：系统组件健康状态检查

## 扩展性设计

系统架构支持以下扩展：

1. **新协议支持**：通过添加新的Handler支持其他协议
2. **新业务功能**：通过DeviceGateway接口扩展新的业务功能
3. **第三方集成**：通过通知系统集成第三方平台
4. **水平扩展**：支持多实例部署和负载均衡

---

*本文档反映了数据存储去重修复后的最新架构，确保了系统的高性能、高可靠性和高可维护性。*
