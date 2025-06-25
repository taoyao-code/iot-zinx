#!/bin/bash

# 充电功能专项测试脚本
# 修复日期: 2025-06-25

echo "🔋 充电功能专项测试开始..."
echo "=========================================="

# 基于网络包分析，两个设备都存在
DEVICE1="04A228CD"
DEVICE2="04A26CF3"
API_BASE="http://localhost:7055/api/v1"

echo ""
echo "📋 测试设备连接状态..."
# 查询设备状态
echo "1. 查询设备 $DEVICE1 状态:"
curl -s -X POST $API_BASE/device/status \
  -H "Content-Type: application/json" \
  -d "{\"deviceId\":\"$DEVICE1\"}" | jq '.' || echo "状态查询失败"

echo ""
echo "2. 查询设备 $DEVICE2 状态:"
curl -s -X POST $API_BASE/device/status \
  -H "Content-Type: application/json" \
  -d "{\"deviceId\":\"$DEVICE2\"}" | jq '.' || echo "状态查询失败"

echo ""
echo "=========================================="
echo "🔌 开始充电控制测试..."

# 测试设备1的充电功能
echo ""
echo "3. 设备 $DEVICE1 - 开始充电 (5分钟):"
ORDER1="CHARGE_TEST_$(date +%s)_001"
curl -s -X POST $API_BASE/charging/start \
  -H "Content-Type: application/json" \
  -d "{
    \"deviceId\":\"$DEVICE1\",
    \"port\":1,
    \"mode\":0,
    \"value\":5,
    \"orderNo\":\"$ORDER1\",
    \"balance\":1000
  }" | jq '.' || echo "充电启动失败"

echo ""
echo "等待3秒观察设备响应..."
sleep 3

echo ""
echo "4. 设备 $DEVICE1 - 停止充电:"
curl -s -X POST $API_BASE/charging/stop \
  -H "Content-Type: application/json" \
  -d "{
    \"deviceId\":\"$DEVICE1\",
    \"port\":1,
    \"orderNo\":\"$ORDER1\"
  }" | jq '.' || echo "充电停止失败"

echo ""
echo "=========================================="

# 测试设备2的充电功能
echo ""
echo "5. 设备 $DEVICE2 - 开始充电 (3分钟):"
ORDER2="CHARGE_TEST_$(date +%s)_002"
curl -s -X POST $API_BASE/charging/start \
  -H "Content-Type: application/json" \
  -d "{
    \"deviceId\":\"$DEVICE2\",
    \"port\":1,
    \"mode\":0,
    \"value\":3,
    \"orderNo\":\"$ORDER2\",
    \"balance\":1000
  }" | jq '.' || echo "充电启动失败"

echo ""
echo "等待3秒观察设备响应..."
sleep 3

echo ""
echo "6. 设备 $DEVICE2 - 停止充电:"
curl -s -X POST $API_BASE/charging/stop \
  -H "Content-Type: application/json" \
  -d "{
    \"deviceId\":\"$DEVICE2\",
    \"port\":1,
    \"orderNo\":\"$ORDER2\"
  }" | jq '.' || echo "充电停止失败"