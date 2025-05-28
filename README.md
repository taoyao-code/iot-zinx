# 充电设备网关

基于 [Zinx](https://github.com/aceld/zinx) 框架实现的充电设备通信网关，用于处理充电设备的TCP连接和协议解析。

## 功能特性

- 支持 DNY 通信协议的解析和处理
- 设备注册和心跳管理
- 连接生命周期管理，包括ICCID和"link"心跳处理
- 设备ID与连接的双向映射，便于消息转发
- 可配置的日志和服务器参数
- 主机-分机模式支持
- 结构化日志

## 快速开始

### 编译

```bash
go build -o bin/gateway ./cmd/gateway/main.go
```

### 运行

```bash
./bin/gateway -config configs/gateway.yaml
```

## 配置说明

配置文件路径: `configs/gateway.yaml`

主要配置项:
- `tcpServer`: TCP服务器配置（端口、最大连接数等）
- `httpApiServer`: HTTP API服务器配置
- `redis`: Redis连接配置
- `logger`: 日志配置（级别、输出路径等）
- `businessPlatform`: 业务平台API配置
- `timeouts`: 超时配置（心跳间隔、初始化超时等）

详细配置请参考配置文件中的注释。

## 项目结构

```
iot-zinx/
├── bin/                   # 编译输出目录
├── cmd/                   # 入口命令
│   └── gateway/           # 网关入口
├── configs/               # 配置文件
├── internal/              # 内部实现
│   ├── adapter/           # 适配器层
│   ├── app/               # 应用服务层
│   ├── domain/            # 领域层
│   │   └── dny_protocol/  # DNY协议定义
│   ├── infrastructure/    # 基础设施层
│   │   ├── config/        # 配置管理
│   │   ├── logger/        # 日志管理
│   │   └── zinx_server/   # Zinx服务器实现
│   │       └── handlers/  # 协议处理器
│   └── port/              # 端口层
├── logs/                  # 日志输出目录
└── pkg/                   # 公共包
    ├── errors/            # 错误处理
    ├── utils/             # 工具函数
    └── validation/        # 验证工具
```

## 开发指南

### 添加新的命令处理器

1. 在 `internal/infrastructure/zinx_server/handlers/` 目录下创建处理器实现
2. 在 `internal/infrastructure/zinx_server/handlers/router.go` 中注册处理器

### 修改协议解析

协议解析器实现在 `internal/infrastructure/zinx_server/datapacker.go` 文件中。

## API接口

HTTP API接口将在后续版本中实现，用于与业务平台交互。

## 更新记录

详细更新记录请参考 [CHANGELOG.md](./CHANGELOG.md)。

## 目录结构

```
charging_gateway/
├── api/               # API定义文件
├── cmd/               # 应用程序入口
├── configs/           # 配置文件
├── deployments/       # 部署相关文件
├── internal/          # 内部包
│   ├── adapter/       # 适配器 (Redis, 业务平台客户端等)
│   ├── app/           # 应用服务层
│   ├── domain/        # 领域模型
│   ├── infrastructure/# 基础设施
│   └── port/          # 接口层
├── pkg/               # 公共包
└── test/              # 测试文件
```

## 环境要求

- Go 1.18+
- Redis
- Linux/macOS/Windows

## 安装与运行

1. 克隆代码仓库
```bash
git clone https://github.com/bujia-iot/iot-zinx.git
cd iot-zinx
```

2. 编译应用
```bash
go build -o bin/gateway cmd/gateway/main.go
```

3. 修改配置文件 `configs/gateway.yaml`

4. 运行应用
```bash
./bin/gateway
```

或者指定配置文件路径:
```bash
./bin/gateway -config /configs/gateway.yaml
```

## API接口

### 设备API (TCP)

设备通过TCP长连接使用DNY二进制协议与网关通信。

### 业务平台API (HTTP)

业务平台通过HTTP API与网关交互，主要包括:

- `POST /gateway/api/v1/devices/:deviceId/commands`: 下发指令给设备

详细API文档请参考 `api/openapi/gateway_api.yaml`。

## 贡献

欢迎提交问题或功能建议到Issue系统，或直接提交Pull Request。

## 许可证

[版权所有] 布甲物联网 