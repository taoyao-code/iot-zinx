#!/bin/bash

# 多设备共享连接架构测试脚本
# 测试设备04A228CD和04A26CF3的共享连接架构

echo "=== 多设备共享连接架构测试 ==="
echo "测试时间: $(date)"
echo ""

# 服务器地址
SERVER_HOST="localhost"
SERVER_PORT="8080"
BASE_URL="http://${SERVER_HOST}:${SERVER_PORT}"

# 设备信息
DEVICE_1="04A228CD"    # 设备1
DEVICE_2="04A26CF3"    # 设备2

echo "设备1: ${DEVICE_1}"
echo "设备2: ${DEVICE_2}"
echo ""

# 测试函数
test_device_info() {
    local device_id=$1
    local device_name=$2

    echo "--- 测试${device_name}信息查询 ---"
    echo "设备ID: ${device_id}"

    response=$(curl -s -X GET "${BASE_URL}/api/v1/device/${device_id}/info")
    echo "响应: ${response}"

    # 解析响应
    if echo "${response}" | grep -q '"code":0'; then
        echo "✅ ${device_name}在线"
        
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
        echo "❌ ${device_name}离线或不存在"
    fi
    echo ""
}

# 测试充电命令
test_charging_command() {
    local device_id=$1
    local device_name=$2

    echo "--- 测试${device_name}充电命令 ---"
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
        echo "✅ ${device_name}充电命令发送成功"
    else
        echo "❌ ${device_name}充电命令发送失败"
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
        
        # 检查两个设备是否都在线
        if echo "${response}" | grep -q "${DEVICE_1}"; then
            echo "✅ 设备${DEVICE_1}在线"
        else
            echo "❌ 设备${DEVICE_1}离线"
        fi

        if echo "${response}" | grep -q "${DEVICE_2}"; then
            echo "✅ 设备${DEVICE_2}在线"
        else
            echo "❌ 设备${DEVICE_2}离线"
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
echo "开始测试多设备共享连接架构..."
echo ""

# 1. 测试设备1
test_device_info "${DEVICE_1}" "设备1"

# 2. 测试设备2
test_device_info "${DEVICE_2}" "设备2"

# 3. 测试设备状态查询
test_device_status

# 4. 测试充电命令
test_charging_command "${DEVICE_1}" "设备1"
test_charging_command "${DEVICE_2}" "设备2"

echo "=== 测试完成 ==="
echo "测试结果总结:"
echo "1. 两个设备应该都能正常查询到信息"
echo "2. 两个设备应该共享同一个ICCID和远程地址"
echo "3. 充电命令应该能正常发送到两个设备"
echo "4. 两个设备在设备组中平等管理，无主从关系"
echo ""
echo "如果以上测试都通过，说明多设备共享连接架构工作正常！"
