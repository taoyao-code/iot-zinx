# IoT-Zinx 第三方平台对接快速开始指南

## 概述

本指南帮助第三方平台快速完成与IoT-Zinx系统的对接，包括接口开发、测试验证和上线部署的完整流程。

## 对接流程概览

```mermaid
graph TD
    A[准备工作] --> B[开发回调接口]
    B --> C[配置IoT-Zinx系统]
    C --> D[测试验证]
    D --> E[上线部署]
    E --> F[监控运维]
```

## 第一步：准备工作

### 1.1 确定对接需求

**计费系统关注的事件：**
- ✅ 充电开始 (charging_start)
- ✅ 充电结束 (charging_end)
- ✅ 充电失败 (charging_failed)
- ✅ 结算数据 (settlement)
- ✅ 功率心跳 (power_heartbeat)
- ✅ 充电功率 (charging_power)

**运营平台关注的事件：**
- ✅ 设备上线 (device_online)
- ✅ 设备离线 (device_offline)
- ✅ 设备注册 (device_register)
- ✅ 设备心跳 (device_heartbeat)
- ✅ 端口心跳 (port_heartbeat)
- ✅ 充电开始/结束/失败
- ✅ 结算数据

### 1.2 技术准备

**必需的技术栈：**
- HTTP服务器（支持POST请求）
- JSON解析库
- 数据库（用于存储事件数据）
- 日志系统
- 监控系统（可选）

**推荐的技术选择：**
- Node.js + Express
- Python + Flask/Django
- Java + Spring Boot
- Go + Gin

### 1.3 网络准备

**网络要求：**
- 提供公网可访问的HTTPS接口
- 配置SSL证书
- 开放相应端口（通常443）
- 配置防火墙规则

## 第二步：开发回调接口

### 2.1 最小化实现

以下是一个最小化的回调接口实现（Node.js示例）：

```javascript
const express = require('express');
const app = express();

app.use(express.json());

// 已处理事件缓存（生产环境建议使用Redis）
const processedEvents = new Set();

app.post('/api/iot/callback', (req, res) => {
  try {
    const { event_id, event_type, device_id, data } = req.body;
    
    // 基础验证
    if (!event_id || !event_type || !device_id) {
      return res.status(400).json({
        code: 400,
        message: 'Missing required fields'
      });
    }
    
    // 幂等性检查
    if (processedEvents.has(event_id)) {
      return res.status(200).json({
        code: 200,
        message: 'Event already processed',
        data: { event_id, received_time: Math.floor(Date.now() / 1000) }
      });
    }
    
    // 处理事件
    console.log(`Received event: ${event_type} from device: ${device_id}`);
    processEvent(req.body);
    processedEvents.add(event_id);
    
    // 返回成功响应
    res.status(200).json({
      code: 200,
      message: 'success',
      data: {
        event_id,
        received_time: Math.floor(Date.now() / 1000)
      }
    });
    
  } catch (error) {
    console.error('Error processing event:', error);
    res.status(500).json({
      code: 500,
      message: 'Internal server error'
    });
  }
});

function processEvent(event) {
  // 根据事件类型处理业务逻辑
  switch (event.event_type) {
    case 'device_online':
      handleDeviceOnline(event);
      break;
    case 'charging_start':
      handleChargingStart(event);
      break;
    case 'settlement':
      handleSettlement(event);
      break;
    default:
      console.log(`Unhandled event type: ${event.event_type}`);
  }
}

function handleDeviceOnline(event) {
  console.log(`Device ${event.device_id} came online`);
  // 更新设备状态到数据库
}

function handleChargingStart(event) {
  console.log(`Charging started on device ${event.device_id} port ${event.port_number}`);
  // 创建充电订单
}

function handleSettlement(event) {
  console.log(`Settlement received for order ${event.data.order_id}`);
  // 处理结算数据
}

app.listen(3000, () => {
  console.log('IoT callback server listening on port 3000');
});
```

### 2.2 数据库设计

**事件日志表：**
```sql
CREATE TABLE iot_events (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  event_id VARCHAR(64) UNIQUE NOT NULL,
  event_type VARCHAR(32) NOT NULL,
  device_id VARCHAR(16) NOT NULL,
  port_number INT,
  event_data JSON,
  received_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  processed BOOLEAN DEFAULT FALSE,
  INDEX idx_device_id (device_id),
  INDEX idx_event_type (event_type),
  INDEX idx_received_time (received_time)
);
```

**设备状态表：**
```sql
CREATE TABLE device_status (
  device_id VARCHAR(16) PRIMARY KEY,
  status ENUM('online', 'offline') NOT NULL,
  last_heartbeat TIMESTAMP,
  firmware_version VARCHAR(32),
  signal_strength INT,
  temperature INT,
  updated_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**充电订单表：**
```sql
CREATE TABLE charging_orders (
  order_id VARCHAR(32) PRIMARY KEY,
  device_id VARCHAR(16) NOT NULL,
  port_number INT NOT NULL,
  start_time TIMESTAMP,
  end_time TIMESTAMP,
  total_energy DECIMAL(10,2),
  total_fee INT,
  status ENUM('charging', 'completed', 'failed') NOT NULL,
  created_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  INDEX idx_device_port (device_id, port_number)
);
```

## 第三步：配置IoT-Zinx系统

### 3.1 配置文件修改

在IoT-Zinx系统的`configs/gateway.yaml`中添加您的回调地址：

```yaml
notification:
  enabled: true
  endpoints:
    - name: "your_platform"
      type: "billing"  # 或 "operation"
      url: "https://your-domain.com/api/iot/callback"
      headers:
        Content-Type: "application/json"
        Authorization: "Bearer your_api_token"
      timeout: "10s"
      event_types:
        - "device_online"
        - "charging_start"
        - "charging_end"
        - "charging_failed"
        - "settlement"
        - "power_heartbeat"
      enabled: true
```

### 3.2 认证配置

**Bearer Token认证：**
```yaml
headers:
  Authorization: "Bearer your_secret_token_here"
```

**API Key认证：**
```yaml
headers:
  X-API-Key: "your_api_key_here"
```

## 第四步：测试验证

### 4.1 本地测试

使用curl命令模拟IoT-Zinx的推送：

```bash
# 测试设备上线事件
curl -X POST https://your-domain.com/api/iot/callback \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_token" \
  -d '{
    "event_id": "test-001",
    "event_type": "device_online",
    "device_id": "04A228CD",
    "timestamp": 1703123456,
    "data": {
      "conn_id": 12345,
      "remote_addr": "192.168.1.100:54321",
      "connect_time": 1703123456
    }
  }'

# 测试充电开始事件
curl -X POST https://your-domain.com/api/iot/callback \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your_token" \
  -d '{
    "event_id": "test-002",
    "event_type": "charging_start",
    "device_id": "04A228CD",
    "port_number": 1,
    "timestamp": 1703123456,
    "data": {
      "order_number": "TEST_ORDER_001",
      "response_code": 0,
      "status_desc": "充电开始成功"
    }
  }'
```

### 4.2 验证清单

- [ ] 接口能正确接收所有事件类型
- [ ] 响应格式符合要求（HTTP 200 + JSON）
- [ ] 幂等性处理正确（相同event_id只处理一次）
- [ ] 错误处理机制正常
- [ ] 认证机制工作正常
- [ ] 数据存储正确
- [ ] 日志记录完整

### 4.3 压力测试

```bash
# 使用ab工具进行压力测试
ab -n 1000 -c 10 -p test_event.json -T application/json \
   -H "Authorization: Bearer your_token" \
   https://your-domain.com/api/iot/callback
```

## 第五步：上线部署

### 5.1 生产环境配置

**HTTPS配置：**
```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;
    
    ssl_certificate /path/to/your/cert.pem;
    ssl_certificate_key /path/to/your/key.pem;
    
    location /api/iot/callback {
        proxy_pass http://localhost:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

**PM2部署（Node.js）：**
```json
{
  "name": "iot-callback",
  "script": "app.js",
  "instances": 2,
  "exec_mode": "cluster",
  "env": {
    "NODE_ENV": "production",
    "PORT": 3000
  }
}
```

### 5.2 监控配置

**健康检查接口：**
```javascript
app.get('/health', (req, res) => {
  res.status(200).json({
    status: 'healthy',
    timestamp: new Date().toISOString(),
    uptime: process.uptime()
  });
});
```

**监控指标：**
- 接口响应时间
- 请求成功率
- 错误率
- 队列长度（如果使用异步处理）

## 第六步：监控运维

### 6.1 日志监控

**关键日志内容：**
```javascript
function logEvent(event, status, error = null) {
  const logData = {
    timestamp: new Date().toISOString(),
    event_id: event.event_id,
    event_type: event.event_type,
    device_id: event.device_id,
    status,
    processing_time: Date.now() - startTime,
    error: error ? error.message : null
  };
  
  console.log(JSON.stringify(logData));
}
```

### 6.2 告警配置

**告警规则：**
- 接口响应时间 > 5秒
- 错误率 > 5%
- 连续5分钟无事件接收
- 数据库连接失败

### 6.3 数据对账

**定期对账脚本：**
```javascript
async function dailyReconciliation() {
  const today = new Date().toISOString().split('T')[0];
  
  // 统计今日事件数量
  const eventCounts = await db.query(`
    SELECT event_type, COUNT(*) as count 
    FROM iot_events 
    WHERE DATE(received_time) = ? 
    GROUP BY event_type
  `, [today]);
  
  // 发送对账报告
  await sendReconciliationReport(eventCounts);
}
```

## 常见问题解决

### Q1: 接收到重复事件怎么办？
A: 使用event_id进行去重，相同event_id的事件只处理一次。

### Q2: 如何处理网络超时？
A: IoT-Zinx会自动重试，确保您的接口在10秒内响应即可。

### Q3: 数据格式不符合预期怎么办？
A: 参考[数据字典文档](./third_party_data_dictionary.md)确认字段定义。

### Q4: 如何确保数据安全？
A: 使用HTTPS协议，配置API认证，设置IP白名单。

## 技术支持

如需技术支持，请联系：
- 技术支持邮箱：support@bujia.com
- 技术文档：https://docs.bujia.com/iot-zinx
- 示例代码：https://github.com/bujia-iot/integration-examples

## 相关文档

- [第三方平台对接API文档](./third_party_integration_api.md)
- [第三方平台对接示例](./third_party_integration_examples.md)
- [第三方平台数据字典](./third_party_data_dictionary.md)
- [通知系统概览](./notification_system_overview.md)
