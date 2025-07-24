# Go Pkg
GO_PKG_MOD=github.com/bujia-iot/iot-zinx
# Go 项目各组件名称
GATEWAY_NAME=iot_gateway
DEVICE_SIMULATOR_NAME=device-simulator
NOTIFICATION_STATS_NAME=notification-stats
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
DEVICE_SIMULATOR_DIR=./cmd/device-simulator
NOTIFICATION_STATS_DIR=./cmd/notification-stats
# 输出目录
OUTPUT_DIR=./bin

.PHONY: all build clean test help swagger build-all build-gateway build-device-simulator build-notification-stats run-gateway run-device-simulator run-notification-stats fmt lint cover tidy

all: build-all

# 构建所有组件
build-all: build-gateway build-device-simulator build-notification-stats

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

# 构建设备模拟器组件
build-device-simulator: $(OUTPUT_DIR)
	@echo "==> Building Device Simulator..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building Device Simulator for $(TARGET_PLATFORM)"; \
		GOOS_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f2); \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(DEVICE_SIMULATOR_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(DEVICE_SIMULATOR_DIR); \
	else \
		env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(DEVICE_SIMULATOR_NAME) -ldflags="-s -w" -trimpath $(DEVICE_SIMULATOR_DIR); \
	fi
	@echo "==> Device Simulator build complete."

# 构建通知统计组件
build-notification-stats: $(OUTPUT_DIR)
	@echo "==> Building Notification Stats..."
	@if [ -n "$(TARGET_PLATFORM)" ]; then \
		echo "  Building Notification Stats for $(TARGET_PLATFORM)"; \
		GOOS_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $(TARGET_PLATFORM) | cut -d'/' -f2); \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(NOTIFICATION_STATS_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(NOTIFICATION_STATS_DIR); \
	else \
		env CGO_ENABLED=0 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) $(GOBUILD) -o $(OUTPUT_DIR)/$(NOTIFICATION_STATS_NAME) -ldflags="-s -w" -trimpath $(NOTIFICATION_STATS_DIR); \
	fi
	@echo "==> Notification Stats build complete."

# 构建所有组件的多平台版本
build-multi-platform: $(OUTPUT_DIR)
	@echo "==> Building all components for multiple platforms..."
	@for PLATFORM in $(PLATFORMS); do \
		GOOS_VAL=$$(echo $$PLATFORM | cut -d'/' -f1); \
		GOARCH_VAL=$$(echo $$PLATFORM | cut -d'/' -f2); \
		echo "  Building for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(GATEWAY_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(GATEWAY_DIR) || echo "❌ Gateway build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(DEVICE_SIMULATOR_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(DEVICE_SIMULATOR_DIR) || echo "❌ Device Simulator build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
		env CGO_ENABLED=0 GOOS=$$GOOS_VAL GOARCH=$$GOARCH_VAL $(GOBUILD) -o $(OUTPUT_DIR)/$(NOTIFICATION_STATS_NAME)_$$GOOS_VAL_$$GOARCH_VAL -ldflags="-s -w" -trimpath $(NOTIFICATION_STATS_DIR) || echo "❌ Notification Stats build failed for $$GOOS_VAL/$$GOARCH_VAL"; \
	done; \
	echo "==> Multi-platform build complete."

# 运行网关组件
run-gateway:
	@echo "==> Running Gateway..."
	@$(GO) run $(GATEWAY_DIR)

# 运行设备模拟器组件
run-device-simulator:
	@echo "==> Running Device Simulator..."
	@$(GO) run $(DEVICE_SIMULATOR_DIR)

# 运行通知统计组件
run-notification-stats:
	@echo "==> Running Notification Stats..."
	@$(GO) run $(NOTIFICATION_STATS_DIR)

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
	@echo "IoT-Zinx Gateway Project - Build System"
	@echo "========================================"
	@echo ""
	@echo "Targets:"
	@echo "  all                      Builds all components (default)"
	@echo "  build-all                Builds all components (gateway, device-simulator, notification-stats)"
	@echo "  build-gateway            Builds only the gateway component"
	@echo "  build-device-simulator   Builds only the device simulator component"
	@echo "  build-notification-stats Builds only the notification stats component"
	@echo "  build-multi-platform     Builds all components for multiple platforms"
	@echo "                           Default platforms: $(PLATFORMS)"
	@echo "  run-gateway              Runs the gateway component"
	@echo "  run-device-simulator     Runs the device simulator component"
	@echo "  run-notification-stats   Runs the notification stats component"
	@echo "  clean                    Cleans build artifacts"
	@echo "  test                     Runs tests"
	@echo "  tidy                     Tidies go.mod file"
	@echo "  fmt                      Formats Go code"
	@echo "  lint                     Runs code linter (requires golangci-lint)"
	@echo "  cover                    Generates test coverage report"
	@echo "  swagger                  Generates Swagger API documentation"
	@echo "  help                     Shows this help message"
	@echo ""
	@echo "Options:"
	@echo "  TARGET_PLATFORM          Build for a specific platform (format: os/arch)"
	@echo "                           Example: make build-gateway TARGET_PLATFORM=linux/arm64"
	@echo ""
	@echo "Current Settings:"
	@echo "  Module:                  $(GO_PKG_MOD)"
	@echo "  Default OS:              $(DEFAULT_GOOS)"
	@echo "  Default Arch:            $(DEFAULT_GOARCH)"
	@echo "  Output Directory:        $(OUTPUT_DIR)"
	@echo ""
	@echo "Components:"
	@echo "  Gateway:                 IoT charging device gateway server"
	@echo "  Device Simulator:        Test device simulator for development"
	@echo "  Notification Stats:      Notification system statistics tool"
	@echo ""

# 确保输出目录存在
$(OUTPUT_DIR):
	mkdir -p $(OUTPUT_DIR)

# 格式化代码
fmt:
	@echo "==> Formatting code..."
	@go fmt ./...

# 运行代码检查
lint:
	@echo "==> Running linter..."
	@golangci-lint run

# 生成测试覆盖率报告
cover:
	@echo "==> Generating test coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "==> Coverage report generated: coverage.html"
