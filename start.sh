#!/bin/bash

# IoT-Zinx 简化架构启动脚本
# 基于3层架构：Handler → GlobalStore → API

set -e

echo "========================================="
echo "IoT-Zinx 简化架构启动"
echo "========================================="

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "错误: Go未安装或未在PATH中"
    exit 1
fi

echo "Go版本: $(go version)"

# 创建必要的目录
mkdir -p logs
mkdir -p configs

# 下载依赖
echo "下载依赖..."
go mod tidy

# 构建项目
echo "构建项目..."
go build -o bin/iot-zinx cmd/main.go

# 启动服务
echo "启动服务..."
./bin/iot-zinx &

# 等待服务启动
sleep 2

# 检查服务状态
if pgrep -f "iot-zinx" > /dev/null; then
    echo "========================================="
    echo "服务启动成功！"
    echo "TCP端口: 8999"
    echo "HTTP端口: 8080"
    echo "日志目录: ./logs/"
    echo "========================================="
    echo "查看日志: tail -f logs/iot-zinx.log"
    echo "停止服务: pkill -f iot-zinx"
else
    echo "错误: 服务启动失败"
    exit 1
fi