# 充电设备网关系统 (IOT-Zinx)

基于 Zinx 网络框架的充电设备网关系统，实现与充电桩设备的通信和管理。

## 🚀 最新架构更新 (2025 年 8 月)

### 数据存储去重修复和架构优化

项目完成了重大的数据存储去重修复，实现了高性能、高可靠性的新架构：

#### ✨ 修复亮点

- **🎯 单一数据源**: Device 结构作为设备信息的唯一来源，消除三重数据存储
- **📦 职责分离**: ConnectionSession 管理连接级别数据，Device 管理设备级别数据
- **🔧 并发安全**: 所有数据更新都有适当的 mutex 保护，消除数据竞争
- **⚡ 性能优化**: 减少内存占用 50%+，简化数据同步逻辑

#### 🏗️ 新架构特性

```go
// 统一数据获取方式
import "github.com/bujia-iot/iot-zinx/pkg/core"

// 获取设备信息（新方式）
tcpManager := core.GetGlobalTCPManager()
device, exists := tcpManager.GetDeviceByID(deviceID)
if exists {
    deviceID := device.DeviceID
    physicalID := device.PhysicalID
    iccid := device.ICCID
    status := device.Status
}

// 心跳更新（统一接口）
tcpManager.UpdateHeartbeat(deviceID)

// 设备注册（原子操作）
err := tcpManager.RegisterDevice(conn, deviceID, physicalID, iccid)
```

#### 📊 架构改进成果

- **内存优化**: 消除重复数据存储，减少内存占用
- **数据一致性**: 单一数据源确保数据一致性
- **并发安全**: 完善的锁机制保障多线程安全
- **接口统一**: 所有模块通过 TCPManager 统一访问数据

#### 📖 相关文档

- **数据流图**: [docs/architecture/data-flow-diagram.md](docs/architecture/data-flow-diagram.md)
- **系统架构**: [docs/architecture/system-architecture.md](docs/architecture/system-architecture.md)
- **修复报告**: [issues/数据存储去重修复\_完成报告.md](issues/数据存储去重修复_完成报告.md)

---

## 项目介绍

本系统是一个基于 TCP 协议的充电设备网关，负责连接和管理充电桩设备，处理设备上报的各种数据，并将业务请求转发给设备。系统采用六边形架构（端口与适配器架构），实现了业务逻辑与技术实现的分离。

### 主要功能

- **设备连接管理**：处理设备上线、注册和离线，支持 ICCID 和设备 ID 双重识别
- **统一数据管理**：Device 作为设备信息单一数据源，确保数据一致性
- **多种心跳管理**：支持标准心跳、主机心跳、Link 心跳等多种保活机制
- **智能设备识别**：支持十进制/十六进制 DeviceID 格式自动转换
- **充电控制**：向设备发送充电启停命令，控制充电过程
- **设备状态监控**：实时监控设备心跳状态，自动清理超时连接
- **并发安全保障**：完善的 mutex 保护机制，确保多线程环境下的数据安全
- **高性能架构**：消除重复数据存储，减少内存占用，提升系统性能
- **RESTful API**：提供标准化的 HTTP API 接口，支持设备查询和控制

### 技术栈

- **Go 语言开发**：高性能并发编程语言
- **Zinx 网络框架**：高性能 TCP 服务器框架
- **分层架构**：API 层、网关层、核心层、数据层、网络层分离
- **DNY 协议**：设备通信协议完整支持
- **并发安全**：sync.RWMutex 和 sync.Map 保障线程安全
- **RESTful API**：标准化的 HTTP 接口设计

## 目录结构

- `cmd/gateway`: 网关程序
- `cmd/dny-parser`: DNY 协议解析工具
- `internal/app`: 应用层代码
- `internal/domain`: 领域层代码
- `internal/infrastructure`: 基础设施层代码
- `internal/ports`: 接口层代码
- `pkg`: 可重用工具包 (重构后的工具类集合)
- `examples`: 示例代码
- `test`: 测试代码

## 工具包使用

项目的可重用工具类已重构到`pkg`目录中，包括：

- `pkg/protocol`: DNY 协议处理相关工具
- `pkg/network`: 网络通信相关工具
- `pkg/monitor`: 设备和连接监控工具
- `pkg/utils`: 通用工具类

### 快速开始

```go
import "github.com/bujia-iot/iot-zinx/pkg/core"

// 获取全局TCP管理器
tcpManager := core.GetGlobalTCPManager()

// 设备注册（新架构）
err := tcpManager.RegisterDevice(conn, deviceId, physicalId, iccid)

// 获取设备信息（统一数据源）
device, exists := tcpManager.GetDeviceByID(deviceId)
if exists {
    fmt.Printf("设备状态: %v", device.Status)
    fmt.Printf("最后心跳: %v", device.LastHeartbeat)
}

// 心跳更新（统一接口）
tcpManager.UpdateHeartbeat(deviceId)

// 设备状态查询
isOnline := tcpManager.IsDeviceOnline(deviceId)
```

详细说明请参考 [pkg/README.md](pkg/README.md)。

## 开发指南

### 环境要求

- Go 1.18+
- 支持 TCP 协议的网络环境

### 构建与运行

1. 克隆项目

```bash
git clone https://github.com/bujia-iot/iot-zinx.git
cd iot-zinx
```

2. 安装依赖

```bash
go mod tidy
```

3. 构建项目

```bash
make build
```

4. 运行网关

```bash
./bin/gateway --config configs/gateway.yaml
```

### 开发流程

1. 领域层开发：在 domain 目录下定义设备通信协议和业务模型
2. 业务层开发：在 app 目录下实现业务逻辑
3. 适配器开发：在 adapter 目录下实现与外部系统的对接
4. 处理器开发：在 infrastructure/zinx_server/handlers 目录下添加命令处理器

## 项目结构说明

### 命令处理器

系统支持以下命令处理器：

- `DeviceRegisterHandler`：设备注册请求处理器 (0x20)
- `HeartbeatHandler`：标准心跳包处理器 (0x01/0x21)
- `MainHeartbeatHandler`：主机心跳包处理器 (0x11)
- `DeviceStatusHandler`：设备状态处理器 (0x81)
- `GetServerTimeHandler`：获取服务器时间处理器 (0x12/0x22)
- `ChargeControlHandler`：充电控制处理器 (0x82)
- `SimCardHandler`：SIM 卡号/ICCID 处理器
- `LinkHeartbeatHandler`：Link 心跳处理器
- `NonDNYDataHandler`：非 DNY 协议数据处理器
- `ParameterSettingHandler`：参数设置处理器 (0x83)
- `DeviceLocateHandler`：设备定位处理器 (0x96)
- `DeviceVersionHandler`：设备版本信息处理器 (0x35)
- `SettlementHandler`：结算处理器 (0x03)

### 核心组件

#### 数据管理层

- `pkg/core/tcp_manager.go`：核心 TCP 管理器，统一管理连接和设备
- `pkg/core/connection_device_group.go`：设备组管理，支持一对多设备关系
- `pkg/core/device_monitor.go`：设备状态监控器，监控设备心跳状态

#### 网络传输层

- `pkg/network/tcp_writer.go`：统一发送通道，支持同步/异步发送
- `pkg/network/connection_hooks.go`：连接生命周期钩子函数
- `internal/domain/dny_protocol/decoder.go`：DNY 协议解析拦截器

#### 业务处理层

- `pkg/gateway/device_gateway.go`：设备网关接口，提供统一的设备操作
- `pkg/utils/physical_id_helper.go`：PhysicalID 转换工具
- `pkg/utils/device_id_converter.go`：DeviceID 格式转换工具

#### 数据结构

- `ConnectionSession`：连接级别数据管理（SessionID、ConnID、RemoteAddr）
- `DeviceGroup`：设备组管理（ICCID、ConnID、Devices 映射）
- `Device`：设备信息管理（DeviceID、PhysicalID、Status、LastHeartbeat）

### 端口和适配器架构

项目采用六边形架构（也称为端口和适配器架构），实现了业务逻辑与技术实现的分离：

1. **核心结构**：

   - `internal/domain`：领域层，包含核心业务模型和协议定义
   - `internal/app`：应用层，包含业务服务实现
   - `internal/ports`：端口层，定义系统与外部交互的边界
   - `internal/adapter`：适配器层，实现与外部系统的交互
   - `internal/infrastructure`：基础设施层，提供技术支持

2. **关键端口**：

   - `ports/tcp_server.go`：TCP 服务器启动入口
   - `ports/http_server.go`：HTTP API 服务入口

3. **核心适配器**：
   - `adapter/http`：HTTP 请求处理适配器
   - `infrastructure/zinx_server`：Zinx 网络框架适配器
   - `infrastructure/redis`：Redis 数据存储适配器
   - `infrastructure/config`：配置管理适配器
   - `infrastructure/logger`：日志适配器

### 设备连接生命周期

1. **连接建立**：设备与网关建立 TCP 连接，网关创建 ConnectionSession 并分配连接 ID
2. **初始化识别**：设备发送 ICCID(SIM 卡号)，网关验证并存储到连接属性
3. **设备注册**：设备发送注册请求(0x20)，网关创建 Device 对象并建立设备索引映射
4. **心跳保活**：设备定期发送心跳包，网关通过统一接口更新 Device.LastHeartbeat
5. **业务交互**：通过 DeviceGateway 接口处理设备控制命令和状态查询
6. **状态监控**：网关监控设备心跳状态，自动检测离线设备
7. **连接断开**：设备断开时，网关清理 ConnectionSession 并更新设备状态

#### 数据流特点

- **单一数据源**：Device 作为设备信息的唯一来源
- **职责分离**：ConnectionSession 管理连接，Device 管理设备信息
- **并发安全**：所有数据更新都有 mutex 保护
- **统一接口**：所有模块通过 TCPManager 访问数据

## 协议支持

本系统实现了 DNY 协议（设备通信协议），支持以下功能：

1. 设备注册与认证
2. 心跳保活（设备心跳、主机心跳、Link 心跳等）
3. 刷卡消费
4. 充电控制
5. 设备状态监控
6. 服务器时间同步
7. 参数设置

## 日志系统

本项目使用了改进的日志系统，基于 `logrus` 和 `lumberjack`，提供了统一的日志管理、自动轮转、结构化日志和 Zinx 框架集成等功能。

### 日志特性

- **统一日志管理**: 基于 logrus 的强大日志功能，支持多种日志级别
- **自动日志轮转**: 基于 lumberjack 实现自动日志轮转，避免日志文件过大
- **多路输出**: 同时支持控制台和文件输出，便于开发和生产环境使用
- **结构化日志**: 支持结构化字段，便于日志分析和监控
- **Zinx 框架集成**: 通过适配器模式统一 Zinx 框架日志

### 日志配置

```yaml
# 日志配置
logger:
  level: "debug" # 日志级别：trace, debug, info, warn, error, fatal, panic
  format: "json" # 输出格式：json, text
  filePath: "./logs/gateway.log" # 日志文件路径
  maxSizeMB: 100 # 最大文件大小（MB）
  maxBackups: 10 # 最大备份文件数量
  maxAgeDays: 30 # 最大保留天数
  logHexDump: true # 是否记录十六进制数据
  enableConsole: true # 是否同时输出到控制台
  enableStructured: true # 是否启用结构化日志
```

### 典型日志输出

**Zinx 框架调试日志**：

```json
{
  "level": "debug",
  "msg": "read buffer 444e590900cd28a2043b0222ee02",
  "source": "zinx",
  "time": "2025-06-02 18:03:08"
}
```

**业务结构化日志**：

```json
{
  "level": "info",
  "msg": "设备连接",
  "component": "device_handler",
  "conn_id": 12345,
  "remote_addr": "192.168.1.100:45678",
  "time": "2025-06-02 18:03:08"
}
```

详细的日志系统使用指南请参考：[日志系统使用指南](docs/日志系统使用指南.md)

## 版权与许可

Copyright © 2025 bujia-iot

Licensed under the MIT License.
