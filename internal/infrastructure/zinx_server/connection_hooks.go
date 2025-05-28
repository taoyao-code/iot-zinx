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
	PropKeyConnStatus    = "connStatus"    // 连接状态

	// 连接状态
	ConnStatusActive   = "active"   // 活跃
	ConnStatusInactive = "inactive" // 不活跃
	ConnStatusClosed   = "closed"   // 已关闭

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

// 初始化超时和心跳配置
var (
	readDeadLine    = time.Second * 60 // 增加读取超时时间到60秒
	keepAlivePeriod = time.Second * 30 // 减少keepalive间隔到30秒
)

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
	if err := tcpConn.SetKeepAlive(true); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Failed to set TCP keepalive")
	}
	if err := tcpConn.SetKeepAlivePeriod(keepAlivePeriod); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Failed to set TCP keepalive period")
	}
	if err := tcpConn.SetReadDeadline(time.Now().Add(readDeadLine)); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Failed to set TCP read deadline")
	}
	if err := tcpConn.SetNoDelay(true); err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Failed to set TCP nodelay")
	}

	// 记录连接信息
	remoteAddr := conn.RemoteAddr().String()
	conn.SetProperty(PropKeyRemoteAddr, remoteAddr)
	conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

	logger.WithFields(logrus.Fields{
		"remoteAddr": remoteAddr,
		"connID":     conn.GetConnID(),
	}).Info("新连接已建立")
}

// OnConnectionStop 当连接断开时的钩子函数
func OnConnectionStop(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 更新连接状态
	conn.SetProperty(PropKeyConnStatus, ConnStatusClosed)

	// 获取最后的心跳时间
	var lastHeartbeat int64
	if val, err := conn.GetProperty(PropKeyLastHeartbeat); err == nil && val != nil {
		lastHeartbeat = val.(int64)
	}

	// 尝试获取设备信息并清理
	if deviceId, err := conn.GetProperty(PropKeyDeviceId); err == nil && deviceId != nil {
		deviceIdStr := deviceId.(string)
		// 清理映射
		deviceIdToConnMap.Delete(deviceIdStr)
		connIdToDeviceIdMap.Delete(connID)

		// 更新设备状态
		UpdateDeviceStatus(deviceIdStr, "offline")

		logger.WithFields(logrus.Fields{
			"deviceId":       deviceIdStr,
			"remoteAddr":     remoteAddr,
			"connID":         connID,
			"lastHeartbeat":  time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
			"timeSinceHeart": time.Since(time.Unix(lastHeartbeat, 0)).Seconds(),
		}).Info("设备连接断开")
	} else {
		logger.WithFields(logrus.Fields{
			"remoteAddr":     remoteAddr,
			"connID":         connID,
			"lastHeartbeat":  time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
			"timeSinceHeart": time.Since(time.Unix(lastHeartbeat, 0)).Seconds(),
		}).Info("未知设备连接断开")
	}
}

// HandlePacket 处理接收到的数据包
func HandlePacket(conn ziface.IConnection, data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// 更新读取超时时间和处理错误
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(time.Now().Add(readDeadLine)); err != nil {
			conn.SetProperty(PropKeyConnStatus, ConnStatusInactive)

			// 获取最后的心跳时间
			var lastHeartbeat int64
			if val, err := conn.GetProperty(PropKeyLastHeartbeat); err == nil && val != nil {
				lastHeartbeat = val.(int64)
			}

			logger.WithFields(logrus.Fields{
				"error":          err.Error(),
				"connID":         conn.GetConnID(),
				"remoteAddr":     conn.RemoteAddr().String(),
				"lastHeartbeat":  time.Unix(lastHeartbeat, 0).Format("2006-01-02 15:04:05"),
				"timeSinceHeart": time.Since(time.Unix(lastHeartbeat, 0)).Seconds(),
			}).Error("Failed to update TCP read deadline")
			return false
		}
	}

	// 尝试解析为ICCID (20字节ASCII数字字符串)
	if len(data) == 20 {
		// 检查是否都是ASCII数字字符
		if isValidICCIDBytes(data) {
			iccidStr := string(data)
			conn.SetProperty(PropKeyICCID, iccidStr)

			// 将ICCID作为设备ID进行绑定
			BindDeviceIdToConnection(iccidStr, conn)

			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"iccid":  iccidStr,
			}).Info("收到ICCID并绑定设备")
			return true
		}
	}

	// 处理link心跳
	if len(data) == 4 && string(data) == LinkHeartbeat {
		now := time.Now().Unix()
		conn.SetProperty(PropKeyLastLink, now)
		conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
		}).Debug("收到link心跳")
		return true
	}

	// 处理DNY协议数据
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		// 更新心跳时间和连接状态
		UpdateLastHeartbeatTime(conn)
		conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"dataHex":    fmt.Sprintf("%X", data),
		}).Info("收到DNY协议数据")

		// 处理DNY协议数据
		return handleDNYProtocol(conn, data)
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

// isValidICCIDBytes 验证字节数组是否为有效的ICCID格式
func isValidICCIDBytes(data []byte) bool {
	// ICCID长度必须为20字节
	if len(data) != 20 {
		return false
	}

	// 检查每个字节是否为ASCII数字字符
	for _, b := range data {
		if b < '0' || b > '9' {
			return false
		}
	}

	return true
}

// isValidICCID 验证是否为有效的ICCID格式
func isValidICCID(str string) bool {
	// ICCID长度必须为20字符
	if len(str) != 20 {
		return false
	}

	// 检查每个字符是否为ASCII格式的数字
	for _, c := range str {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// UpdateLastHeartbeatTime 更新最后一次DNY心跳时间、连接状态并更新设备状态
func UpdateLastHeartbeatTime(conn ziface.IConnection) {
	now := time.Now().Unix()
	conn.SetProperty(PropKeyLastHeartbeat, now)

	// 更新连接状态
	conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

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

// handleDNYProtocol 处理DNY协议数据
func handleDNYProtocol(conn ziface.IConnection, data []byte) bool {
	// 根据文档中的DNY协议格式进行解析
	// DNY协议格式：包头(3字节) + 长度(2字节) + 物理ID(4字节) + 消息ID(2字节) + 命令(1字节) + 数据(n字节) + 校验(2字节)

	if len(data) < 14 { // 最小包长度：3+2+4+2+1+0+2 = 14
		logger.WithFields(logrus.Fields{
			"connID":  conn.GetConnID(),
			"dataLen": len(data),
		}).Warn("DNY协议数据包长度不足")
		return false
	}

	// 解析包头
	if string(data[:3]) != "DNY" {
		return false
	}

	// 解析长度（小端模式）
	length := uint16(data[3]) | uint16(data[4])<<8
	expectedLength := len(data) - 5 // 总长度减去包头和长度字段

	if int(length) != expectedLength {
		logger.WithFields(logrus.Fields{
			"connID":         conn.GetConnID(),
			"expectedLength": length,
			"actualLength":   expectedLength,
		}).Warn("DNY协议数据包长度不匹配")
		return false
	}

	// 解析物理ID（小端模式）
	physicalID := uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24

	// 解析消息ID（小端模式）
	messageID := uint16(data[9]) | uint16(data[10])<<8

	// 解析命令
	command := data[11]

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
	}).Info("解析DNY协议数据")

	// 根据命令类型进行处理
	switch command {
	case 0x01: // 设备心跳包
		return handleDeviceHeartbeat(conn, data, physicalID, messageID)
	case 0x11: // 主机状态心跳包
		return handleHostHeartbeat(conn, data, physicalID, messageID)
	case 0x12: // 主机获取服务器时间
		return handleGetServerTime(conn, data, physicalID, messageID)
	case 0x20: // 设备注册包
		return handleDeviceRegister(conn, data, physicalID, messageID)
	case 0x21: // 设备状态包
		return handleDeviceStatus(conn, data, physicalID, messageID)
	default:
		logger.WithFields(logrus.Fields{
			"connID":  conn.GetConnID(),
			"command": fmt.Sprintf("0x%02X", command),
		}).Debug("未知DNY协议命令")
	}

	return true
}

// handleDeviceHeartbeat 处理设备心跳包
func handleDeviceHeartbeat(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Debug("处理设备心跳包")

	// 发送心跳应答
	return sendHeartbeatResponse(conn, physicalID, messageID, 0x01)
}

// handleHostHeartbeat 处理主机状态心跳包
func handleHostHeartbeat(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Info("处理主机状态心跳包")

	// 主机状态心跳包无需应答
	return true
}

// handleGetServerTime 处理获取服务器时间请求
func handleGetServerTime(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Info("处理获取服务器时间请求")

	// 发送服务器时间应答
	return sendServerTimeResponse(conn, physicalID, messageID)
}

// handleDeviceRegister 处理设备注册包
func handleDeviceRegister(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Info("处理设备注册包")

	// 将物理ID作为设备ID进行绑定
	deviceIdStr := fmt.Sprintf("%d", physicalID)
	BindDeviceIdToConnection(deviceIdStr, conn)

	// 发送注册应答
	return sendRegisterResponse(conn, physicalID, messageID)
}

// handleDeviceStatus 处理设备状态包
func handleDeviceStatus(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Info("处理设备状态包")

	// 设备状态包无需应答
	return true
}

// sendHeartbeatResponse 发送心跳应答
func sendHeartbeatResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8) bool {
	// 构造应答数据包
	response := make([]byte, 0, 16)

	// 包头
	response = append(response, 'D', 'N', 'Y')

	// 长度（固定10字节：物理ID+消息ID+命令+应答+校验）
	response = append(response, 0x0A, 0x00)

	// 物理ID（小端模式）
	response = append(response, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	response = append(response, byte(messageID), byte(messageID>>8))

	// 命令
	response = append(response, command)

	// 应答（0=成功）
	response = append(response, 0x00)

	// 计算校验和
	checksum := calculateChecksum(response)
	response = append(response, byte(checksum), byte(checksum>>8))

	// 发送应答
	if err := conn.SendMsg(0, response); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("发送心跳应答失败")
		return false
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Debug("已发送心跳应答")

	return true
}

// sendServerTimeResponse 发送服务器时间应答
func sendServerTimeResponse(conn ziface.IConnection, physicalID uint32, messageID uint16) bool {
	// 构造应答数据包
	response := make([]byte, 0, 20)

	// 包头
	response = append(response, 'D', 'N', 'Y')

	// 长度（固定13字节：物理ID+消息ID+命令+时间戳+校验）
	response = append(response, 0x0D, 0x00)

	// 物理ID（小端模式）
	response = append(response, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	response = append(response, byte(messageID), byte(messageID>>8))

	// 命令
	response = append(response, 0x12)

	// 当前时间戳（小端模式）
	timestamp := uint32(time.Now().Unix())
	response = append(response, byte(timestamp), byte(timestamp>>8), byte(timestamp>>16), byte(timestamp>>24))

	// 计算校验和
	checksum := calculateChecksum(response)
	response = append(response, byte(checksum), byte(checksum>>8))

	// 发送应答
	if err := conn.SendMsg(0, response); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("发送服务器时间应答失败")
		return false
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"timestamp":  timestamp,
	}).Info("已发送服务器时间应答")

	return true
}

// sendRegisterResponse 发送注册应答
func sendRegisterResponse(conn ziface.IConnection, physicalID uint32, messageID uint16) bool {
	// 构造应答数据包
	response := make([]byte, 0, 16)

	// 包头
	response = append(response, 'D', 'N', 'Y')

	// 长度（固定10字节：物理ID+消息ID+命令+应答+校验）
	response = append(response, 0x0A, 0x00)

	// 物理ID（小端模式）
	response = append(response, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	response = append(response, byte(messageID), byte(messageID>>8))

	// 命令
	response = append(response, 0x20)

	// 应答（0=成功）
	response = append(response, 0x00)

	// 计算校验和
	checksum := calculateChecksum(response)
	response = append(response, byte(checksum), byte(checksum>>8))

	// 发送应答
	if err := conn.SendMsg(0, response); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": conn.GetConnID(),
			"error":  err.Error(),
		}).Error("发送注册应答失败")
		return false
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
	}).Info("已发送注册应答")

	return true
}

// calculateChecksum 计算校验和
func calculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}
