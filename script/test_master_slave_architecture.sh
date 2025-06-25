#!/bin/bash

# 主从设备架构测试脚本
# 测试主机04A228CD和从设备04A26CF3的连接架构

echo "=== 主从设备架构测试 ==="
echo "测试时间: $(date)"
echo ""

# 服务器地址
SERVER_HOST="localhost"
SERVER_PORT="8080"
BASE_URL="http://${SERVER_HOST}:${SERVER_PORT}"

# 设备信息
PRIMARY_DEVICE="04A228CD"    # 主设备
SECONDARY_DEVICE="04A26CF3"  # 从设备

echo "主设备: ${PRIMARY_DEVICE}"
echo "从设备: ${SECONDARY_DEVICE}"
echo ""

# 测试函数
test_device_info() {
    local device_id=$1
    local device_type=$2
    
    echo "--- 测试${device_type}设备信息查询 ---"
    echo "设备ID: ${device_id}"
    
    response=$(curl -s -X GET "${BASE_URL}/api/v1/device/${device_id}/info")
    echo "响应: ${response}"
    
    # 解析响应
    if echo "${response}" | grep -q '"code":0'; then
        echo "✅ ${device_type}设备在线"
        
        # 检查是否为主设备
        if echo "${response}" | grep -q '"isPrimary":true'; then
            echo "✅ 确认为主设备"
        elif echo "${response}" | grep -q '"isPrimary":false'; then
            echo "✅ 确认为从设备"
        else
            echo "⚠️  未找到主从设备标识"
        fi
        
        # 提取ICCID
        iccid=$(echo "${response}" | grep -o '"iccid":"[^"]*"' | cut -d'"' -f4)
        if [ -n "${iccid}" ]; then
            echo "📱 ICCID: ${iccid}"
        fi
        
        # 提取远程地址
        remote_addr=$(echo "${response}" | grep -o '"remoteAddr":"[^"]*"' | cut -d'"' -f4)
        if [ -n "${remote_addr}" ]; then
            echo "🌐 远程地址: ${remote_addr}"
        fi
        
    else
        echo "❌ ${device_type}设备离线或不存在"
    fi
    echo ""
}

# 测试充电命令
test_charging_command() {
    local device_id=$1
    local device_type=$2
    
    echo "--- 测试${device_type}设备充电命令 ---"
    echo "设备ID: ${device_id}"
    
    # 构造充电请求
    charge_request='{
        "deviceId": "'${device_id}'",
        "port": 1,
        "duration": 60,
        "amount": 100
    }'
    
    response=$(curl -s -X POST "${BASE_URL}/api/v1/charging/start" \
        -H "Content-Type: application/json" \
        -d "${charge_request}")
    
    echo "充电请求: ${charge_request}"
    echo "响应: ${response}"
    
    if echo "${response}" | grep -q '"code":0'; then
        echo "✅ ${device_type}设备充电命令发送成功"
    else
        echo "❌ ${device_type}设备充电命令发送失败"
    fi
    echo ""
}

# 测试设备状态查询
test_device_status() {
    echo "--- 测试所有设备状态 ---"
    
    response=$(curl -s -X GET "${BASE_URL}/api/v1/devices/status")
    echo "响应: ${response}"
    
    if echo "${response}" | grep -q '"code":0'; then
        echo "✅ 设备状态查询成功"
        
        # 统计在线设备数量
        device_count=$(echo "${response}" | grep -o '"deviceId":"[^"]*"' | wc -l)
        echo "📊 在线设备数量: ${device_count}"
        
        # 检查主从设备是否都在线
        if echo "${response}" | grep -q "${PRIMARY_DEVICE}"; then
            echo "✅ 主设备${PRIMARY_DEVICE}在线"
        else
            echo "❌ 主设备${PRIMARY_DEVICE}离线"
        fi
        
        if echo "${response}" | grep -q "${SECONDARY_DEVICE}"; then
            echo "✅ 从设备${SECONDARY_DEVICE}在线"
        else
            echo "❌ 从设备${SECONDARY_DEVICE}离线"
        fi
    else
        echo "❌ 设备状态查询失败"
    fi
    echo ""
}

# 等待服务器启动
echo "等待服务器启动..."
sleep 3

# 执行测试
echo "开始测试主从设备架构..."
echo ""

# 1. 测试主设备
test_device_info "${PRIMARY_DEVICE}" "主"

# 2. 测试从设备
test_device_info "${SECONDARY_DEVICE}" "从"

# 3. 测试设备状态查询
test_device_status

# 4. 测试充电命令
test_charging_command "${PRIMARY_DEVICE}" "主"
test_charging_command "${SECONDARY_DEVICE}" "从"

echo "=== 测试完成 ==="
echo "测试结果总结:"
echo "1. 主设备和从设备应该都能正常查询到信息"
echo "2. 主设备的isPrimary应该为true，从设备应该为false"
echo "3. 两个设备应该共享同一个ICCID和远程地址"
echo "4. 充电命令应该能正常发送到两个设备"
echo ""
echo "如果以上测试都通过，说明主从设备架构工作正常！"
