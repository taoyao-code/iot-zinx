package zinx_server

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/common"
	"github.com/sirupsen/logrus"
)

// TCPMonitor TCP监视器，实现IConnectionMonitor接口
type TCPMonitor struct {
	enabled bool

	// 存储所有设备ID到连接的映射，用于消息转发
	deviceIdToConnMap   sync.Map // map[string]ziface.IConnection
	connIdToDeviceIdMap sync.Map // map[uint64]string
}

// 全局TCP数据监视器
var (
	globalMonitorOnce sync.Once
	globalMonitor     *TCPMonitor
)

// GetGlobalMonitor 获取全局监视器实例
func GetGlobalMonitor() common.IConnectionMonitor {
	globalMonitorOnce.Do(func() {
		globalMonitor = &TCPMonitor{
			enabled: true,
		}
		fmt.Println("TCP数据监视器已初始化")
	})
	return globalMonitor
}

// InitTCPMonitor 初始化TCP监视器（向后兼容）
func InitTCPMonitor() common.IConnectionMonitor {
	return GetGlobalMonitor()
}

// OnConnectionEstablished 当连接建立时通知TCP监视器
func (m *TCPMonitor) OnConnectionEstablished(conn ziface.IConnection) {
	// 这里调用TCP监视器的连接建立方法
	fmt.Printf("\n[%s] 连接已建立 - ConnID: %d, 远程地址: %s\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		conn.GetConnID(),
		conn.RemoteAddr().String())
}

// OnConnectionClosed 当连接关闭时通知TCP监视器
func (m *TCPMonitor) OnConnectionClosed(conn ziface.IConnection) {
	// 这里调用TCP监视器的连接关闭方法
	fmt.Printf("\n[%s] 连接已关闭 - ConnID: %d, 远程地址: %s\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		conn.GetConnID(),
		conn.RemoteAddr().String())
}

// OnRawDataReceived 当接收到原始数据时调用
func (m *TCPMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	if m.enabled {
		// 获取连接信息
		remoteAddr := conn.RemoteAddr().String()
		connID := conn.GetConnID()

		// 打印数据日志
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		fmt.Printf("\n[%s] 接收数据 - ConnID: %d, 远程地址: %s\n", timestamp, connID, remoteAddr)
		fmt.Printf("数据(HEX): %s\n", hex.EncodeToString(data))

		// 使用logger记录接收的数据，确保INFO级别
		logger.WithFields(logrus.Fields{
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"dataLen":    len(data),
			"dataHex":    hex.EncodeToString(data),
			"timestamp":  timestamp,
		}).Info("接收数据 - read buffer")

		// 解析DNY协议数据
		if len(data) >= 3 && data[0] == 0x44 && data[1] == 0x4E && data[2] == 0x59 {
			// 如果是DNY协议数据，解析并显示详细信息
			if result := ParseDNYProtocol(data); result != "" {
				fmt.Println(result)

				// 解析命令字段进行更详细的记录
				if len(data) >= 12 {
					command := data[11]
					// 解析物理ID（小端模式）
					physicalID := uint32(0)
					if len(data) >= 9 {
						physicalID = uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24
					}
					// 解析消息ID（小端模式）
					messageID := uint16(0)
					if len(data) >= 11 {
						messageID = uint16(data[9]) | uint16(data[10])<<8
					}

					logger.WithFields(logrus.Fields{
						"connID":     connID,
						"command":    fmt.Sprintf("0x%02X", command),
						"physicalID": physicalID,
						"messageID":  messageID,
						"dataHex":    hex.EncodeToString(data),
					}).Info("接收DNY协议数据")
				}
			}
		}

		fmt.Println("----------------------------------------")
	}
}

// OnRawDataSent 当发送原始数据时调用
func (m *TCPMonitor) OnRawDataSent(conn ziface.IConnection, data []byte) {
	if m.enabled {
		// 获取连接信息
		remoteAddr := conn.RemoteAddr().String()
		connID := conn.GetConnID()

		// 打印数据日志
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		fmt.Printf("\n[%s] 发送数据 - ConnID: %d, 远程地址: %s\n", timestamp, connID, remoteAddr)
		fmt.Printf("数据(HEX): %s\n", hex.EncodeToString(data))

		// 使用logger记录发送的数据，确保INFO级别
		logger.WithFields(logrus.Fields{
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"dataLen":    len(data),
			"dataHex":    hex.EncodeToString(data),
			"timestamp":  timestamp,
		}).Info("发送数据 - write buffer")

		// 解析DNY协议数据
		if len(data) >= 3 && data[0] == 0x44 && data[1] == 0x4E && data[2] == 0x59 {
			// 如果是DNY协议数据，解析并显示详细信息
			if result := ParseDNYProtocol(data); result != "" {
				fmt.Println(result)

				// 解析命令字段进行更详细的记录
				if len(data) >= 12 {
					command := data[11]
					// 解析物理ID（小端模式）
					physicalID := uint32(0)
					if len(data) >= 9 {
						physicalID = uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24
					}
					// 解析消息ID（小端模式）
					messageID := uint16(0)
					if len(data) >= 11 {
						messageID = uint16(data[9]) | uint16(data[10])<<8
					}

					logger.WithFields(logrus.Fields{
						"connID":     connID,
						"command":    fmt.Sprintf("0x%02X", command),
						"physicalID": physicalID,
						"messageID":  messageID,
						"dataHex":    hex.EncodeToString(data),
					}).Info("发送DNY协议数据")
				}
			}
		}

		fmt.Println("----------------------------------------")
	}
}

// BindDeviceIdToConnection 绑定设备ID到连接并更新在线状态
func (m *TCPMonitor) BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	m.deviceIdToConnMap.Store(deviceId, conn)
	m.connIdToDeviceIdMap.Store(conn.GetConnID(), deviceId)
	conn.SetProperty(common.PropKeyDeviceId, deviceId)
	m.UpdateDeviceStatus(deviceId, "online")

	logger.WithFields(logrus.Fields{
		"deviceId":   deviceId,
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
	}).Info("设备ID已绑定到连接")
}

// GetConnectionByDeviceId 根据设备ID获取连接
func (m *TCPMonitor) GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	connVal, ok := m.deviceIdToConnMap.Load(deviceId)
	if !ok {
		return nil, false
	}
	conn, ok := connVal.(ziface.IConnection)
	return conn, ok
}

// GetDeviceIdByConnId 根据连接ID获取设备ID
func (m *TCPMonitor) GetDeviceIdByConnId(connId uint64) (string, bool) {
	deviceIdVal, ok := m.connIdToDeviceIdMap.Load(connId)
	if !ok {
		return "", false
	}
	deviceId, ok := deviceIdVal.(string)
	return deviceId, ok
}

// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态
func (m *TCPMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	now := time.Now()

	// 更新心跳时间（时间戳）
	conn.SetProperty(common.PropKeyLastHeartbeat, now.Unix())

	// 更新心跳时间（格式化字符串）
	conn.SetProperty(common.PropKeyLastHeartbeatStr, now.Format("2006-01-02 15:04:05"))

	// 更新连接状态
	conn.SetProperty(common.PropKeyConnStatus, common.ConnStatusActive)

	// 更新 TCP 读取超时
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		// 使用common包中定义的超时常量
		if err := tcpConn.SetReadDeadline(now.Add(common.TCPReadDeadLine)); err != nil {
			logger.WithFields(logrus.Fields{
				"error":    err.Error(),
				"connID":   conn.GetConnID(),
				"deadline": now.Add(common.TCPReadDeadLine).Format("2006-01-02 15:04:05"),
			}).Error("设置读取超时失败")
		}
	}

	// 获取设备ID并更新在线状态
	deviceID := "unknown"
	if val, err := conn.GetProperty(common.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
		m.UpdateDeviceStatus(deviceID, "online")
	}

	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceID,
		"remoteAddr":    conn.RemoteAddr().String(),
		"heartbeatTime": now.Format("2006-01-02 15:04:05"),
		"nextDeadline":  now.Add(common.TCPReadDeadLine).Format("2006-01-02 15:04:05"),
		"connStatus":    common.ConnStatusActive,
	}).Debug("已更新心跳时间")
}

// UpdateDeviceStatus 更新设备在线状态
func (m *TCPMonitor) UpdateDeviceStatus(deviceId string, status string) {
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceStatusUpdate(deviceId, status)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Debug("设备状态已更新")
}
