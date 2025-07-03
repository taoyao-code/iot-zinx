# IoT-Zinx 第三方平台对接示例

## 完整业务流程示例

### 典型充电流程事件序列

以下是一个完整充电流程的事件推送序列：

#### 1. 设备上线
```json
{
  "event_id": "evt_001",
  "event_type": "device_online",
  "device_id": "04A228CD",
  "timestamp": 1703123400,
  "data": {
    "conn_id": 12345,
    "remote_addr": "192.168.1.100:54321",
    "connect_time": 1703123400,
    "device_type": 1,
    "firmware_version": "V2.1.0"
  }
}
```

#### 2. 设备注册
```json
{
  "event_id": "evt_002",
  "event_type": "device_register",
  "device_id": "04A228CD",
  "timestamp": 1703123401,
  "data": {
    "physical_id": "0x04A228CD",
    "iccid": "89860318123456789012",
    "device_version": "V2.1.0",
    "heartbeat_period": 30
  }
}
```

#### 3. 设备心跳（显示端口状态）
```json
{
  "event_id": "evt_003",
  "event_type": "device_heartbeat",
  "device_id": "04A228CD",
  "timestamp": 1703123430,
  "data": {
    "voltage": 220.5,
    "port_count": 10,
    "port_statuses": [
      {"port": 1, "status": 0, "status_desc": "空闲"},
      {"port": 2, "status": 0, "status_desc": "空闲"}
    ],
    "signal_strength": 85,
    "temperature": 25
  }
}
```

#### 4. 充电开始
```json
{
  "event_id": "evt_004",
  "event_type": "charging_start",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123500,
  "data": {
    "order_number": "ORD20231221001",
    "response_code": 0,
    "status_desc": "充电开始成功",
    "start_time": 1703123500
  }
}
```

#### 5. 端口状态变化
```json
{
  "event_id": "evt_005",
  "event_type": "port_heartbeat",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123501,
  "data": {
    "port_status": 1,
    "port_status_desc": "充电中",
    "previous_status": 0,
    "previous_status_desc": "空闲",
    "is_charging": true
  }
}
```

#### 6. 功率心跳（充电过程中）
```json
{
  "event_id": "evt_006",
  "event_type": "power_heartbeat",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123530,
  "data": {
    "current_power": 2200.5,
    "cumulative_energy": 0.61,
    "voltage": 220.5,
    "current": 10.2,
    "charging_status": 1,
    "charging_status_desc": "正常充电"
  }
}
```

#### 7. 充电功率实时数据
```json
{
  "event_id": "evt_007",
  "event_type": "charging_power",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703125300,
  "data": {
    "instant_power": 2150.0,
    "cumulative_energy": 3.58,
    "charging_duration": 1800,
    "voltage": 220.2,
    "current": 9.8,
    "order_id": "ORD20231221001"
  }
}
```

#### 8. 充电结束
```json
{
  "event_id": "evt_008",
  "event_type": "charging_end",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703127100,
  "data": {
    "order_id": "ORD20231221001",
    "total_energy": 15.50,
    "charge_duration": 3600,
    "start_time": "2023-12-21 10:00:00",
    "end_time": "2023-12-21 11:00:00",
    "stop_reason": 1,
    "stop_reason_desc": "手动停止"
  }
}
```

#### 9. 结算数据
```json
{
  "event_id": "evt_009",
  "event_type": "settlement",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703127101,
  "data": {
    "order_id": "ORD20231221001",
    "card_number": "12345678",
    "total_energy": 15.50,
    "total_fee": 1550,
    "charge_fee": 1400,
    "service_fee": 150,
    "start_time": 1703123500,
    "end_time": 1703127100,
    "settlement_type": "normal"
  }
}
```

## 异常情况处理示例

### 充电失败场景

#### 1. 设备故障导致充电失败
```json
{
  "event_id": "evt_err_001",
  "event_type": "charging_failed",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123500,
  "data": {
    "order_number": "ORD20231221002",
    "response_code": 7,
    "status_desc": "插座弹片卡住故障",
    "failure_reason": "插座弹片卡住故障",
    "error_code": 7,
    "failed_time": 1703123500
  }
}
```

#### 2. 端口故障状态推送
```json
{
  "event_id": "evt_err_002",
  "event_type": "port_heartbeat",
  "device_id": "04A228CD",
  "port_number": 1,
  "timestamp": 1703123501,
  "data": {
    "port_status": 7,
    "port_status_desc": "插座弹片卡住故障",
    "previous_status": 0,
    "previous_status_desc": "空闲",
    "is_charging": false,
    "fault_detected": true
  }
}
```

### 设备离线场景

#### 1. 心跳超时离线
```json
{
  "event_id": "evt_offline_001",
  "event_type": "device_offline",
  "device_id": "04A228CD",
  "timestamp": 1703123600,
  "data": {
    "conn_id": 12345,
    "offline_reason": "heartbeat_timeout",
    "last_heartbeat_time": 1703123540,
    "offline_duration": 60,
    "connection_duration": 3600
  }
}
```

#### 2. 网络连接断开
```json
{
  "event_id": "evt_offline_002",
  "event_type": "device_offline",
  "device_id": "04A228CD",
  "timestamp": 1703123700,
  "data": {
    "conn_id": 12345,
    "offline_reason": "connection_lost",
    "last_heartbeat_time": 1703123680,
    "offline_duration": 20,
    "connection_duration": 7200
  }
}
```

## 第三方平台接收端实现示例

### Node.js Express 实现

```javascript
const express = require('express');
const app = express();

app.use(express.json());

// 事件处理映射
const eventHandlers = {
  'device_online': handleDeviceOnline,
  'device_offline': handleDeviceOffline,
  'charging_start': handleChargingStart,
  'charging_end': handleChargingEnd,
  'charging_failed': handleChargingFailed,
  'settlement': handleSettlement,
  'power_heartbeat': handlePowerHeartbeat
};

// 主要回调接口
app.post('/api/iot/callback', async (req, res) => {
  try {
    const { event_id, event_type, device_id, data } = req.body;
    
    // 验证必填字段
    if (!event_id || !event_type || !device_id) {
      return res.status(400).json({
        code: 400,
        message: 'Missing required fields',
        data: { event_id }
      });
    }
    
    // 幂等性检查
    if (await isEventProcessed(event_id)) {
      return res.status(200).json({
        code: 200,
        message: 'Event already processed',
        data: { event_id, received_time: Date.now() / 1000 }
      });
    }
    
    // 处理事件
    const handler = eventHandlers[event_type];
    if (handler) {
      await handler(req.body);
      await markEventProcessed(event_id);
    } else {
      console.warn(`Unknown event type: ${event_type}`);
    }
    
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
      message: 'Internal server error',
      data: { event_id: req.body.event_id }
    });
  }
});

// 事件处理函数示例
async function handleDeviceOnline(event) {
  const { device_id, data } = event;
  console.log(`Device ${device_id} came online`);
  
  // 更新设备状态
  await updateDeviceStatus(device_id, 'online', data);
  
  // 发送通知
  await sendNotification('device_online', device_id, data);
}

async function handleChargingStart(event) {
  const { device_id, port_number, data } = event;
  console.log(`Charging started on device ${device_id} port ${port_number}`);
  
  // 创建充电订单
  await createChargingOrder(data.order_number, device_id, port_number, data);
  
  // 更新端口状态
  await updatePortStatus(device_id, port_number, 'charging');
}

async function handleSettlement(event) {
  const { device_id, port_number, data } = event;
  console.log(`Settlement received for device ${device_id} port ${port_number}`);
  
  // 处理结算
  await processSettlement(data.order_id, data);
  
  // 生成账单
  await generateBill(data.order_id, data.total_fee);
}

app.listen(3000, () => {
  console.log('IoT callback server listening on port 3000');
});
```

### Python Flask 实现

```python
from flask import Flask, request, jsonify
import time
import logging

app = Flask(__name__)
logging.basicConfig(level=logging.INFO)

# 已处理事件缓存（生产环境建议使用Redis）
processed_events = set()

@app.route('/api/iot/callback', methods=['POST'])
def iot_callback():
    try:
        data = request.get_json()
        
        # 验证必填字段
        required_fields = ['event_id', 'event_type', 'device_id']
        for field in required_fields:
            if field not in data:
                return jsonify({
                    'code': 400,
                    'message': f'Missing required field: {field}',
                    'data': {'event_id': data.get('event_id')}
                }), 400
        
        event_id = data['event_id']
        event_type = data['event_type']
        device_id = data['device_id']
        
        # 幂等性检查
        if event_id in processed_events:
            return jsonify({
                'code': 200,
                'message': 'Event already processed',
                'data': {
                    'event_id': event_id,
                    'received_time': int(time.time())
                }
            })
        
        # 处理事件
        handler = event_handlers.get(event_type)
        if handler:
            handler(data)
            processed_events.add(event_id)
        else:
            logging.warning(f'Unknown event type: {event_type}')
        
        # 返回成功响应
        return jsonify({
            'code': 200,
            'message': 'success',
            'data': {
                'event_id': event_id,
                'received_time': int(time.time())
            }
        })
        
    except Exception as e:
        logging.error(f'Error processing event: {e}')
        return jsonify({
            'code': 500,
            'message': 'Internal server error',
            'data': {'event_id': data.get('event_id') if 'data' in locals() else None}
        }), 500

# 事件处理函数
def handle_device_online(event):
    device_id = event['device_id']
    event_data = event['data']
    logging.info(f'Device {device_id} came online')
    
    # 处理设备上线逻辑
    update_device_status(device_id, 'online', event_data)

def handle_charging_start(event):
    device_id = event['device_id']
    port_number = event['port_number']
    event_data = event['data']
    logging.info(f'Charging started on device {device_id} port {port_number}')
    
    # 处理充电开始逻辑
    create_charging_order(event_data['order_number'], device_id, port_number, event_data)

# 事件处理器映射
event_handlers = {
    'device_online': handle_device_online,
    'device_offline': handle_device_offline,
    'charging_start': handle_charging_start,
    'charging_end': handle_charging_end,
    'settlement': handle_settlement
}

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, debug=False)
```

## 数据验证和错误处理

### 数据验证规则

```javascript
const eventValidationRules = {
  'device_online': {
    required: ['device_id', 'data.conn_id', 'data.connect_time'],
    optional: ['data.device_type', 'data.firmware_version']
  },
  'charging_start': {
    required: ['device_id', 'port_number', 'data.order_number'],
    optional: ['data.response_code', 'data.start_time']
  },
  'settlement': {
    required: ['device_id', 'port_number', 'data.order_id', 'data.total_energy', 'data.total_fee'],
    optional: ['data.settlement_type', 'data.card_number']
  }
};

function validateEvent(event) {
  const rules = eventValidationRules[event.event_type];
  if (!rules) return { valid: true };
  
  const errors = [];
  
  // 检查必填字段
  for (const field of rules.required) {
    if (!getNestedValue(event, field)) {
      errors.push(`Missing required field: ${field}`);
    }
  }
  
  return {
    valid: errors.length === 0,
    errors
  };
}

function getNestedValue(obj, path) {
  return path.split('.').reduce((current, key) => current && current[key], obj);
}
```

### 错误处理最佳实践

1. **记录详细日志**
```javascript
function logEvent(event, status, error = null) {
  const logData = {
    timestamp: new Date().toISOString(),
    event_id: event.event_id,
    event_type: event.event_type,
    device_id: event.device_id,
    status,
    error: error ? error.message : null
  };
  
  console.log(JSON.stringify(logData));
}
```

2. **实现监控和告警**
```javascript
function monitorEventProcessing(event, processingTime, success) {
  // 发送监控指标
  metrics.increment('iot.events.received', {
    event_type: event.event_type,
    success: success.toString()
  });
  
  metrics.histogram('iot.events.processing_time', processingTime, {
    event_type: event.event_type
  });
  
  // 失败告警
  if (!success) {
    alerting.sendAlert('iot_event_processing_failed', {
      event_id: event.event_id,
      event_type: event.event_type,
      device_id: event.device_id
    });
  }
}
```

## 性能优化建议

### 1. 异步处理
```javascript
// 使用消息队列异步处理事件
async function handleEventAsync(event) {
  // 立即返回确认
  const response = {
    code: 200,
    message: 'success',
    data: { event_id: event.event_id, received_time: Date.now() / 1000 }
  };
  
  // 异步处理事件
  messageQueue.publish('iot_events', event);
  
  return response;
}
```

### 2. 批量处理
```javascript
// 批量处理相同类型的事件
const eventBuffer = new Map();

function bufferEvent(event) {
  const key = `${event.event_type}_${event.device_id}`;
  if (!eventBuffer.has(key)) {
    eventBuffer.set(key, []);
  }
  eventBuffer.get(key).push(event);
  
  // 达到批量大小或超时时处理
  if (eventBuffer.get(key).length >= BATCH_SIZE) {
    processBatch(key);
  }
}
```

### 3. 缓存优化
```javascript
// 使用Redis缓存设备状态
const redis = require('redis');
const client = redis.createClient();

async function cacheDeviceStatus(deviceId, status) {
  await client.setex(`device:${deviceId}:status`, 3600, JSON.stringify(status));
}

async function getCachedDeviceStatus(deviceId) {
  const cached = await client.get(`device:${deviceId}:status`);
  return cached ? JSON.parse(cached) : null;
}
```

这份详细的对接文档提供了完整的API规范、实现示例和最佳实践，第三方平台可以根据这些文档快速完成对接工作。
