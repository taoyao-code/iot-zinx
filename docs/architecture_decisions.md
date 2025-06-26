# IoT-Zinx 架构决策记录

## 概述

本文档记录了 IoT-Zinx 系统中关键架构决策的背景、原因和实施细节，为团队提供清晰的技术上下文。

## 决策记录

### ADR-001: 串联设备模型架构选择

**日期**: 2025-06-26  
**状态**: 已实施  
**决策者**: 系统架构团队  

#### 背景

IoT-Zinx 系统需要处理复杂的设备连接场景：
- 一个带 ICCID 的主设备通过 TCP 连接到服务器
- 该主设备通过串口连接一个或多个从设备
- 所有设备共享同一个 TCP 连接进行通信
- 需要支持对任意子设备的精确命令发送

#### 问题

系统中存在两种冲突的设备组管理模型：
1. `pkg/monitor/device_group.go` - 基于 ICCID 的设备组管理
2. `pkg/core/connection_device_group.go` - 基于 TCP 连接的设备组管理

这种冲突导致：
- 设备查找失败
- 命令发送错误
- 架构不一致
- 维护困难

#### 决策

**选择 `pkg/core/connection_device_group.go` 作为唯一的设备组管理架构**

#### 理由

1. **业务模型匹配**
   - 完美匹配"串联设备"的业务场景
   - 一个 TCP 连接对应一个设备组
   - 设备组内的所有设备共享同一连接

2. **技术优势**
   - 连接与设备组一一对应，逻辑清晰
   - 支持通过设备 ID 快速查找对应的 TCP 连接
   - 避免了 ICCID 与连接管理的复杂映射

3. **实现简洁**
   - 统一的设备注册：`RegisterDevice(conn, deviceID, physicalID, iccid)`
   - 统一的设备查找：`GetConnectionByDeviceID(deviceID)`
   - 统一的心跳处理：`HandleHeartbeat(deviceID, conn)`

#### 实施细节

1. **废弃冲突架构**
   ```
   pkg/monitor/device_group.go → device_group.go.deprecated
   ```

2. **统一设备注册**
   ```go
   // 设备注册处理器
   unifiedSystem.GroupManager.RegisterDevice(conn, deviceId, physicalIdStr, iccidFromProp)
   
   // 心跳处理器
   unifiedSystem.GroupManager.HandleHeartbeat(deviceId, conn)
   ```

3. **统一数据发送**
   ```go
   // 通过设备组管理器查找连接
   conn, exists := groupManager.GetConnectionByDeviceID(deviceID)
   ```

#### 影响

- ✅ 解决设备查找失败问题
- ✅ 统一命令发送逻辑
- ✅ 简化架构，提高可维护性
- ✅ 支持串联设备的精确控制

---

### ADR-002: 统一数据发送服务

**日期**: 2025-06-26  
**状态**: 已实施  
**决策者**: 系统架构团队  

#### 背景

系统中存在多个分散的数据发送逻辑：
- API 处理器直接调用协议发送
- 不同模块使用不同的设备查找方式
- 缺乏统一的发送日志和统计

#### 决策

**创建 `UnifiedDataSender` 作为所有下行命令的唯一出口**

#### 实施

1. **统一发送接口**
   ```go
   sender := service.GetGlobalUnifiedSender()
   result, err := sender.SendDataToDevice(deviceID, commandID, payload)
   ```

2. **集成设备组管理**
   - 自动通过设备组管理器查找 TCP 连接
   - 支持串联设备的精确发送

3. **完整的发送统计**
   - 发送成功率统计
   - 错误记录和分析
   - 性能监控

#### 影响

- ✅ 统一所有 API 的发送逻辑
- ✅ 提供完整的发送统计和监控
- ✅ 简化新功能的开发
- ✅ 提高系统可观测性

---

### ADR-003: 专用通信日志系统

**日期**: 2025-06-26  
**状态**: 已实施  
**决策者**: 系统架构团队  

#### 背景

系统缺乏专门的通信日志记录：
- 发送和接收数据分散在不同日志中
- 难以追踪完整的通信流程
- 调试困难

#### 决策

**建立专用的通信日志系统 `logs/communication.log`**

#### 实施

1. **专用日志记录器**
   ```go
   logger.InitCommunicationLogger(logDir)
   communicationLog := logger.GetCommunicationLogger()
   ```

2. **统一日志格式**
   ```
   [SEND] 设备ID: 04A228CD, 命令: 0x96, 消息ID: 0x1234
   [RECV] 连接ID: 12345, 数据类型: DNY_STANDARD, 设备ID: 04A228CD
   ```

3. **完整覆盖**
   - 发送日志：在 `UnifiedDataSender` 中记录
   - 接收日志：在 `dny_decoder.go` 中记录

#### 影响

- ✅ 提供完整的通信追踪
- ✅ 简化问题诊断
- ✅ 支持通信审计
- ✅ 提高系统可调试性

---

## 迁移指南

### 从旧架构迁移

1. **设备组管理**
   ```go
   // 旧方式
   monitor.GetGlobalDeviceGroupManager()
   
   // 新方式
   core.GetGlobalConnectionGroupManager()
   ```

2. **设备注册**
   ```go
   // 旧方式
   deviceGroupManager.AddDeviceToGroup(deviceId, iccid, session)
   
   // 新方式
   groupManager.RegisterDevice(conn, deviceID, physicalID, iccid)
   ```

3. **设备查找**
   ```go
   // 旧方式
   conn, exists := deviceService.GetDeviceConnection(deviceID)
   
   // 新方式
   conn, exists := groupManager.GetConnectionByDeviceID(deviceID)
   ```

4. **命令发送**
   ```go
   // 旧方式
   pkg.Protocol.SendDNYResponse(conn, physicalID, messageID, command, data)
   
   // 新方式
   sender := service.GetGlobalUnifiedSender()
   result, err := sender.SendDataToDevice(deviceID, command, data)
   ```

### 注意事项

1. **向后兼容性**
   - 废弃的文件保留为 `.deprecated` 后缀
   - 旧接口暂时保留，但标记为废弃

2. **测试验证**
   - 验证设备注册流程
   - 验证心跳处理
   - 验证命令发送
   - 验证通信日志记录

3. **性能监控**
   - 监控发送成功率
   - 监控响应时间
   - 监控错误率

---

## 总结

通过这次架构重构，IoT-Zinx 系统实现了：

1. **架构统一**: 消除了冲突的设备组管理模型
2. **逻辑清晰**: 建立了清晰的串联设备处理流程
3. **功能完整**: 提供了统一的数据发送和日志记录
4. **可维护性**: 简化了代码结构，提高了可维护性
5. **可观测性**: 增强了系统的监控和调试能力

这些改进为系统的稳定运行和未来扩展奠定了坚实的基础。
