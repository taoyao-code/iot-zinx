# 设备组管理系统

## 概述

设备组管理系统支持一个ICCID（SIM卡号）下管理多个设备的并发在线场景，解决了原有架构中设备会话冲突的问题。

## 架构设计

### 原有架构问题
- 1个ICCID → 1个DeviceSession（后注册的设备会覆盖前面的）
- 无法支持同一SIM卡下的多设备并发

### 新架构优势
- 1个ICCID → 1个DeviceGroup → 多个DeviceSession
- 支持多设备并发在线
- 独立的设备状态管理
- 精确的消息路由

## 核心组件

### 1. DeviceGroup（设备组）
```go
type DeviceGroup struct {
    ICCID     string                    // SIM卡号
    Devices   map[string]*DeviceSession // DeviceID -> DeviceSession
    CreatedAt time.Time                 // 创建时间
    UpdatedAt time.Time                 // 最后更新时间
    mutex     sync.RWMutex              // 读写锁
}
```

### 2. DeviceGroupManager（设备组管理器）
```go
type DeviceGroupManager struct {
    groups sync.Map // ICCID -> *DeviceGroup
}
```

### 3. 增强的SessionManager
- 集成设备组管理
- 支持多设备会话查询
- 自动维护设备组关系

## 使用方法

### 1. 获取设备组管理器
```go
groupManager := pkg.Monitor.GetDeviceGroupManager()
```

### 2. 创建或获取设备组
```go
// 自动创建设备组（如果不存在）
group := groupManager.GetOrCreateGroup("898604D9162390488297")

// 仅获取设备组
group, exists := groupManager.GetGroup("898604D9162390488297")
```

### 3. 设备会话管理
```go
// 获取ICCID下的所有设备
devices := pkg.Monitor.GetSessionsByICCID("898604D9162390488297")

// 获取特定设备会话
session, exists := pkg.Monitor.GetDeviceSession("04A26CF3")

// 创建设备会话（自动加入设备组）
session := pkg.Monitor.CreateDeviceSession("04A26CF3", conn)
```

### 4. 设备组操作
```go
// 向设备组广播消息
successCount := pkg.Monitor.BroadcastToGroup("898604D9162390488297", data)

// 获取设备组统计信息
stats := pkg.Monitor.GetGroupStatistics()
```

## 设备注册流程

### 新的注册逻辑
1. **检查设备重连**：优先检查该设备是否已有会话
2. **多设备支持**：检查同一ICCID下是否有其他设备
3. **创建新会话**：为新设备创建独立会话
4. **自动分组**：自动将设备加入对应的设备组

```go
// 设备注册处理示例
if existSession, exists := sessionManager.GetSession(deviceIdStr); exists {
    // 设备重连，恢复现有会话
    session = existSession
    isReconnect = true
    sessionManager.ResumeSession(deviceIdStr, conn)
} else {
    // 新设备注册，检查同一ICCID下的其他设备
    existingDevices := sessionManager.GetAllSessionsByICCID(iccid)
    
    if len(existingDevices) > 0 {
        logger.Info("同一ICCID下发现其他设备，支持多设备并发")
    }
    
    // 创建新的设备会话（自动加入设备组）
    session = sessionManager.CreateSession(deviceIdStr, conn)
}
```

## 连接断开处理

### 智能断开管理
- 设备断开时，只影响该设备的会话
- 同一ICCID下的其他设备继续正常工作
- 自动清理空的设备组

```go
// 连接断开处理示例
if session, exists := sessionManager.GetSession(deviceID); exists {
    // 挂起设备会话
    sessionManager.SuspendSession(deviceID)
    
    // 检查同一ICCID下的其他设备
    if session.ICCID != "" {
        allDevices := sessionManager.GetAllSessionsByICCID(session.ICCID)
        activeDevices := 0
        
        for otherDeviceID, otherSession := range allDevices {
            if otherDeviceID != deviceID && otherSession.Status == constants.DeviceStatusOnline {
                activeDevices++
            }
        }
        
        logger.Info("设备断开连接，ICCID下仍有其他活跃设备", activeDevices)
    }
}
```

## API接口

### 设备组管理API
```go
// 获取设备组
group, exists := pkg.Monitor.GetDeviceGroup(iccid)

// 添加设备到组
pkg.Monitor.AddDeviceToGroup(iccid, deviceID, session)

// 从组中移除设备
pkg.Monitor.RemoveDeviceFromGroup(iccid, deviceID)

// 向组广播消息
successCount := pkg.Monitor.BroadcastToGroup(iccid, data)

// 获取组统计信息
stats := pkg.Monitor.GetGroupStatistics()
```

### 设备会话管理API
```go
// 创建设备会话
session := pkg.Monitor.CreateDeviceSession(deviceID, conn)

// 获取设备会话
session, exists := pkg.Monitor.GetDeviceSession(deviceID)

// 获取ICCID下的所有设备会话
devices := pkg.Monitor.GetSessionsByICCID(iccid)

// 挂起设备会话
success := pkg.Monitor.SuspendDeviceSession(deviceID)

// 恢复设备会话
success := pkg.Monitor.ResumeDeviceSession(deviceID, conn)

// 移除设备会话
success := pkg.Monitor.RemoveDeviceSession(deviceID)
```

## 监控和统计

### 设备组统计
```go
stats := pkg.Monitor.GetGroupStatistics()
// 返回：
// {
//     "totalGroups": 5,
//     "totalDevices": 12
// }
```

### 会话统计
```go
sessionManager := pkg.Monitor.GetSessionManager()
stats := sessionManager.GetSessionStatistics()
// 返回：
// {
//     "totalSessions": 12,
//     "activeSessions": 10,
//     "suspendedSessions": 2,
//     "deviceGroups": {
//         "totalGroups": 5,
//         "totalDevices": 12
//     }
// }
```

## 最佳实践

### 1. 设备标识管理
- 使用PhysicalID作为设备的唯一标识
- ICCID作为设备组的标识
- 保持设备ID的一致性

### 2. 消息路由
- 使用精确的设备ID进行点对点通信
- 使用ICCID进行设备组广播
- 避免不必要的广播操作

### 3. 错误处理
- 检查设备是否在线再发送消息
- 处理设备组为空的情况
- 监控设备会话的生命周期

### 4. 性能优化
- 使用读写锁保护并发访问
- 定期清理过期会话
- 监控设备组的大小

## 故障排查

### 常见问题
1. **设备无法注册**：检查ICCID格式和长度
2. **消息发送失败**：确认设备在线状态
3. **设备组为空**：检查设备会话是否正确创建
4. **内存泄漏**：确保正确清理断开的设备会话

### 调试方法
- 查看设备组统计信息
- 检查设备会话状态
- 监控连接建立和断开日志
- 使用设备ID和ICCID进行精确查询 