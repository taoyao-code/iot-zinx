package monitor

import (
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/constants"
	"github.com/bujia-iot/iot-zinx/pkg/protocol"
	"github.com/sirupsen/logrus"
)

// TCPMonitor TCP监视器
type TCPMonitor struct {
	enabled bool

	// 存储所有设备ID到连接的映射，用于消息转发
	deviceIdToConnMap   sync.Map // map[string]ziface.IConnection
	connIdToDeviceIdMap sync.Map // map[uint64]string
}

// 确保TCPMonitor实现了IConnectionMonitor接口
var _ IConnectionMonitor = (*TCPMonitor)(nil)

// 全局TCP数据监视器
var (
	globalMonitorOnce sync.Once
	globalMonitor     *TCPMonitor
)

// GetGlobalMonitor 获取全局监视器实例
func GetGlobalMonitor() *TCPMonitor {
	globalMonitorOnce.Do(func() {
		globalMonitor = &TCPMonitor{
			enabled: true,
		}
		fmt.Println("TCP数据监视器已初始化")
	})
	return globalMonitor
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
	// 获取连接ID和远程地址
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 记录连接关闭
	fmt.Printf("\n[%s] 连接已关闭 - ConnID: %d, 远程地址: %s\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		connID,
		remoteAddr)

	// 获取关联的设备ID
	deviceID := "unknown"
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)

		// 更新设备状态为离线
		if UpdateDeviceStatusFunc != nil {
			UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
		}

		// 记录设备离线
		logger.WithFields(logrus.Fields{
			"deviceId":   deviceID,
			"connID":     connID,
			"remoteAddr": remoteAddr,
		}).Info("设备连接已关闭，状态更新为离线")

		// 清理映射关系
		m.deviceIdToConnMap.Delete(deviceID)
	}

	// 清理连接ID映射
	m.connIdToDeviceIdMap.Delete(connID)
}

// OnRawDataReceived 当接收到原始数据时调用
func (m *TCPMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	if m.enabled {
		// 获取连接信息
		remoteAddr := conn.RemoteAddr().String()
		connID := conn.GetConnID()

		// 强制打印到控制台和标准输出，确保可见性
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")

		// 使用logger记录接收的数据，确保INFO级别
		logger.WithFields(logrus.Fields{
			"connID":     connID,
			"remoteAddr": remoteAddr,
			"dataLen":    len(data),
			"dataHex":    hex.EncodeToString(data),
			"timestamp":  timestamp,
		}).Info("TCP数据接收 - 原始数据包")

		// 解析DNY协议数据
		if len(data) >= 3 && data[0] == 0x44 && data[1] == 0x4E && data[2] == 0x59 {
			fmt.Printf("【DNY协议】检测到DNY协议数据包\n")
			// 如果是DNY协议数据，解析并显示详细信息
			if result := protocol.ParseDNYProtocol(data); result != "" {
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
			if result := protocol.ParseDNYProtocol(data); result != "" {
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
	conn.SetProperty(constants.PropKeyDeviceId, deviceId)

	// 更新设备状态
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceId, constants.DeviceStatusOnline)
	}

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
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())

	// 更新心跳时间（格式化字符串）
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format("2006-01-02 15:04:05"))

	// 更新连接状态
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 更新 TCP 读取超时
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		// 从配置中获取超时值，如果未配置则使用默认值60秒
		cfg := config.GetConfig().DeviceConnection
		heartbeatTimeout := time.Duration(cfg.HeartbeatTimeoutSeconds) * time.Second
		if heartbeatTimeout == 0 {
			heartbeatTimeout = 60 * time.Second // 默认60秒
		}

		if err := tcpConn.SetReadDeadline(now.Add(heartbeatTimeout)); err != nil {
			logger.WithFields(logrus.Fields{
				"error":    err.Error(),
				"connID":   conn.GetConnID(),
				"deadline": now.Add(heartbeatTimeout).Format("2006-01-02 15:04:05"),
			}).Error("设置读取超时失败")
		}
	}

	// 获取设备ID并更新在线状态
	deviceID := "unknown"
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
		if UpdateDeviceStatusFunc != nil {
			UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOnline)
		}
	}

	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceID,
		"remoteAddr":    conn.RemoteAddr().String(),
		"heartbeatTime": now.Format("2006-01-02 15:04:05"),
		"nextDeadline":  now.Add(60 * time.Second).Format("2006-01-02 15:04:05"),
		"connStatus":    constants.ConnStatusActive,
	}).Debug("已更新心跳时间")
}

// 更新设备状态的函数类型定义
type UpdateDeviceStatusFuncType = constants.UpdateDeviceStatusFuncType

// UpdateDeviceStatusFunc 更新设备状态的函数，需要外部设置
var UpdateDeviceStatusFunc UpdateDeviceStatusFuncType

// SetUpdateDeviceStatusFunc 设置更新设备状态的函数
func SetUpdateDeviceStatusFunc(fn UpdateDeviceStatusFuncType) {
	UpdateDeviceStatusFunc = fn
}

// UpdateDeviceStatus 更新设备状态
func (m *TCPMonitor) UpdateDeviceStatus(deviceId string, status string) {
	// 根据设备ID查找连接
	if conn, exists := m.GetConnectionByDeviceId(deviceId); exists {
		// 记录设备状态变更
		logger.WithFields(logrus.Fields{
			"deviceId":   deviceId,
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"status":     status,
		}).Info("设备状态更新")

		// 如果设备离线，更新连接状态
		if status == constants.DeviceStatusOffline {
			conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusInactive)
		} else if status == constants.DeviceStatusOnline {
			conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)
			// 更新最后心跳时间
			m.UpdateLastHeartbeatTime(conn)
		}
	} else {
		// 设备不在线，只记录状态变更
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"status":   status,
		}).Info("设备状态更新(设备不在线)")
	}

	// 调用外部提供的设备状态更新函数
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceId, status)
	}
}
