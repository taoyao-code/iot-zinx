# 充电设备网关系统 (IOT-Zinx)

基于Zinx网络框架的充电设备网关系统，实现与充电桩设备的通信和管理。

## 项目介绍

本系统是一个基于TCP协议的充电设备网关，负责连接和管理充电桩设备，处理设备上报的各种数据，并将业务请求转发给设备。系统采用六边形架构（端口与适配器架构），实现了业务逻辑与技术实现的分离。

### 主要功能

- **设备连接管理**：处理设备上线、注册和离线，支持ICCID和设备ID双重识别
- **多种心跳管理**：支持标准心跳、主机心跳、Link心跳等多种保活机制
- **刷卡消费**：处理设备刷卡请求，验证卡片有效性并授权消费
- **充电控制**：向设备发送充电启停命令，控制充电过程
- **设备状态监控**：实时监控设备心跳状态，自动清理超时连接
- **服务器时间同步**：提供精准时间服务，确保设备与服务器时间同步
- **命令重发机制**：关键业务命令支持超时重发，确保命令可靠送达
- **原始数据记录**：完整记录设备与服务器之间的通信数据，便于问题排查

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
│   ├── dny-parser/       # DNY协议解析工具
│   │   └── main.go       # 解析工具入口
│   └── gateway/          # 网关服务入口
│       └── main.go       # 主程序入口
├── conf/                 # 默认配置目录
│   └── zinx.json         # Zinx框架默认配置
├── configs/              # 应用配置文件目录
│   └── gateway.yaml      # 网关配置文件
├── deployments/          # 部署相关文件
├── docs/                 # 项目文档
│   ├── 进度/             # 进度文档
│   │   └── 阶段一完成报告.md # 阶段性报告
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
│   │       ├── frame.go
│   │       └── message_types.go
│   ├── infrastructure/   # 基础设施层
│   │   ├── config/       # 配置管理
│   │   │   └── config.go
│   │   ├── logger/       # 日志服务
│   │   │   └── logger.go
│   │   ├── redis/        # Redis客户端
│   │   │   └── client.go
│   │   └── zinx_server/  # Zinx服务器实现
│   │       ├── command_manager.go # 命令管理器
│   │       ├── common/    # 公共组件
│   │       │   └── monitor.go
│   │       ├── connection_hooks.go # 连接生命周期钩子
│   │       ├── device_monitor.go # 设备状态监控
│   │       ├── handlers/ # 命令处理器
│   │       │   ├── charge_control_handler.go
│   │       │   ├── connection_monitor.go
│   │       │   ├── device_register_handler.go
│   │       │   ├── device_status_handler.go
│   │       │   ├── dny_handler_base.go
│   │       │   ├── dny_protocol_parser.go
│   │       │   ├── get_server_time_handler.go
│   │       │   ├── heartbeat_check_router.go
│   │       │   ├── heartbeat_handler.go
│   │       │   ├── iccid_handler.go
│   │       │   ├── link_heartbeat_handler.go
│   │       │   ├── main_heartbeat_handler.go
│   │       │   ├── non_dny_data_handler.go
│   │       │   ├── parameter_setting_handler.go
│   │       │   ├── power_heartbeat_handler.go
│   │       │   ├── router.go
│   │       │   ├── settlement_handler.go
│   │       │   ├── swipe_card_handler.go
│   │       │   └── tcp_data_logger.go
│   │       ├── heartbeat.go # 心跳处理
│   │       ├── logger_adapter.go # 日志适配器
│   │       ├── monitor.go # 监控组件
│   │       ├── packet.go # 数据包处理器
│   │       ├── raw_data_handler.go # 原始数据处理
│   │       └── raw_data_hook.go # 原始数据钩子
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
├── script/               # 脚本工具
│   ├── parse-example.sh  # 示例解析脚本
│   ├── parse-log.sh      # 日志解析脚本
│   └── push.sh           # 推送脚本
├── test/                 # 测试代码
│   └── test_admin_tool.go # 测试管理工具
├── web/                  # Web相关文件
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
- `DeviceStatusHandler`：设备状态处理器 (0x21)
- `GetServerTimeHandler`：获取服务器时间处理器 (0x12/0x22)
- `SwipeCardHandler`：刷卡请求处理器 (0x02)
- `ChargeControlHandler`：充电控制处理器 (0x82)
- `ICCIDHandler`：ICCID识别处理器
- `LinkHeartbeatHandler`：Link心跳处理器
- `NonDNYDataHandler`：非DNY协议数据处理器
- `ParameterSettingHandler`：参数设置处理器
- `PowerHeartbeatHandler`：电源心跳处理器
- `SettlementHandler`：结算处理器

### 核心组件

- `connection_hooks.go`：连接生命周期钩子函数，处理设备连接建立和断开
- `packet.go`：数据包处理器，负责DNY协议数据的封包和解包
- `device_monitor.go`：设备状态监控器，监控设备心跳状态
- `command_manager.go`：命令管理器，管理发送命令的确认和超时重发
- `monitor.go`：TCP数据监视器，记录设备数据传输过程
- `raw_data_handler.go`：原始数据处理器，处理非结构化数据

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

1. **连接建立**：设备与网关建立TCP连接，网关创建连接对象并分配连接ID
2. **初始化识别**：设备可能发送ICCID(SIM卡号)或Link心跳等初始化数据
3. **设备注册**：设备发送注册请求(0x20)，网关解析设备信息并完成注册
4. **心跳保活**：设备定期发送心跳包(0x01/0x11/0x21)，网关更新设备状态
5. **业务交互**：设备发送业务请求(如刷卡0x02)或网关下发控制命令(如充电控制0x82)
6. **连接监控**：网关监控设备心跳状态，自动清理超时连接
7. **连接断开**：设备主动断开连接或网关检测到连接超时，释放连接资源

## 协议支持

本系统实现了DNY协议（设备通信协议），支持以下功能：

1. 设备注册与认证
2. 心跳保活（设备心跳、主机心跳、Link心跳等）
3. 刷卡消费
4. 充电控制
5. 设备状态监控
6. 服务器时间同步
7. 参数设置

## 版权与许可

Copyright © 2025 bujia-iot

Licensed under the MIT License. 