
# 按时间充电（充电30分钟）：
curl -X POST http://localhost:7055/api/v1/charging/start \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A26CF3",
    "port": 1,
    "mode": 0,
    "value": 30,
    "orderNo": "ORDER_'$(date +%s)'_001",
    "balance": 1000
  }'

  #按电量充电（充电5度电，即50个0.1度单位）
  curl -X POST http://localhost:7055/api/v1/charging/start \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A26CF3",
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

  # 停止充电指令
  curl -X POST http://localhost:7055/api/v1/charging/stop \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A26CF3",
    "port": 1,
    "orderNo": "ORDER_XXXXX"
  }'

  # 发送设备定位命令
curl -X POST http://localhost:7055/api/v1/device/locate \
  -H "Content-Type: application/json" \
  -d '{
    "deviceId": "04A26CF3",
    "locateTime": 10
  }'