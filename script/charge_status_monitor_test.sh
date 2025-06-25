#!/bin/bash

# 充电状态监控测试脚本
# 验证功率心跳包是否正确解析充电状态

echo "🔋 充电状态监控测试脚本"
echo "=========================================="
echo "目标：验证功率心跳包中的充电状态解析"
echo ""

# 测试设备
DEVICE1="04A228CD"
DEVICE2="04A26CF3"
API_BASE="http://localhost:7055/api/v1"

echo "📋 开始充电状态监控测试..."

# 发送充电命令
echo ""
echo "1. 设备 $DEVICE1 - 开始充电测试:"
ORDER1="CHARGE_STATUS_TEST_$(date +%s)"

curl -s -X POST $API_BASE/charging/start \
  -H "Content-Type: application/json" \
  -d "{
    \"deviceId\":\"$DEVICE1\",
    \"port\":1,
    \"mode\":0,
    \"value\":2,
    \"orderNo\":\"$ORDER1\",
    \"balance\":1000
  }" | jq '.' || echo "充电启动失败"

echo ""
echo "⏳ 等待15秒，观察功率心跳包中的充电状态变化..."
echo "预期结果：应该看到 '⚡ 设备充电状态：正在充电' 的日志"
sleep 15

echo ""
echo "2. 停止充电:"
curl -s -X POST $API_BASE/charging/stop \
  -H "Content-Type: application/json" \
  -d "{
    \"deviceId\":\"$DEVICE1\",
    \"port\":1,
    \"orderNo\":\"$ORDER1\"
  }" | jq '.' || echo "充电停止失败"

echo ""
echo "⏳ 等待15秒，观察充电停止后的状态变化..."
echo "预期结果：应该看到 '🔌 设备充电状态：未充电' 的日志"
sleep 15

echo ""
echo "=========================================="
echo "✅ 充电状态监控测试完成"
echo ""
echo "📝 验证要点："
echo "1. 查看日志中是否出现 '⚡ 设备充电状态：正在充电'"
echo "2. 查看日志中是否出现 '🚨 充电状态监控：设备正在充电'"
echo "3. 充电停止后是否出现 '🔌 设备充电状态：未充电'"
echo "4. 确认充电状态、充电时长、累计电量等字段被正确解析"
