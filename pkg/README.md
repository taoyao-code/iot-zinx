# pkg 目录

## 简介

`pkg` 目录包含了可重用的工具和库代码，这些代码被设计为与具体业务逻辑解耦，可以被项目内部或外部其他项目引用。这是从原`internal/infrastructure/zinx_server`目录重构而来的工具类集合，经过了模块化和接口抽象的优化。

## 目录结构

```
pkg/
├── export.go        # 统一导出接口
├── init.go          # 包初始化和依赖设置
├── network/         # 网络通信相关工具
│   ├── command_manager.go      # 命令管理器
│   ├── connection_hooks.go     # 连接钩子
│   ├── heartbeat.go            # 心跳机制
│   ├── interface.go            # 接口定义
│   └── raw_data_handler.go     # 原始数据处理器
├── protocol/        # 协议处理相关工具
│   ├── dny_packet.go           # DNY协议数据包处理器
│   ├── interface.go            # 接口定义
│   ├── parser.go               # 协议解析器
│   ├── sender.go               # 协议发送工具
│   └── raw_data_hook.go        # 原始数据处理钩子
├── monitor/         # 监控相关工具
│   ├── device_monitor.go       # 设备监控器
│   ├── interface.go            # 接口定义
│   └── tcp_monitor.go          # TCP连接监控器
└── utils/           # 通用工具类
    └── logger_adapter.go       # 日志适配器
```

## 功能模块

### 协议处理 (protocol)

- **DNY协议包处理**：提供DNY协议的封包和解包功能，包括校验和计算
- **协议解析**：解析DNY协议和十六进制数据的工具，支持可视化输出
- **协议发送**：发送DNY协议响应数据，支持命令确认和跟踪
- **原始数据处理**：处理ICCID、AT命令等原始数据的能力

### 网络通信 (network)

- **命令管理**：管理命令的发送、确认和超时重试，支持命令回调
- **连接钩子**：提供连接建立和关闭时的钩子函数，设置连接属性
- **心跳机制**：处理设备心跳包和超时检测，自动处理离线设备
- **原始数据处理**：处理不符合DNY协议的原始数据，如AT命令

### 监控 (monitor)

- **设备监控**：监控设备状态、心跳超时等，支持状态变更通知
- **TCP监控**：监控TCP连接的建立、数据传输和关闭，记录原始数据

### 工具类 (utils)

- **日志适配器**：适配Zinx框架的日志系统，统一日志格式

## 初始化

在使用pkg包之前，需要先初始化包之间的依赖关系：

```go
import "github.com/bujia-iot/iot-zinx/pkg"

func main() {
    // 初始化pkg包依赖关系
    pkg.InitPackages()
    
    // 现在可以使用pkg包的功能了
    // ...
}
```

`pkg.InitPackages()`会执行以下操作：
1. 设置Zinx使用自定义日志系统
2. 建立包之间的依赖关系
3. 启动命令管理器
4. 设置命令发送函数

## 使用方法

### 通过 export.go 使用（推荐）

推荐通过 `export.go` 中定义的导出接口来使用各个功能：

```go
import (
    "time"
    "github.com/bujia-iot/iot-zinx/pkg"
)

// 初始化
pkg.InitPackages()

// 使用协议相关功能
packet := pkg.Protocol.NewDNYDataPackFactory().NewDataPack(true)
pkg.Protocol.ParseDNYProtocol(data)
pkg.Protocol.SendDNYResponse(conn, physicalId, messageId, command, data)

// 使用网络相关功能
hooks := pkg.Network.NewConnectionHooks(
    60*time.Second,  // 读超时
    60*time.Second,  // 写超时
    120*time.Second, // KeepAlive周期
)
pkg.Network.GetCommandManager().RegisterCommand(conn, physicalId, messageId, command, data)

// 使用监控相关功能
monitor := pkg.Monitor.GetGlobalMonitor()
monitor.BindDeviceIdToConnection(deviceId, conn)
monitor.UpdateLastHeartbeatTime(conn)

// 使用工具类
pkg.Utils.SetupZinxLogger()
```

### 直接使用具体包

如果需要使用更多高级功能，可以直接导入具体的包：

```go
import (
    "github.com/bujia-iot/iot-zinx/pkg/protocol"
    "github.com/bujia-iot/iot-zinx/pkg/network"
    "github.com/bujia-iot/iot-zinx/pkg/monitor"
)

// 使用具体功能
rawHook := protocol.NewRawDataHook(protocol.DefaultRawDataHandler)
cmdMgr := network.GetCommandManager()
tcpMonitor := monitor.GetGlobalMonitor()
```

## 接口设计

各个模块都定义了清晰的接口，便于扩展和模拟测试：

- `protocol.IDataPackFactory` - 数据包工厂接口
- `network.ICommandManager` - 命令管理器接口
- `network.IConnectionHooks` - 连接钩子接口
- `monitor.IConnectionMonitor` - 连接监控接口
- `monitor.IDeviceMonitor` - 设备监控接口

## 从原zinx_server包迁移

如果您的代码仍在使用原`zinx_server`包，可以按照以下步骤迁移：

1. 引入pkg包并初始化：
```go
import "github.com/bujia-iot/iot-zinx/pkg"

// 初始化包依赖
pkg.InitPackages()
```

2. 替换函数调用：

| 原zinx_server函数                      | pkg包函数                                                 |
| -------------------------------------- | --------------------------------------------------------- |
| `zinx_server.SendDNYResponse`          | `pkg.Protocol.SendDNYResponse`                            |
| `zinx_server.ParseDNYProtocol`         | `pkg.Protocol.ParseDNYProtocol`                           |
| `zinx_server.GetCommandManager`        | `pkg.Network.GetCommandManager`                           |
| `zinx_server.GetGlobalMonitor`         | `pkg.Monitor.GetGlobalMonitor`                            |
| `zinx_server.UpdateLastHeartbeatTime`  | `pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime`  |
| `zinx_server.BindDeviceIdToConnection` | `pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection` |

3. 替换常量引用：

| 原常量                         | pkg包常量                       |
| ------------------------------ | ------------------------------- |
| `zinx_server.PropKeyICCID`     | 自定义常量如 `PropKeyICCID`     |
| `zinx_server.PropKeyDeviceId`  | 自定义常量如 `PropKeyDeviceId`  |
| `zinx_server.ConnStatusActive` | 自定义常量如 `ConnStatusActive` |

## 注意事项

1. 本目录下的代码应当保持与具体业务逻辑解耦
2. 避免在此目录下引用内部应用逻辑代码
3. 保持接口稳定，添加新功能时尽量不破坏现有接口
4. 每个模块都应有清晰的职责边界，避免功能重叠
5. 始终先调用`pkg.InitPackages()`来确保包间依赖关系正确设置
6. 如果您编写的是新代码，建议直接使用pkg包而不是使用兼容层 