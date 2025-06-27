package network

import (
	"fmt"
	"testing"
	"time"
)

// MockConnectionMonitor 模拟连接监控器
type MockConnectionMonitor struct{}

func (m *MockConnectionMonitor) OnConnectionEstablished(conn interface{})                   {}
func (m *MockConnectionMonitor) OnConnectionClosed(conn interface{})                        {}
func (m *MockConnectionMonitor) OnRawDataReceived(conn interface{}, data []byte)            {}
func (m *MockConnectionMonitor) OnRawDataSent(conn interface{}, data []byte)                {}
func (m *MockConnectionMonitor) BindDeviceIdToConnection(deviceId string, conn interface{}) {}
func (m *MockConnectionMonitor) GetConnectionByDeviceId(deviceId string) (interface{}, bool) {
	return nil, false
}
func (m *MockConnectionMonitor) GetDeviceIdByConnId(connId uint64) (string, bool)  { return "", false }
func (m *MockConnectionMonitor) UpdateLastHeartbeatTime(conn interface{})          {}
func (m *MockConnectionMonitor) UpdateDeviceStatus(deviceId string, status string) {}
func (m *MockConnectionMonitor) ForEachConnection(callback func(deviceId string, conn interface{}) bool) {
}

// TestNewUnifiedSender 测试统一发送器创建
func TestNewUnifiedSender(t *testing.T) {
	sender := NewUnifiedSender(nil)

	if sender == nil {
		t.Fatal("统一发送器创建失败")
	}

	if sender.tcpWriter == nil {
		t.Error("TCP写入器未初始化")
	}

	// 监控器可以为 nil，这是允许的

	if sender.retryConfig.MaxRetries != DefaultRetryConfig.MaxRetries {
		t.Errorf("重试配置错误：期望%d，实际%d", DefaultRetryConfig.MaxRetries, sender.retryConfig.MaxRetries)
	}

	t.Log("统一发送器创建成功")
}

// TestSendConfig 测试发送配置
func TestSendConfig(t *testing.T) {
	config := DefaultSendConfig

	if config.Type != SendTypeDNYPacket {
		t.Errorf("默认发送类型错误：期望%d，实际%d", SendTypeDNYPacket, config.Type)
	}

	if config.MaxRetries <= 0 {
		t.Error("最大重试次数应该大于0")
	}

	if config.RetryDelay <= 0 {
		t.Error("重试延迟应该大于0")
	}

	if !config.HealthCheck {
		t.Error("健康检查应该默认启用")
	}

	t.Logf("默认发送配置验证通过：%+v", config)
}

// TestCalculateRetryDelay 测试重试延迟计算
func TestCalculateRetryDelay(t *testing.T) {
	sender := NewUnifiedSender(nil)

	baseDelay := 100 * time.Millisecond

	// 测试指数退避
	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},  // 2^0 * 100ms = 100ms
		{1, 200 * time.Millisecond},  // 2^1 * 100ms = 200ms
		{2, 400 * time.Millisecond},  // 2^2 * 100ms = 400ms
		{3, 800 * time.Millisecond},  // 2^3 * 100ms = 800ms
		{4, 1600 * time.Millisecond}, // 2^4 * 100ms = 1600ms
		{5, 3200 * time.Millisecond}, // 2^5 * 100ms = 3200ms
		{6, 5000 * time.Millisecond}, // 超过最大延迟，应该是5000ms
	}

	for _, tc := range testCases {
		actual := sender.calculateRetryDelay(tc.attempt, baseDelay)
		if actual != tc.expected {
			t.Errorf("重试延迟计算错误：attempt=%d，期望%v，实际%v", tc.attempt, tc.expected, actual)
		}
	}

	t.Log("重试延迟计算测试通过")
}

// TestCalculateAdaptiveTimeout 测试自适应超时计算
func TestCalculateAdaptiveTimeout(t *testing.T) {
	sender := NewUnifiedSender(nil)

	baseTimeout := 30 * time.Second

	// 测试超时时间递增
	testCases := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 30 * time.Second}, // 30s + 0*5s = 30s
		{1, 35 * time.Second}, // 30s + 1*5s = 35s
		{2, 40 * time.Second}, // 30s + 2*5s = 40s
		{3, 45 * time.Second}, // 30s + 3*5s = 45s
	}

	for _, tc := range testCases {
		actual := sender.calculateAdaptiveTimeout(nil, baseTimeout, tc.attempt)
		if actual != tc.expected {
			t.Errorf("自适应超时计算错误：attempt=%d，期望%v，实际%v", tc.attempt, tc.expected, actual)
		}
	}

	// 测试最大超时限制
	largeAttempt := 100
	maxTimeout := 120 * time.Second
	actual := sender.calculateAdaptiveTimeout(nil, baseTimeout, largeAttempt)
	if actual != maxTimeout {
		t.Errorf("最大超时限制错误：期望%v，实际%v", maxTimeout, actual)
	}

	t.Log("自适应超时计算测试通过")
}

// TestShouldContinueRetry 测试重试判断逻辑
func TestShouldContinueRetry(t *testing.T) {
	sender := NewUnifiedSender(nil)

	// 测试正常重试
	if !sender.shouldContinueRetry(nil, fmt.Errorf("timeout"), 0, 3) {
		t.Error("超时错误应该重试")
	}

	// 测试达到最大重试次数
	if sender.shouldContinueRetry(nil, fmt.Errorf("timeout"), 3, 3) {
		t.Error("达到最大重试次数应该停止重试")
	}

	// 测试连接关闭错误
	if sender.shouldContinueRetry(nil, fmt.Errorf("use of closed network connection"), 0, 3) {
		t.Error("连接关闭错误不应该重试")
	}

	// 测试成功情况
	if sender.shouldContinueRetry(nil, nil, 0, 3) {
		t.Error("成功情况不应该重试")
	}

	t.Log("重试判断逻辑测试通过")
}

// TestGlobalSenderFunctions 测试全局发送器函数
func TestGlobalSenderFunctions(t *testing.T) {
	// 测试全局发送器未初始化的情况
	err := SendRaw(nil, []byte("test"))
	if err == nil {
		t.Error("全局发送器未初始化应该返回错误")
	}

	err = SendDNY(nil, []byte("test"))
	if err == nil {
		t.Error("全局发送器未初始化应该返回错误")
	}

	err = SendResponse(nil, 0x04A228CD, 0x1234, 0x82, []byte("test"))
	if err == nil {
		t.Error("全局发送器未初始化应该返回错误")
	}

	err = SendCommand(nil, 0x04A228CD, 0x1234, 0x82, []byte("test"))
	if err == nil {
		t.Error("全局发送器未初始化应该返回错误")
	}

	t.Log("全局发送器函数测试通过")
}

// BenchmarkUnifiedSender 性能测试
func BenchmarkUnifiedSender(b *testing.B) {
	sender := NewUnifiedSender(nil)

	physicalID := uint32(0x04A228CD)
	messageID := uint16(0x1234)
	command := uint8(0x82)
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		packet := sender.buildDNYPacket(physicalID, messageID, command, data)
		_ = packet
	}
}
