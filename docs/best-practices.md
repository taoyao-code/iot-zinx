# TCP 连接管理模块最佳实践指南

## 📋 概述

本文档提供了使用统一 TCP 管理器架构的最佳实践，帮助开发者充分利用新架构的优势。

## 🎯 核心原则

### 1. 单一数据源原则

- **始终使用统一 TCP 管理器**作为唯一的数据源
- **避免直接访问**底层存储结构
- **通过标准接口**进行所有数据操作

### 2. 生命周期管理原则

- **连接注册** → **设备注册** → **状态管理** → **连接注销**
- **确保完整的生命周期**，避免资源泄漏
- **使用 defer 语句**确保资源清理

### 3. 错误处理原则

- **检查所有返回的错误**
- **提供有意义的错误信息**
- **实现优雅的错误恢复**

## 🔧 使用模式

### 1. 初始化模式

#### ✅ 推荐方式

```go
func InitializeSystem() error {
    // 获取统一TCP管理器
    tcpManager := core.GetGlobalUnifiedTCPManager()

    // 启动管理器
    if err := tcpManager.Start(); err != nil {
        return fmt.Errorf("启动TCP管理器失败: %w", err)
    }

    // 异步初始化适配器（避免循环导入）
    go func() {
        time.Sleep(100 * time.Millisecond)
        core.InitializeAllAdapters()
    }()

    return nil
}

func ShutdownSystem() error {
    tcpManager := core.GetGlobalUnifiedTCPManager()
    return tcpManager.Stop()
}
```

#### ❌ 避免的方式

```go
// 不要直接初始化多个管理器
func InitializeSystem() {
    InitializeConnectionManager()    // ❌ 已弃用
    InitializeSessionManager()       // ❌ 已弃用
    InitializeStateManager()         // ❌ 已弃用
}
```

### 2. 连接管理模式

#### ✅ 推荐方式

```go
func HandleNewConnection(conn ziface.IConnection) error {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    // 1. 注册连接
    session, err := tcpManager.RegisterConnection(conn)
    if err != nil {
        return fmt.Errorf("注册连接失败: %w", err)
    }

    // 2. 设置连接关闭回调
    conn.AddCloseCallback(nil, nil, func() {
        tcpManager.UnregisterConnection(conn.GetConnID())
    })

    logger.Info("连接注册成功", "connID", session.ConnID, "sessionID", session.SessionID)
    return nil
}

func HandleDeviceRegistration(conn ziface.IConnection, deviceID, physicalID, iccid string) error {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    // 注册设备
    err := tcpManager.RegisterDevice(conn, deviceID, physicalID, iccid)
    if err != nil {
        return fmt.Errorf("注册设备失败: %w", err)
    }

    // 更新设备状态为在线
    err = tcpManager.UpdateDeviceStatus(deviceID, constants.DeviceStatus("online"))
    if err != nil {
        logger.Warn("更新设备状态失败", "deviceID", deviceID, "error", err)
    }

    logger.Info("设备注册成功", "deviceID", deviceID, "physicalID", physicalID)
    return nil
}
```

#### ❌ 避免的方式

```go
// 不要使用多个管理器
func HandleNewConnection(conn ziface.IConnection) error {
    connManager := GetGlobalConnectionManager()     // ❌ 已弃用
    sessionManager := GetGlobalSessionManager()     // ❌ 已弃用

    connManager.RegisterConnection(conn)
    sessionManager.CreateSession(deviceID, conn)
}
```

### 3. 查询模式

#### ✅ 推荐方式

```go
func GetDeviceSession(deviceID string) (*core.ConnectionSession, error) {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    session, exists := tcpManager.GetSessionByDeviceID(deviceID)
    if !exists {
        return nil, fmt.Errorf("设备会话不存在: %s", deviceID)
    }

    return session, nil
}

func GetConnectionSession(connID uint64) (*core.ConnectionSession, error) {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    session, exists := tcpManager.GetSessionByConnID(connID)
    if !exists {
        return nil, fmt.Errorf("连接会话不存在: %d", connID)
    }

    return session, nil
}

func GetAllActiveSessions() map[string]*core.ConnectionSession {
    tcpManager := core.GetGlobalUnifiedTCPManager()
    return tcpManager.GetAllSessions()
}
```

### 4. 状态管理模式

#### ✅ 推荐方式

```go
func UpdateDeviceHeartbeat(deviceID string) error {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    err := tcpManager.UpdateHeartbeat(deviceID)
    if err != nil {
        return fmt.Errorf("更新心跳失败: %w", err)
    }

    // 可选：检查设备状态
    session, exists := tcpManager.GetSessionByDeviceID(deviceID)
    if exists && time.Since(session.LastHeartbeat) > 5*time.Minute {
        logger.Warn("设备心跳超时", "deviceID", deviceID, "lastHeartbeat", session.LastHeartbeat)
    }

    return nil
}

func UpdateDeviceStatus(deviceID string, status constants.DeviceStatus) error {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    err := tcpManager.UpdateDeviceStatus(deviceID, status)
    if err != nil {
        return fmt.Errorf("更新设备状态失败: %w", err)
    }

    logger.Info("设备状态更新", "deviceID", deviceID, "status", status)
    return nil
}
```

### 5. 统计信息模式

#### ✅ 推荐方式

```go
func GetSystemStats() (*core.TCPManagerStats, error) {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    stats := tcpManager.GetStats()
    if stats == nil {
        return nil, fmt.Errorf("无法获取系统统计信息")
    }

    return stats, nil
}

func MonitorSystemHealth() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        stats, err := GetSystemStats()
        if err != nil {
            logger.Error("获取系统统计失败", "error", err)
            continue
        }

        logger.Info("系统状态",
            "totalConnections", stats.TotalConnections,
            "activeConnections", stats.ActiveConnections,
            "totalDevices", stats.TotalDevices,
            "onlineDevices", stats.OnlineDevices,
        )

        // 健康检查
        if stats.ActiveConnections > 1000 {
            logger.Warn("连接数过高", "count", stats.ActiveConnections)
        }
    }
}
```

## 🚀 性能优化

### 1. 批量操作

#### ✅ 推荐方式

```go
func BatchUpdateHeartbeats(deviceIDs []string) {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    // 并发更新心跳
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, 10) // 限制并发数

    for _, deviceID := range deviceIDs {
        wg.Add(1)
        go func(id string) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            if err := tcpManager.UpdateHeartbeat(id); err != nil {
                logger.Warn("更新心跳失败", "deviceID", id, "error", err)
            }
        }(deviceID)
    }

    wg.Wait()
}
```

### 2. 缓存策略

#### ✅ 推荐方式

```go
type SessionCache struct {
    cache sync.Map
    ttl   time.Duration
}

func (c *SessionCache) GetSession(deviceID string) (*core.ConnectionSession, bool) {
    if value, ok := c.cache.Load(deviceID); ok {
        entry := value.(*cacheEntry)
        if time.Since(entry.timestamp) < c.ttl {
            return entry.session, true
        }
        c.cache.Delete(deviceID)
    }

    // 从统一管理器获取
    tcpManager := core.GetGlobalUnifiedTCPManager()
    session, exists := tcpManager.GetSessionByDeviceID(deviceID)
    if exists {
        c.cache.Store(deviceID, &cacheEntry{
            session:   session,
            timestamp: time.Now(),
        })
    }

    return session, exists
}
```

### 3. 连接池管理

#### ✅ 推荐方式

```go
func ManageConnectionPool() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        tcpManager := core.GetGlobalUnifiedTCPManager()
        allSessions := tcpManager.GetAllSessions()

        for deviceID, session := range allSessions {
            // 清理超时连接
            if time.Since(session.LastActivity) > 10*time.Minute {
                logger.Info("清理超时连接", "deviceID", deviceID)
                tcpManager.UnregisterConnection(session.ConnID)
            }
        }
    }
}
```

## 🧪 测试最佳实践

### 1. 单元测试

```go
func TestDeviceLifecycle(t *testing.T) {
    // 设置测试环境
    tcpManager := core.GetGlobalUnifiedTCPManager()
    err := tcpManager.Start()
    require.NoError(t, err)
    defer tcpManager.Stop()

    // 创建模拟连接
    mockConn := &MockConnection{connID: 12345}

    // 测试连接注册
    session, err := tcpManager.RegisterConnection(mockConn)
    require.NoError(t, err)
    assert.Equal(t, uint64(12345), session.ConnID)

    // 测试设备注册
    err = tcpManager.RegisterDevice(mockConn, "TEST_DEVICE", "04A228CD", "89860000000000000001")
    require.NoError(t, err)

    // 验证设备会话
    retrievedSession, exists := tcpManager.GetSessionByDeviceID("TEST_DEVICE")
    require.True(t, exists)
    assert.Equal(t, "TEST_DEVICE", retrievedSession.DeviceID)

    // 测试状态更新
    err = tcpManager.UpdateHeartbeat("TEST_DEVICE")
    require.NoError(t, err)

    // 测试连接注销
    err = tcpManager.UnregisterConnection(12345)
    require.NoError(t, err)
}
```

### 2. 基准测试

```go
func BenchmarkSessionQuery(b *testing.B) {
    tcpManager := core.GetGlobalUnifiedTCPManager()
    tcpManager.Start()
    defer tcpManager.Stop()

    b.ResetTimer()

    for i := 0; i < b.N; i++ {
        _, _ = tcpManager.GetSessionByDeviceID("TEST_DEVICE")
    }
}
```

## ⚠️ 常见陷阱

### 1. 避免直接访问内部结构

#### ❌ 错误方式

```go
// 不要直接访问内部sync.Map
tcpManager := core.GetGlobalUnifiedTCPManager()
// tcpManager.connections.Range(...) // ❌ 不要这样做
```

#### ✅ 正确方式

```go
// 使用提供的接口
tcpManager := core.GetGlobalUnifiedTCPManager()
allSessions := tcpManager.GetAllSessions()
for deviceID, session := range allSessions {
    // 处理会话
}
```

### 2. 避免忘记错误处理

#### ❌ 错误方式

```go
tcpManager.RegisterConnection(conn) // ❌ 忽略错误
```

#### ✅ 正确方式

```go
session, err := tcpManager.RegisterConnection(conn)
if err != nil {
    return fmt.Errorf("注册连接失败: %w", err)
}
```

### 3. 避免资源泄漏

#### ✅ 正确方式

```go
func HandleConnection(conn ziface.IConnection) {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    session, err := tcpManager.RegisterConnection(conn)
    if err != nil {
        return
    }

    // 确保连接关闭时清理资源
    defer func() {
        tcpManager.UnregisterConnection(session.ConnID)
    }()

    // 处理连接逻辑...
}
```

## 📊 监控和调试

### 1. 日志记录

```go
func LogSystemState() {
    tcpManager := core.GetGlobalUnifiedTCPManager()
    stats := tcpManager.GetStats()

    logger.Info("系统状态报告",
        "timestamp", time.Now(),
        "totalConnections", stats.TotalConnections,
        "activeConnections", stats.ActiveConnections,
        "totalDevices", stats.TotalDevices,
        "onlineDevices", stats.OnlineDevices,
        "memoryUsage", getMemoryUsage(),
    )
}
```

### 2. 健康检查

```go
func HealthCheck() error {
    tcpManager := core.GetGlobalUnifiedTCPManager()

    // 检查管理器是否正常运行
    stats := tcpManager.GetStats()
    if stats == nil {
        return fmt.Errorf("无法获取统计信息")
    }

    // 检查内存使用
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    if m.Alloc > 100*1024*1024 { // 100MB
        return fmt.Errorf("内存使用过高: %d MB", m.Alloc/1024/1024)
    }

    return nil
}
```

## 📚 相关文档

- [迁移指南](migration-guide.md) - 从旧架构迁移的详细步骤
- [API 参考](api-reference.md) - 完整的 API 文档
- [架构文档](../issues/TCP连接管理模块统一重构.md) - 详细的架构设计

---

**文档版本**: v1.0
**最后更新**: 2025-01-08
**适用版本**: IoT-Zinx v2.0+
