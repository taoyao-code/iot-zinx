#!/bin/bash

# IoT-Zinx 架构简化：旧代码清理脚本
# 严格按照文档要求删除所有旧代码

set -e

echo "========================================="
echo "IoT-Zinx 旧代码清理开始"
echo "========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 清理函数
cleanup_dir() {
    if [ -d "$1" ]; then
        rm -rf "$1"
        echo -e "${GREEN}✅ 删除目录: $1${NC}"
    else
        echo -e "${YELLOW}⚠️  目录不存在: $1${NC}"
    fi
}

cleanup_file() {
    if [ -f "$1" ]; then
        rm -f "$1"
        echo -e "${GREEN}✅ 删除文件: $1${NC}"
    else
        echo -e "${YELLOW}⚠️  文件不存在: $1${NC}"
    fi
}

# 阶段1：基础架构清理
echo ""
echo "🗑️ 阶段1：基础架构清理"
echo "---------------------"

cleanup_dir "pkg/databus"
cleanup_dir "pkg/session"
cleanup_dir "pkg/monitor"

# 阶段2：TCP层清理
echo ""
echo "🗑️ 阶段2：TCP层清理"
echo "-------------------"

cleanup_dir "internal/infrastructure/zinx_server/handlers"
cleanup_file "pkg/network/unified_network_manager.go"
cleanup_file "pkg/network/unified_sender.go"
cleanup_file "pkg/network/monitoring_manager.go"
cleanup_file "pkg/network/global_response_manager.go"

# 阶段3：HTTP层清理
echo ""
echo "🗑️ 阶段3：HTTP层清理"
echo "-------------------"

cleanup_dir "internal/app/service"
cleanup_dir "internal/adapter"
cleanup_dir "internal/domain"
cleanup_dir "pkg/databus"

# 阶段4：基础设施清理
echo ""
echo "🗑️ 阶段4：基础设施清理"
echo "---------------------"

cleanup_dir "internal/infrastructure/config"
cleanup_dir "internal/infrastructure/logger"
cleanup_file "configs/zinx.json"

# 阶段5：网络层彻底清理
echo ""
echo "🗑️ 阶段5：网络层彻底清理"
echo "-----------------------"

cleanup_file "pkg/network/command_manager.go"
cleanup_file "pkg/network/command_queue.go"
cleanup_file "pkg/network/response_waiter.go"
cleanup_file "pkg/network/response_handler.go"
cleanup_file "pkg/network/raw_data_handler.go"
cleanup_file "pkg/network/connection_health_checker.go"
cleanup_file "pkg/network/connection_hooks.go"

# 清理其他复杂组件
cleanup_file "internal/app/global_databus.go"
cleanup_file "internal/app/service_manager.go"
cleanup_file "internal/ports/global_integrator.go"
cleanup_file "internal/ports/heartbeat_manager.go"

# 清理旧的主程序
cleanup_file "cmd/gateway/main.go"

# 清理依赖
echo ""
echo "🧹 清理依赖..."
echo "-------------"
go mod tidy
echo -e "${GREEN}✅ 依赖清理完成${NC}"

# 验证清理结果
echo ""
echo "🔍 验证清理结果..."
echo "------------------"

# 检查是否还有旧文件
OLD_FILES_FOUND=0

# 检查目录
CHECK_DIRS=("pkg/databus" "pkg/session" "pkg/monitor" "internal/app/service" "internal/adapter" "internal/domain" "internal/infrastructure/zinx_server/handlers" "internal/infrastructure/logger" "internal/infrastructure/config")

for dir in "${CHECK_DIRS[@]}"; do
    if [ -d "$dir" ]; then
        echo -e "${RED}❌ 目录仍存在: $dir${NC}"
        OLD_FILES_FOUND=1
    fi
done

# 检查文件
CHECK_FILES=("pkg/network/unified_network_manager.go" "pkg/network/unified_sender.go" "pkg/network/command_manager.go" "pkg/network/response_waiter.go" "configs/zinx.json" "cmd/gateway/main.go")

for file in "${CHECK_FILES[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${RED}❌ 文件仍存在: $file${NC}"
        OLD_FILES_FOUND=1
    fi
done

if [ $OLD_FILES_FOUND -eq 0 ]; then
    echo -e "${GREEN}🎉 所有旧代码清理完成！${NC}"
else
    echo -e "${RED}❌ 仍有旧代码未清理${NC}"
    exit 1
fi

# 创建清理完成标记
touch .cleanup_complete
echo -e "${GREEN}✅ 清理完成标记已创建${NC}"

echo ""
echo "========================================="
echo "旧代码清理完成！"
echo "========================================="
echo "新架构已就绪，包含："
echo "  • 3层极简架构"
echo "  • 统一GlobalDeviceStore"
echo "  • 直接Handler模式"
echo "  • 零冗余设计"
echo ""
echo "下一步：运行 ./scripts/final_verification.sh 进行最终验证"