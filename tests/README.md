# IoT-Zinx 测试模块使用指南

## 📁 测试结构

```
tests/
├── common/                    # 测试基础设施
│   ├── test_suite.go         # 统一测试套件管理
│   ├── protocol_helper.go    # 协议测试辅助函数
│   ├── connection_helper.go  # TCP连接管理辅助
│   └── assertion_helper.go   # 标准化测试断言
├── connectivity_test.go      # 基础连通性测试
├── protocol_test.go         # TCP协议测试
└── README.md               # 本使用指南
```

## 🚀 基本测试命令

### 运行所有测试
```bash
# 运行所有测试（详细输出）
go test ./tests/ -v

# 运行所有测试（简洁输出）
go test ./tests/

# 运行所有测试（带超时）
go test ./tests/ -v -timeout=60s
```

### 运行特定测试类别
```bash
# 只运行连通性测试
go test ./tests/ -run TestConnectivity -v

# 只运行协议测试
go test ./tests/ -run TestProtocol -v

# 运行特定子测试
go test ./tests/ -run TestConnectivity/TCP连接测试 -v
go test ./tests/ -run TestProtocol/协议包构建测试 -v
```

### 运行性能基准测试
```bash
# 运行所有基准测试
go test ./tests/ -bench=.

# 运行基准测试（指定时间）
go test ./tests/ -bench=. -benchtime=5s

# 运行特定基准测试
go test ./tests/ -bench=BenchmarkTCPConnection
go test ./tests/ -bench=BenchmarkHTTPRequest
```

## 📊 测试类别详解

### 1. 连通性测试 (connectivity_test.go)

**测试内容**：
- TCP连接测试 - 验证TCP服务器可达性
- HTTP连接测试 - 验证HTTP服务可用性
- 健康检查API测试 - 验证健康检查端点
- 连接重试测试 - 验证连接重试机制
- 连接超时测试 - 验证超时处理

**运行命令**：
```bash
# 运行所有连通性测试
go test ./tests/ -run TestConnectivity -v

# 运行特定连通性测试
go test ./tests/ -run TestConnectivity/TCP连接测试 -v
go test ./tests/ -run TestConnectivity/HTTP连接测试 -v
go test ./tests/ -run TestConnectivity/健康检查API测试 -v
```

### 2. 协议测试 (protocol_test.go)

**测试内容**：
- 协议包构建测试 - 验证统一协议构建函数
- 异常协议帧测试 - 验证服务器异常处理稳定性
  - 无效包头处理
  - 长度错误处理
  - 数据截断处理
  - 空数据包处理

**运行命令**：
```bash
# 运行所有协议测试
go test ./tests/ -run TestProtocol -v

# 运行特定协议测试
go test ./tests/ -run TestProtocol/协议包构建测试 -v
go test ./tests/ -run TestProtocol/异常协议帧测试 -v
```

## 🔧 测试配置

### 默认配置
- **HTTP服务地址**: http://localhost:7055
- **TCP服务地址**: localhost:7054
- **超时时间**: 10秒
- **并发数**: 5
- **重试次数**: 3次
- **重试延迟**: 1秒

### 自定义配置
测试配置在 `tests/common/test_suite.go` 中的 `DefaultTestConfig()` 函数中定义。

## 📈 测试报告解读

### 测试摘要报告
每个测试函数执行后会显示详细的测试摘要：

```
============================================================
📊 测试摘要报告
============================================================
总测试数: 5
通过: 5
失败: 0
跳过: 0
成功率: 100.00%
============================================================
```

### 性能基准报告
```
BenchmarkTCPConnection-8   	   10000	    588220 ns/op
BenchmarkHTTPRequest-8     	    9099	    917769 ns/op
```
- `10000`: 执行次数
- `588220 ns/op`: 每次操作平均耗时（纳秒）

## 🛠️ 开发和扩展

### 添加新测试
1. 在相应的测试文件中添加新的测试函数
2. 使用统一的测试辅助工具：
   - `common.TestSuite` - 测试套件管理
   - `common.ProtocolHelper` - 协议辅助
   - `common.ConnectionHelper` - 连接管理
   - `common.AssertionHelper` - 测试断言

### 测试最佳实践
```go
func TestNewFeature(t *testing.T) {
    // 创建测试套件
    suite := common.NewTestSuite(common.DefaultTestConfig())
    connHelper := common.DefaultConnectionHelper
    protocolHelper := common.DefaultProtocolHelper
    assertHelper := common.DefaultAssertionHelper

    t.Run("具体测试场景", func(t *testing.T) {
        start := time.Now()
        
        // 测试逻辑
        // ...
        
        // 记录测试结果
        suite.RecordTestResult("测试名称", "测试类型", success, time.Since(start), err, "描述", responseData)
        
        // 断言验证
        assertHelper.AssertNoError(t, err, "操作描述")
    })
    
    // 打印测试摘要
    suite.PrintSummary()
}
```

## 🔍 故障排查

### 常见问题

1. **TCP连接失败**
   - 检查服务器是否运行：`netstat -an | grep 7054`
   - 检查防火墙设置
   - 确认服务器地址配置正确

2. **HTTP请求失败**
   - 检查HTTP服务是否运行：`curl http://localhost:7055/health`
   - 检查端口是否被占用
   - 确认HTTP服务配置正确

3. **测试超时**
   - 增加超时时间：`go test ./tests/ -timeout=120s`
   - 检查网络连接
   - 确认服务器响应正常

### 调试技巧

1. **详细日志输出**
   ```bash
   go test ./tests/ -v -run TestProtocol
   ```

2. **单独运行失败的测试**
   ```bash
   go test ./tests/ -run TestConnectivity/TCP连接测试 -v
   ```

3. **查看测试覆盖率**
   ```bash
   go test ./tests/ -cover
   ```

## 📚 相关文档

- [Go测试官方文档](https://golang.org/pkg/testing/)
- [Go基准测试指南](https://golang.org/pkg/testing/#hdr-Benchmarks)
- [项目架构文档](../README.md)

## 🎯 测试目标

- **连通性测试**: 确保服务可达性 ≥ 95%
- **协议测试**: 确保协议包格式正确率 = 100%
- **性能测试**: TCP连接 < 1ms, HTTP请求 < 2ms
- **稳定性测试**: 异常处理不导致服务崩溃

---

**更新时间**: 2025年8月2日  
**版本**: v1.0  
**维护者**: IoT-Zinx开发团队
