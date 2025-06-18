# 充电桩API接口文档

## 概述

本文档描述了IoT-Zinx充电桩系统提供的REST API接口，供第三方平台集成使用。

## 基础信息

- **基础URL**: `http://localhost:7055/api/v1`
- **认证方式**: API Key + Secret
- **数据格式**: JSON
- **字符编码**: UTF-8

## 认证

所有API请求都需要在请求头中包含认证信息：

```http
X-API-Key: your_api_key_here
X-API-Secret: your_secret_here
Content-Type: application/json
```

## 通用响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": {},
  "timestamp": 1640995200
}
```

- `code`: 状态码，0表示成功，非0表示错误
- `message`: 响应消息
- `data`: 响应数据
- `timestamp`: 响应时间戳

## 错误码说明

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 参数错误 |
| 1002 | 设备不存在 |
| 1003 | 设备离线 |
| 1004 | 端口占用 |
| 1005 | 余额不足 |
| 1006 | 充电中 |
| 1007 | 设备故障 |
| 2001 | 认证失败 |
| 2002 | 权限不足 |
| 5001 | 系统错误 |

## API接口

### 1. 设备管理

#### 1.1 获取设备列表

**请求**
```http
GET /devices
```

**响应**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "devices": [
      {
        "device_id": "04ceaa40",
        "name": "充电桩001",
        "status": "online",
        "location": "停车场A区",
        "ports": [
          {
            "port_number": 1,
            "status": "available",
            "max_power": 2200
          }
        ],
        "last_heartbeat": "2024-01-01T12:00:00Z"
      }
    ],
    "total": 1
  }
}
```

#### 1.2 获取设备状态

**请求**
```http
GET /devices/{device_id}/status
```

**参数**
- `device_id`: 设备ID

**响应**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "device_id": "04ceaa40",
    "status": "online",
    "ports": [
      {
        "port_number": 1,
        "status": "charging",
        "current_power": 2200,
        "voltage": 220,
        "current": 10.0,
        "temperature": 25.5,
        "order_number": "ORDER_123456"
      }
    ],
    "last_update": "2024-01-01T12:00:00Z"
  }
}
```

### 2. 充电控制

#### 2.1 开始充电

**请求**
```http
POST /charging/start
```

**请求体**
```json
{
  "device_id": "04ceaa40",
  "port_number": 1,
  "order_number": "ORDER_123456",
  "charge_mode": 1,
  "charge_value": 60,
  "amount": 10.00,
  "card_id": 12345678,
  "max_power": 2200
}
```

**参数说明**
- `device_id`: 设备ID
- `port_number`: 端口号
- `order_number`: 订单号
- `charge_mode`: 充电模式 (1=按时间, 2=按电量)
- `charge_value`: 充电值 (时间单位:分钟, 电量单位:0.1度)
- `amount`: 预付金额
- `card_id`: 卡号
- `max_power`: 最大功率

**响应**
```json
{
  "code": 0,
  "message": "充电启动成功",
  "data": {
    "order_number": "ORDER_123456",
    "device_id": "04ceaa40",
    "port_number": 1,
    "status": "charging_started",
    "start_time": "2024-01-01T12:00:00Z"
  }
}
```

#### 2.2 停止充电

**请求**
```http
POST /charging/stop
```

**请求体**
```json
{
  "device_id": "04ceaa40",
  "port_number": 1,
  "order_number": "ORDER_123456",
  "reason": "用户主动停止"
}
```

**响应**
```json
{
  "code": 0,
  "message": "充电停止成功",
  "data": {
    "order_number": "ORDER_123456",
    "device_id": "04ceaa40",
    "port_number": 1,
    "status": "charging_stopped",
    "stop_time": "2024-01-01T12:30:00Z",
    "duration": 1800,
    "consumed_energy": 1.5,
    "consumed_amount": 7.50
  }
}
```

#### 2.3 查询充电状态

**请求**
```http
GET /charging/{order_number}/status
```

**参数**
- `order_number`: 订单号

**响应**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_number": "ORDER_123456",
    "device_id": "04ceaa40",
    "port_number": 1,
    "status": "charging",
    "start_time": "2024-01-01T12:00:00Z",
    "duration": 900,
    "current_power": 2200,
    "consumed_energy": 0.75,
    "consumed_amount": 3.75,
    "remaining_time": 2700
  }
}
```

### 3. 事件通知

系统会向第三方平台推送以下事件：

#### 3.1 设备上线事件

```json
{
  "event_type": "device_online",
  "data": {
    "device_id": "04ceaa40",
    "iccid": "89860000000000000000",
    "timestamp": 1640995200
  },
  "timestamp": 1640995200
}
```

#### 3.2 设备下线事件

```json
{
  "event_type": "device_offline",
  "data": {
    "device_id": "04ceaa40",
    "reason": "connection_lost",
    "timestamp": 1640995200
  },
  "timestamp": 1640995200
}
```

#### 3.3 充电开始事件

```json
{
  "event_type": "charging_start",
  "data": {
    "device_id": "04ceaa40",
    "port_number": 1,
    "card_id": 12345678,
    "order_number": "ORDER_123456",
    "timestamp": 1640995200
  },
  "timestamp": 1640995200
}
```

#### 3.4 充电结束事件

```json
{
  "event_type": "charging_end",
  "data": {
    "device_id": "04ceaa40",
    "port_number": 1,
    "order_number": "ORDER_123456",
    "reason": "completed",
    "consumed_energy": 1.5,
    "consumed_amount": 7.50,
    "timestamp": 1640995200
  },
  "timestamp": 1640995200
}
```

#### 3.5 充电状态变更事件

```json
{
  "event_type": "charging_status",
  "data": {
    "device_id": "04ceaa40",
    "port_number": 1,
    "order_number": "ORDER_123456",
    "status": "charging",
    "current_power": 2200,
    "total_energy": 0.75,
    "timestamp": 1640995200
  },
  "timestamp": 1640995200
}
```

### 4. 批量操作

#### 4.1 批量查询设备状态

**请求**
```http
POST /devices/batch/status
```

**请求体**
```json
{
  "device_ids": ["04ceaa40", "04ceaa41", "04ceaa42"]
}
```

**响应**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "devices": [
      {
        "device_id": "04ceaa40",
        "status": "online",
        "ports": [...]
      }
    ],
    "success_count": 3,
    "failed_count": 0
  }
}
```

## SDK示例

### JavaScript/Node.js

```javascript
const axios = require('axios');

class ChargingAPI {
  constructor(baseURL, apiKey, secret) {
    this.client = axios.create({
      baseURL: baseURL,
      headers: {
        'X-API-Key': apiKey,
        'X-API-Secret': secret,
        'Content-Type': 'application/json'
      }
    });
  }

  async startCharging(params) {
    try {
      const response = await this.client.post('/charging/start', params);
      return response.data;
    } catch (error) {
      throw new Error(`充电启动失败: ${error.response?.data?.message || error.message}`);
    }
  }

  async stopCharging(params) {
    try {
      const response = await this.client.post('/charging/stop', params);
      return response.data;
    } catch (error) {
      throw new Error(`充电停止失败: ${error.response?.data?.message || error.message}`);
    }
  }

  async getChargingStatus(orderNumber) {
    try {
      const response = await this.client.get(`/charging/${orderNumber}/status`);
      return response.data;
    } catch (error) {
      throw new Error(`查询充电状态失败: ${error.response?.data?.message || error.message}`);
    }
  }
}

// 使用示例
const api = new ChargingAPI('http://localhost:7055/api/v1', 'your_api_key', 'your_secret');

// 开始充电
api.startCharging({
  device_id: '04ceaa40',
  port_number: 1,
  order_number: 'ORDER_' + Date.now(),
  charge_mode: 1,
  charge_value: 60,
  amount: 10.00,
  card_id: 12345678,
  max_power: 2200
}).then(result => {
  console.log('充电启动成功:', result);
}).catch(error => {
  console.error('充电启动失败:', error.message);
});
```

### Python

```python
import requests
import json

class ChargingAPI:
    def __init__(self, base_url, api_key, secret):
        self.base_url = base_url
        self.headers = {
            'X-API-Key': api_key,
            'X-API-Secret': secret,
            'Content-Type': 'application/json'
        }
    
    def start_charging(self, params):
        url = f"{self.base_url}/charging/start"
        response = requests.post(url, headers=self.headers, json=params)
        
        if response.status_code == 200:
            return response.json()
        else:
            raise Exception(f"充电启动失败: {response.text}")
    
    def stop_charging(self, params):
        url = f"{self.base_url}/charging/stop"
        response = requests.post(url, headers=self.headers, json=params)
        
        if response.status_code == 200:
            return response.json()
        else:
            raise Exception(f"充电停止失败: {response.text}")
    
    def get_charging_status(self, order_number):
        url = f"{self.base_url}/charging/{order_number}/status"
        response = requests.get(url, headers=self.headers)
        
        if response.status_code == 200:
            return response.json()
        else:
            raise Exception(f"查询充电状态失败: {response.text}")

# 使用示例
api = ChargingAPI('http://localhost:7055/api/v1', 'your_api_key', 'your_secret')

try:
    result = api.start_charging({
        'device_id': '04ceaa40',
        'port_number': 1,
        'order_number': f'ORDER_{int(time.time())}',
        'charge_mode': 1,
        'charge_value': 60,
        'amount': 10.00,
        'card_id': 12345678,
        'max_power': 2200
    })
    print('充电启动成功:', result)
except Exception as e:
    print('充电启动失败:', str(e))
```

## 注意事项

1. **请求频率限制**: 每个API Key每分钟最多1000次请求
2. **超时设置**: 建议设置30秒的请求超时时间
3. **重试机制**: 建议实现指数退避的重试机制
4. **事件处理**: 第三方平台需要提供事件接收接口
5. **安全性**: 请妥善保管API Key和Secret，不要在客户端代码中暴露

## 联系支持

如有问题，请联系技术支持：
- 邮箱: support@example.com
- 电话: 400-000-0000
