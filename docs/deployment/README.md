# IoT-Zinx 充电桩系统部署指南

## 概述

本文档提供IoT-Zinx充电桩系统的完整部署指南，包括环境配置、系统安装、第三方平台对接和故障排查。

## 系统要求

### 硬件要求
- CPU: 2核心以上
- 内存: 4GB以上
- 存储: 20GB以上可用空间
- 网络: 稳定的网络连接

### 软件要求
- 操作系统: Linux (推荐Ubuntu 20.04+) / macOS / Windows
- Go语言: 1.19+
- 数据库: MySQL 8.0+ / PostgreSQL 13+ (可选)
- Redis: 6.0+ (可选，用于缓存)

## 快速开始

### 1. 下载源码

```bash
git clone https://github.com/your-org/iot-zinx.git
cd iot-zinx
```

### 2. 安装依赖

```bash
go mod download
```

### 3. 配置文件

复制配置模板：
```bash
cp configs/config.example.yaml configs/config.yaml
```

编辑配置文件：
```yaml
# 服务器配置
server:
  host: "0.0.0.0"
  port: 7055
  
# 业务平台配置
business_platform:
  base_url: "http://your-platform.com"
  api_key: "your_api_key"
  secret: "your_secret"
  
# 日志配置
logging:
  level: "info"
  file_path: "./logs/iot-zinx.log"
```

### 4. 启动服务

```bash
# 启动主服务
go run cmd/server/main.go

# 启动API服务
go run cmd/server-api/main.go
```

### 5. 验证安装

```bash
# 检查服务状态
curl http://localhost:7055/health

# 测试充电流程
./scripts/run_charging_test.sh quick_test
```

## 详细配置

### 业务平台对接配置

编辑 `configs/config.yaml`：

```yaml
business_platform:
  # 业务平台API地址
  base_url: "https://api.your-platform.com"
  
  # 认证信息
  api_key: "your_api_key_here"
  secret: "your_secret_here"
  
  # 连接配置
  timeout: 10s
  retry_count: 3
  retry_interval: 2s
  
  # 异步推送配置
  enable_async: true
  queue_size: 1000
  worker_count: 5
```

### 充电流程配置

编辑 `configs/charging_flow.yaml`：

```yaml
# 测试设备配置
devices:
  test_devices:
    - device_id: "your_device_id"
      name: "充电桩001"
      ports: [1, 2, 3, 4]
      max_power: 2200
      enabled: true

# 测试场景配置
charging_scenarios:
  production_test:
    name: "生产环境测试"
    description: "生产环境充电功能验证"
    duration: 300  # 5分钟
    amount: 10.0
    mode: 1
    concurrent: false
```

### 监控配置

```yaml
monitoring:
  # 充电状态监控
  charging_monitor:
    check_interval: 30s
    max_monitor_time: 8h
    timeout_threshold: 5m
    enable_alerts: true
    enable_auto_recover: true
  
  # 审计日志
  audit:
    enabled: true
    log_dir: "./logs/audit"
    max_file_size: 100  # MB
    retention_days: 30
```

## 生产环境部署

### 1. 使用Docker部署

创建 `Dockerfile`：
```dockerfile
FROM golang:1.19-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o iot-zinx cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/iot-zinx .
COPY --from=builder /app/configs ./configs
CMD ["./iot-zinx"]
```

构建和运行：
```bash
docker build -t iot-zinx .
docker run -d -p 7055:7055 -v $(pwd)/configs:/root/configs iot-zinx
```

### 2. 使用Docker Compose

创建 `docker-compose.yml`：
```yaml
version: '3.8'
services:
  iot-zinx:
    build: .
    ports:
      - "7055:7055"
    volumes:
      - ./configs:/root/configs
      - ./logs:/root/logs
    environment:
      - GO_ENV=production
    restart: unless-stopped
  
  redis:
    image: redis:6-alpine
    ports:
      - "6379:6379"
    restart: unless-stopped
```

启动：
```bash
docker-compose up -d
```

### 3. 系统服务配置

创建systemd服务文件 `/etc/systemd/system/iot-zinx.service`：
```ini
[Unit]
Description=IoT-Zinx Charging System
After=network.target

[Service]
Type=simple
User=iot-zinx
WorkingDirectory=/opt/iot-zinx
ExecStart=/opt/iot-zinx/iot-zinx
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

启用服务：
```bash
sudo systemctl enable iot-zinx
sudo systemctl start iot-zinx
sudo systemctl status iot-zinx
```

## 第三方平台对接

### 1. 配置事件接收端点

第三方平台需要提供事件接收接口：

```http
POST /api/v1/events
Content-Type: application/json
X-API-Key: your_api_key
X-API-Secret: your_secret

{
  "event_type": "charging_start",
  "data": {
    "device_id": "04ceaa40",
    "port_number": 1,
    "order_number": "ORDER_123456",
    "timestamp": 1640995200
  },
  "timestamp": 1640995200
}
```

### 2. 响应格式

平台应返回标准响应：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "event_id": "evt_123456",
    "received_at": "2024-01-01T12:00:00Z"
  }
}
```

### 3. 错误处理

- 网络错误：系统会自动重试
- 业务错误：记录日志，继续处理
- 超时错误：根据配置进行重试

## 监控和维护

### 1. 日志管理

日志文件位置：
- 主服务日志: `./logs/iot-zinx.log`
- 充电流程日志: `./logs/charging_flow.log`
- 审计日志: `./logs/audit/`

日志轮转配置：
```bash
# 添加到 /etc/logrotate.d/iot-zinx
/opt/iot-zinx/logs/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 iot-zinx iot-zinx
    postrotate
        systemctl reload iot-zinx
    endscript
}
```

### 2. 性能监控

关键指标：
- 设备在线率
- 充电成功率
- 平均响应时间
- 错误率

监控脚本示例：
```bash
#!/bin/bash
# 检查服务状态
curl -f http://localhost:7055/health || echo "服务异常"

# 检查设备连接数
netstat -an | grep :7055 | grep ESTABLISHED | wc -l

# 检查日志错误
tail -n 100 /opt/iot-zinx/logs/iot-zinx.log | grep ERROR
```

### 3. 备份策略

配置文件备份：
```bash
# 每日备份配置
tar -czf /backup/iot-zinx-config-$(date +%Y%m%d).tar.gz /opt/iot-zinx/configs/
```

日志备份：
```bash
# 每周备份日志
tar -czf /backup/iot-zinx-logs-$(date +%Y%m%d).tar.gz /opt/iot-zinx/logs/
```

## 故障排查

### 常见问题

#### 1. 设备连接失败
```bash
# 检查网络连接
telnet device_ip 7055

# 检查防火墙
sudo ufw status
sudo iptables -L

# 检查服务状态
systemctl status iot-zinx
```

#### 2. 充电启动失败
```bash
# 查看充电日志
tail -f logs/charging_flow.log

# 检查设备状态
curl http://localhost:7055/api/v1/devices/04ceaa40/status

# 运行诊断测试
./scripts/run_charging_test.sh quick_test
```

#### 3. 业务平台通信失败
```bash
# 检查网络连接
curl -v http://your-platform.com/api/v1/events

# 检查配置
grep -A 10 "business_platform" configs/config.yaml

# 查看错误日志
grep "business_platform" logs/iot-zinx.log
```

### 调试模式

启用调试模式：
```yaml
logging:
  level: "debug"
  console_output: true
```

或使用环境变量：
```bash
export LOG_LEVEL=debug
./iot-zinx
```

## 安全配置

### 1. 网络安全
- 使用防火墙限制访问
- 启用TLS/SSL加密
- 定期更新系统补丁

### 2. 应用安全
- 定期轮换API密钥
- 使用强密码策略
- 启用访问日志审计

### 3. 数据安全
- 定期备份重要数据
- 加密敏感配置信息
- 实施访问控制策略

## 性能优化

### 1. 系统优化
```bash
# 调整文件描述符限制
echo "* soft nofile 65536" >> /etc/security/limits.conf
echo "* hard nofile 65536" >> /etc/security/limits.conf

# 优化网络参数
echo "net.core.somaxconn = 65536" >> /etc/sysctl.conf
sysctl -p
```

### 2. 应用优化
- 调整连接池大小
- 优化日志级别
- 启用缓存机制

## 联系支持

如遇到问题，请提供以下信息：
1. 系统版本和配置
2. 错误日志
3. 复现步骤
4. 环境信息

技术支持：
- 邮箱: support@example.com
- 文档: https://docs.example.com
- 社区: https://community.example.com
