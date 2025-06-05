package network

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ConnectionHooks 连接钩子，处理Zinx连接事件
type ConnectionHooks struct {
	// 事件回调函数
	onConnectionEstablished func(conn ziface.IConnection)
	onConnectionClosed      func(conn ziface.IConnection, reason string)
	onDeviceHeartbeat       func(deviceID string, conn ziface.IConnection)
	onDeviceDisconnect      func(deviceID string, conn ziface.IConnection, reason string)

	// TCP连接参数
	readDeadLine    time.Duration
	writeDeadLine   time.Duration
	keepAlivePeriod time.Duration
}

// NewConnectionHooks 创建连接钩子
func NewConnectionHooks(readDeadLine, writeDeadLine, keepAlivePeriod time.Duration) *ConnectionHooks {
	return &ConnectionHooks{
		readDeadLine:    readDeadLine,
		writeDeadLine:   writeDeadLine,
		keepAlivePeriod: keepAlivePeriod,
	}
}

// SetOnConnectionEstablished 设置连接建立回调
func (ch *ConnectionHooks) SetOnConnectionEstablished(callback func(conn ziface.IConnection)) {
	ch.onConnectionEstablished = callback
}

// SetOnConnectionClosed 设置连接关闭回调
func (ch *ConnectionHooks) SetOnConnectionClosed(callback func(conn ziface.IConnection, reason string)) {
	ch.onConnectionClosed = callback
}

// SetOnDeviceHeartbeat 设置设备心跳回调
func (ch *ConnectionHooks) SetOnDeviceHeartbeat(callback func(deviceID string, conn ziface.IConnection)) {
	ch.onDeviceHeartbeat = callback
}

// SetOnDeviceDisconnect 设置设备断开回调
func (ch *ConnectionHooks) SetOnDeviceDisconnect(callback func(deviceID string, conn ziface.IConnection, reason string)) {
	ch.onDeviceDisconnect = callback
}

// SetOnConnectionClosedFunc 设置连接关闭回调函数
func (ch *ConnectionHooks) SetOnConnectionClosedFunc(fn func(conn ziface.IConnection)) {
	ch.onConnectionClosed = func(conn ziface.IConnection, reason string) {
		fn(conn)
	}
}

// SetOnConnectionEstablishedFunc 设置连接建立回调函数
func (ch *ConnectionHooks) SetOnConnectionEstablishedFunc(fn func(conn ziface.IConnection)) {
	ch.onConnectionEstablished = fn
}

// OnConnectionStart 当连接建立时的钩子函数
// 按照 Zinx 生命周期最佳实践，在连接建立时设置 TCP 参数和连接属性
func (ch *ConnectionHooks) OnConnectionStart(conn ziface.IConnection) {
	// 获取连接信息
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 设置连接属性
	now := time.Now()
	ch.setConnectionInitialProperties(conn, now, remoteAddr)

	// 获取TCP连接并设置TCP参数
	ch.setupTCPParameters(conn, now)

	// 记录连接信息
	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"timestamp":  now.Format(constants.TimeFormatDefault),
		"connStatus": constants.ConnStatusActive,
	}).Info("新连接已建立")

	// 调用自定义连接建立回调
	if ch.onConnectionEstablished != nil {
		ch.onConnectionEstablished(conn)
	}
}

// 设置连接初始属性
func (ch *ConnectionHooks) setConnectionInitialProperties(conn ziface.IConnection, now time.Time, remoteAddr string) {
	// 设置最后心跳时间
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))
	// 设置连接状态为活跃
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)
	conn.SetProperty("RemoteAddr", remoteAddr)
}

// 设置TCP连接参数
func (ch *ConnectionHooks) setupTCPParameters(conn ziface.IConnection, now time.Time) {
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		// 设置TCP KeepAlive参数，适应移动网络的弱连接特性
		tcpConn.SetKeepAlive(true)
		// 使用配置的保活探测间隔
		tcpConn.SetKeepAlivePeriod(ch.keepAlivePeriod)

		// 设置读写超时
		readDeadline := now.Add(ch.readDeadLine)
		writeDeadline := now.Add(ch.writeDeadLine)
		ch.setTCPDeadlines(conn, tcpConn, readDeadline, writeDeadline)
	} else {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
		}).Warn("无法获取TCP连接，跳过TCP参数设置")
	}
}

// setTCPDeadlines 设置TCP读写超时
func (ch *ConnectionHooks) setTCPDeadlines(conn ziface.IConnection, tcpConn *net.TCPConn, readDeadline, writeDeadline time.Time) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()
	readDeadlineStr := readDeadline.Format(constants.TimeFormatDefault)
	writeDeadlineStr := writeDeadline.Format(constants.TimeFormatDefault)

	// 设置读取超时
	if err := tcpConn.SetReadDeadline(readDeadline); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"deadline":   readDeadlineStr,
		}).Error("设置TCP读取超时失败")
	}

	// 设置写入超时 - 增加5秒缓冲，避免因网络延迟导致写入超时
	if err := tcpConn.SetWriteDeadline(writeDeadline); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"deadline":   writeDeadlineStr,
		}).Error("设置TCP写入超时失败")
	}

	// 设置TCP缓冲区大小以提高性能
	// 提高接收缓冲区大小
	if err := tcpConn.SetReadBuffer(65536); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
		}).Warn("设置TCP读取缓冲区失败")
	}

	// 提高发送缓冲区大小
	if err := tcpConn.SetWriteBuffer(65536); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
		}).Warn("设置TCP写入缓冲区失败")
	}

	// 禁用Nagle算法，减少延迟
	if err := tcpConn.SetNoDelay(true); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
		}).Warn("禁用TCP Nagle算法失败")
	}
}

// OnConnectionStop 当连接断开时的钩子函数
func (ch *ConnectionHooks) OnConnectionStop(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 更新连接状态
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusClosed)

	// 获取心跳信息
	lastHeartbeatStr, timeSinceHeart := ch.getHeartbeatInfo(conn)

	// 尝试获取设备信息，优化连接断开日志记录
	deviceId, hasDeviceId := conn.GetProperty(constants.PropKeyDeviceId)
	var deviceIdStr string

	if hasDeviceId == nil && deviceId != nil {
		deviceIdStr = deviceId.(string)
	} else {
		deviceIdStr = "unregistered"
	}

	// 🔧 重要：清理该连接的所有命令队列
	commandManager := GetCommandManager()
	if commandManager != nil {
		// 在连接关闭前确保命令队列被清理
		commandManager.ClearConnectionCommands(connID)
		logger.WithFields(logrus.Fields{
			"connID":   connID,
			"deviceID": deviceIdStr,
		}).Info("已清理断开连接的命令队列")
	}

	// 尝试获取物理ID
	var physicalIDStr string
	physicalID, hasPhysicalID := conn.GetProperty(PropKeyDNYPhysicalID)
	if hasPhysicalID == nil && physicalID != nil {
		if id, ok := physicalID.(uint32); ok {
			physicalIDStr = fmt.Sprintf("0x%08X", id)

			// 如果设备有物理ID，通知其他系统组件该设备已断开连接
			// 这可以帮助其他组件及时清理与该设备相关的资源
			logger.WithFields(logrus.Fields{
				"physicalID": physicalIDStr,
				"connID":     connID,
			}).Info("设备物理ID连接已断开")
		}
	}

	// 记录连接断开日志
	logFields := logrus.Fields{
		"deviceId":       deviceIdStr,
		"remoteAddr":     remoteAddr,
		"connID":         connID,
		"lastHeartbeat":  lastHeartbeatStr,
		"timeSinceHeart": timeSinceHeart,
		"connStatus":     constants.ConnStatusClosed,
	}

	// 如果有物理ID，添加到日志字段
	if physicalIDStr != "" {
		logFields["physicalID"] = physicalIDStr
	}

	logger.WithFields(logFields).Info("设备连接断开")

	// 调用自定义连接关闭回调
	if ch.onConnectionClosed != nil {
		ch.onConnectionClosed(conn, "normal")
	}
}

// 获取心跳信息
func (ch *ConnectionHooks) getHeartbeatInfo(conn ziface.IConnection) (string, float64) {
	var lastHeartbeatStr string
	var timeSinceHeart float64

	if val, err := conn.GetProperty(constants.PropKeyLastHeartbeatStr); err == nil && val != nil {
		lastHeartbeatStr = val.(string)
	} else {
		// 降级使用时间戳
		if val, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil && val != nil {
			if timestamp, ok := val.(int64); ok {
				lastHeartbeatStr = time.Unix(timestamp, 0).Format(constants.TimeFormatDefault)
				timeSinceHeart = time.Since(time.Unix(timestamp, 0)).Seconds()
			} else {
				lastHeartbeatStr = "invalid"
			}
		} else {
			lastHeartbeatStr = "never"
		}
	}

	return lastHeartbeatStr, timeSinceHeart
}

// OnConnectionLost 连接丢失处理
func (ch *ConnectionHooks) OnConnectionLost(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 获取连接的设备ID
	var deviceID string
	var iccid string

	if prop, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && prop != nil {
		deviceID = prop.(string)
	}

	if prop, err := conn.GetProperty(constants.PropKeyICCID); err == nil && prop != nil {
		iccid = prop.(string)
	}

	// 记录连接关闭的详细信息
	fields := logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"time":       time.Now().Format(constants.TimeFormatDefault),
	}

	if deviceID != "" {
		fields["deviceID"] = deviceID
	}

	if iccid != "" {
		fields["iccid"] = iccid
	}

	// 获取断开原因
	disconnectReason := "未知原因"
	if prop, err := conn.GetProperty("close_reason"); err == nil && prop != nil {
		disconnectReason = prop.(string)
	}
	fields["reason"] = disconnectReason

	// 获取最后心跳时间
	var lastHeartbeatTime time.Time
	if prop, err := conn.GetProperty(constants.PropKeyLastHeartbeat); err == nil && prop != nil {
		if timestamp, ok := prop.(int64); ok {
			lastHeartbeatTime = time.Unix(timestamp, 0)
			fields["lastHeartbeat"] = lastHeartbeatTime.Format(constants.TimeFormatDefault)
			fields["heartbeatAge"] = time.Since(lastHeartbeatTime).String()
		}
	}

	// 分析断开类型，优化日志级别
	var logLevel string
	switch {
	case strings.Contains(disconnectReason, "i/o timeout"):
		logLevel = "warn"
		disconnectReason = "连接超时"
	case strings.Contains(disconnectReason, "connection reset by peer"):
		logLevel = "warn"
		disconnectReason = "对端重置连接"
	case strings.Contains(disconnectReason, "EOF"):
		logLevel = "info"
		disconnectReason = "客户端正常关闭"
	case strings.Contains(disconnectReason, "use of closed network connection"):
		logLevel = "info"
		disconnectReason = "服务器关闭连接"
	default:
		logLevel = "info"
	}

	fields["reasonCategory"] = disconnectReason

	// 根据不同日志级别记录日志
	switch logLevel {
	case "warn":
		logger.WithFields(fields).Warn("连接断开")
	case "error":
		logger.WithFields(fields).Error("连接异常断开")
	default:
		logger.WithFields(fields).Info("连接关闭")
	}

	// 调用连接关闭回调
	if ch.onConnectionClosed != nil {
		ch.onConnectionClosed(conn, disconnectReason)
	}

	// 如果有设备ID，通知设备监控器
	if deviceID != "" && ch.onDeviceDisconnect != nil {
		disconnectType := "normal"
		if logLevel == "warn" || logLevel == "error" {
			disconnectType = "abnormal"
		}

		// 将断开类型作为原因传递
		ch.onDeviceDisconnect(deviceID, conn, disconnectType+":"+disconnectReason)
	}
}
