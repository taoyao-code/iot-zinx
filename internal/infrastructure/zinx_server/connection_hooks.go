package zinx_server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
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

	// 设置一个默认的临时设备ID，避免"no property found"错误
	tempDeviceId := fmt.Sprintf("TempID-Conn-%d", conn.GetConnID())
	conn.SetProperty(PropKeyDeviceId, tempDeviceId)

	// 启动一个goroutine处理特殊情况（如ICCID上报和"link"心跳）
	go handlePreConnectionPhase(conn)
}

// OnConnectionStop 当连接断开时的钩子函数
func OnConnectionStop(conn ziface.IConnection) {
	connID := conn.GetConnID()
	remoteAddr := conn.RemoteAddr().String()

	// 尝试获取设备ID
	deviceId, err := conn.GetProperty(PropKeyDeviceId)
	if err == nil && deviceId != nil {
		deviceIdStr := deviceId.(string)

		// 获取ICCID（如果有）
		iccid := ""
		if iccidVal, err := conn.GetProperty(PropKeyICCID); err == nil && iccidVal != nil {
			iccid = iccidVal.(string)
		}

		// 记录连接断开
		logger.WithFields(logrus.Fields{
			"deviceId":   deviceIdStr,
			"iccid":      iccid,
			"remoteAddr": remoteAddr,
			"connID":     connID,
		}).Info("设备连接断开")

		// 只有非临时设备ID才从映射中移除
		if !strings.HasPrefix(deviceIdStr, "TempID-") {
			deviceIdToConnMap.Delete(deviceIdStr)
			connIdToDeviceIdMap.Delete(connID)

			// 通知业务层设备离线
			deviceService := app.GetServiceManager().DeviceService
			go deviceService.HandleDeviceOffline(deviceIdStr, iccid)
		} else {
			// 临时连接断开
			logger.WithFields(logrus.Fields{
				"deviceId":   deviceIdStr,
				"remoteAddr": remoteAddr,
				"connID":     connID,
			}).Debug("临时连接断开")
		}
	} else {
		// 未绑定设备ID的连接断开（这种情况现在应该很少见）
		logger.WithFields(logrus.Fields{
			"remoteAddr": remoteAddr,
			"connID":     connID,
			"error":      "no property found",
		}).Info("未注册连接断开")
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

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"remoteAddr": conn.RemoteAddr().String(),
		"timeout":    timeoutSec,
	}).Debug("开始处理连接初始化阶段")

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
				}).Info("连接初始化完成（收到ICCID，等待DNY协议数据）")
				return
			}
			// 如果既没有收到ICCID，也没有检测到DNY协议头，记录但不断开连接
			// 可能是客户端连接后需要时间发送数据
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"remoteAddr": conn.RemoteAddr().String(),
			}).Warn("连接初始化超时，但保持连接等待后续数据")
			return
		default:
			// 设置读取超时，避免长时间阻塞
			_ = tcpConn.SetReadDeadline(time.Now().Add(1 * time.Second))

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
					}).Debug("收到EOF，已完成ICCID初始化")
					return
				}
				// 客户端断开连接，记录但不主动断开
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"remoteAddr": conn.RemoteAddr().String(),
				}).Debug("客户端断开连接（EOF）")
				return
			}

			// 处理其他错误
			if err != nil {
				// 检查连接是否仍然有效
				if _, getErr := conn.GetProperty(PropKeyRemoteAddr); getErr != nil {
					// 连接已无效
					logger.WithFields(logrus.Fields{
						"connID": conn.GetConnID(),
						"error":  err.Error(),
					}).Debug("连接已关闭，停止读取")
					return
				}

				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"remoteAddr": conn.RemoteAddr().String(),
					"error":      err.Error(),
				}).Warn("读取连接数据时出现错误，继续等待")
				continue
			}

			// 处理读取到的数据
			if n > 0 {
				logger.WithFields(logrus.Fields{
					"connID":     conn.GetConnID(),
					"dataLength": n,
					"dataHex":    fmt.Sprintf("%X", data[:n]),
				}).Debug("接收到数据")

				// 检查是否为"link"心跳
				if n == 4 && string(data[:4]) == LinkHeartbeat {
					now := time.Now().Unix()
					conn.SetProperty(PropKeyLastLink, now)
					logger.WithFields(logrus.Fields{
						"connID":     conn.GetConnID(),
						"remoteAddr": conn.RemoteAddr().String(),
					}).Debug("收到'link'心跳")
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
							"connID":     conn.GetConnID(),
							"remoteAddr": conn.RemoteAddr().String(),
							"iccid":      potentialICCID,
						}).Info("收到ICCID")

						// 更新设备ID，使用ICCID作为临时标识
						tempDeviceId := "TempID-" + potentialICCID
						conn.SetProperty(PropKeyDeviceId, tempDeviceId)

						// 不返回，继续等待后续数据
						continue
					}
				}

				// 检查是否为DNY协议头
				if n >= 3 && string(data[:3]) == "DNY" {
					logger.WithFields(logrus.Fields{
						"connID":     conn.GetConnID(),
						"remoteAddr": conn.RemoteAddr().String(),
					}).Debug("检测到DNY协议头，转交给Zinx处理")
					return
				}

				// 记录未识别的数据
				if n <= 100 {
					logger.WithFields(logrus.Fields{
						"connID":     conn.GetConnID(),
						"remoteAddr": conn.RemoteAddr().String(),
						"dataText":   string(data[:n]),
						"dataHex":    fmt.Sprintf("%X", data[:n]),
					}).Debug("收到未识别数据")
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

// UpdateDeviceStatus 更新设备状态（online/offline）
func UpdateDeviceStatus(deviceId string, status string) {
	logger.WithFields(logrus.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Debug("更新设备状态")

	// 根据状态调用相应的业务层方法
	deviceService := app.GetServiceManager().DeviceService
	switch status {
	case "online":
		// 在线状态更新，通常在设备活跃时调用
		go deviceService.HandleDeviceStatusUpdate(deviceId, status)
	case "offline":
		// 离线状态通常在连接断开时处理，这里主要用于记录
		go deviceService.HandleDeviceStatusUpdate(deviceId, status)
	default:
		go deviceService.HandleDeviceStatusUpdate(deviceId, status)
	}
}
