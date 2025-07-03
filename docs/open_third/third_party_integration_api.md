# IoT-Zinx 第三方平台对接 API 文档

## 文档概述

本文档详细说明了 IoT-Zinx 系统与第三方平台的数据对接规范，包括通知推送的数据格式、参数说明、响应要求等。

## 对接方式

### HTTP 推送方式

IoT-Zinx 系统采用 HTTP POST 方式主动推送数据到第三方平台指定的回调地址。

**推送特性：**

- 协议：HTTP/HTTPS
- 方法：POST
- 内容类型：application/json
- 字符编码：UTF-8
- 超时时间：10 秒
- 重试机制：最多 3 次，指数退避（1s → 2s → 4s）

## 通用请求格式

### 请求头

```http
POST /your/callback/url HTTP/1.1
Host: your-domain.com
Content-Type: application/json
Authorization: Bearer ${API_TOKEN}
User-Agent: IoT-Zinx/1.0
X-Event-Type: {event_type}
X-Device-ID: {device_id}
X-Timestamp: {unix_timestamp}
```

### 请求体结构

```json
{
  "event_id": "uuid-string",
  "event_type": "event_type_name",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    // 具体事件数据，根据事件类型而定
  }
}
```

### 通用字段说明

| 字段名      | 类型    | 必填 | 说明                             |
| ----------- | ------- | ---- | -------------------------------- |
| event_id    | string  | 是   | 事件唯一标识符，UUID 格式        |
| event_type  | string  | 是   | 事件类型，见下方事件类型列表     |
| device_id   | string  | 是   | 设备 ID，8 位十六进制字符串      |
| port_number | integer | 否   | 端口号，1-based，范围 1-N        |
| timestamp   | integer | 是   | 事件时间戳，Unix 时间戳（秒）    |
| data        | object  | 是   | 事件具体数据，结构因事件类型而异 |

## 事件类型详细说明

### 1. 设备上线事件 (device_online)

**触发时机：** 设备建立 TCP 连接并完成注册时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "event_type": "device_online",
  "device_id": "04A228CD",
  "port_number": 0,
  "timestamp": 1703123456,
  "data": {
    "conn_id": 12345,
    "remote_addr": "192.168.1.100:54321",
    "connect_time": 1703123456,
    "device_type": 1,
    "firmware_version": "V2.1.0",
    "iccid": "89860318123456789012"
  }
}
```

**data 字段说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| conn_id | integer | 连接 ID |
| remote_addr | string | 设备 IP 地址和端口 |
| connect_time | integer | 连接建立时间戳 |
| device_type | integer | 设备类型（1=充电桩） |
| firmware_version | string | 固件版本号 |
| iccid | string | SIM 卡 ICCID 号码 |

### 2. 设备离线事件 (device_offline)

**触发时机：** 设备连接断开或心跳超时时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440001",
  "event_type": "device_offline",
  "device_id": "04A228CD",
  "port_number": 0,
  "timestamp": 1703123456,
  "data": {
    "conn_id": 12345,
    "offline_reason": "heartbeat_timeout",
    "last_heartbeat_time": 1703123400,
    "offline_duration": 56,
    "connection_duration": 3600
  }
}
```

**data 字段说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| conn_id | integer | 连接 ID |
| offline_reason | string | 离线原因：connection_lost, heartbeat_timeout, manual_disconnect |
| last_heartbeat_time | integer | 最后心跳时间戳 |
| offline_duration | integer | 离线时长（秒） |
| connection_duration | integer | 本次连接持续时长（秒） |

### 3. 设备注册事件 (device_register)

**触发时机：** 设备发送注册包（0x20 指令）时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440002",
  "event_type": "device_register",
  "device_id": "04A228CD",
  "port_number": 0,
  "timestamp": 1703123456,
  "data": {
    "physical_id": "0x04A228CD",
    "physical_id_decimal": 77915341,
    "iccid": "89860318123456789012",
    "device_version": "V2.1.0",
    "device_type": 1,
    "heartbeat_period": 30,
    "register_time": 1703123456,
    "command": "0x20",
    "conn_id": 12345,
    "remote_addr": "192.168.1.100:54321"
  }
}
```

**data 字段说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| physical_id | string | 物理 ID 十六进制格式 |
| physical_id_decimal | integer | 物理 ID 十进制格式 |
| iccid | string | SIM 卡 ICCID 号码 |
| device_version | string | 设备版本号 |
| device_type | integer | 设备类型 |
| heartbeat_period | integer | 心跳周期（秒） |
| register_time | integer | 注册时间戳 |
| command | string | 协议指令码 |

### 4. 设备心跳事件 (device_heartbeat)

**触发时机：** 设备发送心跳包（0x01/0x21 指令）时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440003",
  "event_type": "device_heartbeat",
  "device_id": "04A228CD",
  "port_number": 0,
  "timestamp": 1703123456,
  "data": {
    "voltage": 220.5,
    "port_count": 10,
    "port_statuses": [
      { "port": 1, "status": 1, "status_desc": "充电中" },
      { "port": 2, "status": 0, "status_desc": "空闲" },
      { "port": 3, "status": 2, "status_desc": "有充电器但未充电(未启动)" }
    ],
    "signal_strength": 85,
    "temperature": 25,
    "command": "0x21",
    "conn_id": 12345,
    "remote_addr": "192.168.1.100:54321"
  }
}
```

**data 字段说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| voltage | float | 电压值（伏特） |
| port_count | integer | 端口总数 |
| port_statuses | array | 端口状态数组 |
| signal_strength | integer | 信号强度（0-100） |
| temperature | integer | 环境温度（摄氏度） |

**port_statuses 数组元素说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| port | integer | 端口号 |
| status | integer | 状态码（见端口状态码表） |
| status_desc | string | 状态描述 |

### 5. 端口心跳事件 (port_heartbeat)

**触发时机：** 端口状态发生变化时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440004",
  "event_type": "port_heartbeat",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "port_status": 1,
    "port_status_desc": "充电中",
    "voltage": 220.5,
    "temperature": 25,
    "is_charging": true,
    "previous_status": 0,
    "previous_status_desc": "空闲",
    "status_change_time": 1703123456
  }
}
```

### 6. 充电开始事件 (charging_start)

**触发时机：** 充电控制指令成功响应且为开始充电时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440005",
  "event_type": "charging_start",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "order_number": "ORD20231221001",
    "response_code": 0,
    "status_desc": "充电开始成功",
    "command": "0x82",
    "message_id": "0x1234",
    "conn_id": 12345,
    "remote_addr": "192.168.1.100:54321",
    "start_time": 1703123456
  }
}
```

### 7. 充电结束事件 (charging_end)

**触发时机：** 充电控制指令响应为停止充电或结算时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440006",
  "event_type": "charging_end",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "order_id": "ORD20231221001",
    "total_energy": 15.5,
    "charge_duration": 3600,
    "start_time": "2023-12-21 10:00:00",
    "end_time": "2023-12-21 11:00:00",
    "stop_reason": 1,
    "stop_reason_desc": "手动停止",
    "settlement_triggered": true,
    "command": "0x82"
  }
}
```

**data 字段说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| order_id | string | 订单号 |
| total_energy | float | 总电量（度） |
| charge_duration | integer | 充电时长（秒） |
| start_time | string | 开始时间 |
| end_time | string | 结束时间 |
| stop_reason | integer | 停止原因码 |
| stop_reason_desc | string | 停止原因描述 |
| settlement_triggered | boolean | 是否由结算触发 |

### 8. 充电失败事件 (charging_failed)

**触发时机：** 充电控制指令响应失败时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440007",
  "event_type": "charging_failed",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "order_number": "ORD20231221001",
    "response_code": 1,
    "status_desc": "端口故障",
    "failure_reason": "端口故障",
    "error_code": 1,
    "command": "0x82",
    "message_id": "0x1234",
    "failed_time": 1703123456
  }
}
```

### 9. 功率心跳事件 (power_heartbeat)

**触发时机：** 设备发送功率心跳包（0x06/0x26 指令）时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440008",
  "event_type": "power_heartbeat",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "current_power": 2200.5,
    "max_power": 2500.0,
    "min_power": 1800.0,
    "avg_power": 2100.0,
    "cumulative_energy": 5.25,
    "voltage": 220.5,
    "current": 10.2,
    "charging_status": 1,
    "charging_status_desc": "正常充电",
    "command": "0x06"
  }
}
```

### 10. 充电功率实时数据事件 (charging_power)

**触发时机：** 充电过程中的实时功率数据推送

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440009",
  "event_type": "charging_power",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "instant_power": 2200.5,
    "cumulative_energy": 5.25,
    "charging_duration": 1800,
    "voltage": 220.5,
    "current": 10.2,
    "power_factor": 0.95,
    "frequency": 50.0,
    "order_id": "ORD20231221001"
  }
}
```

### 11. 结算事件 (settlement)

**触发时机：** 设备上报结算数据（0x03/0x23 指令）时

**数据结构：**

```json
{
  "event_id": "550e8400-e29b-41d4-a716-446655440010",
  "event_type": "settlement",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123456,
  "data": {
    "order_id": "ORD20231221001",
    "card_number": "12345678",
    "total_energy": 15.5,
    "total_fee": 1550,
    "charge_fee": 1400,
    "service_fee": 150,
    "start_time": 1703119856,
    "end_time": 1703123456,
    "charge_duration": 3600,
    "settlement_id": "SETTLE_04A228CD_1703123456",
    "settlement_type": "normal",
    "command": "0x03"
  }
}
```

**data 字段说明：**
| 字段名 | 类型 | 说明 |
|--------|------|------|
| total_fee | integer | 总费用（分） |
| charge_fee | integer | 充电费用（分） |
| service_fee | integer | 服务费（分） |
| settlement_type | string | 结算类型：normal, time_billing |

## 端口状态码对照表

| 状态码 | 状态描述                 | 是否充电 | 说明                     |
| ------ | ------------------------ | -------- | ------------------------ |
| 0x00   | 空闲                     | 否       | 端口空闲，可以开始充电   |
| 0x01   | 充电中                   | 是       | 正在充电                 |
| 0x02   | 有充电器但未充电(未启动) | 否       | 充电器已插入但未开始充电 |
| 0x03   | 有充电器但未充电(已充满) | 否       | 充电完成                 |
| 0x04   | 该路无法计量             | 否       | 计量模块故障             |
| 0x05   | 浮充                     | 是       | 维持充电状态             |
| 0x06   | 存储器损坏               | 否       | 存储器故障               |
| 0x07   | 插座弹片卡住故障         | 否       | 机械故障                 |
| 0x08   | 接触不良或保险丝烧断故障 | 否       | 电气故障                 |
| 0x09   | 继电器粘连               | 否       | 继电器故障               |
| 0x0A   | 霍尔开关损坏             | 否       | 传感器故障               |
| 0x0B   | 继电器坏或保险丝断       | 否       | 电气故障                 |
| 0x0C   | 负载短路                 | 否       | 短路故障                 |
| 0x0D   | 继电器粘连(预检)         | 否       | 预检测到继电器故障       |
| 0x0E   | 刷卡芯片损坏故障         | 否       | 刷卡模块故障             |
| 0x0F   | 检测电路故障             | 否       | 检测电路故障             |

## 响应要求

### 成功响应

第三方平台接收到通知后，应返回 HTTP 200 状态码，响应体格式如下：

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "event_id": "550e8400-e29b-41d4-a716-446655440000",
    "received_time": 1703123456
  }
}
```

### 错误响应

如果处理失败，应返回相应的 HTTP 错误状态码：

```json
{
  "code": 400,
  "message": "Invalid request format",
  "data": {
    "event_id": "550e8400-e29b-41d4-a716-446655440000",
    "error_details": "Missing required field: device_id"
  }
}
```

**支持的错误状态码：**

- 400 Bad Request - 请求格式错误
- 401 Unauthorized - 认证失败
- 403 Forbidden - 权限不足
- 404 Not Found - 接口不存在
- 500 Internal Server Error - 服务器内部错误

## 重试机制

### 重试条件

IoT-Zinx 系统在以下情况下会进行重试：

- HTTP 状态码为 5xx（服务器错误）
- 连接超时（10 秒）
- 网络连接失败
- 响应格式不正确

### 重试策略

- 最大重试次数：3 次
- 重试间隔：指数退避（1 秒 → 2 秒 → 4 秒）
- 重试总时长：最长 7 秒

### 幂等性保证

每个事件都有唯一的`event_id`，第三方平台应根据此 ID 进行去重处理，避免重复处理同一事件。

## 安全要求

### 认证方式

支持以下认证方式：

1. **Bearer Token 认证**

```http
Authorization: Bearer your_api_token_here
```

2. **API Key 认证**

```http
X-API-Key: your_api_key_here
```

### HTTPS 要求

生产环境强烈建议使用 HTTPS 协议，确保数据传输安全。

### IP 白名单

建议配置 IP 白名单，只允许 IoT-Zinx 系统的 IP 地址访问回调接口。

## 数据格式说明

### 时间格式

- Unix 时间戳：整数，精确到秒
- 格式化时间：字符串，格式为"YYYY-MM-DD HH:mm:ss"，使用北京时间（UTC+8）

### 数值格式

- 功率：浮点数，单位瓦特（W）
- 电压：浮点数，单位伏特（V）
- 电流：浮点数，单位安培（A）
- 电量：浮点数，单位度（kWh）
- 费用：整数，单位分（1 元=100 分）
- 温度：整数，单位摄氏度（℃）

### 设备 ID 格式

设备 ID 为 8 位十六进制字符串，例如："04A228CD"

- 对应十进制值：77915341
- 显示格式：保持十六进制大写格式

## 配置示例

### 计费系统端点配置

```yaml
notification:
  enabled: true
  endpoints:
    - name: "billing_system"
      type: "billing"
      url: "https://billing.example.com/api/charging/callback"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer your_billing_token"
      timeout: "10s"
      event_types:
        - "charging_start"
        - "charging_end"
        - "charging_failed"
        - "settlement"
        - "power_heartbeat"
        - "charging_power"
      enabled: true
```

### 运营平台端点配置

```yaml
- name: "operation_platform"
  type: "operation"
  url: "https://operation.example.com/api/device/callback"
  headers:
    Content-Type: "application/json"
    X-API-Key: "your_operation_key"
  timeout: "10s"
  event_types:
    - "device_online"
    - "device_offline"
    - "device_register"
    - "device_heartbeat"
    - "port_heartbeat"
    - "charging_start"
    - "charging_end"
    - "charging_failed"
  enabled: true
```

## 测试工具

### 模拟推送工具

可以使用以下 curl 命令模拟 IoT-Zinx 系统的推送：

```bash
curl -X POST https://your-domain.com/api/callback \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_token" \
  -H "X-Event-Type: device_online" \
  -H "X-Device-ID: 04A228CD" \
  -d '{
    "event_id": "550e8400-e29b-41d4-a716-446655440000",
    "event_type": "device_online",
    "device_id": "04A228CD",
    "port_number": 0,
    "timestamp": 1703123456,
    "data": {
      "conn_id": 12345,
      "remote_addr": "192.168.1.100:54321",
      "connect_time": 1703123456
    }
  }'
```

### 验证清单

在对接完成后，请验证以下功能：

- [ ] 能够正确接收所有事件类型
- [ ] 响应格式符合要求
- [ ] 错误处理机制正常
- [ ] 幂等性处理正确
- [ ] 认证机制工作正常
- [ ] 超时处理合理
- [ ] 日志记录完整

## 常见问题

### Q1: 如何处理重复事件？

A: 使用 event_id 进行去重，相同 event_id 的事件只处理一次。

### Q2: 推送失败如何处理？

A: IoT-Zinx 会自动重试 3 次，如果仍然失败，事件会被记录到错误日志中。

### Q3: 如何确保数据完整性？

A: 建议实现接收确认机制，并定期与 IoT-Zinx 系统进行数据对账。

### Q4: 支持批量推送吗？

A: 目前只支持单个事件推送，确保实时性和可靠性。

### Q5: 如何处理网络中断？

A: IoT-Zinx 会在网络恢复后继续推送，但可能存在数据延迟。

## 技术支持

如有技术问题，请联系：

- 技术支持邮箱：support@bujia.com
- 技术文档：https://docs.bujia.com/iot-zinx
- API 测试环境：https://test-api.bujia.com
