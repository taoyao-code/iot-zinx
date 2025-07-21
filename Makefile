# Go Pkg
GO_PKG_MOD=iot-platform
# Go 项目各组件名称
GATEWAY_NAME=iot_gateway
CLIENT_NAME=client
SERVER_API_NAME=server-api
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
DEFAULT_GOOS=$(shell go env GOOS)
DEFAULT_GOARCH=$(shell go env GOARCH)

# 可选的编译平台，格式为 GOOS/GOARCH
PLATFORMS ?= "linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"
# 如果只想编译特定平台，可以设置 TARGET_PLATFORM 变量，例如：
# make build TARGET_PLATFORM=linux/amd64
TARGET_PLATFORM ?=

# 程序入口文件
GATEWAY_DIR=./cmd/gateway
CLIENT_DIR=./cmd/client
SERVER_API_DIR=./cmd/server-api
DNY_PARSER_DIR=./cmd/dny-parser
# 输出目录
OUTPUT_DIR=./bin

.PHONY: all build clean test help swagger build-all build-gateway build-client build-server-api build-dny-parser run-gateway run-client run-server-api run-dny-parser

all: build-all

# 构建所有组件
build-all: build-gateway build-client build-server-api build-dny-parser

# 构建网关组件
build-gateway: $(OUTPUT_DIR)
	@echo "==> Building Gateway..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building Gateway for $(TARGET_PLATFORM)"; \
		GOOS_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f2); \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(GATEWAY_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(GATEWAY_DIR); \
	else \
		env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(GATEWAY_NAME) -ldflags="-s -w" -trimpath $(GATEWAY_DIR); \
	fi
	@echo "==> Gateway build complete."

# 构建客户端组件
build-client: $(OUTPUT_DIR)
	@echo "==> Building Client..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building Client for $(TARGET_PLATFORM)"; \
		GOOS_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f2); \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(CLIENT_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(CLIENT_DIR); \
	else \
		env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(CLIENT_NAME) -ldflags="-s -w" -trimpath $(CLIENT_DIR); \
	fi
	@echo "==> Client build complete."

# 构建服务端API组件
build-server-api: $(OUTPUT_DIR)
	@echo "==> Building Server API..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building Server API for $(TARGET_PLATFORM)"; \
		GOOS_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f2); \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(SERVER_API_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(SERVER_API_DIR); \
	else \
		env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(SERVER_API_NAME) -ldflags="-s -w" -trimpath $(SERVER_API_DIR); \
	fi
	@echo "==> Server API build complete."

# 构建DNY解析器工具
build-dny-parser: $(OUTPUT_DIR)
	@echo "==> Building DNY parser tool..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building DNY parser for $(TARGET_PLATFORM)"; \
		GOOS_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f2); \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(DNY_PARSER_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(DNY_PARSER_DIR); \
	else \
		env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(DNY_PARSER_NAME) -ldflags="-s -w" -trimpath $(DNY_PARSER_DIR); \
	fi
	@echo "==> DNY parser tool built."

# 构建所有组件的多平台版本
build-multi-platform: $(OUTPUT_DIR)
	@echo "==> Building all components for multiple platforms..."
	@for PLATFORM in $(PLATFORMS); do \
		GOOS_VAL=$$(echo $$PLATFORM | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $$PLATFORM | cut -d'/' -f2); \
		echo "  Building for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(GATEWAY_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(GATEWAY_DIR) || echo "❌ Gateway build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(CLIENT_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(CLIENT_DIR) || echo "❌ Client build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(SERVER_API_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(SERVER_API_DIR) || echo "❌ Server-API build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(DNY_PARSER_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(DNY_PARSER_DIR) || echo "❌ DNY-Parser build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
	done; \
	echo "==> Multi-platform build complete."

# 运行网关组件
run-gateway:
	@echo "==> Running Gateway..."
	@$(GO) run $(GATEWAY_DIR)

# 运行客户端组件
run-client:
	@echo "==> Running Client..."
	@$(GO) run $(CLIENT_DIR)

# 运行服务端API组件
run-server-api:
	@echo "==> Running Server API..."
	@$(GO) run $(SERVER_API_DIR)

# 运行DNY解析器工具
run-dny-parser:
	@echo "==> Running DNY parser tool..."
	@$(GO) run $(DNY_PARSER_DIR)

# 为兼容旧版本，保留原有构建命令
build: build-gateway

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
	@$(SWAG) init -g cmd/gateway/main.go
	@echo "==> Swagger documentation generated in docs/ directory"

# 显示帮助信息
help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                Builds all components (default)"
	@echo "  build-all          Builds all components (gateway, client, server-api, dny-parser)"
	@echo "  build-gateway      Builds only the gateway component"
	@echo "  build-client       Builds only the client component"
	@echo "  build-server-api   Builds only the server-api component"
	@echo "  build-dny-parser   Builds only the dny-parser component"
	@echo "  build-multi-platform  Builds all components for multiple platforms"
	@echo "                     Default platforms: $(PLATFORMS)"
	@echo "  run-gateway        Runs the gateway component"
	@echo "  run-client         Runs the client component"
	@echo "  run-server-api     Runs the server-api component"
	@echo "  run-dny-parser     Runs the dny-parser component"
	@echo "  clean              Cleans build artifacts"
	@echo "  test               Runs tests"
	@echo "  tidy               Tidies go.mod file"
	@echo "  swagger            Generates Swagger API documentation"
	@echo "  help               Shows this help message"
	@echo ""
	@echo "Options:"
	@echo "  TARGET_PLATFORM    Build for a specific platform (format: os/arch)"
	@echo "                     Example: make build-gateway TARGET_PLATFORM=linux/arm64"
	@echo ""
	@echo "Current Settings:"
	@echo "  Default OS:        $(DEFAULT_GOOS)"
	@echo "  Default Arch:      $(DEFAULT_GOARCH)"
	@echo "  Output Directory:  $(OUTPUT_DIR)"
	@echo ""

# 确保输出目录存在
$(OUTPUT_DIR):
	mkdir -p $(OUTPUT_DIR)

# 格式化代码
fmt:
	@echo "==> Formatting code..."
	@go fmt ./...


# 运行测试
test:
	@go test -v ./...

# 运行代码检查
lint:
	@golangci-lint run

# 生成测试覆盖率报告
cover:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
