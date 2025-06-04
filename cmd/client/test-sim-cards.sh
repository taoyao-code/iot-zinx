#!/bin/bash

# SIM卡模拟器启动脚本

# 编译客户端程序
echo "编译客户端程序..."
cd /Users/zhanghai/Documents/dockerLNMP/dnmp/www/bujia-frame/iot-zinx
go build -o bin/multi_client cmd/client/*.go

# 确认编译成功
if [ $? -ne 0 ]; then
    echo "编译失败，请检查代码"
    exit 1
fi

echo "编译成功，开始启动多SIM卡，多设备模拟..."
echo ""

# 启动多SIM卡，共享模式
echo "--- 启动共享SIM卡模式 ---"
bin/multi_client -mode sim -sim-mode shared -sim-count 2 -dev-per-sim 3 -server localhost:7054
