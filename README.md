# 充电设备网关系统 (IOT-Zinx)

基于Zinx网络框架的充电设备网关系统，实现与充电桩设备的通信和管理。

## 项目介绍

本系统是一个基于TCP协议的充电设备网关，负责连接和管理充电桩设备，处理设备上报的各种数据，并将业务请求转发给设备。系统采用六边形架构（端口与适配器架构），实现了业务逻辑与技术实现的分离。

### 主要功能

- 设备连接管理：处理设备上线、注册和离线
- 心跳管理：主机心跳、分机心跳等各类心跳包处理
- 刷卡消费：处理设备刷卡请求
- 充电控制：向设备发送充电启停命令
- 设备状态监控：监控设备心跳状态，自动清理超时连接

### 技术栈

- Go语言开发
- Zinx网络框架
- 六边形架构（端口与适配器架构）
- DNY协议（设备通信协议）

## 目录结构

```
iot-zinx/
├── bin/                  # 编译后的可执行文件
│   └── gateway           # 网关服务可执行文件
├── cmd/                  # 命令行入口
│   └── gateway/          # 网关服务入口
│       └── main.go       # 主程序入口
├── conf/                 # 默认配置目录
│   └── zinx.json         # Zinx框架默认配置
├── configs/              # 应用配置文件目录
│   └── gateway.yaml      # 网关配置文件
├── deployments/          # 部署相关文件
├── dosc/                 # 项目文档
│   ├── 进度/             # 进度文档
│   ├── 1.设计方案.md      # 设计方案文档
│   ├── AP3000-设备与服务器通信协议.md
│   ├── 对接硬件.md
│   └── 主机-服务器通信协议.md
├── internal/             # 内部代码，不对外暴露
│   ├── adapter/          # 适配器层，对接外部系统
│   │   └── http/         # HTTP适配器
│   │       └── handlers.go # HTTP请求处理器
│   ├── app/              # 应用层，核心业务逻辑
│   │   ├── dto/          # 数据传输对象
│   │   │   ├── charge_control_dto.go
│   │   │   └── swipe_card_dto.go
│   │   ├── service/      # 业务服务
│   │   │   └── device_service.go
│   │   └── service_manager.go
│   ├── domain/           # 领域层，核心业务模型
│   │   └── dny_protocol/ # DNY协议相关定义
│   │       ├── constants.go
│   │       └── frame.go
│   ├── infrastructure/   # 基础设施层
│   │   ├── config/       # 配置管理
│   │   │   └── config.go
│   │   ├── logger/       # 日志服务
│   │   │   └── logger.go
│   │   ├── redis/        # Redis客户端
│   │   │   └── client.go
│   │   └── zinx_server/  # Zinx服务器实现
│   │       ├── connection_hooks.go
│   │       ├── datapacker.go
│   │       ├── device_monitor.go
│   │       └── handlers/ # 命令处理器
│   │           ├── charge_control_handler.go
│   │           ├── device_register_handler.go
│   │           ├── get_server_time_handler.go
│   │           ├── heartbeat_handler.go
│   │           ├── main_heartbeat_handler.go
│   │           ├── router.go
│   │           └── swipe_card_handler.go
│   └── ports/            # 端口层，定义系统边界
│       ├── http_server.go
│       └── tcp_server.go
├── logs/                 # 日志文件目录
│   └── gateway.log       # 网关日志文件
├── pkg/                  # 可共享的代码包
│   ├── errors/           # 错误处理
│   │   └── errors.go
│   ├── utils/            # 工具函数
│   └── validation/       # 数据验证
├── test/                 # 测试代码
│   ├── mock/             # 测试模拟
│   │   └── mock.go
│   └── unit/             # 单元测试
│       └── device_service_test.go
├── go.mod                # Go模块文件
├── go.sum                # Go依赖校验文件
├── Makefile              # 构建脚本
├── CHANGELOG.md          # 变更日志
└── README.md             # 项目说明文档
```

## 开发指南

### 环境要求

- Go 1.18+
- 支持TCP协议的网络环境

### 构建与运行

1. 克隆项目

```bash
git clone https://github.com/bujia-iot/iot-zinx.git
cd iot-zinx
```

2. 安装依赖

```bash
go mod tidy
```

3. 构建项目

```bash
make build
```

4. 运行网关

```bash
./bin/gateway --config configs/gateway.yaml
```

### 开发流程

1. 领域层开发：在domain目录下定义设备通信协议和业务模型
2. 业务层开发：在app目录下实现业务逻辑
3. 适配器开发：在adapter目录下实现与外部系统的对接
4. 处理器开发：在infrastructure/zinx_server/handlers目录下添加命令处理器

## 项目结构说明

### 命令处理器

系统支持以下命令处理器：

- `DeviceRegisterHandler`：设备注册请求处理器 (0x20)
- `HeartbeatHandler`：标准心跳包处理器 (0x01)
- `MainHeartbeatHandler`：主机心跳包处理器 (0x11)
- `SlaveHeartbeatHandler`：分机心跳包处理器 (0x21)
- `GetServerTimeHandler`：获取服务器时间处理器 (0x12)
- `SwipeCardHandler`：刷卡请求处理器 (0x02)
- `ChargeControlHandler`：充电控制处理器 (0x82)

### 端口和适配器架构

项目采用六边形架构（也称为端口和适配器架构），实现了业务逻辑与技术实现的分离：

1. **核心结构**：
   - `internal/domain`：领域层，包含核心业务模型和协议定义
   - `internal/app`：应用层，包含业务服务实现
   - `internal/ports`：端口层，定义系统与外部交互的边界
   - `internal/adapter`：适配器层，实现与外部系统的交互
   - `internal/infrastructure`：基础设施层，提供技术支持

2. **关键端口**：
   - `ports/tcp_server.go`：TCP服务器启动入口
   - `ports/http_server.go`：HTTP API服务入口

3. **核心适配器**：
   - `adapter/http`：HTTP请求处理适配器
   - `infrastructure/zinx_server`：Zinx网络框架适配器
   - `infrastructure/redis`：Redis数据存储适配器
   - `infrastructure/config`：配置管理适配器
   - `infrastructure/logger`：日志适配器

### 设备连接生命周期

1. 设备发送ICCID (SIM卡号)：网关保存ICCID并等待后续消息
2. 设备发送设备注册请求 (0x20)：网关解析设备信息并完成注册
3. 设备发送心跳包 (0x01/0x11/0x21)：网关更新设备状态
4. 设备发送刷卡请求 (0x02)：网关验证卡片并响应
5. 网关发送充电控制命令 (0x82)：设备执行充电操作并响应结果

## 协议支持

本系统实现了DNY协议（设备通信协议），支持以下功能：

1. 设备注册与认证
2. 心跳保活
3. 刷卡消费
4. 充电控制
5. 设备状态查询

## 版权与许可

Copyright © 2025 bujia-iot

Licensed under the MIT License. 