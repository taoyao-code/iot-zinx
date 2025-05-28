# 充电设备网关 (Charging Gateway)

这是一个基于Zinx框架的充电设备网关服务，用于处理充电设备与业务平台之间的通信。

## 功能特性

- 支持DNY协议解析与封装
- 设备连接管理
- ICCID和"link"心跳处理
- 设备注册与心跳处理
- 设备状态管理
- 业务平台API交互
- 主机-分机模式支持
- 结构化日志

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

## 配置说明

`configs/gateway.yaml` 包含以下配置项:

- `tcpServer`: TCP服务器配置
- `httpApiServer`: HTTP API服务器配置
- `redis`: Redis连接配置
- `logger`: 日志配置
- `businessPlatform`: 业务平台API配置
- `timeouts`: 超时配置

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