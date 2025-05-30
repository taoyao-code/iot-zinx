package monitor

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
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

		// 获取设备监控器
		deviceMonitor := NewDeviceMonitor(func(fn func(deviceId string, conn ziface.IConnection) bool) {
			m.deviceIdToConnMap.Range(func(key, value interface{}) bool {
				return fn(key.(string), value.(ziface.IConnection))
			})
		})

		// 通知设备监控器设备断开连接
		deviceMonitor.OnDeviceDisconnect(deviceID, conn, "connection_closed")

		// 更新设备状态为离线或重连中
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
	// 获取连接ID
	connID := conn.GetConnID()

	// 检查之前的映射关系
	oldConn, exists := m.deviceIdToConnMap.Load(deviceId)

	// 如果该设备已有连接，先处理原连接（可能是重连）
	if exists && oldConn != nil {
		oldConnObj := oldConn.(ziface.IConnection)
		oldConnID := oldConnObj.GetConnID()

		if oldConnID != connID {
			// 不同的连接，说明设备可能重连
			logger.WithFields(logrus.Fields{
				"deviceId":  deviceId,
				"oldConnID": oldConnID,
				"newConnID": connID,
			}).Info("设备更换连接，可能是重连")

			// 移除旧连接的映射（避免资源泄漏）
			m.connIdToDeviceIdMap.Delete(oldConnID)

			// 尝试关闭旧连接（如果还没关闭）
			oldConnObj.Stop()
		}
	}

	// 更新双向映射
	m.deviceIdToConnMap.Store(deviceId, conn)
	m.connIdToDeviceIdMap.Store(connID, deviceId)

	// 设置设备ID属性到连接
	conn.SetProperty(constants.PropKeyDeviceId, deviceId)

	// 设置连接状态为活跃
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 记录设备上线日志
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
	}).Info("设备连接绑定成功")

	// 更新设备状态为在线
	if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceId, constants.DeviceStatusOnline)
	}

	// 通知设备监控器设备已注册
	deviceMonitor := NewDeviceMonitor(func(fn func(deviceId string, conn ziface.IConnection) bool) {
		m.deviceIdToConnMap.Range(func(key, value interface{}) bool {
			return fn(key.(string), value.(ziface.IConnection))
		})
	})

	deviceMonitor.OnDeviceRegistered(deviceId, conn)
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

// UpdateLastHeartbeatTime 更新最后一次心跳时间、连接状态并更新设备状态
func (m *TCPMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	// 获取当前时间
	now := time.Now()
	timestamp := now.Unix()
	timeStr := now.Format("2006-01-02 15:04:05.000")

	// 更新心跳时间属性
	conn.SetProperty(constants.PropKeyLastHeartbeat, timestamp)
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, timeStr)
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 获取设备ID
	deviceId := "unknown"
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceId = val.(string)
	}

	// 记录心跳日志
	logger.WithFields(logrus.Fields{
		"deviceId":      deviceId,
		"connID":        conn.GetConnID(),
		"heartbeatTime": timeStr,
	}).Debug("更新设备心跳时间")

	// 更新设备状态为在线
	if UpdateDeviceStatusFunc != nil && deviceId != "unknown" {
		UpdateDeviceStatusFunc(deviceId, constants.DeviceStatusOnline)
	}

	// 通知设备监控器设备心跳
	if deviceId != "unknown" {
		deviceMonitor := NewDeviceMonitor(func(fn func(deviceId string, conn ziface.IConnection) bool) {
			m.deviceIdToConnMap.Range(func(key, value interface{}) bool {
				return fn(key.(string), value.(ziface.IConnection))
			})
		})

		deviceMonitor.OnDeviceHeartbeat(deviceId, conn)
	}
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

// ForEachConnection 遍历所有设备连接
func (m *TCPMonitor) ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool) {
	// 遍历设备ID到连接的映射
	m.deviceIdToConnMap.Range(func(key, value interface{}) bool {
		deviceId, ok1 := key.(string)
		conn, ok2 := value.(ziface.IConnection)

		if ok1 && ok2 {
			// 忽略临时连接
			if strings.HasPrefix(deviceId, "TempID-") {
				return true
			}

			// 检查连接是否仍然有效
			if conn == nil || conn.GetTCPConnection() == nil {
				logger.WithFields(logrus.Fields{
					"deviceId": deviceId,
				}).Warn("发现无效连接，将从映射中移除")
				m.deviceIdToConnMap.Delete(deviceId)
				return true
			}

			// 检查连接状态
			if val, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && val != nil {
				status := val.(string)
				if status == constants.ConnStatusClosed || status == constants.ConnStatusInactive {
					logger.WithFields(logrus.Fields{
						"deviceId": deviceId,
						"status":   status,
					}).Debug("跳过非活跃连接")
					return true
				}
			}

			// 执行回调函数
			return callback(deviceId, conn)
		}
		return true
	})
}
