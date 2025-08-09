# 修复 TCP 数据发送双重封装问题

## 问题描述

#### 修复位置：

1. `WriteWithRetry()` 方法 - 第 67 行
2. `SendM## 后续测试

3. 重启网关服务
4. 发送设备定位命令
5. **🆕 验证设备定位应答**：应该看到设备返回的 `0x96` 应答消息
6. 检查设备响应和日志输出
7. 验证 DNY 协议数据格式正确性

## 📋 日志分析结果

### ✅ 确认修复生效：

```log
INFO[0056] 🔥 直接发送原始DNY协议数据（无Zinx封装）
INFO[0056] ✅ 命令发送成功（含重试机制）- TCP写入完成
INFO[0056] 🔊 设备定位命令发送成功，设备将播放语音并闪灯
```

### ❗ 发现新问题：

- 设备未发送定位应答消息（按协议应该有 `0x96` 应答）
- 原因：定位响应 handler 在 router 中被禁用
- **已修复**：重新启用了 `DeviceLocateHandler`Retry()` 方法 - 第 207 行

#### ⚡ 追加修复：设备定位响应 handler

**问题发现**：通过日志分析发现设备未应答定位命令，检查后发现：

- 设备**应该**对 96 指令有应答（根据协议文档）
- 但是 `router.go:57` 中定位响应 handler 被注释掉了
- 导致设备的定位应答消息无法被处理

**解决方案**：

````go
// ❌ 原来被注释
// server.AddRouter(constants.CmdDeviceLocate, NewDeviceLocateHandler())

// ✅ 重新启用
server.AddRouter(constants.CmdDeviceLocate, NewDeviceLocateHandler())
```正确接收和解析 DNY 协议数据包，原因是 TCP 写入时存在双重协议封装：

1. **第一层封装**：`UnifiedDNYBuilder.BuildDNYPacket()` - 构建完整的 DNY 协议数据包
2. **第二层封装**：`conn.SendBuffMsg()` - Zinx 框架再次封装消息

## 问题影响

- 设备收到的数据格式不符合 DNY 协议规范
- 设备无法正确解析命令和数据
- 定位命令、充电控制等功能失效

## 解决方案

### 修改文件：`pkg/network/tcp_writer.go`

#### 🔥 关键修复：

```go
// ❌ 错误方式（双重封装）
err := conn.SendBuffMsg(msgID, data)

// ✅ 正确方式（直接发送原始数据）
tcpConn := conn.GetTCPConnection()
_, err := tcpConn.Write(data)
````

#### 修复位置：

1. `WriteWithRetry()` 方法 - 第 67 行
2. `SendMsgWithRetry()` 方法 - 第 207 行

#### 新增功能：

- 添加原始数据发送日志记录
- 显示详细的 Hex 数据用于调试
- 标识数据发送方法为 `RAW_TCP_WRITE`

## 技术细节

### DNY 协议数据包结构：

```
[起始字符] [设备ID] [命令序号] [命令ID] [数据长度] [数据] [校验和] [结束字符]
    DN       4字节     2字节      1字节     1字节     N字节   1字节      Y
```

### 修复前后对比：

```
修复前：[Zinx封装头] + [DNY协议包] → 设备无法解析
修复后：[DNY协议包] → 设备正常解析
```

## 验证结果

- ✅ 编译通过
- ✅ 消除双重封装问题
- ✅ 添加详细调试日志
- ✅ 保持重试机制完整性

## 后续测试

1. 重启网关服务
2. 发送设备定位命令
3. 检查设备响应和日志输出
4. 验证 DNY 协议数据格式正确性

---

**修复时间**: 2025-08-09  
**影响范围**: 所有设备 TCP 通信  
**优先级**: 🔥 极高（核心通信问题）
