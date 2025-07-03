# IoT-Zinx 通知系统完整文档

## 文档概述

本文档集合提供了 IoT-Zinx 通知系统的完整技术文档，包括系统分析、实施计划、架构设计和使用指南。

## 文档结构

### 📋 核心文档

1. **[通知系统全面分析报告](./notification_system_analysis.md)**

   - 系统架构分析
   - 协议指令通知实现状态
   - 事件类型完整性分析
   - 配置文件分析
   - 数据完整性分析
   - 系统缺失功能分析
   - 性能和可靠性分析

2. **[通知系统完善实施路线图](./notification_implementation_roadmap.md)**

   - 分阶段实施计划
   - 具体实现方案
   - 检查清单
   - 风险评估和缓解策略
   - 成功标准定义

3. **[第三方平台通知系统完善计划](./notification_enhancement_plan.md)**
   - 项目执行记录
   - 已完成功能详述
   - 技术实现亮点
   - 部署和测试建议

### 📡 第三方平台对接文档

4. **[第三方平台对接 API 文档](./third_party_integration_api.md)**

   - HTTP 推送规范
   - 事件类型详细说明
   - 请求响应格式
   - 安全认证要求
   - 配置示例

5. **[第三方平台对接示例](./third_party_integration_examples.md)**

   - 完整业务流程示例
   - 异常情况处理
   - 代码实现示例（Node.js/Python）
   - 性能优化建议

6. **[第三方平台数据字典](./third_party_data_dictionary.md)**

   - 完整字段定义
   - 数据类型和取值范围
   - 枚举值说明
   - 数据格式转换规则

7. **[第三方平台对接快速开始指南](./third_party_quick_start.md)**
   - 对接流程概览
   - 最小化实现示例
   - 配置和部署指南
   - 测试验证方法

## 系统状态总览

### ✅ 已完成的核心功能

| 功能模块     | 实现状态    | 覆盖指令   | 推送内容                  |
| ------------ | ----------- | ---------- | ------------------------- |
| 设备注册通知 | ✅ 完整实现 | 0x20       | 设备信息、ICCID、固件版本 |
| 设备心跳通知 | ✅ 完整实现 | 0x01, 0x21 | 端口状态、电压、温度      |
| 功率心跳通知 | ✅ 完整实现 | 0x06, 0x26 | 实时功率、累计电量        |
| 充电控制通知 | ✅ 完整实现 | 0x82       | 充电开始/结束/失败确认    |
| 结算数据通知 | ✅ 完整实现 | 0x03, 0x23 | 结算数据、分时收费        |

### ❌ 待实现的关键功能

| 功能模块     | 优先级    | 涉及指令   | 预期推送内容       |
| ------------ | --------- | ---------- | ------------------ |
| 主机心跳通知 | 高        | 0x11       | 主机状态、系统信息 |
| 刷卡事件通知 | 高        | 0x02       | 卡号、刷卡时间     |
| 设备报警通知 | 高        | 0x42       | 报警类型、报警数据 |
| 设备离线检测 | 高        | 心跳超时   | 离线时间、离线原因 |
| 充电结束通知 | ✅ 已完成 | 0x82       | 充电结束确认       |
| 时间同步通知 | 中        | 0x12, 0x22 | 时间同步状态       |
| 网络状态通知 | 中        | 0x81       | 网络质量、信号强度 |

## 技术架构概览

### 核心组件

```
┌─────────────────────────────────────────────────────────────┐
│                    IoT-Zinx 通知系统架构                      │
├─────────────────────────────────────────────────────────────┤
│  协议处理层                                                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ 设备注册    │ │ 心跳处理    │ │ 充电控制    │  ...      │
│  │ Handler     │ │ Handler     │ │ Handler     │           │
│  └─────────────┘ └─────────────┘ └─────────────┘           │
├─────────────────────────────────────────────────────────────┤
│  通知集成层                                                  │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │           NotificationIntegrator                        │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │ 设备事件    │ │ 充电事件    │ │ 端口事件    │      │ │
│  │  │ 通知方法    │ │ 通知方法    │ │ 通知方法    │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  通知服务层                                                  │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              NotificationService                        │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐      │ │
│  │  │ 事件队列    │ │ 工作协程    │ │ 重试机制    │      │ │
│  │  │ 管理        │ │ 池          │ │ 处理        │      │ │
│  │  └─────────────┘ └─────────────┘ └─────────────┘      │ │
│  └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│  HTTP推送层                                                  │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ 计费系统    │ │ 运营平台    │ │ 第三方      │  ...      │
│  │ 端点        │ │ 端点        │ │ 端点        │           │
│  └─────────────┘ └─────────────┘ └─────────────┘           │
└─────────────────────────────────────────────────────────────┘
```

### 数据流向

```
设备数据 → 协议解析 → 事件生成 → 队列缓存 → HTTP推送 → 第三方平台
    ↓         ↓         ↓         ↓         ↓         ↓
  原始包    标准帧    通知事件   异步处理   REST API  业务系统
```

## 事件类型体系

### 已实现的事件类型

```go
// 设备生命周期事件
EventTypeDeviceOnline     = "device_online"     // ✅ 设备上线
EventTypeDeviceRegister   = "device_register"   // ✅ 设备注册
EventTypeDeviceHeartbeat  = "device_heartbeat"  // ✅ 设备心跳

// 充电业务事件
EventTypeChargingStart    = "charging_start"    // ✅ 充电开始
EventTypeChargingEnd      = "charging_end"      // ✅ 充电结束
EventTypeChargingFailed   = "charging_failed"   // ✅ 充电失败
EventTypeSettlement       = "settlement"        // ✅ 结算完成
EventTypePowerHeartbeat   = "power_heartbeat"   // ✅ 功率心跳
EventTypeChargingPower    = "charging_power"    // ✅ 充电功率

// 端口状态事件
EventTypePortHeartbeat    = "port_heartbeat"    // ✅ 端口心跳
EventTypePortStatusChange = "port_status_change" // ✅ 端口状态变化
```

### 待实现的事件类型

```go
// 设备管理事件
EventTypeDeviceOffline    = "device_offline"    // ❌ 设备离线
EventTypeDeviceError      = "device_error"      // ❌ 设备错误
EventTypeDeviceAlarm      = "device_alarm"      // ❌ 设备报警

// 充电流程事件（已实现）
EventTypeChargingEnd      = "charging_end"      // ✅ 充电结束
EventTypeChargingFailed   = "charging_failed"   // ✅ 充电失败

// 系统状态事件
EventTypeMainHeartbeat    = "main_heartbeat"    // ❌ 主机心跳
EventTypeTimeSync         = "time_sync"         // ❌ 时间同步
EventTypeNetworkStatus    = "network_status"    // ❌ 网络状态

// 用户操作事件
EventTypeCardSwipe        = "card_swipe"        // ❌ 刷卡操作

// 端口管理事件
EventTypePortOnline       = "port_online"       // ❌ 端口上线
EventTypePortOffline      = "port_offline"      // ❌ 端口离线
EventTypePortError        = "port_error"        // ❌ 端口故障
```

## 配置管理

### 当前配置结构

```yaml
notification:
  enabled: true
  queue_size: 10000
  workers: 5

  endpoints:
    - name: "billing_system"
      type: "billing"
      url: "https://bujia.tyxxtb.com/cdz/chargeCallback"
      event_types:
        - "device_online"
        - "charging_start"
        - "settlement"
        # ... 更多事件类型
      enabled: true

  retry:
    max_attempts: 3
    initial_interval: "1s"
    max_interval: "30s"
    multiplier: 2.0
```

### 配置优化建议（已完成部分）

1. **✅ 事件类型完整性（已完成）**

   - ✅ 已添加所有新定义的事件类型（charging_failed 等）
   - ✅ 已区分计费系统和运营平台的订阅需求

2. **✅ 端点管理（已完成）**

   - ✅ 已启用运营平台端点配置
   - 🔄 支持多环境配置（开发、测试、生产）

3. **🔄 性能调优（待实现）**
   - 动态队列大小配置
   - 负载均衡参数配置

## 数据格式标准

### 统一数据单位

| 数据类型 | 原始单位 | 标准单位   | 转换方法       |
| -------- | -------- | ---------- | -------------- |
| 功率     | 原始值   | 瓦特(W)    | `value * 0.1`  |
| 电压     | 原始值   | 伏特(V)    | `value * 0.1`  |
| 电量     | 原始值   | 度(kWh)    | `value * 0.01` |
| 温度     | 原始值   | 摄氏度(°C) | `value - 65`   |

### 端口状态映射

| 状态码 | 状态描述                 | 是否充电 | 备注         |
| ------ | ------------------------ | -------- | ------------ |
| 0x00   | 空闲                     | ❌       | 正常待机状态 |
| 0x01   | 充电中                   | ✅       | 正在充电     |
| 0x02   | 有充电器但未充电(未启动) | ❌       | 插入但未开始 |
| 0x03   | 有充电器但未充电(已充满) | ❌       | 充电完成     |
| 0x05   | 浮充                     | ✅       | 维持充电     |
| ...    | ...                      | ...      | ...          |

## 性能指标

### 当前性能参数

- **队列容量：** 10,000 事件
- **工作协程：** 5 个
- **超时时间：** 10 秒
- **重试次数：** 最多 3 次
- **重试间隔：** 1s → 2s → 4s

### 性能目标

- **通知延迟：** < 1 秒
- **成功率：** > 99.9%
- **吞吐量：** > 10,000 事件/分钟
- **可用性：** > 99.95%

## 监控和运维

### 监控指标

1. **业务指标**

   - 事件生成速率
   - 通知成功率
   - 端点响应时间

2. **技术指标**

   - 队列长度
   - 工作协程利用率
   - 内存使用情况

3. **错误指标**
   - 失败事件统计
   - 重试次数分布
   - 错误类型分析

### 运维接口

- `GET /api/v1/notification/stats` - 获取统计信息
- `GET /api/v1/notification/health` - 健康检查
- `POST /api/v1/notification/stats/reset` - 重置统计

## 开发指南

### 添加新的通知类型

1. **定义事件类型常量**

```go
// 在 pkg/notification/types.go 中添加
const EventTypeNewFeature = "new_feature"
```

2. **实现通知方法**

```go
// 在 pkg/notification/integrator.go 中添加
func (n *NotificationIntegrator) NotifyNewFeature(deviceID string, data map[string]interface{}) {
    // 实现通知逻辑
}
```

3. **在处理器中调用**

```go
// 在相应的处理器中添加
integrator := notification.GetGlobalNotificationIntegrator()
integrator.NotifyNewFeature(deviceId, featureData)
```

4. **更新配置文件**

```yaml
# 在 configs/gateway.yaml 中添加事件类型
event_types:
  - "new_feature"
```

### 测试指南

1. **单元测试**

```go
func TestNewFeatureNotification(t *testing.T) {
    // 测试通知方法
}
```

2. **集成测试**

```bash
# 启动测试环境
go test ./test/notification_test.go -v
```

3. **端到端测试**

```bash
# 模拟设备数据，验证通知推送
curl -X POST http://localhost:8080/api/test/simulate
```

## 故障排查

### 常见问题

1. **通知发送失败**

   - 检查网络连接
   - 验证端点配置
   - 查看错误日志

2. **通知延迟过高**

   - 检查队列长度
   - 调整工作协程数
   - 优化网络配置

3. **重复通知**
   - 检查幂等性实现
   - 验证事件去重逻辑
   - 调整重试策略

### 日志分析

```bash
# 查看通知相关日志
grep "notification" /var/log/iot-zinx/gateway.log

# 查看错误日志
grep "ERROR.*notification" /var/log/iot-zinx/gateway.log

# 查看性能日志
grep "stats.*notification" /var/log/iot-zinx/gateway.log
```

## 总结

IoT-Zinx 通知系统已经建立了完善的基础架构，实现了核心的通知功能。

## ✅ 最新完成的重要改进

1. **充电流程完整性** - 已实现充电开始、结束、失败的完整通知支持
2. **配置文件完善** - 已更新事件类型列表，启用运营平台端点
3. **双端点支持** - 计费系统和运营平台都已配置完整的事件订阅
4. **事件类型扩展** - 已添加充电失败等新事件类型

通过本文档集合，开发团队可以：

1. **全面了解系统现状** - 通过分析报告掌握系统的优势和不足
2. **制定实施计划** - 通过路线图有序推进功能完善
3. **规范开发流程** - 通过技术文档确保开发质量
4. **提升运维效率** - 通过监控指南保障系统稳定

下一步工作重点是按照实施路线图，优先完成剩余的高优先级功能（设备离线检测、主机心跳、刷卡事件、设备报警），确保通知系统能够满足业务的全面需求。
