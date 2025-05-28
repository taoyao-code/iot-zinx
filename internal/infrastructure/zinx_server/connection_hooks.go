package zinx_server

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/sirupsen/logrus"
)

const (
	// 连接属性键
	PropKeyDeviceId      = "deviceId"      // 物理ID
	PropKeyICCID         = "iccid"         // ICCID
	PropKeyLastHeartbeat = "lastHeartbeat" // 最后一次DNY心跳时间
	PropKeyLastLink      = "lastLink"      // 最后一次"link"心跳时间
	PropKeyRemoteAddr    = "remoteAddr"    // 远程地址

	// Link心跳字符串
	LinkHeartbeat = "link"
)

// 存储所有设备ID到连接的映射，用于消息转发
var (
	// deviceIdToConnMap 物理ID到连接的映射
	deviceIdToConnMap sync.Map // map[string]ziface.IConnection

	// connIdToDeviceIdMap 连接ID到物理ID的映射
	connIdToDeviceIdMap sync.Map // map[uint64]string
)

// 初始化读取超时时间
var readDeadLine = time.Second * 3

// OnConnectionStart 当连接建立时的钩子函数
func OnConnectionStart(conn ziface.IConnection) {
	// 获取TCP连接并设置选项
	tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn)
	if !ok {
		logger.Error("Failed to get TCP connection")
		conn.Stop()
		return
	}

	// 设置TCP选项
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(time.Second * 60)
	_ = tcpConn.SetReadDeadline(time.Now().Add(readDeadLine))
	_ = tcpConn.SetNoDelay(true)

	// 记录连接信息
	remoteAddr := conn.RemoteAddr().String()
	conn.SetProperty(PropKeyRemoteAddr, remoteAddr)

	logger.WithFields(logrus.Fields{
		"remoteAddr": remoteAddr,
		"connID":     conn.GetConnID(),
	}).Info("新连接已建立")
}

// OnConnectionStop 当连接断开时的钩子函数
func OnConnectionStop(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 尝试获取设备信息并清理
	if deviceId, err := conn.GetProperty(PropKeyDeviceId); err == nil && deviceId != nil {
		deviceIdStr := deviceId.(string)
		// 清理映射
		deviceIdToConnMap.Delete(deviceIdStr)
		connIdToDeviceIdMap.Delete(connID)

		// 更新设备状态
		UpdateDeviceStatus(deviceIdStr, "offline")

		logger.WithFields(logrus.Fields{
			"deviceId":   deviceIdStr,
			"remoteAddr": remoteAddr,
			"connID":     connID,
		}).Info("设备连接断开")
	}
}

// HandlePacket 处理接收到的数据包
func HandlePacket(conn ziface.IConnection, data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// 处理link心跳
	if len(data) == 4 && string(data) == LinkHeartbeat {
		now := time.Now().Unix()
		conn.SetProperty(PropKeyLastLink, now)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
		}).Debug("收到link心跳")
		return true
	}

	// 处理DNY协议数据
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"data":       data[:3],
		}).Debug("收到DNY协议数据")
		return true
	}

	// 记录未知数据包
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    fmt.Sprintf("%X", data),
	}).Debug("收到未知数据包")
	return false
}

// BindDeviceIdToConnection 绑定设备ID到连接并更新在线状态
func BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	deviceIdToConnMap.Store(deviceId, conn)
	connIdToDeviceIdMap.Store(conn.GetConnID(), deviceId)
	conn.SetProperty(PropKeyDeviceId, deviceId)
	UpdateDeviceStatus(deviceId, "online")

	logger.WithFields(logrus.Fields{
		"deviceId":   deviceId,
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Info("Device ID bound to connection")
}

// GetConnectionByDeviceId 根据设备ID获取连接
func GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	connVal, ok := deviceIdToConnMap.Load(deviceId)
	if !ok {
		return nil, false
	}
	conn, ok := connVal.(ziface.IConnection)
	return conn, ok
}

// GetDeviceIdByConnId 根据连接ID获取设备ID
func GetDeviceIdByConnId(connId uint64) (string, bool) {
	deviceIdVal, ok := connIdToDeviceIdMap.Load(connId)
	if !ok {
		return "", false
	}
	deviceId, ok := deviceIdVal.(string)
	return deviceId, ok
}

// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间并更新设备状态
func UpdateLastHeartbeatTime(conn ziface.IConnection) {
	now := time.Now().Unix()
	conn.SetProperty(PropKeyLastHeartbeat, now)

	// 获取设备ID并更新在线状态
	if deviceId, err := conn.GetProperty(PropKeyDeviceId); err == nil && deviceId != nil {
		deviceIdStr := deviceId.(string)
		UpdateDeviceStatus(deviceIdStr, "online")
	}
}

// UpdateDeviceStatus 更新设备在线状态
func UpdateDeviceStatus(deviceId string, status string) {
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceStatusUpdate(deviceId, status)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Debug("设备状态已更新")
}
