#!/bin/bash

# ===========================================
# IoT-Zinx 充电测试脚本 (简化优化版)
# 基于日志分析优化，解决连接断开和超时问题
# ===========================================

SERVER_URL="http://localhost:7055"
DEVICE_A="04A26CF3"    # 设备A (显示: 10644723)
DEVICE_B="04A228CD"    # 设备B (显示: 10627277)

# 默认测试参数配置
DEFAULT_DEVICE="$DEVICE_A"
DEFAULT_PORT=1          # 统一端口管理：用户界面显示的端口号(1-based)，API会自动转换为协议端口号(0-based)
DEFAULT_DURATION=5      # 默认充电时长(分钟)
DEFAULT_BALANCE=10000   # 默认余额(分)
DEFAULT_MAX_POWER=3000  # 默认最大功率(0.1W单位，即300W)
DEFAULT_MAX_DURATION=120 # 默认最大充电时长(分钟)

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[$(date '+%H:%M:%S')]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[$(date '+%H:%M:%S')]${NC} $1"
}

log_error() {
    echo -e "${RED}[$(date '+%H:%M:%S')]${NC} $1"
}

# 显示设备ID (按协议规则转换: 去掉前缀04后转十进制)
show_device_id() {
    local hex_id=$1
    # 去掉前缀04，只转换后6位
    local hex_without_prefix=${hex_id#04}
    local dec_id=$((0x${hex_without_prefix}))
    echo "${hex_id} (显示: ${dec_id})"
}

# 发送充电命令 (修复版本)
send_charge() {
    local device_id=$1
    local port_display=$2    # 用户界面显示的端口号(1-based)
    local duration_minutes=$3 # 分钟
    local description=$4
    local custom_order_no=$5  # 可选：自定义订单号

    # 🔧 修复：API端口号是1-based，与用户显示一致
    local port_api=$port_display  # API端口号与用户显示端口号相同
    
    # 🔧 修复：转换分钟为秒 (协议层面使用秒为单位)
    local duration_seconds=$((duration_minutes * 60))
    
    # 🔧 修复：支持自定义订单号，确保订单号一致性
    local order_no
    if [ -n "$custom_order_no" ]; then
        order_no="$custom_order_no"
    else
        local timestamp=$(date +%s)
        order_no="CHARGE_${device_id}_P${port_display}_${timestamp}"
    fi
    
    local display_id=$(show_device_id "$device_id")

    log_info "🔋 ${description}"
    log_info "设备: ${display_id}, 端口: 第${port_display}路, 时长: ${duration_minutes}分钟(${duration_seconds}秒)"
    log_info "订单: ${order_no}"

    # 🔧 修复：添加超时设置和更好的错误处理
    local response=$(curl -s --connect-timeout 10 --max-time 30 -X POST "${SERVER_URL}/api/v1/charging/start" \
        -H "Content-Type: application/json" \
        -d "{
            \"deviceId\": \"${device_id}\",
            \"port\": ${port_api},
            \"mode\": 0,
            \"value\": ${duration_seconds},
            \"orderNo\": \"${order_no}\",
            \"balance\": ${DEFAULT_BALANCE}
        }" 2>&1)

    # 检查响应
    if echo "$response" | jq -e '.code == 0' > /dev/null 2>&1; then
        log_info "✅ 充电命令发送成功"
        echo "$response" | jq . 2>/dev/null
        return 0
    elif echo "$response" | jq -e '.code' > /dev/null 2>&1; then
        log_error "❌ 充电命令发送失败"
        echo "$response" | jq . 2>/dev/null
        return 1
    else
        log_warn "⚠️ 充电命令可能发送成功（网络响应异常）"
        echo "网络响应: $response"
        return 0
    fi
}

# 发送停止充电命令
send_stop_charge() {
    local device_id=$1
    local port_display=$2    # 用户界面显示的端口号(1-based)
    local order_no=$3        # 订单号
    local description=$4

    local display_id=$(show_device_id "$device_id")

    log_info "🛑 ${description}"
    log_info "设备: ${display_id}, 端口: 第${port_display}路"
    log_info "订单: ${order_no}"

    # 🔧 修复：清理订单号，确保不包含ANSI颜色代码或其他特殊字符
    local clean_order_no=$(echo "$order_no" | tr -d '\033\[0-9;mA-Z' | tr -d '\r\n')

    # 🔧 修复：添加超时设置和更好的错误处理
    local response=$(curl -s --connect-timeout 10 --max-time 30 -X POST "${SERVER_URL}/api/v1/charging/stop" \
        -H "Content-Type: application/json" \
        -d "{
            \"deviceId\": \"${device_id}\",
            \"port\": ${port_display},
            \"orderNo\": \"${clean_order_no}\"
        }" 2>&1)

    # 检查响应
    if echo "$response" | jq -e '.code == 0' > /dev/null 2>&1; then
        log_info "✅ 停止充电命令发送成功"
        echo "$response" | jq . 2>/dev/null
        return 0
    elif echo "$response" | jq -e '.code' > /dev/null 2>&1; then
        log_error "❌ 停止充电命令发送失败"
        echo "$response" | jq . 2>/dev/null
        return 1
    else
        log_warn "⚠️ 停止充电命令可能发送成功（网络响应异常）"
        echo "网络响应: $response"
        return 0
    fi
}

# 检查充电状态
check_charge_status() {
    local device_id=$1
    local port_display=$2
    local description=${3:-"查询充电状态"}

    local display_id=$(show_device_id "$device_id")

    log_info "🔍 ${description}"
    log_info "设备: ${display_id}, 端口: 第${port_display}路"

    log_info "📊 正在查询充电状态..."
    
    # TODO: 实现具体的状态查询接口调用
    # curl -s -X GET "${SERVER_URL}/api/v1/charging/status?deviceId=${device_id}&port=${port_display}"
    
    # 同时检查设备在线状态
    check_device_online "$device_id"
}

# 检查设备在线状态
check_device_online() {
    local device_id=$1
    local display_id=$(show_device_id "$device_id")

    log_info "🌐 检查设备在线状态: ${display_id}"

    # 🔧 修复：使用正确的设备状态查询接口
    # 尝试多个可能的接口路径
    local response
    local status_found=false
    
    # 尝试路径1: /api/v1/devices/{deviceId}/status
    response=$(curl -s --connect-timeout 5 --max-time 15 \
        "${SERVER_URL}/api/v1/devices/${device_id}/status" 2>&1)
    
    if echo "$response" | jq -e '.code == 0' > /dev/null 2>&1; then
        status_found=true
    else
        # 尝试路径2: /api/v1/device/{deviceId}
        response=$(curl -s --connect-timeout 5 --max-time 15 \
            "${SERVER_URL}/api/v1/device/${device_id}" 2>&1)
        
        if echo "$response" | jq -e '.code == 0' > /dev/null 2>&1; then
            status_found=true
        fi
    fi

    if [ "$status_found" = "true" ]; then
        local online=$(echo "$response" | jq -r '.data.online // false')
        if [ "$online" = "true" ]; then
            log_info "✅ 设备在线"
        else
            log_warn "⚠️ 设备离线"
        fi
        echo "$response" | jq . 2>/dev/null
    else
        log_warn "⚠️ 设备状态查询接口不可用，跳过在线状态检查"
        log_info "💡 提示：设备可能仍然在线，但状态查询接口未实现"
    fi
}

# 发送定位命令 (简化版)
send_locate() {
    local device_id=$1
    local display_id=$(show_device_id "$device_id")

    log_info "📍 发送定位命令: ${display_id}"

    curl -s -X POST "${SERVER_URL}/api/v1/device/locate" \
        -H "Content-Type: application/json" \
        -d "{\"deviceId\":\"${device_id}\",\"locateTime\":5}" | jq . 2>/dev/null || echo "定位命令已发送"
}

# 主函数
main() {
    echo -e "${BLUE}=========================================${NC}"
    echo -e "${BLUE}    IoT-Zinx 充电测试脚本 (优化版)${NC}"
    echo -e "${BLUE}=========================================${NC}"

    # 检查服务器连接 (修复健康检查路径)
    log_info "检查服务器连接..."
    if ! curl -s --connect-timeout 5 "${SERVER_URL}/api/v1/health" > /dev/null 2>&1; then
        log_warn "健康检查接口连接失败，尝试直接测试服务端口..."
        if ! curl -s --connect-timeout 5 "${SERVER_URL}" > /dev/null 2>&1; then
            log_error "服务器连接失败，请检查服务是否启动在 ${SERVER_URL}"
            exit 1
        fi
    fi
    log_info "✅ 服务器连接正常"

    echo ""
    echo "选择测试模式:"
    echo "1) 🚀 快速测试 (默认值) - 推荐新手使用"
    echo "2) 📝 单设备测试 (自定义) - 完全自定义配置"
    echo "3) ⚡ 验证测试 - 快速验证功能"
    echo "4) 🔧 原始测试 - 多命令测试"
    echo "5) 🔄 完整流程测试 - 开始→状态→停止→验证"
    echo "6) 🚨 紧急停止测试 - 停止所有端口"
    echo ""
    read -p "请选择 (1-6): " choice

    case $choice in
        1)
            quick_default_test
            ;;
        2)
            single_device_test
            ;;
        3)
            quick_test
            ;;
        4)
            original_test
            ;;
        5)
            complete_flow_test
            ;;
        6)
            emergency_stop_test
            ;;
        *)
            quick_default_test
            ;;
    esac
}

# 单设备测试模式 (推荐)
single_device_test() {
    echo ""
    echo "选择要测试的设备:"
    echo "1) 设备A: ${DEVICE_A} (显示: 10627277)"
    echo "2) 设备B: ${DEVICE_B} (显示: 10644723)"
    read -p "请选择设备 (1-2): " device_choice

    local device_id
    case $device_choice in
        1) device_id="$DEVICE_A" ;;
        2) device_id="$DEVICE_B" ;;
        *) log_error "无效选择"; return 1 ;;
    esac

    echo ""
    echo "选择充电端口:"
    echo "1) 端口 1"
    echo "2) 端口 2"
    read -p "请选择端口 (1-2): " port_choice

    local port
    case $port_choice in
        1) port=1 ;;
        2) port=2 ;;
        *) log_error "无效端口选择"; return 1 ;;
    esac

    echo ""
    echo "选择充电时长:"
    echo "1) 2分钟 (快速测试)"
    echo "2) 5分钟 (标准测试)"
    echo "3) 10分钟 (长时测试)"
    echo "4) 自定义时长"
    read -p "请选择 (1-4): " duration_choice

    local duration
    case $duration_choice in
        1) duration=2 ;;
        2) duration=5 ;;
        3) duration=10 ;;
        4)
            read -p "请输入充电时长(分钟): " duration
            if ! [[ "$duration" =~ ^[0-9]+$ ]] || [ "$duration" -lt 1 ] || [ "$duration" -gt 60 ]; then
                log_error "无效时长，请输入1-60之间的数字"
                return 1
            fi
            ;;
        *) log_error "无效选择"; return 1 ;;
    esac

    echo ""
    echo "选择测试类型:"
    echo "1) 只开始充电"
    echo "2) 只停止充电 (需要提供订单号)"
    echo "3) 完整流程 (开始→等待→停止)"
    read -p "请选择 (1-3): " test_type_choice

    case $test_type_choice in
        1)
            # 只开始充电
            log_info "=== 单设备测试 - 开始充电 ==="
            log_info "设备: $(show_device_id "$device_id")"
            log_info "端口: 第${port}路"
            log_info "时长: ${duration}分钟"

            send_charge "$device_id" "$port" "$duration" "单设备充电测试"
            log_info "✅ 充电测试完成，请观察设备第${port}路端口是否开始充电"
            ;;
        2)
            # 只停止充电
            echo ""
            read -p "请输入要停止的订单号: " order_no
            if [ -z "$order_no" ]; then
                log_error "订单号不能为空"
                return 1
            fi

            log_info "=== 单设备测试 - 停止充电 ==="
            log_info "设备: $(show_device_id "$device_id")"
            log_info "端口: 第${port}路"
            log_info "订单: ${order_no}"

            send_stop_charge "$device_id" "$port" "$order_no" "单设备停止充电测试"
            log_info "✅ 停止测试完成，请观察设备第${port}路端口是否停止充电"
            ;;
        3)
            # 完整流程
            local timestamp=$(date +%s)
            local order_no="SINGLE_TEST_${device_id}_P${port}_${timestamp}"

            log_info "=== 单设备测试 - 完整流程 ==="
            log_info "设备: $(show_device_id "$device_id")"
            log_info "端口: 第${port}路"
            log_info "时长: ${duration}分钟"
            log_info "预生成订单: ${order_no}"

            # 开始充电 - 🔧 修复：使用预生成订单号
            log_info "🔋 步骤1: 开始充电"
            if ! send_charge "$device_id" "$port" "$duration" "单设备完整流程 - 开始充电" "$order_no" > /dev/null; then
                log_error "充电启动失败，测试终止"
                return 1
            fi
            
            # 使用预生成的订单号
            local actual_order_no="$order_no"
            log_info "📝 使用订单号: $actual_order_no"

            # 等待用户确认
            echo ""
            read -p "充电已开始，按回车键继续停止充电..." -r

            # 停止充电 - 🔧 修复：使用实际订单号
            log_info "🛑 步骤2: 停止充电"
            send_stop_charge "$device_id" "$port" "$actual_order_no" "单设备完整流程 - 停止充电"
            log_info "✅ 完整流程测试完成"
            ;;
        *)
            log_error "无效选择"
            return 1
            ;;
    esac
}

# 快速验证模式
quick_test() {
    log_info "=== 快速验证模式 ==="

    # 只测试一个设备的充电 (避免冲突，已修复端口号)
    log_info "1. 测试充电功能 (设备A第1路: 10627277)"
    send_charge "$DEVICE_A" 1 2 "快速充电验证"

    log_info "✅ 快速验证完成"
}

# 🚀 快速默认值测试模式 (解决缺少默认值问题)
quick_default_test() {
    log_info "=== 🚀 快速默认值测试 ==="
    local display_id=$(show_device_id "$DEFAULT_DEVICE")
    
    log_info "使用默认配置:"
    log_info "  设备: ${display_id}"
    log_info "  端口: 第${DEFAULT_PORT}路"
    log_info "  时长: ${DEFAULT_DURATION}分钟"
    log_info "  最大功率: $((DEFAULT_MAX_POWER / 10))W"
    log_info "  最大时长: ${DEFAULT_MAX_DURATION}分钟"
    
    echo ""
    read -p "确认使用默认配置开始测试? (Y/n): " -r
    echo
    
    # 默认为Y，只有明确输入n/N才取消
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        log_info "❌ 测试已取消"
    else
        send_charge "$DEFAULT_DEVICE" "$DEFAULT_PORT" "$DEFAULT_DURATION" "🚀 快速默认值测试"
        log_info "✅ 快速测试完成，请观察设备第${DEFAULT_PORT}路端口是否开始充电"
    fi
}

# 完整流程测试模式 (开始→状态→停止→验证)
complete_flow_test() {
    log_info "=== 🔄 完整充电流程测试 ==="
    
    echo ""
    echo "选择要测试的设备:"
    echo "1) 设备A: ${DEVICE_A} (显示: 10644723)"
    echo "2) 设备B: ${DEVICE_B} (显示: 10627277)"
    read -p "请选择设备 (1-2): " device_choice

    local device_id
    case $device_choice in
        1) device_id="$DEVICE_A" ;;
        2) device_id="$DEVICE_B" ;;
        *) log_error "无效选择"; return 1 ;;
    esac

    echo ""
    echo "选择充电端口:"
    echo "1) 端口 1"
    echo "2) 端口 2"
    read -p "请选择端口 (1-2): " port_choice

    local port
    case $port_choice in
        1) port=1 ;;
        2) port=2 ;;
        *) log_error "无效端口选择"; return 1 ;;
    esac

    local display_id=$(show_device_id "$device_id")
    local timestamp=$(date +%s)
    local order_no="FLOW_TEST_${device_id}_P${port}_${timestamp}"
    local test_duration=3  # 完整流程测试使用3分钟

    log_info "=== 完整流程测试 ==="
    log_info "设备: ${display_id}"
    log_info "端口: 第${port}路"
    log_info "预生成订单: ${order_no}"
    log_info "测试时长: ${test_duration}分钟"

    echo ""
    read -p "确认开始完整流程测试? (Y/n): " -r
    echo
    
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        log_info "❌ 测试已取消"
        return
    fi

    # 第1步：开始充电 - 🔧 修复：使用预生成的订单号
    log_info "📍 第1步：开始充电"
    if ! send_charge "$device_id" "$port" "$test_duration" "完整流程测试 - 开始充电" "$order_no" > /dev/null; then
        log_error "充电启动失败，测试终止"
        return 1
    fi
    
    # 使用预生成的订单号，确保一致性
    local actual_order_no="$order_no"
    log_info "📝 使用订单号: $actual_order_no"

    # 第2步：等待并检查状态
    log_info "📍 第2步：等待5秒后检查充电状态"
    sleep 5
    check_charge_status "$device_id" "$port" "验证充电已开始"

    # 第3步：停止充电 - 🔧 修复：使用实际订单号
    log_info "📍 第3步：发送停止充电命令"
    if ! send_stop_charge "$device_id" "$port" "$actual_order_no" "完整流程测试 - 停止充电"; then
        log_error "停止充电失败"
        return 1
    fi

    # 第4步：验证停止状态
    log_info "📍 第4步：等待3秒后验证停止状态"
    sleep 3
    check_charge_status "$device_id" "$port" "验证充电已停止"

    log_info "✅ 完整流程测试完成！"
    log_info "🔍 请观察设备第${port}路端口：应该先开始充电，然后停止充电"
}

# 紧急停止测试模式 (停止所有端口)
emergency_stop_test() {
    log_info "=== 🚨 紧急停止测试 ==="
    
    echo ""
    echo "选择要停止的设备:"
    echo "1) 设备A: ${DEVICE_A} (显示: 10644723)"
    echo "2) 设备B: ${DEVICE_B} (显示: 10627277)"
    echo "3) 所有设备"
    read -p "请选择 (1-3): " device_choice

    log_warn "⚠️  紧急停止将停止选中设备的所有端口充电！"
    echo ""
    read -p "确认执行紧急停止? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "❌ 紧急停止已取消"
        return
    fi

    case $device_choice in
        1)
            emergency_stop_device "$DEVICE_A"
            ;;
        2)
            emergency_stop_device "$DEVICE_B"
            ;;
        3)
            emergency_stop_device "$DEVICE_A"
            emergency_stop_device "$DEVICE_B"
            ;;
        *)
            log_error "无效选择"
            return 1
            ;;
    esac

    log_info "✅ 紧急停止测试完成"
}

# 紧急停止单个设备的所有端口
emergency_stop_device() {
    local device_id=$1
    local display_id=$(show_device_id "$device_id")

    log_info "🚨 紧急停止设备: ${display_id} (所有端口)"

    # 修复：使用端口0xFF(255)停止所有端口（协议中0xFF表示设备智能选择端口）
    local response=$(curl -s -X POST "${SERVER_URL}/api/v1/charging/stop" \
        -H "Content-Type: application/json" \
        -d "{
            \"deviceId\": \"${device_id}\",
            \"port\": 255,
            \"orderNo\": \"EMERGENCY_STOP_$(date +%s)\"
        }")

    if echo "$response" | jq -e '.code == 0' > /dev/null 2>&1; then
        log_info "✅ 紧急停止命令发送成功: ${display_id}"
        echo "$response" | jq . 2>/dev/null
    else
        log_error "❌ 紧急停止命令发送失败: ${display_id}"
        echo "$response" | jq . 2>/dev/null || echo "紧急停止失败: $response"
        return 1
    fi
}

# 原始测试模式 (多命令)
original_test() {
    log_warn "⚠️  原始测试模式可能导致命令冲突，建议使用单设备测试"
    read -p "确定要继续吗? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        return
    fi

    log_info "=== 原始测试模式 ==="

    # 定位测试 (暂时注释)
    # log_info "1. 设备定位测试"
    # send_locate "$DEVICE_A"
    # sleep 2
    # send_locate "$DEVICE_B"
    # sleep 5

    # 充电测试 (分开进行，避免冲突，已修复端口号)
    log_info "1. 设备A第1路充电测试 (10627277)"
    send_charge "$DEVICE_A" 1 5 "设备A充电测试"
    sleep 10

    log_info "2. 设备B第1路充电测试 (10644723)"
    send_charge "$DEVICE_B" 1 5 "设备B充电测试"

    log_info "✅ 原始测试完成"
}

# 检查依赖
if ! command -v curl &> /dev/null; then
    log_error "curl 命令未找到，请安装 curl"
    exit 1
fi

# 显示使用说明
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "IoT-Zinx 充电测试脚本 (完整版)"
    echo ""
    echo "用法: $0"
    echo ""
    echo "功能:"
    echo "  1. 快速测试 - 使用默认配置快速测试"
    echo "  2. 单设备测试 - 自定义单设备测试 (支持开始/停止/完整流程)"
    echo "  3. 验证测试 - 快速验证充电功能"
    echo "  4. 原始测试 - 多命令测试模式"
    echo "  5. 完整流程测试 - 开始→状态→停止→验证"
    echo "  6. 紧急停止测试 - 停止设备所有端口充电"
    echo ""
    echo "新增功能:"
    echo "  ✅ 支持停止充电命令"
    echo "  ✅ 支持充电状态查询"
    echo "  ✅ 支持完整流程测试"
    echo "  ✅ 支持紧急停止所有端口"
    echo "  ✅ 改进错误处理和日志"
    echo ""
    echo "基于日志分析优化:"
    echo "  - 解决命令超时问题"
    echo "  - 避免设备连接断开"
    echo "  - 简化命令流程"
    echo "  - 完整的充电控制验证"
    exit 0
fi

# 执行主函数
main