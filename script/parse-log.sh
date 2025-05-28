#!/bin/bash

# 解析DNY协议日志数据的脚本
# 用法: ./script/parse-log.sh <日志文件路径>

# 检查参数
if [ $# -lt 1 ]; then
    echo "用法: $0 <日志文件路径>"
    exit 1
fi

LOG_FILE=$1

# 检查文件是否存在
if [ ! -f "$LOG_FILE" ]; then
    echo "错误: 文件 '$LOG_FILE' 不存在"
    exit 1
fi

# 检查DNY解析器是否已编译
if [ ! -f "./bin/dny-parser" ]; then
    echo "DNY解析器未找到，正在编译..."
    make dny-parser
fi

# 提取并解析日志中的十六进制数据
echo "从日志 '$LOG_FILE' 中提取DNY协议数据..."
echo ""

# 查找日志中的read buffer行
grep -i "read buffer" "$LOG_FILE" | while read -r line; do
    # 提取十六进制数据
    hex_data=$(echo "$line" | grep -o '[0-9a-fA-F]*$')
    
    # 检查是否以DNY开头
    if [[ $hex_data == 444e59* ]]; then
        echo "原始日志行: $line"
        echo "提取的十六进制数据: $hex_data"
        echo "解析结果:"
        ./bin/dny-parser -hex "$hex_data"
        echo "------------------------------------------"
    fi
done

echo "解析完成"
exit 0 