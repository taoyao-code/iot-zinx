#!/bin/bash

# API文档生成和更新脚本
# 用于自动生成和更新Swagger API文档

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCS_DIR="${PROJECT_ROOT}/docs"
MAIN_FILE="${PROJECT_ROOT}/cmd/gateway/main.go"

echo -e "${BLUE}🚀 IoT-Zinx API文档生成工具${NC}"
echo -e "${BLUE}================================${NC}"

# 检查swag工具是否安装
check_swag() {
    if ! command -v swag &> /dev/null; then
        echo -e "${RED}❌ swag工具未安装${NC}"
        echo -e "${YELLOW}正在安装swag工具...${NC}"
        go install github.com/swaggo/swag/cmd/swag@latest
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}✅ swag工具安装成功${NC}"
        else
            echo -e "${RED}❌ swag工具安装失败${NC}"
            exit 1
        fi
    else
        echo -e "${GREEN}✅ swag工具已安装${NC}"
    fi
}

# 检查项目结构
check_project_structure() {
    echo -e "${BLUE}🔍 检查项目结构...${NC}"
    
    if [ ! -f "${MAIN_FILE}" ]; then
        echo -e "${RED}❌ 主程序文件不存在: ${MAIN_FILE}${NC}"
        exit 1
    fi
    
    if [ ! -d "${PROJECT_ROOT}/internal/apis" ]; then
        echo -e "${RED}❌ API目录不存在: ${PROJECT_ROOT}/internal/apis${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✅ 项目结构检查通过${NC}"
}

# 清理旧文档
clean_old_docs() {
    echo -e "${BLUE}🧹 清理旧文档...${NC}"
    
    if [ -f "${DOCS_DIR}/docs.go" ]; then
        rm -f "${DOCS_DIR}/docs.go"
        echo -e "${GREEN}✅ 删除旧的docs.go${NC}"
    fi
    
    if [ -f "${DOCS_DIR}/swagger.json" ]; then
        rm -f "${DOCS_DIR}/swagger.json"
        echo -e "${GREEN}✅ 删除旧的swagger.json${NC}"
    fi
    
    if [ -f "${DOCS_DIR}/swagger.yaml" ]; then
        rm -f "${DOCS_DIR}/swagger.yaml"
        echo -e "${GREEN}✅ 删除旧的swagger.yaml${NC}"
    fi
}

# 生成Swagger文档
generate_docs() {
    echo -e "${BLUE}📖 生成Swagger文档...${NC}"
    
    cd "${PROJECT_ROOT}"
    
    # 生成文档
    swag init -g cmd/gateway/main.go -o docs --parseDependency --parseInternal
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ Swagger文档生成成功${NC}"
    else
        echo -e "${RED}❌ Swagger文档生成失败${NC}"
        exit 1
    fi
}

# 验证生成的文档
validate_docs() {
    echo -e "${BLUE}🔍 验证生成的文档...${NC}"
    
    required_files=("docs.go" "swagger.json" "swagger.yaml")
    
    for file in "${required_files[@]}"; do
        if [ -f "${DOCS_DIR}/${file}" ]; then
            echo -e "${GREEN}✅ ${file} 生成成功${NC}"
        else
            echo -e "${RED}❌ ${file} 生成失败${NC}"
            exit 1
        fi
    done
    
    # 检查swagger.json是否有效
    if command -v jq &> /dev/null; then
        if jq empty "${DOCS_DIR}/swagger.json" 2>/dev/null; then
            echo -e "${GREEN}✅ swagger.json 格式有效${NC}"
        else
            echo -e "${RED}❌ swagger.json 格式无效${NC}"
            exit 1
        fi
    fi
}

# 显示文档信息
show_docs_info() {
    echo -e "${BLUE}📊 文档信息${NC}"
    echo -e "${BLUE}============${NC}"
    
    if [ -f "${DOCS_DIR}/swagger.json" ]; then
        if command -v jq &> /dev/null; then
            title=$(jq -r '.info.title' "${DOCS_DIR}/swagger.json")
            version=$(jq -r '.info.version' "${DOCS_DIR}/swagger.json")
            description=$(jq -r '.info.description' "${DOCS_DIR}/swagger.json")
            
            echo -e "${GREEN}📖 标题: ${title}${NC}"
            echo -e "${GREEN}🏷️  版本: ${version}${NC}"
            echo -e "${GREEN}📝 描述: ${description}${NC}"
            
            # 统计API端点数量
            paths_count=$(jq '.paths | length' "${DOCS_DIR}/swagger.json")
            echo -e "${GREEN}🔗 API端点数量: ${paths_count}${NC}"
        fi
    fi
    
    echo -e "${BLUE}============${NC}"
    echo -e "${GREEN}📁 生成的文件:${NC}"
    ls -la "${DOCS_DIR}"/{docs.go,swagger.json,swagger.yaml} 2>/dev/null || true
}

# 显示访问信息
show_access_info() {
    echo -e "${BLUE}🌐 访问信息${NC}"
    echo -e "${BLUE}============${NC}"
    echo -e "${GREEN}📖 Swagger UI: http://localhost:7055/swagger/index.html${NC}"
    echo -e "${GREEN}📄 JSON文档: http://localhost:7055/swagger/doc.json${NC}"
    echo -e "${GREEN}📄 YAML文档: ${DOCS_DIR}/swagger.yaml${NC}"
    echo -e "${BLUE}============${NC}"
    echo -e "${YELLOW}💡 提示: 启动服务器后访问上述地址查看API文档${NC}"
}

# 主函数
main() {
    echo -e "${BLUE}开始生成API文档...${NC}"
    
    check_swag
    check_project_structure
    clean_old_docs
    generate_docs
    validate_docs
    show_docs_info
    show_access_info
    
    echo -e "${GREEN}🎉 API文档生成完成！${NC}"
}

# 处理命令行参数
case "${1:-}" in
    "clean")
        echo -e "${BLUE}🧹 仅清理旧文档${NC}"
        clean_old_docs
        echo -e "${GREEN}✅ 清理完成${NC}"
        ;;
    "validate")
        echo -e "${BLUE}🔍 仅验证文档${NC}"
        validate_docs
        echo -e "${GREEN}✅ 验证完成${NC}"
        ;;
    "info")
        echo -e "${BLUE}📊 显示文档信息${NC}"
        show_docs_info
        show_access_info
        ;;
    *)
        main
        ;;
esac
