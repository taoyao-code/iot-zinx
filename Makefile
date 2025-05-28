# Go Pkg
GO_PKG_MOD=iot-platform
# Go 项目的二进制文件名称
BINARY_NAME=iot_gateway
# DNY解析器工具名称
DNY_PARSER_NAME=dny-parser
# Go 编译器
GO=go
# Go 构建命令
GOBUILD=$(GO) build
# Go 清理命令
GOCLEAN=$(GO) clean
# Go 测试命令
GOTEST=$(GO) test
# Go 获取依赖命令
GOGET=$(GO) get
# Go list 命令
GOLIST=$(GO) list
# Go mod tidy 命令
GOMODTIDY=$(GO) mod tidy
# Swagger 命令
SWAG=swag

# 默认编译的操作系统和架构
DEFAULT_GOOS=linux
DEFAULT_GOARCH=amd64

# 可选的编译平台，格式为 GOOS/GOARCH
PLATFORMS ?= "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"
# 如果只想编译特定平台，可以设置 TARGET_PLATFORM 变量，例如：
# make build TARGET_PLATFORM=linux/amd64
TARGET_PLATFORM ?=

# 主程序入口
MAIN_GO_FILE=./cmd/server/main.go
# DNY解析器入口
DNY_PARSER_GO_FILE=./cmd/dny-parser/main.go
# 输出目录
OUTPUT_DIR=./bin

.PHONY: all build clean test help swagger dny-parser

all: build

# 构建项目
# 如果 TARGET_PLATFORM 被设置，则只编译该平台
# 否则，编译 PLATFORMS 中定义的所有平台
build:
	@echo "==> Building..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building for $(TARGET_PLATFORM)"; \
		env CGO_ENABLED=0 $(GOBUILD) -o $(OUTPUT_DIR)/$(BINARY_NAME)_$(subst /,_,$(TARGET_PLATFORM)) -ldflags="-s -w" -trimpath $(MAIN_GO_FILE); \
	else \
		for PLATFORM in $(PLATFORMS); do \
		GOOS_VAL=$$(echo $$PLATFORM | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $$PLATFORM | cut -d'/' -f2); \
		echo "  Building for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(BINARY_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(MAIN_GO_FILE); \
		done; \
	fi
	@echo "==> Build complete."

# 编译默认平台 (linux/amd64)
build-default:
	@echo "==> Building for default platform ($(DEFAULT_GOOS)/$(DEFAULT_GOARCH))..."
	@env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(BINARY_NAME)_$(DEFAULT_GOOS)_$(DEFAULT_GOARCH) -ldflags="-s -w" -trimpath $(MAIN_GO_FILE)
	@echo "==> Build complete."

# 构建DNY解析器工具
dny-parser: $(OUTPUT_DIR)
	@echo "==> Building DNY parser tool..."
	@$(GOBUILD) -o $(OUTPUT_DIR)/$(DNY_PARSER_NAME) $(DNY_PARSER_GO_FILE)
	@echo "==> DNY parser tool built: $(OUTPUT_DIR)/$(DNY_PARSER_NAME)"

# 清理构建产物
clean:
	@echo "==> Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(OUTPUT_DIR)
	@echo "==> Clean complete."

# 运行测试
test:
	@echo "==> Running tests..."
	@$(GOTEST) -v ./...
	@echo "==> Tests complete."

# 整理 go.mod 文件
tidy:
	@echo "==> Tidying go.mod..."
	@$(GOMODTIDY)
	@echo "==> Tidy complete."

# 生成 Swagger 文档
swagger:
	@echo "==> Generating Swagger documentation..."
	@$(SWAG) init -g cmd/server/main.go -o internal/docs
	@echo "==> Swagger documentation generated."

# 显示帮助信息
help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                Builds the application for specified platforms (default)"
	@echo "  build              Builds the application. Set TARGET_PLATFORM=os/arch to build for a single platform."
	@echo "                     Example: make build TARGET_PLATFORM=linux/arm64"
	@echo "                     Default platforms: $(PLATFORMS)"
	@echo "  build-default      Builds the application for the default platform ($(DEFAULT_GOOS)/$(DEFAULT_GOARCH))"
	@echo "  dny-parser         Builds the DNY protocol parser tool"
	@echo "  clean              Cleans build artifacts"
	@echo "  test               Runs tests"
	@echo "  tidy               Tidies go.mod file"
	@echo "  swagger            Generates Swagger API documentation"
	@echo "  help               Shows this help message"
	@echo ""

# 确保输出目录存在
$(OUTPUT_DIR):
	mkdir -p $(OUTPUT_DIR)

# 将 build 依赖于输出目录的创建
build: $(OUTPUT_DIR)
build-default: $(OUTPUT_DIR)