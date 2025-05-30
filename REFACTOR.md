# 项目重构说明

## 重构目标

本次重构的主要目标是将工具类文件整理到`pkg`目录下，在不改变业务逻辑的情况下提高代码的模块化和可维护性。

## 完成工作

1. 分析了原有的`internal/infrastructure/zinx_server`目录下的工具类文件，包括`packet.go`、`heartbeat.go`、`connection_hooks.go`、`monitor.go`等文件。

2. 创建了合理的目录结构用于组织这些工具类：
   - `pkg/protocol` - 存放协议相关工具类
   - `pkg/network` - 存放网络通信相关工具类
   - `pkg/monitor` - 存放监控相关工具类
   - `pkg/utils` - 存放通用工具类

3. 将原有功能迁移到新目录：
   - 将`packet.go`和`raw_data_hook.go`迁移到`pkg/protocol`目录
   - 将`command_manager.go`、`heartbeat.go`、`connection_hooks.go`、`raw_data_handler.go`迁移到`pkg/network`目录
   - 将`monitor.go`和`device_monitor.go`迁移到`pkg/monitor`目录
   - 将`logger_adapter.go`迁移到`pkg/utils`目录

4. 为每个模块定义了接口文件：
   - 在`pkg/network/interface.go`中定义了`PacketHandler`、`ICommandManager`和`IConnectionHooks`接口
   - 在`pkg/monitor/interface.go`中定义了`IConnectionMonitor`和`IDeviceMonitor`接口
   - 在`pkg/protocol/interface.go`中定义了`IDataPackFactory`接口和实现类

5. 确保各个具体实现类实现了相应的接口，如`TCPMonitor`实现了`IConnectionMonitor`接口，`DeviceMonitor`实现了`IDeviceMonitor`接口，`CommandManager`实现了`ICommandManager`接口。

6. 创建了`pkg/export.go`文件，提供了统一的导出接口，便于用户快速使用相关功能。

7. 创建了`pkg/init.go`文件，用于在应用启动时设置各个包之间的依赖关系。

8. 更新了`internal/ports/tcp_server.go`文件，使用新的`pkg`包中的工具类替代原有的`zinx_server`包中的工具类。

9. 创建了示例代码`examples/basic_usage.go`，演示如何在实际项目中使用重构后的工具类。

10. 创建了兼容层`internal/infrastructure/zinx_server/handlers/imports.go`，使旧代码能够顺利过渡到新架构。

11. 完善了`pkg/README.md`文档，详细说明了各个模块的功能和使用方法。

## 文件比较

| 旧文件                                                    | 新文件                            |
| --------------------------------------------------------- | --------------------------------- |
| `internal/infrastructure/zinx_server/packet.go`           | `pkg/protocol/dny_packet.go`      |
| `internal/infrastructure/zinx_server/raw_data_hook.go`    | `pkg/protocol/raw_data_hook.go`   |
| `internal/infrastructure/zinx_server/command_manager.go`  | `pkg/network/command_manager.go`  |
| `internal/infrastructure/zinx_server/heartbeat.go`        | `pkg/network/heartbeat.go`        |
| `internal/infrastructure/zinx_server/connection_hooks.go` | `pkg/network/connection_hooks.go` |
| `internal/infrastructure/zinx_server/raw_data_handler.go` | `pkg/network/raw_data_handler.go` |
| `internal/infrastructure/zinx_server/monitor.go`          | `pkg/monitor/tcp_monitor.go`      |
| `internal/infrastructure/zinx_server/device_monitor.go`   | `pkg/monitor/device_monitor.go`   |
| `internal/infrastructure/zinx_server/logger_adapter.go`   | `pkg/utils/logger_adapter.go`     |

## 使用说明

推荐通过`pkg/export.go`中定义的导出接口来使用各个功能：

```go
import "github.com/bujia-iot/iot-zinx/pkg"

// 初始化包依赖关系
pkg.InitPackages()

// 使用协议相关功能
packet := pkg.Protocol.NewDNYDataPackFactory().NewDataPack(true)

// 使用网络相关功能
hooks := pkg.Network.NewConnectionHooks(
    60*time.Second,  // 读超时
    60*time.Second,  // 写超时
    120*time.Second, // KeepAlive周期
)

// 使用监控相关功能
monitor := pkg.Monitor.GetGlobalMonitor()

// 使用工具类
pkg.Utils.SetupZinxLogger()
```

详细使用说明请参考：
- [pkg/README.md](pkg/README.md) - 了解`pkg`目录的结构和使用方法
- [examples/README.md](examples/README.md) - 查看示例代码的使用说明

## 重构优势

1. **模块化**: 将相关功能组织到不同的模块中，每个模块有明确的职责。
2. **接口隔离**: 通过接口定义明确了各个模块的功能边界。
3. **可测试性**: 接口化设计使得各个模块可以独立测试。
4. **可维护性**: 清晰的目录结构和命名约定使代码更易于维护。
5. **可扩展性**: 可以轻松添加新的实现而不影响现有代码。
6. **可复用性**: `pkg`目录中的代码可以被其他项目重用。

## 重构状态

- [x] 创建pkg目录结构
- [x] 定义模块接口
- [x] 迁移核心功能
- [x] 创建统一导出接口
- [x] 初始化依赖关系
- [x] 更新使用示例
- [x] 创建兼容层
- [x] 完善文档说明
- [ ] 移除原zinx_server目录
- [ ] 更新所有handler代码
- [ ] 移除兼容层

## 后续工作

1. **代码清理**:
   - 移除`internal/infrastructure/zinx_server`目录中的原始代码，全部使用`pkg`包中的工具类。
   - 移除`internal/infrastructure/zinx_server/handlers/imports.go`兼容层文件。

2. **迁移handlers**:
   - 更新`handlers`目录中的所有处理器，直接使用`pkg`包，不再通过兼容层。

3. **测试覆盖**:
   - 为`pkg`目录下的所有关键功能添加单元测试。
   - 确保测试覆盖率达到70%以上。

4. **文档更新**:
   - 更新项目主README.md，说明项目架构变化。
   - 为每个模块添加详细的API文档。

5. **性能优化**:
   - 分析新架构下的性能瓶颈。
   - 优化关键路径上的代码。

6. **代码规范**:
   - 统一代码风格，遵循Go语言最佳实践。
   - 使用go lint等工具检查代码质量。

## 迁移指南

如果您的代码仍在使用原`zinx_server`包，请按照以下步骤迁移：

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

| 原常量                         | 建议替代方案                                      |
| ------------------------------ | ------------------------------------------------- |
| `zinx_server.PropKeyICCID`     | 自定义常量如 `const PropKeyICCID = "ICCID"`       |
| `zinx_server.PropKeyDeviceId`  | 自定义常量如 `const PropKeyDeviceId = "DeviceId"` |
| `zinx_server.ConnStatusActive` | 自定义常量如 `const ConnStatusActive = "active"`  |

## 注意事项

1. 本次重构保持了原有的业务逻辑不变，只是优化了代码组织结构。
2. 原有的`zinx_server`包中的代码暂时保留，待新代码稳定后可以考虑移除。
3. 使用示例代码前，请确保已经理解了相关的接口和实现类。
4. 始终先调用`pkg.InitPackages()`来确保包间依赖关系正确设置。
5. 如果您编写的是新代码，建议直接使用pkg包而不是使用兼容层。 