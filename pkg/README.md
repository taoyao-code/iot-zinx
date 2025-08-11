# IoT-Zinx 包结构说明

本文档描述了IoT-Zinx项目中pkg目录的包结构和使用方法。修复后的架构实现了单一数据源、职责分离和并发安全的设计原则。

## 包结构概览

```
pkg/
├── core/                   # 核心管理层
│   ├── tcp_manager.go      # TCP连接和设备管理器
│   ├── connection_device_group.go  # 设备组管理
│   └── device_monitor.go   # 设备监控
├── gateway/                # 业务网关层
│   └── device_gateway.go   # 设备网关接口
├── network/                # 网络传输层
│   ├── tcp_writer.go       # 统一发送通道
│   └── connection_hooks.go # 连接钩子
├── protocol/               # 协议处理层
│   ├── dny_packet.go       # DNY协议包处理
│   └── raw_data_hook.go    # 原始数据钩子
├── utils/                  # 工具类
│   ├── physical_id_helper.go    # PhysicalID转换工具
│   ├── device_id_converter.go   # DeviceID格式转换
│   └── format_helper.go         # 格式化工具
└── constants/              # 常量定义
    ├── constants.go        # 通用常量
    ├── status.go          # 状态常量
    └── commands.go        # 命令常量
```

## 核心管理层 (pkg/core)

### TCPManager - 核心管理器

TCPManager是系统的核心组件，负责统一管理TCP连接和设备信息。

#### 主要功能

```go
type TCPManager struct {
    connections  sync.Map // connID → *ConnectionSession
    deviceGroups sync.Map // iccid → *DeviceGroup  
    deviceIndex  sync.Map // deviceID → iccid
}
```

#### 核心方法

```go
// 获取全局TCP管理器实例
tcpManager := core.GetGlobalTCPManager()

// 设备注册（原子操作）
err := tcpManager.RegisterDevice(conn, deviceID, physicalID, iccid)

// 获取设备信息（单一数据源）
device, exists := tcpManager.GetDeviceByID(deviceID)

// 心跳更新（统一接口）
err := tcpManager.UpdateHeartbeat(deviceID)

// 设备状态查询
isOnline := tcpManager.IsDeviceOnline(deviceID)

// 连接管理
err := tcpManager.RegisterConnection(conn)
tcpManager.UnregisterConnection(connID)
```

### 数据结构

#### ConnectionSession - 连接级别数据
```go
type ConnectionSession struct {
    SessionID       string                          // 会话标识
    ConnID          uint64                          // 连接ID
    RemoteAddr      string                          // 远程地址
    LastActivity    time.Time                       // 最后活动时间
    ConnectionState constants.ConnStatus            // 连接状态
    mutex           sync.RWMutex                    // 并发保护
}
```

#### DeviceGroup - 设备组管理
```go
type DeviceGroup struct {
    ICCID        string                    // SIM卡标识
    ConnID       uint64                    // 关联的连接ID
    Connection   ziface.IConnection        // Zinx连接对象
    Devices      map[string]*Device        // 设备映射
    LastActivity time.Time                 // 最后活动时间
    mutex        sync.RWMutex             // 并发保护
}
```

#### Device - 设备信息管理
```go
type Device struct {
    DeviceID        string                          // 设备ID
    PhysicalID      uint32                          // 物理ID
    ICCID           string                          // SIM卡ID
    DeviceType      uint16                          // 设备类型
    DeviceVersion   string                          // 设备版本
    Status          constants.DeviceStatus          // 设备状态
    LastHeartbeat   time.Time                       // 最后心跳时间
    Properties      map[string]interface{}          // 扩展属性
    mutex           sync.RWMutex                    // 并发保护
}

// 并发安全方法
func (d *Device) Lock()
func (d *Device) Unlock()
func (d *Device) RLock()
func (d *Device) RUnlock()
```

## 业务网关层 (pkg/gateway)

### DeviceGateway - 设备网关接口

提供统一的设备操作接口，封装底层的TCP管理复杂性。

```go
type DeviceGateway interface {
    // 发送命令到设备
    SendCommandToDevice(deviceID string, command []byte) error
    
    // 检查设备在线状态
    IsDeviceOnline(deviceID string) bool
    
    // 获取设备详细信息
    GetDeviceDetail(deviceID string) (*DeviceDetail, error)
}

// 使用示例
gateway := gateway.NewDeviceGateway()

// 发送充电控制命令
err := gateway.SendCommandToDevice("04A26CF3", chargingCommand)

// 检查设备状态
isOnline := gateway.IsDeviceOnline("04A26CF3")
```

## 网络传输层 (pkg/network)

### TCPWriter - 统一发送通道

```go
// 发送数据到设备
err := network.SendToDevice(conn, data)

// 异步发送
network.SendToDeviceAsync(conn, data, callback)
```

### ConnectionHooks - 连接钩子

```go
// 连接建立钩子
func OnConnStart(conn ziface.IConnection) {
    tcpManager.RegisterConnection(conn)
}

// 连接断开钩子
func OnConnStop(conn ziface.IConnection) {
    tcpManager.UnregisterConnection(conn.GetConnID())
}
```

## 工具类 (pkg/utils)

### PhysicalID转换工具

```go
// DeviceID转PhysicalID
physicalID, err := utils.ParseDeviceIDToPhysicalID(deviceID)

// PhysicalID转DeviceID
deviceID := utils.FormatPhysicalIDToDeviceID(physicalID)

// 格式化显示
displayID := utils.FormatPhysicalID(physicalID) // 十进制显示
```

### DeviceID格式转换

```go
// 智能解析DeviceID（支持多种格式）
deviceID, physicalID, err := utils.ParseDeviceID("A26CF3")    // 6位十六进制
deviceID, physicalID, err := utils.ParseDeviceID("04A26CF3")  // 8位十六进制
deviceID, physicalID, err := utils.ParseDeviceID("10627277")  // 十进制

// 标准化DeviceID
standardID := utils.StandardizeDeviceID(inputID)
```

## 使用最佳实践

### 1. 设备信息获取

```go
// ✅ 正确方式：从Device获取设备信息
tcpManager := core.GetGlobalTCPManager()
device, exists := tcpManager.GetDeviceByID(deviceID)
if exists {
    physicalID := device.PhysicalID
    iccid := device.ICCID
    status := device.Status
}

// ❌ 错误方式：从ConnectionSession获取设备信息（已废弃）
// session.DeviceID, session.PhysicalID 等字段已删除
```

### 2. 心跳更新

```go
// ✅ 正确方式：使用统一接口
tcpManager.UpdateHeartbeat(deviceID)

// ❌ 错误方式：直接修改session字段（已废弃）
// session.LastHeartbeat = time.Now()
```

### 3. 并发安全

```go
// ✅ 正确方式：使用Device的并发安全方法
device.Lock()
device.Status = constants.DeviceStatusOnline
device.LastHeartbeat = time.Now()
device.Unlock()

// ❌ 错误方式：直接修改字段（可能导致数据竞争）
// device.Status = constants.DeviceStatusOnline
```

### 4. 设备注册

```go
// ✅ 正确方式：使用原子操作
err := tcpManager.RegisterDevice(conn, deviceID, physicalID, iccid)

// ❌ 错误方式：分步操作（可能导致数据不一致）
// 手动创建Device和DeviceGroup
```

## 架构优势

### 1. 单一数据源
- Device结构作为设备信息的唯一来源
- 消除了数据重复存储和不一致问题
- 简化了数据同步逻辑

### 2. 职责分离
- ConnectionSession：管理连接级别数据
- Device：管理设备级别数据
- DeviceGroup：管理设备组关系

### 3. 并发安全
- 所有共享数据结构都有mutex保护
- 提供了安全的并发访问方法
- 使用sync.Map提供线程安全的映射操作

### 4. 性能优化
- 减少内存占用50%+
- O(1)设备查找性能
- 无锁数据结构优先使用

### 5. 接口统一
- 所有模块通过TCPManager统一访问数据
- 标准化的API接口
- 一致的错误处理机制

## 迁移指南

如果您正在从旧版本迁移，请注意以下变更：

### 已删除的字段
- `ConnectionSession.DeviceID` → 使用 `tcpManager.GetDeviceByID()`
- `ConnectionSession.PhysicalID` → 使用 `device.PhysicalID`
- `ConnectionSession.ICCID` → 使用 `device.ICCID`
- `ConnectionSession.DeviceStatus` → 使用 `device.Status`
- `DeviceGroup.Sessions` → 已删除，使用 `DeviceGroup.Devices`

### 新增的方法
- `tcpManager.GetDeviceByID()` - 获取设备信息
- `tcpManager.RegisterDevice()` - 原子设备注册
- `device.Lock()/Unlock()` - 并发安全方法

---

*本文档反映了数据存储去重修复后的最新包结构，确保了系统的高性能、高可靠性和高可维护性。*
