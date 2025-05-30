# 测试目录

本目录包含项目的测试代码。

## 测试文件

- `pkg_test.go`: 测试pkg包的基本功能，包括：
  - 初始化包依赖关系
  - 心跳包监控
  - DNY协议解析
  - 连接钩子

## 运行测试

```bash
# 运行全部测试
cd test
go test

# 运行特定测试文件
cd test
go test -v pkg_test.go

# 运行特定测试用例
cd test
go test -v -run TestHeartbeatMonitor
```

## 模拟组件

`pkg_test.go`文件中包含了一些模拟组件，用于测试：

- `mockConnection`: 模拟Zinx连接
- `mockAddr`: 模拟网络地址

这些模拟组件可以用于其他测试中，避免依赖真实的网络连接和设备。

## 编写新测试

添加新测试时，请遵循以下规则：

1. 每个测试函数以`Test`开头
2. 包含对所测试功能的简要描述
3. 测试失败时提供有用的错误消息
4. 使用`assert`包进行断言

例如：

```go
func TestSomething(t *testing.T) {
    // 初始化
    result := someFunction()
    
    // 断言
    assert.Equal(t, expectedValue, result, "函数应返回预期值")
}
``` 