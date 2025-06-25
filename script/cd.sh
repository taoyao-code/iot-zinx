# 按时间充电（充电30分钟）- 修复：使用正确的设备ID：
curl -X POST http://localhost:7055/api/v1/charging/start \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A228CD",
    "port": 1,
    "mode": 0,
    "value": 30,
    "orderNo": "ORDER_'$(date +%s)'_001",
    "balance": 1000
  }'

  #按电量充电（充电5度电，即50个0.1度单位）- 修复：使用正确的设备ID
  curl -X POST http://localhost:7055/api/v1/charging/start \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A228CD",
    "port": 1,
    "mode": 1,
    "value": 50,
    "orderNo": "ORDER_'$(date +%s)'_002",
    "balance": 1000
  }'

  # 为设备04A228CD下发开始充电指令

  curl -X POST http://localhost:7055/api/v1/charging/start \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A228CD",
    "port": 1,
    "mode": 0,
    "value": 15,
    "orderNo": "ORDER_'$(date +%s)'_003",
    "balance": 500
  }'

  # 停止充电指令 - 修复：使用正确的设备ID
  curl -X POST http://localhost:7055/api/v1/charging/stop \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A228CD",
    "port": 1,
    "orderNo": "ORDER_XXXXX"
  }'

  # 发送设备定位命令 - 修复：使用正确的设备ID
curl -X POST http://localhost:7055/api/v1/device/locate \
     -H "Content-Type: application/json" \
     -d '{"deviceId":"04A228CD","locateTime":10}'

# ===========================================
# 设备连接验证测试
# ===========================================

echo "=== 测试设备 04A228CD 的定位功能 ==="
curl -X POST http://localhost:7055/api/v1/device/locate \
     -H "Content-Type: application/json" \
     -d '{"deviceId":"04A228CD","locateTime":10}'

echo ""
echo "=== 测试设备 04A26CF3 的定位功能 ==="
curl -X POST http://localhost:7055/api/v1/device/locate \
     -H "Content-Type: application/json" \
     -d '{"deviceId":"04A26CF3","locateTime":10}'

echo ""
echo "=== 测试设备 04A228CD 的充电功能 ==="
curl -X POST http://localhost:7055/api/v1/charging/start \
     -H "Content-Type: application/json" \
     -d '{"deviceId":"04A228CD","port":1,"mode":0,"value":5,"orderNo":"TEST_04A228CD_001","balance":1000}'

echo ""
echo "=== 测试设备 04A26CF3 的充电功能 ==="
curl -X POST http://localhost:7055/api/v1/charging/start \
     -H "Content-Type: application/json" \
     -d '{"deviceId":"04A26CF3","port":1,"mode":0,"value":5,"orderNo":"TEST_04A26CF3_001","balance":1000}'

# ===========================================
# 原有测试命令
# ===========================================