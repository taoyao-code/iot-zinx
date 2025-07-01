#!/bin/bash

# IoT-Zinx 代码质量检查工具
# 用于定期扫描重复代码、废弃组件和代码质量问题

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 配置
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/reports"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
REPORT_FILE="$REPORT_DIR/code_quality_report_$TIMESTAMP.md"

# 创建报告目录
mkdir -p "$REPORT_DIR"

echo -e "${BLUE}🔍 IoT-Zinx 代码质量检查工具${NC}"
echo -e "${BLUE}===========================================${NC}"
echo "项目路径: $PROJECT_ROOT"
echo "报告文件: $REPORT_FILE"
echo ""

# 初始化报告文件
cat > "$REPORT_FILE" << EOF
# IoT-Zinx 代码质量检查报告

**生成时间**: $(date)  
**项目路径**: $PROJECT_ROOT  

## 📋 检查概述

EOF

# 检查函数
check_duplicate_code() {
    echo -e "${YELLOW}🔍 检查重复代码...${NC}"
    
    echo "## 🔄 重复代码检查" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # 检查重复的函数名
    echo "### 重复函数名检查" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    duplicate_functions=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | \
        xargs grep -h "^func " | \
        sed 's/func \([^(]*\).*/\1/' | \
        sort | uniq -d)
    
    if [ -n "$duplicate_functions" ]; then
        echo "⚠️ 发现重复函数名:" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$duplicate_functions" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo -e "${RED}❌ 发现重复函数名${NC}"
    else
        echo "✅ 未发现重复函数名" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 未发现重复函数名${NC}"
    fi
    echo "" >> "$REPORT_FILE"
    
    # 检查重复的结构体定义
    echo "### 重复结构体检查" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    duplicate_structs=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | \
        xargs grep -h "^type .* struct" | \
        sed 's/type \([^ ]*\) struct.*/\1/' | \
        sort | uniq -d)
    
    if [ -n "$duplicate_structs" ]; then
        echo "⚠️ 发现重复结构体:" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$duplicate_structs" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo -e "${RED}❌ 发现重复结构体${NC}"
    else
        echo "✅ 未发现重复结构体" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 未发现重复结构体${NC}"
    fi
    echo "" >> "$REPORT_FILE"
}

check_deprecated_code() {
    echo -e "${YELLOW}🔍 检查废弃代码...${NC}"
    
    echo "## 🗑️ 废弃代码检查" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # 检查 DEPRECATED 标记
    deprecated_items=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | \
        xargs grep -n "DEPRECATED\|废弃" | head -20)
    
    if [ -n "$deprecated_items" ]; then
        echo "⚠️ 发现废弃代码标记:" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$deprecated_items" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo -e "${YELLOW}⚠️ 发现废弃代码标记${NC}"
    else
        echo "✅ 未发现废弃代码标记" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 未发现废弃代码标记${NC}"
    fi
    echo "" >> "$REPORT_FILE"
    
    # 检查 TODO 和 FIXME
    todo_items=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | \
        xargs grep -n "TODO\|FIXME" | wc -l)
    
    echo "### TODO/FIXME 统计" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "发现 $todo_items 个 TODO/FIXME 项目" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    if [ "$todo_items" -gt 50 ]; then
        echo -e "${RED}❌ TODO/FIXME 项目过多 ($todo_items)${NC}"
    elif [ "$todo_items" -gt 20 ]; then
        echo -e "${YELLOW}⚠️ TODO/FIXME 项目较多 ($todo_items)${NC}"
    else
        echo -e "${GREEN}✅ TODO/FIXME 项目数量合理 ($todo_items)${NC}"
    fi
}

check_unused_files() {
    echo -e "${YELLOW}🔍 检查未使用文件...${NC}"
    
    echo "## 📁 未使用文件检查" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # 检查空目录
    empty_dirs=$(find "$PROJECT_ROOT" -type d -empty -not -path "*/.git/*" -not -path "*/vendor/*")
    
    if [ -n "$empty_dirs" ]; then
        echo "⚠️ 发现空目录:" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$empty_dirs" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo -e "${YELLOW}⚠️ 发现空目录${NC}"
    else
        echo "✅ 未发现空目录" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 未发现空目录${NC}"
    fi
    echo "" >> "$REPORT_FILE"
    
    # 检查可能未使用的测试文件
    orphan_tests=$(find "$PROJECT_ROOT" -name "*_test.go" -not -path "*/vendor/*" | while read test_file; do
        base_file="${test_file%_test.go}.go"
        if [ ! -f "$base_file" ]; then
            echo "$test_file"
        fi
    done)
    
    if [ -n "$orphan_tests" ]; then
        echo "⚠️ 发现可能孤立的测试文件:" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$orphan_tests" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo -e "${YELLOW}⚠️ 发现可能孤立的测试文件${NC}"
    else
        echo "✅ 未发现孤立的测试文件" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 未发现孤立的测试文件${NC}"
    fi
    echo "" >> "$REPORT_FILE"
}

check_code_metrics() {
    echo -e "${YELLOW}🔍 检查代码指标...${NC}"
    
    echo "## 📊 代码指标统计" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # 统计代码行数
    total_lines=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | xargs wc -l | tail -1 | awk '{print $1}')
    total_files=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | wc -l)
    
    echo "| 指标 | 数值 |" >> "$REPORT_FILE"
    echo "|------|------|" >> "$REPORT_FILE"
    echo "| Go 文件总数 | $total_files |" >> "$REPORT_FILE"
    echo "| 代码总行数 | $total_lines |" >> "$REPORT_FILE"
    echo "| 平均每文件行数 | $((total_lines / total_files)) |" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo -e "${BLUE}📊 代码指标: $total_files 个文件, $total_lines 行代码${NC}"
    
    # 检查大文件
    large_files=$(find "$PROJECT_ROOT" -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | \
        xargs wc -l | awk '$1 > 500 {print $2 " (" $1 " lines)"}' | head -10)
    
    if [ -n "$large_files" ]; then
        echo "### 大文件 (>500行)" >> "$REPORT_FILE"
        echo "" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo "$large_files" >> "$REPORT_FILE"
        echo '```' >> "$REPORT_FILE"
        echo -e "${YELLOW}⚠️ 发现大文件${NC}"
    else
        echo "✅ 未发现过大的文件" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 文件大小合理${NC}"
    fi
    echo "" >> "$REPORT_FILE"
}

check_import_cycles() {
    echo -e "${YELLOW}🔍 检查循环导入...${NC}"
    
    echo "## 🔄 循环导入检查" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    # 使用 go mod 检查循环导入
    cd "$PROJECT_ROOT"
    if go list -f '{{.ImportPath}}: {{.Imports}}' ./... 2>/dev/null | grep -q "cycle"; then
        echo "❌ 发现循环导入" >> "$REPORT_FILE"
        echo -e "${RED}❌ 发现循环导入${NC}"
    else
        echo "✅ 未发现循环导入" >> "$REPORT_FILE"
        echo -e "${GREEN}✅ 未发现循环导入${NC}"
    fi
    echo "" >> "$REPORT_FILE"
}

generate_recommendations() {
    echo "## 🎯 改进建议" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "### 代码质量维护建议" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "1. **定期运行此检查工具**：建议每周运行一次代码质量检查" >> "$REPORT_FILE"
    echo "2. **及时清理废弃代码**：发现 DEPRECATED 标记的代码应及时清理" >> "$REPORT_FILE"
    echo "3. **控制文件大小**：单个文件不应超过 500 行，考虑拆分大文件" >> "$REPORT_FILE"
    echo "4. **减少 TODO 项目**：定期处理 TODO 和 FIXME 项目" >> "$REPORT_FILE"
    echo "5. **避免重复代码**：发现重复代码应及时重构" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "### 自动化建议" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    echo "- 将此脚本集成到 CI/CD 流程中" >> "$REPORT_FILE"
    echo "- 设置代码质量阈值，超过阈值时自动告警" >> "$REPORT_FILE"
    echo "- 定期生成代码质量趋势报告" >> "$REPORT_FILE"
    echo "" >> "$REPORT_FILE"
    
    echo "---" >> "$REPORT_FILE"
    echo "**报告生成时间**: $(date)" >> "$REPORT_FILE"
    echo "**检查工具版本**: v2.0.0" >> "$REPORT_FILE"
}

# 主执行流程
main() {
    cd "$PROJECT_ROOT"
    
    # 执行各项检查
    check_duplicate_code
    check_deprecated_code
    check_unused_files
    check_code_metrics
    check_import_cycles
    generate_recommendations
    
    echo ""
    echo -e "${GREEN}✅ 代码质量检查完成${NC}"
    echo -e "${BLUE}📄 报告已生成: $REPORT_FILE${NC}"
    echo ""
    
    # 显示报告摘要
    echo -e "${BLUE}📋 检查摘要:${NC}"
    echo "- 重复代码检查: 完成"
    echo "- 废弃代码检查: 完成"
    echo "- 未使用文件检查: 完成"
    echo "- 代码指标统计: 完成"
    echo "- 循环导入检查: 完成"
    echo ""
    
    # 提供查看报告的建议
    echo -e "${YELLOW}💡 建议:${NC}"
    echo "1. 查看完整报告: cat $REPORT_FILE"
    echo "2. 定期运行此脚本: ./script/code_quality_check.sh"
    echo "3. 将脚本加入 crontab 实现自动检查"
}

# 执行主函数
main "$@"
