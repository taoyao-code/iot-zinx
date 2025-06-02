package monitor

import (
	"encoding/hex"
	"fmt"
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
	globalMonitorOnce     sync.Once
	globalMonitor         *TCPMonitor
	statusUpdateOptimizer *StatusUpdateOptimizer
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
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID := val.(string)

		// 通知全局设备监控器设备断开连接
		deviceMonitor := GetGlobalDeviceMonitor()
		if deviceMonitor != nil {
			deviceMonitor.OnDeviceDisconnect(deviceID, conn, "connection_closed")
		}

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

		// 🔧 使用统一的DNY协议检查和解析接口
		if protocol.IsDNYProtocolData(data) {
			fmt.Printf("【DNY协议】检测到DNY协议数据包\n")
			// 使用新的统一解析接口
			if result, err := protocol.ParseDNYData(data); err == nil {
				fmt.Println(result.String())

				// 记录详细的解析信息
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"command":    fmt.Sprintf("0x%02X", result.Command),
					"physicalID": result.PhysicalID,
					"messageID":  result.MessageID,
					"dataHex":    hex.EncodeToString(data),
				}).Info("接收DNY协议数据")
			} else {
				fmt.Printf("解析失败: %v\n", err)
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

		// 🔧 使用统一的DNY协议检查和解析接口
		if protocol.IsDNYProtocolData(data) {
			// 使用新的统一解析接口
			if result, err := protocol.ParseDNYData(data); err == nil {
				fmt.Println(result.String())

				// 记录详细的解析信息
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"command":    fmt.Sprintf("0x%02X", result.Command),
					"physicalID": result.PhysicalID,
					"messageID":  result.MessageID,
					"dataHex":    hex.EncodeToString(data),
				}).Info("发送DNY协议数据")
			} else {
				fmt.Printf("解析失败: %v\n", err)
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

	// 更新设备状态为在线（使用优化器）
	if statusUpdateOptimizer != nil {
		statusUpdateOptimizer.UpdateDeviceStatus(deviceId, constants.DeviceStatusOnline, "register")
	} else if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceId, constants.DeviceStatusOnline)
	}

	// 通知全局设备监控器设备已注册
	deviceMonitor := GetGlobalDeviceMonitor()
	if deviceMonitor != nil {
		deviceMonitor.OnDeviceRegistered(deviceId, conn)
	}
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
	var deviceId string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceId = val.(string)
	}

	// 只处理已注册的设备心跳
	if deviceId == "" {
		logger.WithFields(logrus.Fields{
			"connID":        conn.GetConnID(),
			"heartbeatTime": timeStr,
		}).Debug("未注册设备心跳，跳过状态更新")
		return
	}

	// 记录心跳日志
	logger.WithFields(logrus.Fields{
		"deviceId":      deviceId,
		"connID":        conn.GetConnID(),
		"heartbeatTime": timeStr,
	}).Debug("更新设备心跳时间")

	// 更新设备状态为在线（使用优化器）
	if statusUpdateOptimizer != nil {
		// 使用优化器进行状态更新，避免冗余调用
		statusUpdateOptimizer.UpdateDeviceStatus(deviceId, constants.DeviceStatusOnline, "heartbeat")
	} else if UpdateDeviceStatusFunc != nil {
		// 后备方案：直接调用原始函数
		UpdateDeviceStatusFunc(deviceId, constants.DeviceStatusOnline)
	}

	// 通知全局设备监控器设备心跳
	deviceMonitor := GetGlobalDeviceMonitor()
	if deviceMonitor != nil {
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

	// 同时初始化状态更新优化器
	if statusUpdateOptimizer == nil {
		statusUpdateOptimizer = NewStatusUpdateOptimizer(fn)
		logger.Info("设备状态更新优化器已初始化并集成到TCP监控器")
	}
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
			// 优化：避免循环调用，直接更新心跳时间属性而不触发递归状态更新
			now := time.Now()
			conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
			conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format("2006-01-02 15:04:05.000"))
		}
	} else {
		// 设备不在线，只记录状态变更
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"status":   status,
		}).Info("设备状态更新(设备不在线)")
	}

	// 调用外部提供的设备状态更新函数（使用优化器）
	if statusUpdateOptimizer != nil {
		statusUpdateOptimizer.UpdateDeviceStatus(deviceId, status, "manual")
	} else if UpdateDeviceStatusFunc != nil {
		UpdateDeviceStatusFunc(deviceId, status)
	}
}

// ForEachConnection 遍历所有设备连接
func (m *TCPMonitor) ForEachConnection(callback func(deviceId string, conn ziface.IConnection) bool) {
	// 用于跟踪需要清理的无效连接
	invalidConnections := make([]string, 0)

	// 遍历设备ID到连接的映射
	m.deviceIdToConnMap.Range(func(key, value interface{}) bool {
		deviceId, ok1 := key.(string)
		conn, ok2 := value.(ziface.IConnection)

		if !ok1 || !ok2 {
			logger.WithFields(logrus.Fields{
				"key": key,
			}).Warn("发现无效的映射关系，将清理")
			invalidConnections = append(invalidConnections, deviceId)
			return true
		}

		// 检查连接是否仍然有效
		if conn == nil || conn.GetTCPConnection() == nil {
			logger.WithFields(logrus.Fields{
				"deviceId": deviceId,
			}).Warn("发现无效连接，将从映射中移除")
			invalidConnections = append(invalidConnections, deviceId)
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
	})

	// 清理无效连接
	for _, deviceId := range invalidConnections {
		m.deviceIdToConnMap.Delete(deviceId)
		// 也需要清理反向映射
		m.connIdToDeviceIdMap.Range(func(connKey, deviceKey interface{}) bool {
			if deviceKey == deviceId {
				m.connIdToDeviceIdMap.Delete(connKey)
				return false
			}
			return true
		})
	}
}
