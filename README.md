# 充电设备网关系统 (IOT-Zinx)

基于 Zinx 网络框架的充电设备网关系统，实现与充电桩设备的通信和管理。

## 🚀 最新架构更新 (2025 年 6 月)

### DNY 协议解析器统一重构

项目完成了重大的协议解析器统一重构，实现了高内聚、低耦合的新架构：

#### ✨ 重构亮点

- **🎯 统一解析入口**: 所有 DNY 协议变体通过 `ParseDNYProtocolData()` 统一处理
- **📦 标准化数据结构**: 使用 `*dny_protocol.Message` 统一消息格式
- **🔧 向后兼容**: 保持现有业务逻辑无缝运行
- **⚡ 性能优化**: 减少重复解析，提升处理效率

#### 🏗️ 新架构特性

```go
// 统一解析器使用示例
import "github.com/bujia-iot/iot-zinx/pkg/protocol"

data := []byte("ICCID12345678901234567890") // 任意DNY协议数据
msg, err := protocol.ParseDNYProtocolData(data)
if err != nil {
    log.Printf("解析失败: %v", err)
    return
}

switch msg.MessageType {
case "standard":    // 标准DNY协议帧
case "iccid":       // ICCID消息
case "heartbeat_link": // Link心跳
case "error":       // 解析错误
}
```

#### 📖 相关文档

- **架构设计**: [docs/DNY 协议解析器统一架构设计.md](docs/DNY协议解析器统一架构设计.md)
- **开发指南**: [docs/DNY 协议解析器\_开发者指南.md](docs/DNY协议解析器_开发者指南.md)
- **完成报告**: [issues/协议解析器统一重构\_完成报告.md](issues/协议解析器统一重构_完成报告.md)

---

## 项目介绍

本系统是一个基于 TCP 协议的充电设备网关，负责连接和管理充电桩设备，处理设备上报的各种数据，并将业务请求转发给设备。系统采用六边形架构（端口与适配器架构），实现了业务逻辑与技术实现的分离。

### 主要功能

- **设备连接管理**：处理设备上线、注册和离线，支持 ICCID 和设备 ID 双重识别
- **多种心跳管理**：支持标准心跳、主机心跳、Link 心跳等多种保活机制
- **刷卡消费**：处理设备刷卡请求，验证卡片有效性并授权消费
- **充电控制**：向设备发送充电启停命令，控制充电过程
- **设备状态监控**：实时监控设备心跳状态，自动清理超时连接
- **服务器时间同步**：提供精准时间服务，确保设备与服务器时间同步
- **命令重发机制**：关键业务命令支持超时重发，确保命令可靠送达
- **原始数据记录**：完整记录设备与服务器之间的通信数据，便于问题排查

### 技术栈

- Go 语言开发
- Zinx 网络框架
- 六边形架构（端口与适配器架构）
- DNY 协议（设备通信协议）

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
import (
    "github.com/bujia-iot/iot-zinx/pkg/constants"
    "github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
)

// 使用协议常量
command := constants.CmdDeviceRegister // 设备注册命令

// 构建充电控制包
packet := dny_protocol.BuildChargeControlPacket(
    physicalID, messageID, rateMode, balance,
    portNumber, chargeCommand, chargeDuration,
    orderNumber, maxChargeDuration, maxPower, qrCodeLight,
)

// 解析DNY协议消息
result := dny_protocol.ParseDNYMessage(rawData)
if result.Error == nil {
    // 处理解析结果
    deviceID := result.PhysicalID
    command := result.Command
}
```

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
- `HeartbeatHandler`：标准心跳包处理器 (0x01)
- `MainHeartbeatHandler`：主机心跳包处理器 (0x11)
- `DeviceStatusHandler`：设备状态处理器 (0x21)
- `GetServerTimeHandler`：获取服务器时间处理器 (0x12/0x22)
- `SwipeCardHandler`：刷卡请求处理器 (0x02)
- `ChargeControlHandler`：充电控制处理器 (0x82)
- `ICCIDHandler`：ICCID 识别处理器
- `LinkHeartbeatHandler`：Link 心跳处理器
- `NonDNYDataHandler`：非 DNY 协议数据处理器
- `ParameterSettingHandler`：参数设置处理器
- `PowerHeartbeatHandler`：电源心跳处理器
- `SettlementHandler`：结算处理器

### 核心组件

- `connection_hooks.go`：连接生命周期钩子函数，处理设备连接建立和断开
- `packet.go`：数据包处理器，负责 DNY 协议数据的封包和解包
- `device_monitor.go`：设备状态监控器，监控设备心跳状态
- `command_manager.go`：命令管理器，管理发送命令的确认和超时重发
- `monitor.go`：TCP 数据监视器，记录设备数据传输过程
- `raw_data_handler.go`：原始数据处理器，处理非结构化数据

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

1. **连接建立**：设备与网关建立 TCP 连接，网关创建连接对象并分配连接 ID
2. **初始化识别**：设备可能发送 ICCID(SIM 卡号)或 Link 心跳等初始化数据
3. **设备注册**：设备发送注册请求(0x20)，网关解析设备信息并完成注册
4. **心跳保活**：设备定期发送心跳包(0x01/0x11/0x21)，网关更新设备状态
5. **业务交互**：设备发送业务请求(如刷卡 0x02)或网关下发控制命令(如充电控制 0x82)
6. **连接监控**：网关监控设备心跳状态，自动清理超时连接
7. **连接断开**：设备主动断开连接或网关检测到连接超时，释放连接资源

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
