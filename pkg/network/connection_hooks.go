package network

import (
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/sirupsen/logrus"
)

// ConnectionHooks 连接钩子管理器
type ConnectionHooks struct {
	// TCP连接参数
	readDeadLine    time.Duration
	writeDeadLine   time.Duration
	keepAlivePeriod time.Duration

	// 连接事件处理函数
	onConnectionEstablishedFunc func(conn ziface.IConnection)
	onConnectionClosedFunc      func(conn ziface.IConnection)
}

// NewConnectionHooks 创建连接钩子管理器
func NewConnectionHooks(
	readDeadLine time.Duration,
	writeDeadLine time.Duration,
	keepAlivePeriod time.Duration,
) *ConnectionHooks {
	return &ConnectionHooks{
		readDeadLine:    readDeadLine,
		writeDeadLine:   writeDeadLine,
		keepAlivePeriod: keepAlivePeriod,
	}
}

// SetOnConnectionEstablishedFunc 设置连接建立回调函数
func (ch *ConnectionHooks) SetOnConnectionEstablishedFunc(fn func(conn ziface.IConnection)) {
	ch.onConnectionEstablishedFunc = fn
}

// SetOnConnectionClosedFunc 设置连接关闭回调函数
func (ch *ConnectionHooks) SetOnConnectionClosedFunc(fn func(conn ziface.IConnection)) {
	ch.onConnectionClosedFunc = fn
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
		"timestamp":  now.Format("2006-01-02 15:04:05"),
		"connStatus": constants.ConnStatusActive,
	}).Info("新连接已建立")

	// 调用自定义连接建立回调
	if ch.onConnectionEstablishedFunc != nil {
		ch.onConnectionEstablishedFunc(conn)
	}
}

// 设置连接初始属性
func (ch *ConnectionHooks) setConnectionInitialProperties(conn ziface.IConnection, now time.Time, remoteAddr string) {
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format("2006-01-02 15:04:05"))
	conn.SetProperty("RemoteAddr", remoteAddr)
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)
}

// 设置TCP连接参数
func (ch *ConnectionHooks) setupTCPParameters(conn ziface.IConnection, now time.Time) {
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		// 设置TCP KeepAlive
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(ch.keepAlivePeriod)

		// 设置读写超时
		deadline := now.Add(ch.readDeadLine)
		ch.setTCPDeadlines(conn, tcpConn, deadline)
	}
}

// 设置TCP读写超时
func (ch *ConnectionHooks) setTCPDeadlines(conn ziface.IConnection, tcpConn *net.TCPConn, deadline time.Time) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()
	deadlineStr := deadline.Format("2006-01-02 15:04:05")

	// 设置读取超时
	if err := tcpConn.SetReadDeadline(deadline); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"deadline":   deadlineStr,
		}).Error("设置TCP读取超时失败")
	}

	// 设置写入超时
	if err := tcpConn.SetWriteDeadline(deadline); err != nil {
		logger.WithFields(logrus.Fields{
			"error":      err.Error(),
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"deadline":   deadlineStr,
		}).Error("设置TCP写入超时失败")
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

	// 记录连接断开日志
	logFields := logrus.Fields{
		"deviceId":       deviceIdStr,
		"remoteAddr":     remoteAddr,
		"connID":         connID,
		"lastHeartbeat":  lastHeartbeatStr,
		"timeSinceHeart": timeSinceHeart,
		"connStatus":     constants.ConnStatusClosed,
	}

	logger.WithFields(logFields).Info("设备连接断开")

	// 调用自定义连接关闭回调
	if ch.onConnectionClosedFunc != nil {
		ch.onConnectionClosedFunc(conn)
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
			if ts, ok := val.(int64); ok {
				lastHeartbeatStr = time.Unix(ts, 0).Format("2006-01-02 15:04:05")
				timeSinceHeart = time.Since(time.Unix(ts, 0)).Seconds()
			} else {
				lastHeartbeatStr = "invalid"
			}
		} else {
			lastHeartbeatStr = "never"
		}
	}

	return lastHeartbeatStr, timeSinceHeart
}
