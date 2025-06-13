package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/bujia-iot/iot-zinx/pkg/session"
	"github.com/sirupsen/logrus"
)

// 🔧 架构重构说明：
// 本文件已更新使用统一的协议解析接口 protocol.ParseDNYData()
// 删除了重复的 DNYProtocolParser，避免重复解析和代码重复

// ConnectionMonitor 连接监控器 - 用于记录和分析连接相关事件
// 本文件已更新使用统一的协议解析接口 protocol.ParseDNYData()
type ConnectionMonitor struct {
	// 配置选项
	enableRawDataLogging bool
	enableHeartbeatCheck bool

	// 日志记录器
	logger *TCPDataLogger

	// 连接映射表
	connections sync.Map

	// 是否启用
	enabled bool
}

// 全局监视器实例
var (
	globalMonitor     *ConnectionMonitor
	globalMonitorOnce sync.Once
)

// GetGlobalMonitor 获取全局监视器实例
func GetGlobalMonitor() *ConnectionMonitor {
	globalMonitorOnce.Do(func() {
		var err error
		globalMonitor = &ConnectionMonitor{
			enabled:              true,
			enableRawDataLogging: true,
			enableHeartbeatCheck: true,
		}

		// 创建日志记录器
		globalMonitor.logger, err = NewTCPDataLogger("logs/tcp_data", true)
		if err != nil {
			fmt.Printf("创建TCP数据记录器失败: %v\n", err)
			globalMonitor.enabled = false
		} else {
			fmt.Printf("TCP数据记录器已创建，日志路径: logs/tcp_data\n")
		}
	})

	return globalMonitor
}

// NewConnectionMonitor 创建连接监控器
func NewConnectionMonitor(options ...func(*ConnectionMonitor)) *ConnectionMonitor {
	monitor := &ConnectionMonitor{
		enableRawDataLogging: true,
		enableHeartbeatCheck: true,
	}

	// 应用选项
	for _, option := range options {
		option(monitor)
	}

	return monitor
}

// WithRawDataLogging 启用/禁用原始数据日志记录
func WithRawDataLogging(enable bool) func(*ConnectionMonitor) {
	return func(m *ConnectionMonitor) {
		m.enableRawDataLogging = enable
	}
}

// WithHeartbeatCheck 启用/禁用心跳检查
func WithHeartbeatCheck(enable bool) func(*ConnectionMonitor) {
	return func(m *ConnectionMonitor) {
		m.enableHeartbeatCheck = enable
	}
}

// OnConnectionEstablished 当连接建立时的回调
func (m *ConnectionMonitor) OnConnectionEstablished(conn ziface.IConnection) {
	// 记录连接建立事件
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("连接已建立")

	// 通过DeviceSession管理连接属性
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateHeartbeat()
		deviceSession.UpdateStatus(constants.DeviceStatusOnline)
		deviceSession.SyncToConnection(conn)
	}
}

// OnConnectionClosed 当连接关闭时的回调
func (m *ConnectionMonitor) OnConnectionClosed(conn ziface.IConnection) {
	// 获取设备ID（如果有）
	var deviceId string
	if prop, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && prop != nil {
		if devId, ok := prop.(string); ok {
			deviceId = devId
		}
	}

	// 记录连接关闭事件
	logFields := logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}

	if deviceId != "" {
		logFields["deviceId"] = deviceId
	}

	// 检查上次心跳时间
	var lastHeartbeat time.Time
	if prop, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil {
		if heartbeat, ok := prop.(time.Time); ok {
			lastHeartbeat = heartbeat
			logFields["lastHeartbeat"] = lastHeartbeat.Format(constants.TimeFormatDefault)
			logFields["heartbeatAge"] = time.Since(lastHeartbeat).String()
		}
	}

	logger.WithFields(logFields).Info("连接已关闭")

	// 通过DeviceSession管理连接状态
	deviceSession := session.GetDeviceSession(conn)
	if deviceSession != nil {
		deviceSession.UpdateStatus(constants.DeviceStatusOffline)
		deviceSession.LastDisconnect = time.Now()
		deviceSession.SyncToConnection(conn)
	}
}

// OnRawDataReceived 当收到原始数据时的回调
func (m *ConnectionMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	if !m.enableRawDataLogging {
		return
	}

	// 尝试解析DNY协议
	if protocol.IsDNYProtocolData(data) {
		result, err := protocol.ParseDNYData(data)
		if err == nil && result != nil {
			// 这是DNY协议数据，已经在其他处理器中处理和记录
			return
		}
	}

	// 非DNY协议数据，进行特殊处理
	dataStr := string(data)
	trimmedData := strings.TrimSpace(dataStr)

	// 检查是否为特殊消息类型
	if protocol.HandleSpecialMessage(data) {
		// 特殊消息已被处理
		return
	}

	// 检查是否为AT命令
	if strings.HasPrefix(trimmedData, "AT") {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"command":    trimmedData,
		}).Info("收到AT命令")
		return
	}

	// 记录未知数据
	dataType := "未知数据"
	if len(data) > 0 && protocol.IsHexString(data) {
		dataType = "十六进制数据"
	} else if len(data) > 0 && protocol.IsAllDigits(data) {
		dataType = "数字数据"
	}

	// 限制数据长度，避免日志过大
	maxLogLen := 100
	displayData := dataStr
	if len(displayData) > maxLogLen {
		displayData = displayData[:maxLogLen] + "..."
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataType":   dataType,
		"dataLen":    len(data),
		"data":       displayData,
	}).Debug("收到未识别数据")

	// 尝试解析为十六进制字符串
	if protocol.IsHexString(data) {
		hexData := string(data)
		result, err := protocol.ParseDNYHexString(hexData)
		if err == nil && result != nil {
			logger.WithFields(logrus.Fields{
				"connID":        conn.GetConnID(),
				"physicalID":    fmt.Sprintf("0x%08X", result.PhysicalID),
				"command":       fmt.Sprintf("0x%02X", result.Command),
				"commandName":   result.CommandName,
				"dataLen":       len(result.Data),
				"checksumValid": result.ChecksumValid,
			}).Info("成功解析十六进制字符串为DNY协议")
		}
	}
}

// OnConnectionOpen 当连接打开时调用
func (m *ConnectionMonitor) OnConnectionOpen(conn ziface.IConnection) {
	if !m.enabled {
		return
	}

	// 记录连接信息
	remoteAddr := conn.RemoteAddr().String()
	m.connections.Store(conn.GetConnID(), remoteAddr)

	// 使用统一的日志记录方式
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": remoteAddr,
		"timestamp":  time.Now().Format(constants.TimeFormatDefault),
	}).Info("连接打开")

	// 记录到TCP日志
	if m.logger != nil {
		m.logger.LogMessage(fmt.Sprintf("连接打开: ID=%d, 远程地址=%s", conn.GetConnID(), remoteAddr))
	}
}

// OnDataReceived 当接收到数据时调用
func (m *ConnectionMonitor) OnDataReceived(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}

	// 获取连接信息
	remoteAddr, ok := m.connections.Load(conn.GetConnID())
	if !ok {
		remoteAddr = "未知"
	}

	// 记录接收到的数据
	if m.logger != nil {
		m.logger.LogData(conn.GetConnID(), remoteAddr.(string), data, "接收")
	}

	// 解析并打印数据
	m.parseAndPrintData(data, "接收", remoteAddr.(string))
}

// OnDataSent 当发送数据时调用
func (m *ConnectionMonitor) OnDataSent(conn ziface.IConnection, data []byte) {
	if !m.enabled {
		return
	}

	// 获取连接信息
	remoteAddr, ok := m.connections.Load(conn.GetConnID())
	if !ok {
		remoteAddr = "未知"
	}

	// 记录发送的数据
	if m.logger != nil {
		m.logger.LogData(conn.GetConnID(), remoteAddr.(string), data, "发送")
	}

	// 解析并打印数据
	m.parseAndPrintData(data, "发送", remoteAddr.(string))
}

// parseAndPrintData 解析并打印数据
func (m *ConnectionMonitor) parseAndPrintData(data []byte, direction, remoteAddr string) {
	// 检查是否为DNY协议数据
	if protocol.IsDNYProtocolData(data) {
		result, err := protocol.ParseDNYData(data)
		if err == nil {
			// 打印解析结果
			timestamp := time.Now().Format(constants.TimeFormatDefault)
			fmt.Printf("\n[%s] %s 数据 - %s\n", timestamp, direction, remoteAddr)
			fmt.Printf("命令: 0x%02X (%s)\n", result.Command, result.CommandName)
			fmt.Printf("物理ID: 0x%08X\n", result.PhysicalID)
			fmt.Printf("消息ID: 0x%04X\n", result.MessageID)
			fmt.Printf("数据长度: %d\n", len(result.Data))
			fmt.Printf("校验结果: %v\n", result.ChecksumValid)
			fmt.Println("----------------------------------------")
		}
	}
}

// Close 关闭监视器
func (m *ConnectionMonitor) Close() {
	if m.enabled && m.logger != nil {
		// 只需标记为已关闭，无需调用日志记录器的Close方法
		m.enabled = false
	}
}

// UpdateLastHeartbeatTime 更新上次心跳时间
func (m *ConnectionMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	// 委托给pkg/monitor中的实现，避免重复逻辑
	monitor.GetGlobalConnectionMonitor().UpdateLastHeartbeatTime(conn)
}

// ParseManualHexData 手动解析十六进制数据
func (m *ConnectionMonitor) ParseManualHexData(hexData, description string) {
	if !m.enabled || m.logger == nil {
		return
	}

	// 记录并解析数据
	m.logger.LogHexData(0, "手动解析", hexData, description)

	// 尝试解析DNY协议
	result, err := protocol.ParseDNYHexString(hexData)
	if err == nil {
		// 打印解析结果
		timestamp := time.Now().Format(constants.TimeFormatDefault)
		fmt.Printf("\n[%s] 手动解析: %s\n", timestamp, description)
		fmt.Printf("命令: 0x%02X (%s)\n", result.Command, result.CommandName)
		fmt.Printf("物理ID: 0x%08X\n", result.PhysicalID)
		fmt.Printf("消息ID: 0x%04X\n", result.MessageID)
		fmt.Printf("数据长度: %d\n", len(result.Data))
		fmt.Printf("校验结果: %v\n", result.ChecksumValid)
		fmt.Println("----------------------------------------")
	} else {
		fmt.Printf("\n[手动解析失败] %s: %v\n", description, err)
	}
}

// BindDeviceIdToConnection 当连接绑定设备ID时调用
func (m *ConnectionMonitor) BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	// 委托给pkg/monitor中的实现，避免重复逻辑
	monitor.GetGlobalConnectionMonitor().BindDeviceIdToConnection(deviceId, conn)
}
