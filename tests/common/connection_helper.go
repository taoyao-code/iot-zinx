package common

import (
	"fmt"
	"io"
	"net"
	"time"
)

// ConnectionHelper TCP连接管理辅助工具
type ConnectionHelper struct {
	defaultTimeout time.Duration
	retryCount     int
	retryDelay     time.Duration
}

// NewConnectionHelper 创建连接辅助工具实例
func NewConnectionHelper(timeout time.Duration) *ConnectionHelper {
	return &ConnectionHelper{
		defaultTimeout: timeout,
		retryCount:     3,
		retryDelay:     1 * time.Second,
	}
}

// EstablishTCPConnection 建立TCP连接
// 统一的TCP连接建立逻辑，替换重复的连接代码
func (ch *ConnectionHelper) EstablishTCPConnection(address string) (net.Conn, error) {
	return ch.EstablishTCPConnectionWithTimeout(address, ch.defaultTimeout)
}

// EstablishTCPConnectionWithTimeout 建立带超时的TCP连接
func (ch *ConnectionHelper) EstablishTCPConnectionWithTimeout(address string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("TCP连接失败 %s: %w", address, err)
	}
	return conn, nil
}

// EstablishTCPConnectionWithRetry 建立带重试的TCP连接
func (ch *ConnectionHelper) EstablishTCPConnectionWithRetry(address string) (net.Conn, error) {
	var lastErr error
	
	for i := 0; i < ch.retryCount; i++ {
		conn, err := ch.EstablishTCPConnection(address)
		if err == nil {
			return conn, nil
		}
		
		lastErr = err
		if i < ch.retryCount-1 {
			time.Sleep(ch.retryDelay)
		}
	}
	
	return nil, fmt.Errorf("TCP连接重试%d次后失败: %w", ch.retryCount, lastErr)
}

// SendProtocolData 发送协议数据
// 统一的数据发送逻辑，包含错误处理和日志记录
func (ch *ConnectionHelper) SendProtocolData(conn net.Conn, data []byte, description string) error {
	if conn == nil {
		return fmt.Errorf("连接为空")
	}
	
	if len(data) == 0 {
		return fmt.Errorf("发送数据为空")
	}
	
	// 设置写超时
	if err := conn.SetWriteDeadline(time.Now().Add(ch.defaultTimeout)); err != nil {
		return fmt.Errorf("设置写超时失败: %w", err)
	}
	
	// 发送数据
	n, err := conn.Write(data)
	if err != nil {
		return fmt.Errorf("发送%s失败: %w", description, err)
	}
	
	if n != len(data) {
		return fmt.Errorf("发送%s不完整: 期望%d字节，实际发送%d字节", description, len(data), n)
	}
	
	return nil
}

// ReadProtocolResponse 读取协议响应
// 统一的响应读取逻辑，包含超时处理
func (ch *ConnectionHelper) ReadProtocolResponse(conn net.Conn, timeout time.Duration) ([]byte, error) {
	if conn == nil {
		return nil, fmt.Errorf("连接为空")
	}
	
	// 设置读超时
	if err := conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, fmt.Errorf("设置读超时失败: %w", err)
	}
	
	// 读取响应数据
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return nil, fmt.Errorf("读取响应超时")
		}
		if err == io.EOF {
			return nil, fmt.Errorf("连接已关闭")
		}
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	
	return buffer[:n], nil
}

// ReadProtocolResponseWithDefaultTimeout 使用默认超时读取协议响应
func (ch *ConnectionHelper) ReadProtocolResponseWithDefaultTimeout(conn net.Conn) ([]byte, error) {
	return ch.ReadProtocolResponse(conn, ch.defaultTimeout)
}

// SendAndReceive 发送数据并接收响应
// 组合发送和接收操作，简化测试代码
func (ch *ConnectionHelper) SendAndReceive(conn net.Conn, data []byte, description string, responseTimeout time.Duration) ([]byte, error) {
	// 发送数据
	if err := ch.SendProtocolData(conn, data, description); err != nil {
		return nil, err
	}
	
	// 等待一小段时间让服务器处理
	time.Sleep(100 * time.Millisecond)
	
	// 读取响应
	response, err := ch.ReadProtocolResponse(conn, responseTimeout)
	if err != nil {
		return nil, fmt.Errorf("接收%s响应失败: %w", description, err)
	}
	
	return response, nil
}

// TestTCPConnectivity 测试TCP连通性
func (ch *ConnectionHelper) TestTCPConnectivity(address string) error {
	conn, err := ch.EstablishTCPConnection(address)
	if err != nil {
		return err
	}
	defer conn.Close()
	
	return nil
}

// CloseConnection 安全关闭连接
func (ch *ConnectionHelper) CloseConnection(conn net.Conn) error {
	if conn == nil {
		return nil
	}
	
	return conn.Close()
}

// SetRetryConfig 设置重试配置
func (ch *ConnectionHelper) SetRetryConfig(retryCount int, retryDelay time.Duration) {
	if retryCount > 0 {
		ch.retryCount = retryCount
	}
	if retryDelay > 0 {
		ch.retryDelay = retryDelay
	}
}

// GetConnectionInfo 获取连接信息
func (ch *ConnectionHelper) GetConnectionInfo(conn net.Conn) map[string]interface{} {
	if conn == nil {
		return map[string]interface{}{
			"status": "nil",
		}
	}
	
	info := map[string]interface{}{
		"local_addr":  conn.LocalAddr().String(),
		"remote_addr": conn.RemoteAddr().String(),
		"network":     conn.LocalAddr().Network(),
	}
	
	// 尝试获取TCP连接特定信息
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		info["tcp_connection"] = true
		
		// 可以添加更多TCP特定信息
		_ = tcpConn // 避免未使用变量警告
	} else {
		info["tcp_connection"] = false
	}
	
	return info
}

// BatchSendData 批量发送数据
// 用于压力测试和并发测试
func (ch *ConnectionHelper) BatchSendData(conn net.Conn, dataList [][]byte, descriptions []string, interval time.Duration) []error {
	if len(dataList) != len(descriptions) {
		return []error{fmt.Errorf("数据列表和描述列表长度不匹配")}
	}
	
	errors := make([]error, len(dataList))
	
	for i, data := range dataList {
		desc := descriptions[i]
		err := ch.SendProtocolData(conn, data, desc)
		errors[i] = err
		
		if i < len(dataList)-1 && interval > 0 {
			time.Sleep(interval)
		}
	}
	
	return errors
}

// WaitForConnection 等待连接可用
func (ch *ConnectionHelper) WaitForConnection(address string, maxWait time.Duration) error {
	start := time.Now()
	
	for time.Since(start) < maxWait {
		if err := ch.TestTCPConnectivity(address); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	return fmt.Errorf("等待连接%s超时(%v)", address, maxWait)
}

// CreateConnectionPool 创建连接池（简化版）
func (ch *ConnectionHelper) CreateConnectionPool(address string, poolSize int) ([]net.Conn, error) {
	connections := make([]net.Conn, 0, poolSize)
	
	for i := 0; i < poolSize; i++ {
		conn, err := ch.EstablishTCPConnection(address)
		if err != nil {
			// 关闭已创建的连接
			for _, existingConn := range connections {
				existingConn.Close()
			}
			return nil, fmt.Errorf("创建连接池失败，第%d个连接: %w", i+1, err)
		}
		connections = append(connections, conn)
	}
	
	return connections, nil
}

// CloseConnectionPool 关闭连接池
func (ch *ConnectionHelper) CloseConnectionPool(connections []net.Conn) []error {
	errors := make([]error, 0)
	
	for i, conn := range connections {
		if err := ch.CloseConnection(conn); err != nil {
			errors = append(errors, fmt.Errorf("关闭连接%d失败: %w", i, err))
		}
	}
	
	return errors
}

// 全局连接辅助工具实例
var DefaultConnectionHelper = NewConnectionHelper(10 * time.Second)
