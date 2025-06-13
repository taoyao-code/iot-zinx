# DNY 协议解析器统一 - 开发者快速指南

**适用对象**: IoT Zinx 项目开发者  
**更新日期**: 2025 年 6 月 13 日  
**难度级别**: 中级

---

## 🚀 快速开始

### 基本使用

#### 1. 解析 DNY 协议数据

```go
import "github.com/bujia-iot/iot-zinx/pkg/protocol"

// 解析任意DNY协议数据
data := []byte("ICCID12345678901234567890") // 或其他DNY数据
msg, err := protocol.ParseDNYProtocolData(data)
if err != nil {
    log.Printf("解析失败: %v", err)
    return
}

// 根据消息类型处理
switch msg.MessageType {
case "standard":
    log.Printf("标准DNY帧: 命令=0x%02X, 物理ID=%d", msg.CommandId, msg.PhysicalId)
case "iccid":
    log.Printf("ICCID消息: %s", msg.ICCIDValue)
case "heartbeat_link":
    log.Printf("Link心跳消息")
case "error":
    log.Printf("解析错误: %s", msg.ErrorMessage)
}
```

#### 2. 检测特殊消息

```go
// 快速检测是否为特殊消息（ICCID或Link心跳）
if protocol.IsSpecialMessage(data) {
    log.Println("这是一个特殊消息")
}
```

#### 3. 构建充电控制包

```go
import "github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"

packet := dny_protocol.BuildChargeControlPacket(
    0x12345678,  // physicalID
    0x1234,      // messageID
    0x01,        // rateMode
    100000,      // balance
    0x01,        // portNumber
    0x01,        // chargeCommand
    120,         // chargeDuration
    "ORDER123",  // orderNumber
    240,         // maxChargeDuration
    2000,        // maxPower
    0x01,        // qrCodeLight
)
```

---

## 🔧 处理器开发

### 推荐方式（使用统一消息）

```go
type MyHandler struct {
    protocol.DNYFrameHandlerBase
}

func (h *MyHandler) Handle(request ziface.IRequest) {
    conn := request.GetConnection()

    // 👍 推荐：使用统一消息接口
    unifiedMsg, err := h.ExtractUnifiedMessage(request)
    if err != nil {
        h.HandleError("MyHandler", err, conn)
        return
    }

    // 获取设备会话
    deviceSession, err := h.GetOrCreateDeviceSession(conn)
    if err != nil {
        h.HandleError("MyHandler", err, conn)
        return
    }

    // 更新设备会话（使用统一消息）
    h.UpdateDeviceSessionFromUnifiedMessage(deviceSession, unifiedMsg)

    // 业务逻辑处理
    h.processBusinessLogic(unifiedMsg, conn, deviceSession)
}

func (h *MyHandler) processBusinessLogic(msg *dny_protocol.Message, conn ziface.IConnection, session *session.DeviceSession) {
    switch msg.MessageType {
    case "standard":
        // 处理标准DNY协议
        h.handleStandardFrame(msg, conn, session)
    case "iccid":
        // 处理ICCID消息
        h.handleICCID(msg, conn, session)
    case "heartbeat_link":
        // 处理Link心跳
        h.handleLinkHeartbeat(msg, conn, session)
    }
}
```

### 兼容方式（使用旧接口）

```go
func (h *MyHandler) Handle(request ziface.IRequest) {
    conn := request.GetConnection()

    // ⚠️ 兼容：使用旧的帧接口（计划废弃）
    decodedFrame, err := h.ExtractDecodedFrame(request)
    if err != nil {
        h.HandleError("MyHandler", err, conn)
        return
    }

    // 获取设备会话
    deviceSession, err := h.GetOrCreateDeviceSession(conn)
    if err != nil {
        h.HandleError("MyHandler", err, conn)
        return
    }

    // 更新设备会话（使用旧帧）
    h.UpdateDeviceSessionFromFrame(deviceSession, decodedFrame)

    // 业务逻辑处理
    h.processLegacyLogic(decodedFrame, conn, deviceSession)
}
```

---

## 📋 常量和配置

### 消息 ID 常量

```go
import "github.com/bujia-iot/iot-zinx/pkg/constants"

// 特殊消息ID
constants.MsgIDErrorFrame    // 0xFF00 - 错误帧
constants.MsgIDICCID         // 0xFF01 - ICCID消息
constants.MsgIDLinkHeartbeat // 0xFF02 - Link心跳
constants.MsgIDUnknown       // 0xFF03 - 未知类型

// 协议常量
constants.IOT_SIM_CARD_LENGTH // 20 - ICCID长度
constants.IOT_LINK_HEARTBEAT  // "link" - Link心跳字符串
constants.DNY_MIN_PACKET_LEN  // 12 - DNY最小包长度
```

### DNY 命令常量

```go
import "github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"

// 常用DNY命令
dny_protocol.CmdHeartbeat      // 0x01 - 设备心跳
dny_protocol.CmdMainHeartbeat  // 0x11 - 主机心跳
dny_protocol.CmdChargeControl  // 0x82 - 充电控制
// ... 其他命令
```

### 路由注册

```go
// 在 router.go 中注册新的处理器
func RegisterRouters(server ziface.IServer) {
    // 特殊消息
    server.AddRouter(constants.MsgIDICCID, &SimCardHandler{})
    server.AddRouter(constants.MsgIDLinkHeartbeat, &LinkHeartbeatHandler{})

    // DNY协议消息
    server.AddRouter(dny_protocol.CmdHeartbeat, &HeartbeatHandler{})
    server.AddRouter(dny_protocol.CmdChargeControl, &ChargeControlHandler{})

    // 你的新处理器
    server.AddRouter(dny_protocol.CmdYourCommand, &YourHandler{})
}
```

---

## 🐛 错误处理

### 错误类型识别

```go
msg, err := protocol.ParseDNYProtocolData(data)
if err != nil {
    // 检查错误消息的类型
    if msg != nil && msg.MessageType == "error" {
        switch {
        case strings.Contains(msg.ErrorMessage, "checksum"):
            log.Println("校验和错误")
        case strings.Contains(msg.ErrorMessage, "length"):
            log.Println("数据长度错误")
        case strings.Contains(msg.ErrorMessage, "header"):
            log.Println("包头错误")
        default:
            log.Printf("其他解析错误: %s", msg.ErrorMessage)
        }
    }
}
```

### 统一错误处理

```go
func (h *MyHandler) Handle(request ziface.IRequest) {
    defer func() {
        if r := recover(); r != nil {
            h.HandleError("MyHandler", fmt.Errorf("panic: %v", r), request.GetConnection())
        }
    }()

    // 你的处理逻辑
}
```

---

## 📊 日志和调试

### 结构化日志

```go
import (
    "github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
    "github.com/sirupsen/logrus"
)

func (h *MyHandler) processMessage(msg *dny_protocol.Message, conn ziface.IConnection) {
    logger.WithFields(logrus.Fields{
        "handler":     "MyHandler",
        "connID":      conn.GetConnID(),
        "messageType": msg.MessageType,
        "physicalID":  fmt.Sprintf("0x%08X", msg.PhysicalId),
        "commandID":   fmt.Sprintf("0x%02X", msg.CommandId),
        "dataLen":     len(msg.Data),
    }).Info("处理消息")
}
```

### 调试技巧

```go
// 打印原始数据
log.Printf("原始数据: %s", hex.EncodeToString(msg.RawData))

// 打印解析结果
log.Printf("解析结果: %+v", msg)

// 验证校验和
log.Printf("校验和: 计算值=0x%04X, 期望值=0x%04X", calculatedChecksum, msg.Checksum)
```

---

## 🧪 测试

### 单元测试模板

```go
func TestMyHandler_Handle(t *testing.T) {
    tests := []struct {
        name    string
        input   []byte
        wantErr bool
    }{
        {
            name:    "标准DNY帧",
            input:   buildTestDNYFrame(),
            wantErr: false,
        },
        {
            name:    "ICCID消息",
            input:   []byte("ICCID12345678901234567890"),
            wantErr: false,
        },
        {
            name:    "空数据",
            input:   []byte{},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            msg, err := protocol.ParseDNYProtocolData(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("ParseDNYProtocolData() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if !tt.wantErr && msg == nil {
                t.Error("ParseDNYProtocolData() returned nil message")
            }
        })
    }
}
```

### 集成测试

```go
func TestProtocolFlow(t *testing.T) {
    // 创建模拟连接
    mockConn := &MockConnection{}

    // 创建测试数据
    testData := []byte("ICCID12345678901234567890")

    // 创建解码器
    decoder := &protocol.DNY_Decoder{}

    // 模拟解码过程
    // ... 测试逻辑
}
```

---

## ⚠️ 迁移指南

### 从旧解析器迁移

#### 步骤 1：替换解析调用

```go
// 旧方式
frame, err := parseFrame(data)

// 新方式
msg, err := protocol.ParseDNYProtocolData(data)
```

#### 步骤 2：更新数据访问

```go
// 旧方式
physicalID := frame.PhysicalID
command := frame.Command

// 新方式
physicalID := msg.PhysicalId
command := msg.CommandId
```

#### 步骤 3：更新错误处理

```go
// 旧方式
if !frame.IsChecksumValid {
    // 处理校验错误
}

// 新方式
if msg.MessageType == "error" {
    log.Printf("解析错误: %s", msg.ErrorMessage)
}
```

---

## 📚 常见问题

### Q: 如何判断消息类型？

```go
A: 使用 msg.MessageType 字段：
   - "standard": 标准DNY协议帧
   - "iccid": ICCID消息
   - "heartbeat_link": Link心跳
   - "error": 解析错误
```

### Q: 如何获取设备物理 ID？

```go
A: 对于标准DNY帧，使用 msg.PhysicalId
   对于特殊消息，物理ID可能不可用
```

### Q: 如何处理校验和错误？

```go
A: 检查 msg.MessageType == "error" 且
   strings.Contains(msg.ErrorMessage, "checksum")
```

### Q: 新旧接口何时废弃？

```go
A: 兼容接口将在所有处理器迁移完成后废弃
   建议新开发直接使用 ExtractUnifiedMessage()
```

---

## 🔗 相关资源

- **完整架构文档**: `docs/DNY协议解析器统一架构设计.md`
- **完成报告**: `issues/协议解析器统一重构_完成报告.md`
- **API 参考**: `pkg/protocol/` 包文档
- **示例代码**: `internal/infrastructure/zinx_server/handlers/` 目录

---

**维护团队**: IoT Zinx 开发组  
**技术支持**: 请在项目 issue 中提问  
**最后更新**: 2025 年 6 月 13 日
