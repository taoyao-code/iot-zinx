package zinx_server

import (
	"bufio"
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/config"
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

	// ICCID最大长度
	MaxICCIDLength = 20
)

// 存储所有设备ID到连接的映射，用于消息转发
var (
	// deviceIdToConnMap 物理ID到连接的映射
	deviceIdToConnMap sync.Map // map[string]ziface.IConnection

	// connIdToDeviceIdMap 连接ID到物理ID的映射
	connIdToDeviceIdMap sync.Map // map[uint64]string
)

// OnConnectionStart 当连接建立时的钩子函数
func OnConnectionStart(conn ziface.IConnection) {
	// 获取远程地址
	remoteAddr := conn.RemoteAddr().String()

	// 记录连接建立
	logger.WithFields(logrus.Fields{
		"remoteAddr": remoteAddr,
		"connID":     conn.GetConnID(),
	}).Info("New connection established")

	// 记录远程地址到连接属性
	conn.SetProperty(PropKeyRemoteAddr, remoteAddr)

	// 启动一个goroutine处理特殊情况（如ICCID上报和"link"心跳）
	go handlePreConnectionPhase(conn)
}

// OnConnectionStop 当连接断开时的钩子函数
func OnConnectionStop(conn ziface.IConnection) {
	// 尝试获取设备ID
	deviceId, err := conn.GetProperty(PropKeyDeviceId)
	if err == nil {
		deviceIdStr := deviceId.(string)

		// 获取ICCID（如果有）
		iccid := ""
		if iccidVal, err := conn.GetProperty(PropKeyICCID); err == nil {
			iccid = iccidVal.(string)
		}

		// 记录连接断开
		logger.WithFields(logrus.Fields{
			"deviceId":   deviceIdStr,
			"iccid":      iccid,
			"remoteAddr": conn.RemoteAddr().String(),
			"connID":     conn.GetConnID(),
		}).Info("Connection closed")

		// 从映射中移除
		deviceIdToConnMap.Delete(deviceIdStr)
		connIdToDeviceIdMap.Delete(conn.GetConnID())

		// 通知业务层设备离线
		deviceService := app.GetServiceManager().DeviceService
		go deviceService.HandleDeviceOffline(deviceIdStr, iccid)
	} else {
		// 未绑定设备ID的连接断开
		logger.WithFields(logrus.Fields{
			"remoteAddr": conn.RemoteAddr().String(),
			"connID":     conn.GetConnID(),
			"error":      err.Error(),
		}).Info("Unregistered connection closed")
	}
}

// handlePreConnectionPhase 处理连接建立初期的特殊情况
// 主要处理ICCID上报和"link"心跳
func handlePreConnectionPhase(conn ziface.IConnection) {
	// 获取配置的超时时间
	timeoutSec := config.GlobalConfig.Timeouts.DeviceInitSeconds
	if timeoutSec <= 0 {
		timeoutSec = 30 // 默认30秒
	}

	// 创建上下文，用于控制初始化超时
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	// 获取原始TCP连接
	tcpConn, ok := conn.GetTCPConnection().(*net.TCPConn)
	if !ok {
		logger.WithField("connID", conn.GetConnID()).Error("Failed to get TCP connection")
		conn.Stop()
		return
	}

	// 创建一个reader
	reader := bufio.NewReader(tcpConn)

	// 标记是否已获取ICCID
	var iccidReceived bool

	// 循环读取数据，直到获取到ICCID或者超时
	for {
		select {
		case <-ctx.Done():
			// 初始化超时，但如果收到过ICCID，我们不应断开连接
			if iccidReceived {
				iccidVal, _ := conn.GetProperty(PropKeyICCID)
				logger.WithFields(logrus.Fields{
					"connID": conn.GetConnID(),
					"iccid":  iccidVal,
				}).Info("连接初始化完成（只收到ICCID，未检测到DNY协议头）")
				return
			}
			// 如果既没有收到ICCID，也没有检测到DNY协议头，则断开连接
			logger.WithField("connID", conn.GetConnID()).Warn("连接初始化超时，未收到ICCID和DNY协议头")
			conn.Stop()
			return
		default:
			// 设置读取超时，避免阻塞
			_ = tcpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))

			// 尝试读取数据
			data := make([]byte, 1024)
			n, err := reader.Read(data)

			// 处理超时错误
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // 超时，继续尝试读取
			}

			// 处理EOF错误
			if err == io.EOF {
				// 连接可能已关闭，但如果已经收到ICCID，我们可以保持连接
				if iccidReceived {
					iccidVal, _ := conn.GetProperty(PropKeyICCID)
					logger.WithFields(logrus.Fields{
						"connID": conn.GetConnID(),
						"iccid":  iccidVal,
					}).Info("收到EOF，但已完成ICCID初始化")
					return
				}
				// 否则断开连接
				logger.WithField("connID", conn.GetConnID()).Warn("收到EOF，未完成初始化")
				conn.Stop()
				return
			}

			// 处理其他错误
			if err != nil {
				logger.WithFields(logrus.Fields{
					"connID": conn.GetConnID(),
					"error":  err.Error(),
				}).Error("从连接读取数据出错")
				conn.Stop()
				return
			}

			// 处理读取到的数据
			if n > 0 {
				// 检查是否为"link"心跳
				if n == 4 && string(data[:4]) == LinkHeartbeat {
					now := time.Now().Unix()
					conn.SetProperty(PropKeyLastLink, now)
					logger.WithField("connID", conn.GetConnID()).Debug("收到'link'心跳")
					continue
				}

				// 检查是否为ICCID（如果尚未获取）
				if !iccidReceived && n <= MaxICCIDLength {
					// 简单验证：ICCID通常是纯数字字符串
					potentialICCID := string(data[:n])
					isValid := true
					for _, c := range potentialICCID {
						if c < '0' || c > '9' {
							isValid = false
							break
						}
					}

					if isValid && len(potentialICCID) >= 10 {
						// 保存ICCID到连接属性
						conn.SetProperty(PropKeyICCID, potentialICCID)
						iccidReceived = true

						logger.WithFields(logrus.Fields{
							"connID": conn.GetConnID(),
							"iccid":  potentialICCID,
						}).Info("收到ICCID")

						// 设置一个临时物理ID，防止连接因"no property found"错误而关闭
						// 等待真正的注册包到来时会更新此值
						conn.SetProperty(PropKeyDeviceId, "TempID-"+potentialICCID)

						// 不返回，继续等待后续数据
					}
				}

				// 检查是否为DNY协议头
				if n >= 3 && string(data[:3]) == "DNY" {
					logger.WithField("connID", conn.GetConnID()).Debug("检测到DNY协议头，返回控制权给Zinx")
					return
				}
			}
		}
	}
}

// BindDeviceIdToConnection 将设备ID绑定到连接
func BindDeviceIdToConnection(deviceId string, conn ziface.IConnection) {
	// 更新映射
	deviceIdToConnMap.Store(deviceId, conn)
	connIdToDeviceIdMap.Store(conn.GetConnID(), deviceId)

	// 设置连接属性
	conn.SetProperty(PropKeyDeviceId, deviceId)

	// 记录绑定信息
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

// UpdateLastHeartbeatTime 更新最后一次心跳时间
func UpdateLastHeartbeatTime(conn ziface.IConnection) {
	now := time.Now().Unix()
	conn.SetProperty(PropKeyLastHeartbeat, now)
}
