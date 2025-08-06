# TCP连接管理模块API参考

## 📋 概述

本文档提供了统一TCP管理器的完整API参考，包括所有公开接口、数据结构和使用示例。

## 🏗️ 核心接口

### IUnifiedTCPManager

统一TCP管理器的主要接口，提供所有TCP连接、会话和设备管理功能。

```go
type IUnifiedTCPManager interface {
    // === 连接管理 ===
    RegisterConnection(conn ziface.IConnection) (*ConnectionSession, error)
    UnregisterConnection(connID uint64) error
    GetConnection(connID uint64) (*ConnectionSession, bool)
    
    // === 设备管理 ===
    RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error
    RegisterDeviceWithDetails(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16, directMode bool) error
    UnregisterDevice(deviceID string) error
    
    // === 会话查询 ===
    GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool)
    GetSessionByConnID(connID uint64) (*ConnectionSession, bool)
    GetAllSessions() map[string]*ConnectionSession
    
    // === 状态管理 ===
    UpdateHeartbeat(deviceID string) error
    UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error
    
    // === 统计信息 ===
    GetStats() *TCPManagerStats
    
    // === 生命周期管理 ===
    Start() error
    Stop() error
    Cleanup() error
}
```

## 📊 数据结构

### ConnectionSession

统一的连接会话数据结构，整合了连接、设备和状态信息。

```go
type ConnectionSession struct {
    // === 核心标识 ===
    SessionID  string `json:"session_id"`  // 会话ID（唯一标识）
    ConnID     uint64 `json:"conn_id"`     // 连接ID
    DeviceID   string `json:"device_id"`   // 设备ID
    PhysicalID string `json:"physical_id"` // 物理ID
    ICCID      string `json:"iccid"`       // SIM卡号
    
    // === 连接信息 ===
    Connection ziface.IConnection `json:"-"`           // TCP连接对象
    RemoteAddr string             `json:"remote_addr"` // 远程地址
    
    // === 设备属性 ===
    DeviceType    uint16 `json:"device_type"`    // 设备类型
    DeviceVersion string `json:"device_version"` // 设备版本
    DirectMode    bool   `json:"direct_mode"`    // 是否直连模式
    
    // === 状态信息 ===
    Status        constants.DeviceStatus `json:"status"`         // 设备状态
    LastHeartbeat time.Time              `json:"last_heartbeat"` // 最后心跳时间
    LastActivity  time.Time              `json:"last_activity"`  // 最后活动时间
    CreatedAt     time.Time              `json:"created_at"`     // 创建时间
    UpdatedAt     time.Time              `json:"updated_at"`     // 更新时间
    
    // === 统计信息 ===
    MessageCount  int64 `json:"message_count"`  // 消息数量
    BytesReceived int64 `json:"bytes_received"` // 接收字节数
    BytesSent     int64 `json:"bytes_sent"`     // 发送字节数
}
```

### TCPManagerStats

TCP管理器统计信息结构。

```go
type TCPManagerStats struct {
    TotalConnections   int64     `json:"total_connections"`    // 总连接数
    ActiveConnections  int64     `json:"active_connections"`   // 活跃连接数
    TotalDevices       int64     `json:"total_devices"`        // 总设备数
    OnlineDevices      int64     `json:"online_devices"`       // 在线设备数
    TotalDeviceGroups  int64     `json:"total_device_groups"`  // 总设备组数
    LastConnectionAt   time.Time `json:"last_connection_at"`   // 最后连接时间
    LastRegistrationAt time.Time `json:"last_registration_at"` // 最后注册时间
    LastUpdateAt       time.Time `json:"last_update_at"`       // 最后更新时间
}
```

## 🔧 API详细说明

### 连接管理

#### RegisterConnection

注册新的TCP连接。

```go
func (m *UnifiedTCPManager) RegisterConnection(conn ziface.IConnection) (*ConnectionSession, error)
```

**参数:**
- `conn`: TCP连接对象

**返回值:**
- `*ConnectionSession`: 创建的连接会话
- `error`: 错误信息

**示例:**
```go
tcpManager := core.GetGlobalUnifiedTCPManager()
session, err := tcpManager.RegisterConnection(conn)
if err != nil {
    return fmt.Errorf("注册连接失败: %w", err)
}
```

#### UnregisterConnection

注销TCP连接。

```go
func (m *UnifiedTCPManager) UnregisterConnection(connID uint64) error
```

**参数:**
- `connID`: 连接ID

**返回值:**
- `error`: 错误信息

**示例:**
```go
err := tcpManager.UnregisterConnection(12345)
if err != nil {
    logger.Error("注销连接失败", "connID", 12345, "error", err)
}
```

#### GetConnection

获取连接会话。

```go
func (m *UnifiedTCPManager) GetConnection(connID uint64) (*ConnectionSession, bool)
```

**参数:**
- `connID`: 连接ID

**返回值:**
- `*ConnectionSession`: 连接会话
- `bool`: 是否存在

**示例:**
```go
session, exists := tcpManager.GetConnection(12345)
if !exists {
    return fmt.Errorf("连接不存在: %d", 12345)
}
```

### 设备管理

#### RegisterDevice

注册设备（简化版本）。

```go
func (m *UnifiedTCPManager) RegisterDevice(conn ziface.IConnection, deviceID, physicalID, iccid string) error
```

**参数:**
- `conn`: TCP连接对象
- `deviceID`: 设备ID
- `physicalID`: 物理ID
- `iccid`: SIM卡号

**返回值:**
- `error`: 错误信息

**示例:**
```go
err := tcpManager.RegisterDevice(conn, "DEVICE_001", "04A228CD", "89860000000000000001")
if err != nil {
    return fmt.Errorf("注册设备失败: %w", err)
}
```

#### RegisterDeviceWithDetails

注册设备（完整版本）。

```go
func (m *UnifiedTCPManager) RegisterDeviceWithDetails(conn ziface.IConnection, deviceID, physicalID, iccid, version string, deviceType uint16, directMode bool) error
```

**参数:**
- `conn`: TCP连接对象
- `deviceID`: 设备ID
- `physicalID`: 物理ID
- `iccid`: SIM卡号
- `version`: 设备版本
- `deviceType`: 设备类型
- `directMode`: 是否直连模式

**返回值:**
- `error`: 错误信息

**示例:**
```go
err := tcpManager.RegisterDeviceWithDetails(
    conn, "DEVICE_001", "04A228CD", "89860000000000000001", 
    "v1.0.0", 1, false,
)
```

#### UnregisterDevice

注销设备。

```go
func (m *UnifiedTCPManager) UnregisterDevice(deviceID string) error
```

**参数:**
- `deviceID`: 设备ID

**返回值:**
- `error`: 错误信息

**示例:**
```go
err := tcpManager.UnregisterDevice("DEVICE_001")
if err != nil {
    logger.Error("注销设备失败", "deviceID", "DEVICE_001", "error", err)
}
```

### 会话查询

#### GetSessionByDeviceID

通过设备ID获取会话。

```go
func (m *UnifiedTCPManager) GetSessionByDeviceID(deviceID string) (*ConnectionSession, bool)
```

**参数:**
- `deviceID`: 设备ID

**返回值:**
- `*ConnectionSession`: 连接会话
- `bool`: 是否存在

**示例:**
```go
session, exists := tcpManager.GetSessionByDeviceID("DEVICE_001")
if !exists {
    return fmt.Errorf("设备会话不存在: %s", "DEVICE_001")
}
```

#### GetSessionByConnID

通过连接ID获取会话。

```go
func (m *UnifiedTCPManager) GetSessionByConnID(connID uint64) (*ConnectionSession, bool)
```

**参数:**
- `connID`: 连接ID

**返回值:**
- `*ConnectionSession`: 连接会话
- `bool`: 是否存在

**示例:**
```go
session, exists := tcpManager.GetSessionByConnID(12345)
if exists {
    logger.Info("找到会话", "deviceID", session.DeviceID)
}
```

#### GetAllSessions

获取所有会话。

```go
func (m *UnifiedTCPManager) GetAllSessions() map[string]*ConnectionSession
```

**返回值:**
- `map[string]*ConnectionSession`: 设备ID到会话的映射

**示例:**
```go
allSessions := tcpManager.GetAllSessions()
for deviceID, session := range allSessions {
    logger.Info("会话信息", "deviceID", deviceID, "connID", session.ConnID)
}
```

### 状态管理

#### UpdateHeartbeat

更新设备心跳。

```go
func (m *UnifiedTCPManager) UpdateHeartbeat(deviceID string) error
```

**参数:**
- `deviceID`: 设备ID

**返回值:**
- `error`: 错误信息

**示例:**
```go
err := tcpManager.UpdateHeartbeat("DEVICE_001")
if err != nil {
    logger.Warn("更新心跳失败", "deviceID", "DEVICE_001", "error", err)
}
```

#### UpdateDeviceStatus

更新设备状态。

```go
func (m *UnifiedTCPManager) UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error
```

**参数:**
- `deviceID`: 设备ID
- `status`: 设备状态

**返回值:**
- `error`: 错误信息

**示例:**
```go
err := tcpManager.UpdateDeviceStatus("DEVICE_001", constants.DeviceStatus("online"))
if err != nil {
    logger.Error("更新设备状态失败", "deviceID", "DEVICE_001", "error", err)
}
```

### 统计信息

#### GetStats

获取统计信息。

```go
func (m *UnifiedTCPManager) GetStats() *TCPManagerStats
```

**返回值:**
- `*TCPManagerStats`: 统计信息

**示例:**
```go
stats := tcpManager.GetStats()
logger.Info("系统统计",
    "totalConnections", stats.TotalConnections,
    "activeConnections", stats.ActiveConnections,
    "totalDevices", stats.TotalDevices,
    "onlineDevices", stats.OnlineDevices,
)
```

### 生命周期管理

#### Start

启动TCP管理器。

```go
func (m *UnifiedTCPManager) Start() error
```

**返回值:**
- `error`: 错误信息

**示例:**
```go
tcpManager := core.GetGlobalUnifiedTCPManager()
if err := tcpManager.Start(); err != nil {
    return fmt.Errorf("启动TCP管理器失败: %w", err)
}
```

#### Stop

停止TCP管理器。

```go
func (m *UnifiedTCPManager) Stop() error
```

**返回值:**
- `error`: 错误信息

**示例:**
```go
if err := tcpManager.Stop(); err != nil {
    logger.Error("停止TCP管理器失败", "error", err)
}
```

#### Cleanup

清理资源。

```go
func (m *UnifiedTCPManager) Cleanup() error
```

**返回值:**
- `error`: 错误信息

**示例:**
```go
if err := tcpManager.Cleanup(); err != nil {
    logger.Error("清理资源失败", "error", err)
}
```

## 🔧 工具函数

### GetGlobalUnifiedTCPManager

获取全局统一TCP管理器实例。

```go
func GetGlobalUnifiedTCPManager() IUnifiedTCPManager
```

**返回值:**
- `IUnifiedTCPManager`: 统一TCP管理器实例

**示例:**
```go
tcpManager := core.GetGlobalUnifiedTCPManager()
```

### InitializeAllAdapters

初始化所有适配器。

```go
func InitializeAllAdapters()
```

**示例:**
```go
// 异步初始化适配器
go func() {
    time.Sleep(100 * time.Millisecond)
    core.InitializeAllAdapters()
}()
```

## 📚 相关文档

- [迁移指南](migration-guide.md) - 从旧架构迁移的详细步骤
- [最佳实践](best-practices.md) - 使用建议和最佳实践
- [架构文档](../issues/TCP连接管理模块统一重构.md) - 详细的架构设计

---

**文档版本**: v1.0  
**最后更新**: 2025-01-08  
**适用版本**: IoT-Zinx v2.0+
