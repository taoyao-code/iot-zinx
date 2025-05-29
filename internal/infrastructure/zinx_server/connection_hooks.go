package zinx_server

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/zinx_server/common"
	"github.com/sirupsen/logrus"
)

const (
	// 连接属性键
	PropKeyDeviceId         = common.PropKeyDeviceId
	PropKeyICCID            = common.PropKeyICCID
	PropKeyLastHeartbeat    = common.PropKeyLastHeartbeat
	PropKeyLastHeartbeatStr = common.PropKeyLastHeartbeatStr
	PropKeyLastLink         = common.PropKeyLastLink
	PropKeyRemoteAddr       = common.PropKeyRemoteAddr
	PropKeyConnStatus       = common.PropKeyConnStatus

	// 连接状态
	ConnStatusActive   = common.ConnStatusActive
	ConnStatusInactive = common.ConnStatusInactive
	ConnStatusClosed   = common.ConnStatusClosed

	// Link心跳字符串
	LinkHeartbeat = common.LinkHeartbeat
)

// 存储所有设备ID到连接的映射，用于消息转发
var (
	// deviceIdToConnMap 物理ID到连接的映射
	deviceIdToConnMap sync.Map // map[string]ziface.IConnection

	// connIdToDeviceIdMap 连接ID到物理ID的映射
	connIdToDeviceIdMap sync.Map // map[uint64]string
)

// 使用common包中定义的超时常量
var (
	readDeadLine    = common.TCPReadDeadLine    // TCP读取超时时间
	writeDeadLine   = common.TCPReadDeadLine    // TCP写入超时时间
	keepAlivePeriod = common.TCPKeepAlivePeriod // TCP keepalive间隔
)

// OnConnectionStart 当连接建立时的钩子函数
// 按照 Zinx 生命周期最佳实践，在连接建立时设置 TCP 参数和连接属性
func OnConnectionStart(conn ziface.IConnection) {
	// 获取TCP连接并设置选项
	tcpConn := conn.GetConnection()
	tcpConn.SetReadDeadline(time.Now().Add(readDeadLine))
	tcpConn.SetWriteDeadline(time.Now().Add(writeDeadLine))

	// 记录连接信息
	remoteAddr := conn.RemoteAddr().String()
	conn.SetProperty(PropKeyRemoteAddr, remoteAddr)
	conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

	logger.WithFields(logrus.Fields{
		"remoteAddr": remoteAddr,
		"connID":     conn.GetConnID(),
	}).Info("新连接已建立")

	// 通知TCP监视器连接已建立
	GetGlobalMonitor().OnConnectionEstablished(conn)

	// 移除直接读取TCP连接的逻辑，让Zinx框架通过正常的数据流处理所有数据
	// 这样可以确保 DNYPacket.GetHeadLen() 和 Unpack() 方法被正确调用
	// ICCID和link心跳等非DNY数据将通过 NonDNYDataHandler 处理
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": remoteAddr,
	}).Info("连接建立完成，等待Zinx框架处理数据流")
}

// 移除自定义数据流处理函数，因为 Zinx 框架已经通过其内部机制处理数据流
// 数据会通过 Packet.Unpack 方法解析，然后路由到相应的处理器

// OnConnectionStop 当连接断开时的钩子函数
func OnConnectionStop(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 通知TCP监视器连接已断开
	GetGlobalMonitor().OnConnectionClosed(conn)

	// 更新连接状态
	conn.SetProperty(PropKeyConnStatus, ConnStatusClosed)

	// 获取最后的心跳时间（优先使用格式化的字符串）
	var lastHeartbeatStr string
	var timeSinceHeart float64

	if val, err := conn.GetProperty(PropKeyLastHeartbeatStr); err == nil && val != nil {
		lastHeartbeatStr = val.(string)
	} else {
		// 降级使用时间戳
		if val, err := conn.GetProperty(PropKeyLastHeartbeat); err == nil && val != nil {
			if ts, ok := val.(int64); ok {
				lastHeartbeatStr = time.Unix(ts, 0).Format("2006-01-02 15:04:05")
				timeSinceHeart = time.Since(time.Unix(ts, 0)).Seconds()
			} else {
				lastHeartbeatStr = "unknown"
			}
		} else {
			lastHeartbeatStr = "never"
		}
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
			"lastHeartbeat":  lastHeartbeatStr,
			"timeSinceHeart": timeSinceHeart,
			"connStatus":     ConnStatusClosed,
		}).Info("设备连接断开")
	} else {
		logger.WithFields(logrus.Fields{
			"remoteAddr":     remoteAddr,
			"connID":         connID,
			"lastHeartbeat":  lastHeartbeatStr,
			"timeSinceHeart": timeSinceHeart,
			"connStatus":     ConnStatusClosed,
		}).Info("未知设备连接断开")
	}
}

// HandlePacket 处理接收到的数据包
func HandlePacket(conn ziface.IConnection, data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// 使用logger强制输出，确保一定会被看到
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// 同时使用logger和fmt输出确保可见
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    hex.EncodeToString(data),
		"timestamp":  timestamp,
	}).Error("HandlePacket被调用 - 数据包处理开始") // 使用ERROR级别确保输出

	// 强制输出接收数据信息到控制台
	fmt.Printf("\n========== HandlePacket被调用 ==========\n")
	fmt.Printf("[%s] 处理数据包 - ConnID: %d, 远程地址: %s\n",
		timestamp, conn.GetConnID(), conn.RemoteAddr().String())
	fmt.Printf("数据长度: %d 字节\n", len(data))
	fmt.Printf("数据(HEX): %X\n", data)
	fmt.Printf("数据(ASCII): %s\n", string(data))
	fmt.Printf("==========================================\n")
	os.Stdout.Sync()

	// 通知TCP监视器收到数据
	GetGlobalMonitor().OnRawDataReceived(conn, data)

	// 检查心跳状态
	now := time.Now()
	var lastHeartbeatStr string
	var timeSinceHeart float64

	// 优先获取格式化的时间字符串
	if val, err := conn.GetProperty(PropKeyLastHeartbeatStr); err == nil && val != nil {
		lastHeartbeatStr = val.(string)
	} else {
		// 降级使用时间戳
		if val, err := conn.GetProperty(PropKeyLastHeartbeat); err == nil && val != nil {
			if ts, ok := val.(int64); ok {
				lastHeartbeatStr = time.Unix(ts, 0).Format("2006-01-02 15:04:05")
				timeSinceHeart = now.Sub(time.Unix(ts, 0)).Seconds()
			}
		}
	}

	// 更新读取超时时间
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		deadline := now.Add(readDeadLine)
		if err := tcpConn.SetReadDeadline(deadline); err != nil {
			conn.SetProperty(PropKeyConnStatus, ConnStatusInactive)

			logger.WithFields(logrus.Fields{
				"error":          err.Error(),
				"connID":         conn.GetConnID(),
				"remoteAddr":     conn.RemoteAddr().String(),
				"lastHeartbeat":  lastHeartbeatStr,
				"timeSinceHeart": timeSinceHeart,
				"deadline":       deadline.Format("2006-01-02 15:04:05"),
				"connStatus":     ConnStatusInactive,
			}).Error("设置 TCP 读取超时失败")
			return false
		}

		logger.WithFields(logrus.Fields{
			"connID":         conn.GetConnID(),
			"lastHeartbeat":  lastHeartbeatStr,
			"timeSinceHeart": timeSinceHeart,
			"deadline":       deadline.Format("2006-01-02 15:04:05"),
			"connStatus":     ConnStatusActive,
		}).Debug("更新读取超时时间成功")
	}

	// 处理十六进制编码的数据
	if isHexEncodedData(data) {
		// 记录详细的十六进制编码数据
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"dataHex":    fmt.Sprintf("%X", data),
			"dataStr":    string(data),
		}).Debug("收到十六进制编码数据")

		// 解码十六进制字符串
		decoded, err := hex.DecodeString(string(data))
		if err != nil {
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"remoteAddr": conn.RemoteAddr().String(),
				"error":      err.Error(),
				"dataHex":    fmt.Sprintf("%X", data),
			}).Error("十六进制解码失败")
			return false
		}

		// 记录解码后的数据
		logger.WithFields(logrus.Fields{
			"connID":        conn.GetConnID(),
			"remoteAddr":    conn.RemoteAddr().String(),
			"decodedLen":    len(decoded),
			"decodedHex":    fmt.Sprintf("%X", decoded),
			"isDNYProtocol": len(decoded) >= 3 && string(decoded[:3]) == "DNY",
		}).Debug("解码十六进制数据完成")

		// 递归处理解码后的数据
		return HandlePacket(conn, decoded)
	}

	// 记录原始数据的十六进制表示
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"dataLen":    len(data),
		"dataHex":    fmt.Sprintf("%X", data),
	}).Debug("处理接收到的数据包")

	// 尝试解析为ICCID (20字节ASCII数字字符串)
	if len(data) == 20 {
		// 检查是否都是ASCII数字字符
		if isValidICCIDBytes(data) {
			iccidStr := string(data)
			conn.SetProperty(PropKeyICCID, iccidStr)

			// 将ICCID作为设备ID进行绑定
			BindDeviceIdToConnection(iccidStr, conn)

			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"remoteAddr": conn.RemoteAddr().String(),
				"iccid":      iccidStr,
			}).Info("收到ICCID并绑定设备")
			return true
		}
	}

	// 处理link心跳
	if len(data) == 4 && string(data) == LinkHeartbeat {
		now := time.Now().Unix()
		conn.SetProperty(PropKeyLastLink, now)
		conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

		// 同时更新通用心跳时间，确保读取超时正确重置
		UpdateLastHeartbeatTime(conn)

		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"remoteAddr": conn.RemoteAddr().String(),
			"dataLen":    len(data),
			"timestamp":  now,
			"timeStr":    time.Unix(now, 0).Format("2006-01-02 15:04:05"),
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
		}).Debug("收到DNY协议数据")

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
	}).Info("设备ID已绑定到连接")
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

// isHexEncodedData 检查数据是否为十六进制编码的字符串
func isHexEncodedData(data []byte) bool {
	// 特殊情况处理：很短的数据通常不是十六进制编码
	// "link"心跳等短字符串应该排除
	if len(data) < 6 {
		return false
	}

	// 如果数据以"DNY"开头，不认为是十六进制编码
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		return false
	}

	// 必须是偶数长度且长度大于0
	if len(data) == 0 || len(data)%2 != 0 {
		return false
	}

	// 检查是否都是ASCII十六进制字符
	for _, b := range data {
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'F') || (b >= 'a' && b <= 'f')) {
			return false
		}
	}

	// 尝试将数据解码为十六进制，验证其有效性
	if decoded, err := hex.DecodeString(string(data)); err != nil || len(decoded) == 0 {
		return false
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
	now := time.Now()

	// 更新心跳时间（时间戳）
	conn.SetProperty(PropKeyLastHeartbeat, now.Unix())

	// 更新心跳时间（格式化字符串）
	conn.SetProperty(PropKeyLastHeartbeatStr, now.Format("2006-01-02 15:04:05"))

	// 更新连接状态
	conn.SetProperty(PropKeyConnStatus, ConnStatusActive)

	// 更新 TCP 读取超时
	if tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn); ok {
		if err := tcpConn.SetReadDeadline(now.Add(readDeadLine)); err != nil {
			logger.WithFields(logrus.Fields{
				"error":    err.Error(),
				"connID":   conn.GetConnID(),
				"deadline": now.Add(readDeadLine).Format("2006-01-02 15:04:05"),
			}).Error("设置读取超时失败")
		}
	}

	// 获取设备ID并更新在线状态
	deviceID := "unknown"
	if val, err := conn.GetProperty(PropKeyDeviceId); err == nil && val != nil {
		deviceID = val.(string)
		UpdateDeviceStatus(deviceID, "online")
	}

	logger.WithFields(logrus.Fields{
		"connID":        conn.GetConnID(),
		"deviceId":      deviceID,
		"remoteAddr":    conn.RemoteAddr().String(),
		"heartbeatTime": now.Format("2006-01-02 15:04:05"),
		"nextDeadline":  now.Add(readDeadLine).Format("2006-01-02 15:04:05"),
		"connStatus":    ConnStatusActive,
	}).Debug("更新心跳时间成功")
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
			"dataHex": fmt.Sprintf("%X", data),
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
			"dataHex":        fmt.Sprintf("%X", data),
		}).Warn("DNY协议数据包长度不匹配")
		return false
	}

	// 解析物理ID（小端模式）
	physicalID := uint32(data[5]) | uint32(data[6])<<8 | uint32(data[7])<<16 | uint32(data[8])<<24

	// 解析消息ID（小端模式）
	messageID := uint16(data[9]) | uint16(data[10])<<8

	// 解析命令
	command := data[11]

	// 直接输出到控制台
	now := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Printf("\n[%s] [REQUEST_DATA] 收到请求 - ConnID: %d, Remote: %s\n",
		now, conn.GetConnID(), conn.RemoteAddr().String())
	fmt.Printf("命令: 0x%02X, 物理ID: %d, 消息ID: %d\n", command, physicalID, messageID)
	fmt.Printf("数据(HEX): %X\n", data)
	fmt.Println("---------------------")

	// 在标准输出直接打印明显的命令信息
	fmt.Printf("\n[RECEIVED_COMMAND] ConnID: %d, Command: 0x%02X, PhysicalID: %d, MessageID: %d\n",
		conn.GetConnID(), command, physicalID, messageID)
	fmt.Printf("数据(HEX): %X\n", data)
	fmt.Println("----------------------------------------")

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataLen":    len(data),
		"dataHex":    fmt.Sprintf("%X", data),
	}).Info("解析DNY协议数据")

	// 将物理ID字符串形式保存到连接属性中
	// 这有助于确保在处理所有命令时，连接都有设备ID关联
	deviceIdStr := fmt.Sprintf("%d", physicalID)
	conn.SetProperty(PropKeyDeviceId, deviceIdStr)

	// 根据命令类型进行处理
	switch command {
	case 0x01: // 设备心跳包
		return handleDeviceHeartbeat(conn, data, physicalID, messageID)
	case 0x02: // 刷卡操作
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": physicalID,
			"messageID":  messageID,
			"command":    "0x02",
			"dataHex":    fmt.Sprintf("%X", data),
		}).Info("收到刷卡操作请求，暂不处理")
		return true
	case 0x11: // 主机状态心跳包
		return handleHostHeartbeat(conn, data, physicalID, messageID)
	case 0x12, 0x22: // 主机/设备获取服务器时间 (支持0x12和0x22两种命令)
		return handleGetServerTime(conn, data, physicalID, messageID, command)
	case 0x20: // 设备注册包
		return handleDeviceRegister(conn, data, physicalID, messageID)
	case 0x21: // 设备状态包
		return handleDeviceStatus(conn, data, physicalID, messageID)
	case 0x03, 0x04, 0x05, 0x06: // 其他设备上报指令
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": physicalID,
			"messageID":  messageID,
			"command":    fmt.Sprintf("0x%02X", command),
			"dataHex":    fmt.Sprintf("%X", data),
		}).Info("收到设备上报指令，暂不处理")

		// 对上报指令发送通用应答
		responseData := []byte{0x00} // 0x00表示成功
		SendDNYResponse(conn, physicalID, messageID, command, responseData)
		return true
	default:
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"command":    fmt.Sprintf("0x%02X", command),
			"physicalID": physicalID,
			"messageID":  messageID,
			"dataHex":    fmt.Sprintf("%X", data),
		}).Info("收到未处理的DNY协议命令")

		// 对未知命令也发送一个通用应答，避免设备重复发送请求
		if command >= 0x01 && command <= 0x22 {
			// 只对可能需要响应的命令发送应答
			responseData := []byte{0x00} // 0x00表示成功
			SendDNYResponse(conn, physicalID, messageID, command, responseData)
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"command":    fmt.Sprintf("0x%02X", command),
				"physicalID": physicalID,
				"messageID":  messageID,
			}).Info("发送了通用应答")
		}
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
	return sendHeartbeatResponse(conn, physicalID, messageID, 0x11)
}

// handleGetServerTime 处理获取服务器时间请求
func handleGetServerTime(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16, command byte) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
	}).Info("处理获取服务器时间请求")

	// 发送服务器时间应答
	return sendServerTimeResponse(conn, physicalID, messageID, command)
}

// handleDeviceRegister 处理设备注册包
func handleDeviceRegister(conn ziface.IConnection, data []byte, physicalID uint32, messageID uint16) bool {
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"dataHex":    fmt.Sprintf("%X", data),
	}).Info("处理设备注册包")

	// 检查数据长度是否符合要求
	if len(data) < 14 {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalID": physicalID,
			"dataLen":    len(data),
		}).Error("设备注册数据长度不足")
		return false
	}

	// 解析设备注册数据(根据实际协议格式)
	// 示例：从数据中提取固件版本、端口数量等
	firmwareVersion := uint16(0)
	portCount := uint8(0)
	virtualID := uint8(0)
	deviceType := uint8(0)

	// 如果数据长度足够，才进行解析
	if len(data) >= 19 { // 14(基本头部) + 5(最少的注册数据)
		firmwareVersion = uint16(data[12]) | uint16(data[13])<<8
		portCount = data[14]
		virtualID = data[15]
		deviceType = data[16]
	}

	// 将物理ID转换为字符串
	deviceIdStr := fmt.Sprintf("%d", physicalID)

	// 将设备ID绑定到连接
	BindDeviceIdToConnection(deviceIdStr, conn)

	// 记录设备详细信息
	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"physicalID":      physicalID,
		"deviceIdStr":     deviceIdStr,
		"firmwareVersion": firmwareVersion,
		"portCount":       portCount,
		"virtualID":       virtualID,
		"deviceType":      deviceType,
	}).Info("设备注册成功")

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
	return sendHeartbeatResponse(conn, physicalID, messageID, 0x21)
}

// buildDNYResponsePacket 构建DNY协议响应数据包
func buildDNYResponsePacket(physicalID uint32, messageID uint16, command uint8, data []byte) []byte {
	// 计算数据段长度（物理ID + 消息ID + 命令 + 数据 + 校验）
	dataLen := 4 + 2 + 1 + len(data) + 2

	// 构建数据包
	packet := make([]byte, 0, 5+dataLen) // 包头(3) + 长度(2) + 数据段

	// 包头 "DNY"
	packet = append(packet, 'D', 'N', 'Y')

	// 长度（小端模式）
	packet = append(packet, byte(dataLen), byte(dataLen>>8))

	// 物理ID（小端模式）
	packet = append(packet, byte(physicalID), byte(physicalID>>8), byte(physicalID>>16), byte(physicalID>>24))

	// 消息ID（小端模式）
	packet = append(packet, byte(messageID), byte(messageID>>8))

	// 命令
	packet = append(packet, command)

	// 数据
	packet = append(packet, data...)

	// 计算校验和（从包头到数据的累加和）
	checksum := calculateResponseChecksum(packet)
	packet = append(packet, byte(checksum), byte(checksum>>8))

	return packet
}

// calculateResponseChecksum 计算响应数据包校验和
func calculateResponseChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// sendHeartbeatResponse 发送心跳应答
func sendHeartbeatResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command uint8) bool {
	// 构建响应数据（仅包含应答码）
	responseData := []byte{0x00} // 0x00 表示成功

	// 构建完整的DNY协议包
	packet := buildDNYResponsePacket(physicalID, messageID, command, responseData)

	// 记录要发送的响应数据包
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataHex":    fmt.Sprintf("%X", packet),
		"dataLen":    len(packet),
	}).Info("发送心跳应答数据包")

	// 使用SendBuffMsg发送完整的DNY协议包
	if err := conn.SendBuffMsg(0, packet); err != nil {
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
		"command":    fmt.Sprintf("0x%02X", command),
	}).Debug("已发送心跳应答")

	return true
}

// sendServerTimeResponse 发送服务器时间应答
func sendServerTimeResponse(conn ziface.IConnection, physicalID uint32, messageID uint16, command byte) bool {
	// 构建响应数据（当前时间戳，4字节小端序）
	timestamp := uint32(time.Now().Unix())
	responseData := make([]byte, 4)
	responseData[0] = byte(timestamp)
	responseData[1] = byte(timestamp >> 8)
	responseData[2] = byte(timestamp >> 16)
	responseData[3] = byte(timestamp >> 24)

	// 构建完整的DNY协议包，使用原始命令
	packet := buildDNYResponsePacket(physicalID, messageID, command, responseData)

	// 记录要发送的响应数据包
	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"physicalID":   physicalID,
		"messageID":    messageID,
		"command":      fmt.Sprintf("0x%02X", command),
		"timestamp":    timestamp,
		"timestampStr": time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"),
		"dataHex":      fmt.Sprintf("%X", packet),
		"dataLen":      len(packet),
	}).Info("发送服务器时间应答数据包")

	// 使用SendBuffMsg发送完整的DNY协议包
	if err := conn.SendBuffMsg(0, packet); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":       conn.GetConnID(),
			"error":        err.Error(),
			"command":      fmt.Sprintf("0x%02X", command),
			"physicalID":   physicalID,
			"messageID":    messageID,
			"timestamp":    timestamp,
			"timestampStr": time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"),
		}).Error("发送服务器时间应答失败")
		return false
	}

	logger.WithFields(logrus.Fields{
		"connID":       conn.GetConnID(),
		"physicalID":   physicalID,
		"messageID":    messageID,
		"command":      fmt.Sprintf("0x%02X", command),
		"timestamp":    timestamp,
		"timestampStr": time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05"),
	}).Info("已发送服务器时间应答")

	return true
}

// sendRegisterResponse 发送设备注册应答
func sendRegisterResponse(conn ziface.IConnection, physicalID uint32, messageID uint16) bool {
	// 构建响应数据（仅包含应答码）
	responseData := []byte{0x00} // 0x00 表示成功

	// 构建完整的DNY协议包
	packet := buildDNYResponsePacket(physicalID, messageID, 0x20, responseData)

	// 记录要发送的响应数据包
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"command":    "0x20",
		"dataHex":    fmt.Sprintf("%X", packet),
		"dataLen":    len(packet),
	}).Info("发送设备注册应答数据包")

	// 使用SendBuffMsg发送完整的DNY协议包
	if err := conn.SendBuffMsg(0, packet); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"error":      err.Error(),
			"physicalID": physicalID,
			"messageID":  messageID,
		}).Error("发送设备注册应答失败")
		return false
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": physicalID,
		"messageID":  messageID,
		"response":   "success",
	}).Info("已发送设备注册应答")

	return true
}

// SendDNYResponse 发送DNY协议响应
func SendDNYResponse(conn ziface.IConnection, physicalId uint32, messageId uint16, command uint8, data []byte) error {
	// 构建响应数据包
	packet := buildDNYResponsePacket(physicalId, messageId, command, data)

	// 日志记录发送的数据包
	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalId": fmt.Sprintf("0x%08X", physicalId),
		"messageId":  messageId,
		"command":    fmt.Sprintf("0x%02X", command),
		"dataHex":    hex.EncodeToString(packet),
		"dataLen":    len(packet),
	}).Debug("发送DNY响应数据包")

	// 将命令注册到命令管理器进行跟踪，除非是不需要回复的命令
	if needConfirmation(command) {
		GetCommandManager().RegisterCommand(conn, physicalId, messageId, command, data)
	}

	// 发送数据包
	err := conn.SendBuffMsg(0, packet)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"messageId":  messageId,
			"command":    fmt.Sprintf("0x%02X", command),
			"error":      err.Error(),
		}).Error("发送DNY响应失败")
		return err
	}

	// 通知监视器发送了原始数据
	GetGlobalMonitor().OnRawDataSent(conn, packet)

	return nil
}

// needConfirmation 判断命令是否需要确认回复
func needConfirmation(command uint8) bool {
	// 心跳类命令不需要确认
	if command == dny_protocol.CmdHeartbeat ||
		command == uint8(dny_protocol.CmdSlaveHeartbeat) ||
		command == dny_protocol.CmdMainHeartbeat {
		return false
	}

	// 查询设备状态命令需要确认
	if command == dny_protocol.CmdNetworkStatus {
		return true
	}

	// 充电控制命令需要确认
	if command == dny_protocol.CmdChargeControl {
		return true
	}

	// 其他命令根据实际需求确定是否需要确认
	return true
}

// ConnectionInfo 连接信息结构体
type ConnectionInfo struct {
	ConnID        uint64
	DeviceID      string
	ICCID         string
	LastHeartbeat int64
	RemoteAddr    string
	ConnStatus    string
}

// RangeDeviceConnections 遍历所有设备连接
func RangeDeviceConnections(fn func(deviceId string, connInfo ConnectionInfo) bool) {
	deviceIdToConnMap.Range(func(key, value interface{}) bool {
		deviceId := key.(string)
		conn := value.(ziface.IConnection)

		// 构造连接信息
		connInfo := ConnectionInfo{
			ConnID:   conn.GetConnID(),
			DeviceID: deviceId,
		}

		// 获取ICCID
		if iccidVal, err := conn.GetProperty(PropKeyICCID); err == nil && iccidVal != nil {
			connInfo.ICCID = iccidVal.(string)
		}

		// 获取最后心跳时间
		if timeVal, err := conn.GetProperty(PropKeyLastHeartbeat); err == nil && timeVal != nil {
			connInfo.LastHeartbeat = timeVal.(int64)
		}

		// 获取远程地址
		if addrVal, err := conn.GetProperty(PropKeyRemoteAddr); err == nil && addrVal != nil {
			connInfo.RemoteAddr = addrVal.(string)
		}

		// 获取连接状态
		if statusVal, err := conn.GetProperty(PropKeyConnStatus); err == nil && statusVal != nil {
			connInfo.ConnStatus = statusVal.(string)
		}

		return fn(deviceId, connInfo)
	})
}

// GetConnectionCount 获取当前连接数量
func GetConnectionCount() int {
	count := 0
	deviceIdToConnMap.Range(func(key, value interface{}) bool {
		count++
		return true
	})
	return count
}

// calculateChecksum 计算校验和
func calculateChecksum(data []byte) uint16 {
	var sum uint16
	for _, b := range data {
		sum += uint16(b)
	}
	return sum
}

// handleConnectionInitialData 处理连接建立初期的非DNY协议数据
// 这个函数在连接建立后的前几秒内监听和处理ICCID、link心跳等数据
func handleConnectionInitialData(conn ziface.IConnection, tcpConn *net.TCPConn) {
	const initialDataTimeout = 10 * time.Second // 10秒初始化超时
	const readBufferSize = 1024                 // 读取缓冲区大小

	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"timeout":    initialDataTimeout,
	}).Debug("开始监听连接初始化数据")

	// 设置初始化阶段的读取超时
	deadline := time.Now().Add(initialDataTimeout)
	if err := tcpConn.SetReadDeadline(deadline); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": connID,
			"error":  err.Error(),
		}).Error("设置初始化数据读取超时失败")
		return
	}

	buffer := make([]byte, readBufferSize)
	initialDataProcessed := false

	for time.Now().Before(deadline) && !initialDataProcessed {
		// 尝试读取数据
		n, err := tcpConn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// 读取超时，继续循环直到总体超时
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
				}).Debug("初始化数据读取超时，继续等待")
				time.Sleep(100 * time.Millisecond)
				continue
			} else {
				// 其他错误，可能是连接断开
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
					"error":      err.Error(),
				}).Debug("初始化数据读取失败，连接可能已断开")
				return
			}
		}

		if n > 0 {
			data := buffer[:n]

			// 记录接收到的初始化数据
			logger.WithFields(logrus.Fields{
				"connID":     connID,
				"remoteAddr": remoteAddr,
				"dataLen":    n,
				"dataHex":    hex.EncodeToString(data),
				"dataStr":    string(data),
			}).Info("收到连接初始化数据")

			// 通知监视器收到原始数据
			GetGlobalMonitor().OnRawDataReceived(conn, data)

			// 处理不同类型的初始化数据
			processed := false

			// 1. 处理ICCID (20字节数字字符串)
			if n == 20 && isValidICCIDBytes(data) {
				iccidStr := string(data)
				conn.SetProperty(PropKeyICCID, iccidStr)
				BindDeviceIdToConnection(iccidStr, conn)

				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
					"iccid":      iccidStr,
				}).Info("处理连接初始化ICCID数据")
				processed = true
			}

			// 2. 处理link心跳
			if n == 4 && string(data) == LinkHeartbeat {
				now := time.Now().Unix()
				conn.SetProperty(PropKeyLastLink, now)
				conn.SetProperty(PropKeyConnStatus, ConnStatusActive)
				UpdateLastHeartbeatTime(conn)

				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
					"timestamp":  now,
				}).Info("处理连接初始化link心跳")
				processed = true
			}

			// 3. 处理十六进制编码数据
			if !processed && isHexEncodedData(data) {
				decoded, err := hex.DecodeString(string(data))
				if err == nil {
					logger.WithFields(logrus.Fields{
						"connID":      connID,
						"remoteAddr":  remoteAddr,
						"originalLen": n,
						"decodedLen":  len(decoded),
						"decodedHex":  hex.EncodeToString(decoded),
					}).Info("处理连接初始化十六进制编码数据")

					// 递归处理解码后的数据
					if handleInitialDataContent(conn, decoded) {
						processed = true
					}
				}
			}

			// 4. 检查是否为DNY协议数据
			if !processed && len(data) >= 3 && string(data[:3]) == "DNY" {
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
					"dataLen":    n,
				}).Info("连接初始化阶段收到DNY协议数据，结束初始化处理")

				// 将DNY数据推回给Zinx处理（这需要特殊处理）
				// 由于我们已经从TCP连接中读取了数据，需要确保这些数据能被Zinx处理
				// 这里我们设置一个标记，让后续的正常消息处理流程知道初始化已完成
				conn.SetProperty("InitialDataProcessed", true)
				initialDataProcessed = true

				// 重新设置正常的读取超时
				if err := tcpConn.SetReadDeadline(time.Now().Add(readDeadLine)); err != nil {
					logger.WithFields(logrus.Fields{
						"connID": connID,
						"error":  err.Error(),
					}).Error("重新设置正常读取超时失败")
				}

				// 由于我们无法直接将数据推回给Zinx，我们将数据保存起来
				// 并修改Unpack方法来检查这种情况
				conn.SetProperty("PendingDNYData", data)

				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
				}).Debug("初始化数据处理完成，移交给正常消息处理流程")
				return
			}

			// 如果数据已处理，继续监听更多初始化数据
			// 如果是未知数据，记录并继续
			if !processed {
				logger.WithFields(logrus.Fields{
					"connID":     connID,
					"remoteAddr": remoteAddr,
					"dataLen":    n,
					"dataHex":    hex.EncodeToString(data),
				}).Debug("收到未知初始化数据，继续监听")
			}
		}
	}

	// 初始化超时或完成，设置标记并恢复正常读取超时
	conn.SetProperty("InitialDataProcessed", true)
	if err := tcpConn.SetReadDeadline(time.Now().Add(readDeadLine)); err != nil {
		logger.WithFields(logrus.Fields{
			"connID": connID,
			"error":  err.Error(),
		}).Error("恢复正常读取超时失败")
	}

	logger.WithFields(logrus.Fields{
		"connID":     connID,
		"remoteAddr": remoteAddr,
		"processed":  initialDataProcessed,
	}).Debug("连接初始化数据处理完成")
}

// handleInitialDataContent 处理初始化数据内容
func handleInitialDataContent(conn ziface.IConnection, data []byte) bool {
	// 处理解码后的初始化数据

	// 检查ICCID
	if len(data) == 20 && isValidICCIDBytes(data) {
		iccidStr := string(data)
		conn.SetProperty(PropKeyICCID, iccidStr)
		BindDeviceIdToConnection(iccidStr, conn)
		return true
	}

	// 检查link心跳
	if len(data) == 4 && string(data) == LinkHeartbeat {
		now := time.Now().Unix()
		conn.SetProperty(PropKeyLastLink, now)
		conn.SetProperty(PropKeyConnStatus, ConnStatusActive)
		UpdateLastHeartbeatTime(conn)
		return true
	}

	// 检查是否为DNY协议数据（这种情况下不在初始化阶段处理）
	if len(data) >= 3 && string(data[:3]) == "DNY" {
		return false // 让正常流程处理
	}

	return false
}
