package network

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// SendType 发送类型枚举
type SendType int

const (
	SendTypeRaw         SendType = iota // 发送原始数据（不封装）
	SendTypeDNYPacket                   // 发送已封装的DNY数据包
	SendTypeDNYResponse                 // 发送DNY协议响应（自动封装）
	SendTypeDNYCommand                  // 发送DNY协议命令（自动封装）
)

// SendConfig 发送配置
type SendConfig struct {
	Type           SendType
	MaxRetries     int
	RetryDelay     time.Duration
	HealthCheck    bool
	TimeoutProtect bool
	LogLevel       logrus.Level
}

// DefaultSendConfig 默认发送配置
var DefaultSendConfig = SendConfig{
	Type:           SendTypeDNYPacket,
	MaxRetries:     3,
	RetryDelay:     100 * time.Millisecond,
	HealthCheck:    true,
	TimeoutProtect: true,
	LogLevel:       logrus.InfoLevel,
}

// UnifiedSender 统一发送器 - 系统中唯一的发送入口
// 🔧 增强版：集成高级重试机制、连接健康管理、动态超时等功能
type UnifiedSender struct {
	tcpWriter     *TCPWriter
	monitor       monitor.IConnectionMonitor
	healthManager interface{} // 连接健康管理器（使用接口避免循环导入）
	retryConfig   RetryConfig // 重试配置
}

// NewUnifiedSender 创建统一发送器
// 🔧 增强版：集成连接健康管理和高级重试机制
func NewUnifiedSender(monitor monitor.IConnectionMonitor) *UnifiedSender {
	tcpWriter := NewTCPWriter(DefaultRetryConfig, nil, logrus.New())

	return &UnifiedSender{
		tcpWriter:     tcpWriter,
		monitor:       monitor,
		healthManager: nil, // 将在需要时延迟初始化，避免循环导入
		retryConfig:   DefaultRetryConfig,
	}
}

// SendRawData 发送原始数据（不封装协议）
// 用于：ICCID响应、AT命令响应等特殊情况
func (s *UnifiedSender) SendRawData(conn ziface.IConnection, data []byte) error {
	config := DefaultSendConfig
	config.Type = SendTypeRaw

	return s.sendWithConfig(conn, data, config, nil)
}

// SendDNYPacket 发送已封装的DNY数据包
// 用于：已经构建好的完整DNY协议包
func (s *UnifiedSender) SendDNYPacket(conn ziface.IConnection, packet []byte) error {
	config := DefaultSendConfig
	config.Type = SendTypeDNYPacket

	return s.sendWithConfig(conn, packet, config, nil)
}

// SendDNYResponse 发送DNY协议响应（自动封装）
// 用于：设备注册响应、充电控制响应等
func (s *UnifiedSender) SendDNYResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, responseData []byte) error {
	// 构建DNY响应包 - 🔧 使用内部构建函数（避免循环导入）
	packet := s.buildDNYPacket(physicalID, messageID, command, responseData)

	config := DefaultSendConfig
	config.Type = SendTypeDNYResponse

	sendInfo := &SendInfo{
		PhysicalID: physicalID,
		MessageID:  messageID,
		Command:    command,
		DataLen:    len(responseData),
	}

	return s.sendWithConfig(conn, packet, config, sendInfo)
}

// SendDNYCommand 发送DNY协议命令（自动封装）
// 用于：充电控制命令、设备查询命令等
func (s *UnifiedSender) SendDNYCommand(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, commandData []byte) error {
	// 构建DNY命令包 - 🔧 使用内部构建函数（避免循环导入）
	packet := s.buildDNYPacket(physicalID, messageID, command, commandData)

	config := DefaultSendConfig
	config.Type = SendTypeDNYCommand

	sendInfo := &SendInfo{
		PhysicalID: physicalID,
		MessageID:  messageID,
		Command:    command,
		DataLen:    len(commandData),
	}

	return s.sendWithConfig(conn, packet, config, sendInfo)
}

// SendInfo 发送信息
type SendInfo struct {
	PhysicalID uint32
	MessageID  uint16
	Command    uint8
	DataLen    int
}

// sendWithConfig 使用配置发送数据
// 🔧 增强版：集成高级重试机制、动态超时、连接健康管理
func (s *UnifiedSender) sendWithConfig(conn ziface.IConnection, data []byte, config SendConfig, info *SendInfo) error {
	// 1. 基本验证
	if conn == nil {
		return fmt.Errorf("连接为空")
	}
	if len(data) == 0 {
		return fmt.Errorf("数据为空")
	}

	// 2. 健康检查
	if config.HealthCheck && !s.isConnectionHealthy(conn) {
		return fmt.Errorf("连接不健康")
	}

	// 3. 记录发送开始
	s.logSendStart(conn, config.Type, data, info)

	// 4. 执行发送 - 🔧 使用增强的发送逻辑
	var err error
	if config.MaxRetries > 0 {
		// 使用高级重试机制（集成动态超时和健康管理）
		err = s.sendWithAdvancedRetry(conn, data, config)
	} else {
		// 直接发送（不重试）
		err = conn.SendBuffMsg(0, data)
	}

	// 5. 记录发送结果
	s.logSendResult(conn, config.Type, data, info, err)

	// 6. 通知监控器
	if s.monitor != nil {
		s.monitor.OnRawDataSent(conn, data)
	}

	return err
}

// isConnectionHealthy 检查连接健康状态
func (s *UnifiedSender) isConnectionHealthy(conn ziface.IConnection) bool {
	// 1. 基本连接检查
	if conn == nil || conn.GetConnID() <= 0 {
		return false
	}

	// 2. 检查最后活动时间
	if lastActivity, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil {
		if timestamp, ok := lastActivity.(int64); ok {
			lastTime := time.Unix(timestamp, 0)
			inactiveTime := time.Since(lastTime)

			// 超过5分钟无活动认为不健康
			if inactiveTime > 5*time.Minute {
				return false
			}
		}
	}

	// 3. 检查TCP连接状态
	if rawConn := conn.GetConnection(); rawConn != nil {
		if tcpConn, ok := rawConn.(*net.TCPConn); ok {
			// 测试连接可用性
			testDeadline := time.Now().Add(1 * time.Millisecond)
			if err := tcpConn.SetWriteDeadline(testDeadline); err != nil {
				return false
			}
			// 重置写超时
			tcpConn.SetWriteDeadline(time.Time{})
		}
	}

	return true
}

// logSendStart 记录发送开始
func (s *UnifiedSender) logSendStart(conn ziface.IConnection, sendType SendType, data []byte, info *SendInfo) {
	fields := logrus.Fields{
		"connID":   conn.GetConnID(),
		"sendType": s.getSendTypeString(sendType),
		"dataLen":  len(data),
	}

	if info != nil {
		fields["physicalID"] = fmt.Sprintf("0x%08X", info.PhysicalID)
		fields["messageID"] = fmt.Sprintf("0x%04X", info.MessageID)
		fields["command"] = fmt.Sprintf("0x%02X", info.Command)
	}

	logger.WithFields(fields).Debug("开始发送数据")
}

// logSendResult 记录发送结果
func (s *UnifiedSender) logSendResult(conn ziface.IConnection, sendType SendType, data []byte, info *SendInfo, err error) {
	fields := logrus.Fields{
		"connID":   conn.GetConnID(),
		"sendType": s.getSendTypeString(sendType),
		"dataLen":  len(data),
		"dataHex":  fmt.Sprintf("%X", data),
	}

	if info != nil {
		fields["physicalID"] = fmt.Sprintf("0x%08X", info.PhysicalID)
		fields["messageID"] = fmt.Sprintf("0x%04X", info.MessageID)
		fields["command"] = fmt.Sprintf("0x%02X", info.Command)
	}

	if err != nil {
		fields["error"] = err.Error()
		logger.WithFields(fields).Error("数据发送失败")
	} else {
		logger.WithFields(fields).Info("数据发送成功")
	}
}

// getSendTypeString 获取发送类型字符串
func (s *UnifiedSender) getSendTypeString(sendType SendType) string {
	switch sendType {
	case SendTypeRaw:
		return "RAW"
	case SendTypeDNYPacket:
		return "DNY_PACKET"
	case SendTypeDNYResponse:
		return "DNY_RESPONSE"
	case SendTypeDNYCommand:
		return "DNY_COMMAND"
	default:
		return "UNKNOWN"
	}
}

// buildDNYPacket 构建DNY协议数据包的内部实现
// 🔧 重构：使用正确的协议规范，长度字段包含校验和
func (s *UnifiedSender) buildDNYPacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 计算数据长度 (物理ID + 消息ID + 命令 + 数据 + 校验和) - 根据协议文档
	contentLen := 4 + 2 + 1 + len(data) + 2 // PhysicalID(4) + MessageID(2) + Command(1) + Data + Checksum(2)

	// 创建包缓冲区
	packet := make([]byte, 0, 3+2+contentLen+2) // Header(3) + Length(2) + Content + Checksum(2)

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 数据长度 (2字节，小端序)
	packet = append(packet, byte(contentLen), byte(contentLen>>8))

	// 物理ID (4字节，小端序)
	packet = append(packet,
		byte(physicalID),
		byte(physicalID>>8),
		byte(physicalID>>16),
		byte(physicalID>>24))

	// 消息ID (2字节，小端序)
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令 (1字节)
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 计算校验和 (从包头"DNY"开始的所有字节，不包括校验和本身)
	var checksum uint16
	for i := 0; i < len(packet); i++ {
		checksum += uint16(packet[i])
	}

	// 校验和 (2字节，小端序)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// sendWithAdvancedRetry 使用高级重试机制发送数据
// 🔧 集成动态超时、连接健康管理、智能重试策略
func (s *UnifiedSender) sendWithAdvancedRetry(conn ziface.IConnection, data []byte, config SendConfig) error {
	connID := conn.GetConnID()
	var lastErr error
	startTime := time.Now()

	// 获取基础超时时间
	baseTimeout := 30 * time.Second
	if config.RetryDelay > 0 {
		baseTimeout = config.RetryDelay * time.Duration(config.MaxRetries)
	}

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// 1. 计算动态超时时间
		adaptiveTimeout := s.calculateAdaptiveTimeout(conn, baseTimeout, attempt)

		// 2. 设置写超时
		if err := s.setWriteTimeout(conn, adaptiveTimeout); err != nil {
			logger.WithFields(logrus.Fields{
				"connID":  connID,
				"attempt": attempt + 1,
				"timeout": adaptiveTimeout.String(),
				"error":   err.Error(),
			}).Warn("设置写超时失败")
		}

		// 3. 执行写操作
		written, err := s.performWrite(conn, data)
		latency := time.Since(startTime)
		success := (err == nil && written == len(data))

		// 4. 更新连接健康指标
		s.updateConnectionHealth(connID, success, latency, err)

		if success {
			logger.WithFields(logrus.Fields{
				"connID":   connID,
				"dataLen":  len(data),
				"written":  written,
				"attempts": attempt + 1,
				"elapsed":  latency.String(),
			}).Debug("高级重试发送成功")
			return nil
		}

		lastErr = err

		// 5. 检查是否应该继续重试
		if !s.shouldContinueRetry(conn, err, attempt, config.MaxRetries) {
			break
		}

		// 6. 重试延迟（指数退避）
		if attempt < config.MaxRetries {
			delay := s.calculateRetryDelay(attempt, config.RetryDelay)
			logger.WithFields(logrus.Fields{
				"connID":     connID,
				"attempt":    attempt + 1,
				"maxRetries": config.MaxRetries + 1,
				"delay":      delay.String(),
				"error":      err.Error(),
			}).Warn("发送失败，准备重试")
			time.Sleep(delay)
		}
	}

	// 所有重试都失败了
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"attempts":   config.MaxRetries + 1,
		"dataSize":   len(data),
		"finalError": lastErr.Error(),
		"totalTime":  time.Since(startTime).String(),
	}).Error("高级重试发送最终失败")

	return fmt.Errorf("发送失败，已重试%d次: %w", config.MaxRetries, lastErr)
}

// calculateAdaptiveTimeout 计算自适应超时时间
func (s *UnifiedSender) calculateAdaptiveTimeout(conn ziface.IConnection, baseTimeout time.Duration, attempt int) time.Duration {
	// 基础超时时间，根据重试次数递增
	timeout := baseTimeout + time.Duration(attempt)*5*time.Second

	// 最大超时限制
	maxTimeout := 120 * time.Second
	if timeout > maxTimeout {
		timeout = maxTimeout
	}

	return timeout
}

// setWriteTimeout 设置写超时
func (s *UnifiedSender) setWriteTimeout(conn ziface.IConnection, timeout time.Duration) error {
	tcpConn := conn.GetTCPConnection()
	if tcpConn == nil {
		return fmt.Errorf("无法获取TCP连接")
	}

	deadline := time.Now().Add(timeout)
	return tcpConn.SetWriteDeadline(deadline)
}

// performWrite 执行写操作
func (s *UnifiedSender) performWrite(conn ziface.IConnection, data []byte) (int, error) {
	tcpConn := conn.GetTCPConnection()
	if tcpConn == nil {
		return 0, fmt.Errorf("无法获取TCP连接")
	}

	return tcpConn.Write(data)
}

// updateConnectionHealth 更新连接健康指标
func (s *UnifiedSender) updateConnectionHealth(connID uint64, success bool, latency time.Duration, err error) {
	// 这里可以集成连接健康管理器，暂时使用简单的日志记录
	if !success {
		logger.WithFields(logrus.Fields{
			"connID":  connID,
			"latency": latency.String(),
			"error":   err.Error(),
		}).Debug("连接健康指标：发送失败")
	}
}

// shouldContinueRetry 判断是否应该继续重试
func (s *UnifiedSender) shouldContinueRetry(conn ziface.IConnection, err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	if err == nil {
		return false
	}

	// 检查错误类型，某些错误不应该重试
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "use of closed") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "broken pipe") {
		return false
	}

	return true
}

// calculateRetryDelay 计算重试延迟（指数退避）
func (s *UnifiedSender) calculateRetryDelay(attempt int, baseDelay time.Duration) time.Duration {
	if baseDelay <= 0 {
		baseDelay = 100 * time.Millisecond
	}

	// 指数退避：2^attempt * baseDelay
	multiplier := 1 << uint(attempt)
	delay := time.Duration(multiplier) * baseDelay

	// 最大延迟限制
	maxDelay := 5 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// 全局统一发送器实例
var globalUnifiedSender *UnifiedSender

// InitGlobalSender 初始化全局发送器
func InitGlobalSender(monitor monitor.IConnectionMonitor) {
	globalUnifiedSender = NewUnifiedSender(monitor)
}

// GetGlobalSender 获取全局发送器
func GetGlobalSender() *UnifiedSender {
	return globalUnifiedSender
}

// 便捷方法 - 直接使用全局发送器

// SendRaw 发送原始数据（全局方法）
func SendRaw(conn ziface.IConnection, data []byte) error {
	if globalUnifiedSender == nil {
		return fmt.Errorf("全局发送器未初始化")
	}
	return globalUnifiedSender.SendRawData(conn, data)
}

// SendDNY 发送DNY数据包（全局方法）
func SendDNY(conn ziface.IConnection, packet []byte) error {
	if globalUnifiedSender == nil {
		return fmt.Errorf("全局发送器未初始化")
	}
	return globalUnifiedSender.SendDNYPacket(conn, packet)
}

// SendResponse 发送DNY响应（全局方法）
func SendResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, responseData []byte) error {
	if globalUnifiedSender == nil {
		return fmt.Errorf("全局发送器未初始化")
	}
	return globalUnifiedSender.SendDNYResponse(conn, physicalID, messageID, command, responseData)
}

// SendCommand 发送DNY命令（全局方法）
func SendCommand(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8, commandData []byte) error {
	if globalUnifiedSender == nil {
		return fmt.Errorf("全局发送器未初始化")
	}
	return globalUnifiedSender.SendDNYCommand(conn, physicalID, messageID, command, commandData)
}
