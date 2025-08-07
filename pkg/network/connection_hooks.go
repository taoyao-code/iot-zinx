package network

import (
	"net"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/core"
	"github.com/sirupsen/logrus"
)

// ConnectionHooks 简化的连接钩子，处理Zinx连接事件
type ConnectionHooks struct {
	// 事件回调函数
	onConnectionEstablished func(conn ziface.IConnection)
	onConnectionClosed      func(conn ziface.IConnection, reason string)
	onDeviceHeartbeat       func(deviceID string, conn ziface.IConnection)
	onDeviceDisconnect      func(deviceID string, conn ziface.IConnection, reason string)

	// TCP连接参数
	initialReadDeadline time.Duration
	defaultReadDeadline time.Duration
	tcpWriteTimeout     time.Duration
	tcpReadTimeout      time.Duration
	keepAlivePeriod     time.Duration
}

// NewConnectionHooks 创建连接钩子
func NewConnectionHooks(initialReadDeadline, defaultReadDeadline, keepAlivePeriod time.Duration) *ConnectionHooks {
	return &ConnectionHooks{
		initialReadDeadline: initialReadDeadline,
		defaultReadDeadline: defaultReadDeadline,
		keepAlivePeriod:     keepAlivePeriod,
	}
}

// OnConnStart 连接建立时的回调
func (ch *ConnectionHooks) OnConnStart(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()
	now := time.Now()

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"timestamp":  now.Format("2006-01-02 15:04:05"),
	}).Info("新连接建立")

	// 注册到TCP管理器
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager != nil {
		_, err := tcpManager.RegisterConnection(conn)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"connID":     connID,
				"remoteAddr": remoteAddr,
				"error":      err.Error(),
			}).Error("注册连接到TCP管理器失败")
		} else {
			logger.WithFields(logrus.Fields{
				"connID":     connID,
				"remoteAddr": remoteAddr,
			}).Info("连接已注册到TCP管理器")
		}
	}

	// 设置TCP连接参数
	ch.setTCPParameters(conn)

	// 调用用户定义的回调
	if ch.onConnectionEstablished != nil {
		ch.onConnectionEstablished(conn)
	}
}

// OnConnStop 连接断开时的回调
func (ch *ConnectionHooks) OnConnStop(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
	}).Info("连接断开")

	// 调用用户定义的回调
	if ch.onConnectionClosed != nil {
		ch.onConnectionClosed(conn, "connection_closed")
	}
}

// setTCPParameters 设置TCP连接参数
func (ch *ConnectionHooks) setTCPParameters(conn ziface.IConnection) {
	// 获取底层TCP连接
	tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn)
	if !ok {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
		}).Warn("无法获取TCP连接，跳过TCP参数设置")
		return
	}

	// 设置Keep-Alive
	if err := tcpConn.SetKeepAlive(true); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Warn("设置Keep-Alive失败")
	}

	// 设置Keep-Alive周期
	if ch.keepAlivePeriod > 0 {
		if err := tcpConn.SetKeepAlivePeriod(ch.keepAlivePeriod); err != nil {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"period": ch.keepAlivePeriod,
				"error":  err.Error(),
			}).Warn("设置Keep-Alive周期失败")
		}
	}

	// 设置TCP_NODELAY
	if err := tcpConn.SetNoDelay(true); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Warn("设置TCP_NODELAY失败")
	}

	logger.WithFields(logrus.Fields{
		"connID":           conn.GetConnID(),
		"keepAlivePeriod":  ch.keepAlivePeriod,
		"tcpNoDelay":       true,
	}).Debug("TCP参数设置完成")
}

// === 全局连接钩子管理 ===

var globalConnectionHooks *ConnectionHooks

// GetGlobalConnectionHooks 获取全局连接钩子
func GetGlobalConnectionHooks() *ConnectionHooks {
	if globalConnectionHooks == nil {
		cfg := config.GetConfig()
		globalConnectionHooks = NewConnectionHooks(
			time.Duration(cfg.TCPServer.InitialReadDeadlineSeconds)*time.Second,
			time.Duration(cfg.TCPServer.DefaultReadDeadlineSeconds)*time.Second,
			time.Duration(cfg.TCPServer.KeepAlivePeriodSeconds)*time.Second,
		)
	}
	return globalConnectionHooks
}

// NotifyDeviceHeartbeat 通知设备心跳
func NotifyDeviceHeartbeat(deviceID string, conn ziface.IConnection) {
	// 更新TCP管理器中的心跳时间
	tcpManager := core.GetGlobalTCPManager()
	if tcpManager != nil {
		if err := tcpManager.UpdateHeartbeat(deviceID); err != nil {
			logger.WithFields(logrus.Fields{
				"deviceID": deviceID,
				"connID":   conn.GetConnID(),
				"error":    err.Error(),
			}).Warn("更新设备心跳失败")
		}
	}
}
