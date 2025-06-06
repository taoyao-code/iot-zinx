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

	// 🔧 新增：支持一对多连接关系 - 一个连接可以承载多个设备的数据
	connIdToDeviceIdsMap sync.Map // map[uint64]map[string]bool - 连接ID -> 设备ID集合

	// 🔧 新增：主机连接映射 - 记录哪个连接是主机建立的
	masterConnectionMap sync.Map // map[uint64]string - 连接ID -> 主机设备ID

	// 并发安全保护锁 - 保护复合操作的原子性
	bindMutex sync.RWMutex
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
		time.Now().Format(constants.TimeFormatDefault),
		conn.GetConnID(),
		conn.RemoteAddr().String())
}

// OnConnectionClosed 当连接关闭时通知TCP监视器
// 🔧 修改：支持主机-分机架构，清理连接下的所有设备
func (m *TCPMonitor) OnConnectionClosed(conn ziface.IConnection) {
	// 获取连接ID和远程地址
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 记录连接关闭
	fmt.Printf("\n[%s] 连接已关闭 - ConnID: %d, 远程地址: %s\n",
		time.Now().Format(constants.TimeFormatDefault),
		connID,
		remoteAddr)

	// 使用锁保护清理操作的原子性
	m.bindMutex.Lock()
	defer m.bindMutex.Unlock()

	// 🔧 新增：检查是否为主机连接
	if masterDeviceId, isMasterConn := m.masterConnectionMap.Load(connID); isMasterConn {
		// 主机连接关闭，清理所有关联的设备
		m.handleMasterConnectionClosed(connID, masterDeviceId.(string), conn)
		return
	}

	// 原有的单设备连接关闭逻辑
	m.handleSingleDeviceConnectionClosed(connID, conn, remoteAddr)
}

// handleMasterConnectionClosed 处理主机连接关闭
func (m *TCPMonitor) handleMasterConnectionClosed(connID uint64, masterDeviceId string, conn ziface.IConnection) {
	logger.WithFields(logrus.Fields{
		"connID":         connID,
		"masterDeviceId": masterDeviceId,
	}).Info("主机连接关闭，清理所有关联设备")

	// 获取该连接下的所有设备
	deviceIds := make([]string, 0)
	if deviceSetVal, exists := m.connIdToDeviceIdsMap.Load(connID); exists {
		deviceSet := deviceSetVal.(map[string]bool)
		for deviceId := range deviceSet {
			deviceIds = append(deviceIds, deviceId)
		}
	} else {
		// 如果没有设备集合记录，至少包含主设备
		deviceIds = append(deviceIds, masterDeviceId)
	}

	// 逐个清理设备映射和状态
	for _, deviceID := range deviceIds {
		// 清理设备映射
		m.deviceIdToConnMap.Delete(deviceID)

		// 获取设备会话信息（用于处理设备组）
		sessionManager := GetSessionManager()
		if session, exists := sessionManager.GetSession(deviceID); exists {
			// 挂起设备会话
			sessionManager.SuspendSession(deviceID)

			// 🔧 处理设备组：检查ICCID下是否还有其他活跃设备
			if session.ICCID != "" {
				allDevices := sessionManager.GetAllSessionsByICCID(session.ICCID)
				activeDevices := 0
				for otherDeviceID, otherSession := range allDevices {
					if otherDeviceID != deviceID && otherSession.Status == constants.DeviceStatusOnline {
						activeDevices++
					}
				}

				logger.WithFields(logrus.Fields{
					"deviceId":      deviceID,
					"iccid":         session.ICCID,
					"activeDevices": activeDevices,
					"totalDevices":  len(allDevices),
				}).Info("设备断开连接，ICCID下仍有其他活跃设备")
			}
		}

		// 通知设备监控器设备断开连接
		deviceMonitor := GetGlobalDeviceMonitor()
		if deviceMonitor != nil {
			deviceMonitor.OnDeviceDisconnect(deviceID, conn, "master_connection_closed")
		}

		// 更新设备状态为离线
		if UpdateDeviceStatusFunc != nil {
			UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
		}

		logger.WithFields(logrus.Fields{
			"deviceId": deviceID,
			"connID":   connID,
		}).Info("设备映射已清理")
	}

	// 清理连接级别的映射
	m.connIdToDeviceIdMap.Delete(connID)
	m.connIdToDeviceIdsMap.Delete(connID)
	m.masterConnectionMap.Delete(connID)

	logger.WithFields(logrus.Fields{
		"connID":         connID,
		"masterDeviceId": masterDeviceId,
		"cleanedDevices": len(deviceIds),
	}).Info("主机连接清理完成")
}

// handleSingleDeviceConnectionClosed 处理单设备连接关闭
func (m *TCPMonitor) handleSingleDeviceConnectionClosed(connID uint64, conn ziface.IConnection, remoteAddr string) {
	// 获取关联的设备ID
	var deviceID string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
	}

	// 安全清理连接ID映射
	m.connIdToDeviceIdMap.Delete(connID)

	// 如果有关联的设备ID，进行设备相关清理
	if deviceID != "" {
		// 验证这确实是该设备的当前连接
		if currentConn, exists := m.deviceIdToConnMap.Load(deviceID); exists {
			if currentConnObj, ok := currentConn.(ziface.IConnection); ok && currentConnObj.GetConnID() == connID {
				// 确认是当前连接，才清理设备映射
				m.deviceIdToConnMap.Delete(deviceID)

				// 🔧 处理设备组中的设备断开
				sessionManager := GetSessionManager()
				if session, exists := sessionManager.GetSession(deviceID); exists {
					// 挂起设备会话
					sessionManager.SuspendSession(deviceID)

					// 检查同一ICCID下的其他设备
					if session.ICCID != "" {
						allDevices := sessionManager.GetAllSessionsByICCID(session.ICCID)
						activeDevices := 0

						for otherDeviceID, otherSession := range allDevices {
							if otherDeviceID != deviceID && otherSession.Status == constants.DeviceStatusOnline {
								activeDevices++
							}
						}

						logger.WithFields(logrus.Fields{
							"deviceId":      deviceID,
							"iccid":         session.ICCID,
							"activeDevices": activeDevices,
							"totalDevices":  len(allDevices),
						}).Info("设备断开连接，ICCID下仍有其他活跃设备")
					}
				}

				// 记录设备离线
				logger.WithFields(logrus.Fields{
					"deviceId":   deviceID,
					"connID":     connID,
					"remoteAddr": remoteAddr,
				}).Info("设备连接已关闭，清理映射关系")
			} else {
				// 这不是当前连接，可能是旧连接，只记录日志
				logger.WithFields(logrus.Fields{
					"deviceId":      deviceID,
					"closedConnID":  connID,
					"currentConnID": currentConnObj.GetConnID(),
					"remoteAddr":    remoteAddr,
				}).Info("关闭的连接不是设备当前连接，跳过设备映射清理")
				return // 不进行设备状态更新
			}
		}

		// 通知全局设备监控器设备断开连接
		deviceMonitor := GetGlobalDeviceMonitor()
		if deviceMonitor != nil {
			deviceMonitor.OnDeviceDisconnect(deviceID, conn, "connection_closed")
		}

		// 更新设备状态为离线或重连中
		if UpdateDeviceStatusFunc != nil {
			UpdateDeviceStatusFunc(deviceID, constants.DeviceStatusOffline)
		}
	}
}

// OnRawDataReceived 当接收到原始数据时调用
func (m *TCPMonitor) OnRawDataReceived(conn ziface.IConnection, data []byte) {
	if m.enabled {
		// 获取连接信息
		remoteAddr := conn.RemoteAddr().String()
		connID := conn.GetConnID()

		// 强制打印到控制台和标准输出，确保可见性
		timestamp := time.Now().Format(constants.TimeFormatDefault)

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
					"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
					"messageID":  fmt.Sprintf("0x%04X", result.MessageID),
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
		timestamp := time.Now().Format(constants.TimeFormatDefault)
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
			// 使用统一解析接口正确解析DNY协议数据
			if result, err := protocol.ParseDNYData(data); err == nil {
				// 记录详细的解析信息
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"command":    fmt.Sprintf("0x%02X", result.Command),
					"physicalID": fmt.Sprintf("0x%08X", result.PhysicalID),
					"messageID":  fmt.Sprintf("0x%04X", result.MessageID),
					"dataHex":    hex.EncodeToString(data),
				}).Info("发送DNY协议数据1")
			} else {
				fmt.Printf("解析失败: %v\n", err)
			}
		}

		fmt.Println("----------------------------------------")
	}
}

// BindDeviceIdToConnection 绑定设备ID到连接并更新在线状态
// 🔧 修改：支持主机-分机架构，一个连接可以承载多个设备的数据
func (m *TCPMonitor) BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	// 使用锁保护整个绑定操作的原子性
	m.bindMutex.Lock()
	defer m.bindMutex.Unlock()

	// 获取连接ID
	connID := conn.GetConnID()

	// 🔧 新增：判断设备类型（基于deviceId格式：前2位16进制为设备识别码）
	isMasterDevice := m.isMasterDevice(deviceId)

	if isMasterDevice {
		// 主机设备：建立主连接，负责整个设备组的通信
		m.handleMasterDeviceBinding(deviceId, conn, connID)
	} else {
		// 分机设备：通过主机连接通信，需要找到对应的主机连接
		m.handleSlaveDeviceBinding(deviceId, conn, connID)
	}
}

// isMasterDevice 判断是否为主机设备
// 主机设备的识别码为 09，分机设备为 04, 05, 06 等
func (m *TCPMonitor) isMasterDevice(deviceId string) bool {
	if len(deviceId) >= 8 {
		// deviceId格式：04A228CD -> 识别码为04
		// 主机识别码为09
		return deviceId[:2] == "09"
	}
	return false
}

// 🔧 新增：公开的主机设备判断方法
func (m *TCPMonitor) IsMasterDevice(deviceId string) bool {
	return m.isMasterDevice(deviceId)
}

// handleMasterDeviceBinding 处理主机设备绑定
func (m *TCPMonitor) handleMasterDeviceBinding(deviceId string, conn ziface.IConnection, connID uint64) {
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
		"type":     "master",
	}).Info("绑定主机设备")

	// 检查是否已有该主机的连接
	if oldConn, exists := m.deviceIdToConnMap.Load(deviceId); exists {
		if oldConnObj, ok := oldConn.(ziface.IConnection); ok && oldConnObj.GetConnID() != connID {
			// 主机重连，清理旧连接的所有设备映射
			m.cleanupMasterConnection(oldConnObj.GetConnID(), deviceId)
		}
	}

	// 建立主机设备绑定
	m.deviceIdToConnMap.Store(deviceId, conn)
	m.connIdToDeviceIdMap.Store(connID, deviceId)

	// 🔧 标记为主机连接
	m.masterConnectionMap.Store(connID, deviceId)

	// 🔧 初始化连接的设备集合
	deviceSet := make(map[string]bool)
	deviceSet[deviceId] = true
	m.connIdToDeviceIdsMap.Store(connID, deviceSet)

	// 设置连接属性
	m.setConnectionProperties(deviceId, conn)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
		"type":     "master",
	}).Info("主机设备绑定成功")
}

// handleSlaveDeviceBinding 处理分机设备绑定
func (m *TCPMonitor) handleSlaveDeviceBinding(deviceId string, conn ziface.IConnection, connID uint64) {
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
		"type":     "slave",
	}).Info("绑定分机设备")

	// 获取ICCID信息，用于记录日志
	iccid := m.getICCIDFromConnection(conn)

	// 修改：优先支持分机设备独立通信模式
	// 直接建立分机到连接的映射，不要求必须通过主机连接
	m.deviceIdToConnMap.Store(deviceId, conn)
	m.connIdToDeviceIdMap.Store(connID, deviceId)

	// 创建新的设备集合
	deviceSet := make(map[string]bool)
	deviceSet[deviceId] = true
	m.connIdToDeviceIdsMap.Store(connID, deviceSet)

	// 设置连接属性
	m.setConnectionProperties(deviceId, conn)

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   connID,
		"iccid":    iccid,
	}).Info("分机设备已成功绑定到独立连接")

	// 尝试关联主机连接（仅用于优化通信，非必须）
	// 方案1：检查当前连接是否为主机连接
	if _, isMasterConn := m.masterConnectionMap.Load(connID); isMasterConn {
		logger.WithFields(logrus.Fields{
			"deviceId": deviceId,
			"connID":   connID,
		}).Debug("分机设备使用主机连接，无需关联")
		return
	}

	// 方案2：可选地尝试关联主机连接（用于组网场景），但不要求必须关联
	if iccid != "" {
		if masterConnID := m.findMasterConnectionByICCID(iccid); masterConnID != 0 {
			if _, exists := m.getMasterConnection(masterConnID); exists {
				// 记录设备组关联关系，但不改变设备的独立通信能力
				logger.WithFields(logrus.Fields{
					"slaveDeviceId": deviceId,
					"masterConnID":  masterConnID,
					"iccid":         iccid,
				}).Info("分机设备已关联到主机连接（仅组网关系）")
			}
		}
	}
}

// addSlaveToMasterConnection 将分机添加到主机连接
func (m *TCPMonitor) addSlaveToMasterConnection(deviceId string, masterConn ziface.IConnection, masterConnID uint64, masterDeviceId string) {
	// 绑定分机到主机连接
	m.deviceIdToConnMap.Store(deviceId, masterConn)

	// 🔧 更新连接的设备集合
	if deviceSetVal, exists := m.connIdToDeviceIdsMap.Load(masterConnID); exists {
		deviceSet := deviceSetVal.(map[string]bool)
		deviceSet[deviceId] = true
		m.connIdToDeviceIdsMap.Store(masterConnID, deviceSet)
	} else {
		// 创建新的设备集合
		deviceSet := make(map[string]bool)
		deviceSet[deviceId] = true
		if masterDeviceId != "" {
			deviceSet[masterDeviceId] = true
		}
		m.connIdToDeviceIdsMap.Store(masterConnID, deviceSet)
	}

	// 设置分机设备属性（但不覆盖连接的主设备属性）
	m.setDeviceProperties(deviceId, masterConn)

	logger.WithFields(logrus.Fields{
		"slaveDeviceId":  deviceId,
		"masterConnID":   masterConnID,
		"masterDeviceId": masterDeviceId,
	}).Info("分机设备已添加到主机连接")
}

// setConnectionProperties 设置连接属性（用于主机）
func (m *TCPMonitor) setConnectionProperties(deviceId string, conn ziface.IConnection) {
	// 设置设备ID属性到连接
	conn.SetProperty(constants.PropKeyDeviceId, deviceId)

	// 设置连接状态为活跃
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 设置绑定时间
	now := time.Now()
	conn.SetProperty(constants.PropKeyLastHeartbeat, now.Unix())
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))

	// 记录设备上线日志
	logger.WithFields(logrus.Fields{
		"deviceId":   deviceId,
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
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

// setDeviceProperties 设置设备属性（用于分机，不影响连接级别属性）
func (m *TCPMonitor) setDeviceProperties(deviceId string, conn ziface.IConnection) {
	// 更新设备状态为在线
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

	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"connID":   conn.GetConnID(),
	}).Info("分机设备属性设置完成")
}

// cleanupMasterConnection 清理主机连接的所有设备映射
func (m *TCPMonitor) cleanupMasterConnection(oldConnID uint64, masterDeviceId string) {
	logger.WithFields(logrus.Fields{
		"oldConnID":      oldConnID,
		"masterDeviceId": masterDeviceId,
	}).Info("清理主机连接的所有设备映射")

	// 获取该连接下的所有设备
	if deviceSetVal, exists := m.connIdToDeviceIdsMap.Load(oldConnID); exists {
		deviceSet := deviceSetVal.(map[string]bool)

		// 清理所有设备的映射
		for deviceId := range deviceSet {
			m.deviceIdToConnMap.Delete(deviceId)
			logger.WithFields(logrus.Fields{
				"deviceId":  deviceId,
				"oldConnID": oldConnID,
			}).Debug("已清理设备映射")
		}

		// 清理连接映射
		m.connIdToDeviceIdsMap.Delete(oldConnID)
	}

	// 清理主机连接标记
	m.masterConnectionMap.Delete(oldConnID)
	m.connIdToDeviceIdMap.Delete(oldConnID)
}

// getICCIDFromConnection 从连接获取ICCID
func (m *TCPMonitor) getICCIDFromConnection(conn ziface.IConnection) string {
	if val, err := conn.GetProperty(constants.PropKeyICCID); err == nil && val != nil {
		return val.(string)
	}
	return ""
}

// findMasterConnectionByICCID 根据ICCID查找主机连接
func (m *TCPMonitor) findMasterConnectionByICCID(iccid string) uint64 {
	// 遍历所有主机连接，查找匹配的ICCID
	var foundConnID uint64 = 0
	m.masterConnectionMap.Range(func(connIDVal, masterDeviceIdVal interface{}) bool {
		connID := connIDVal.(uint64)
		if masterConn, exists := m.getMasterConnection(connID); exists {
			if connICCID := m.getICCIDFromConnection(masterConn); connICCID == iccid {
				foundConnID = connID
				return false // 停止遍历
			}
		}
		return true // 继续遍历
	})
	return foundConnID
}

// getMasterConnection 获取主机连接
func (m *TCPMonitor) getMasterConnection(connID uint64) (ziface.IConnection, bool) {
	if masterDeviceIdVal, exists := m.masterConnectionMap.Load(connID); exists {
		masterDeviceId := masterDeviceIdVal.(string)
		if connVal, exists := m.deviceIdToConnMap.Load(masterDeviceId); exists {
			if conn, ok := connVal.(ziface.IConnection); ok {
				return conn, true
			}
		}
	}
	return nil, false
}

// GetConnectionByDeviceId 根据设备ID获取连接
// 🔧 支持主从架构：分机设备返回主机连接
func (m *TCPMonitor) GetConnectionByDeviceId(deviceId string) (ziface.IConnection, bool) {
	connVal, ok := m.deviceIdToConnMap.Load(deviceId)
	if !ok {
		return nil, false
	}
	conn, ok := connVal.(ziface.IConnection)
	return conn, ok
}

// GetMasterConnectionForDevice 为设备获取主机连接信息
// 返回：主机连接、主机设备ID、是否找到
// 🔧 主从架构支持：分机设备返回主机连接，主机设备返回自身连接
func (m *TCPMonitor) GetMasterConnectionForDevice(deviceId string) (ziface.IConnection, string, bool) {
	// 如果是主机设备，直接返回自身连接
	if m.isMasterDevice(deviceId) {
		if conn, exists := m.GetConnectionByDeviceId(deviceId); exists {
			return conn, deviceId, true
		}
		return nil, "", false
	}

	// 分机设备，查找对应的主机连接
	if conn, exists := m.GetConnectionByDeviceId(deviceId); exists {
		// 分机设备已绑定，获取连接ID
		connID := conn.GetConnID()

		// 查找主机设备ID
		if masterDeviceIdVal, isMasterConn := m.masterConnectionMap.Load(connID); isMasterConn {
			masterDeviceId := masterDeviceIdVal.(string)
			return conn, masterDeviceId, true
		}
	}

	return nil, "", false
}

// GetDeviceIdByConnId 根据连接ID获取设备ID
// 🔧 实现接口要求的方法，支持主从架构
func (m *TCPMonitor) GetDeviceIdByConnId(connId uint64) (string, bool) {
	// 首先尝试从单设备映射获取
	if deviceIdVal, exists := m.connIdToDeviceIdMap.Load(connId); exists {
		if deviceId, ok := deviceIdVal.(string); ok {
			return deviceId, true
		}
	}

	// 然后尝试从主机连接映射获取（返回主机设备ID）
	if masterDeviceIdVal, exists := m.masterConnectionMap.Load(connId); exists {
		if masterDeviceId, ok := masterDeviceIdVal.(string); ok {
			return masterDeviceId, true
		}
	}

	return "", false
}

// 🔧 新增：检查设备是否为分机设备且已绑定到主机连接
func (m *TCPMonitor) IsSlaveDeviceBound(deviceId string) bool {
	if !m.isMasterDevice(deviceId) {
		// 分机设备，检查是否已绑定到某个主机连接
		if _, exists := m.deviceIdToConnMap.Load(deviceId); exists {
			return true
		}
	}
	return false
}

// 🔧 新增：获取指定连接下的所有分机设备ID列表
// 用于心跳管理和主机断开时处理分机设备
func (m *TCPMonitor) GetSlaveDevicesForConnection(connID uint64) []string {
	slaveDevices := make([]string, 0)

	// 检查是否为主机连接
	if masterDeviceId, isMasterConn := m.masterConnectionMap.Load(connID); isMasterConn {
		// 获取该连接下的所有设备
		if deviceSetVal, exists := m.connIdToDeviceIdsMap.Load(connID); exists {
			deviceSet := deviceSetVal.(map[string]bool)

			// 筛选出分机设备（排除主机设备本身）
			masterDeviceIdStr := masterDeviceId.(string)
			for deviceId := range deviceSet {
				if deviceId != masterDeviceIdStr && !m.isMasterDevice(deviceId) {
					slaveDevices = append(slaveDevices, deviceId)
				}
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"connID":       connID,
		"slaveCount":   len(slaveDevices),
		"slaveDevices": slaveDevices,
	}).Debug("获取主机连接下的分机设备列表")

	return slaveDevices
}

// UpdateLastHeartbeatTime 更新最后一次心跳时间、连接状态并更新设备状态
func (m *TCPMonitor) UpdateLastHeartbeatTime(conn ziface.IConnection) {
	// 获取当前时间
	now := time.Now()
	timestamp := now.Unix()
	timeStr := now.Format(constants.TimeFormatDefault)

	// 更新心跳时间属性
	conn.SetProperty(constants.PropKeyLastHeartbeat, timestamp)
	conn.SetProperty(constants.PropKeyLastHeartbeatStr, timeStr)
	conn.SetProperty(constants.PropKeyConnStatus, constants.ConnStatusActive)

	// 安全获取设备ID
	var deviceId string
	if val, err := conn.GetProperty(constants.PropKeyDeviceId); err == nil && val != nil {
		if id, ok := val.(string); ok {
			deviceId = id
		} else {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"type":   fmt.Sprintf("%T", val),
			}).Warn("设备ID类型不正确")
		}
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
			conn.SetProperty(constants.PropKeyLastHeartbeatStr, now.Format(constants.TimeFormatDefault))
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

		// 检查连接状态 - 只跳过已关闭的连接，保留inactive状态的连接用于心跳
		if val, err := conn.GetProperty(constants.PropKeyConnStatus); err == nil && val != nil {
			status := val.(string)
			if status == constants.ConnStatusClosed {
				logger.WithFields(logrus.Fields{
					"deviceId": deviceId,
					"status":   status,
				}).Debug("跳过已关闭连接")
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
