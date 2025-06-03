package handlers

import (
	"fmt"
	"time"

	"github.com/bujia-iot/iot-zinx/pkg"
	"github.com/bujia-iot/iot-zinx/pkg/constants"

	"github.com/aceld/zinx/ziface"
	"github.com/bujia-iot/iot-zinx/internal/app"
	"github.com/bujia-iot/iot-zinx/internal/domain/dny_protocol"
	"github.com/bujia-iot/iot-zinx/internal/infrastructure/logger"
	"github.com/bujia-iot/iot-zinx/pkg/monitor"
	"github.com/sirupsen/logrus"
)

// DeviceRegisterHandler 处理设备注册请求 (命令ID: 0x20)
type DeviceRegisterHandler struct {
	DNYHandlerBase
}

// 预处理
func (h *DeviceRegisterHandler) PreHandle(request ziface.IRequest) {
	// 🔧 关键修复：调用基类PreHandle确保命令确认逻辑执行
	// 这将调用CommandManager.ConfirmCommand()以避免超时重传
	h.DNYHandlerBase.PreHandle(request)

	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("收到设备注册请求")
}

// Handle 处理设备注册请求
func (h *DeviceRegisterHandler) Handle(request ziface.IRequest) {
	// 获取请求消息
	msg := request.GetMessage()
	conn := request.GetConnection()

	// 🔧 修复：处理标准Zinx消息，直接获取纯净的DNY数据
	data := msg.GetData()

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"msgID":       msg.GetMsgID(),
		"messageType": fmt.Sprintf("%T", msg),
		"dataLen":     len(data),
	}).Info("✅ 设备注册处理器：开始处理标准Zinx消息")

	// 🔧 关键修复：从DNY协议消息中获取真实的PhysicalID
	var physicalId uint32
	if dnyMsg, ok := msg.(*dny_protocol.Message); ok {
		physicalId = dnyMsg.GetPhysicalId()
		fmt.Printf("🔧 从DNY协议消息获取真实PhysicalID: 0x%08X\n", physicalId)
	} else {
		// 从连接属性中获取PhysicalID
		if prop, err := conn.GetProperty("DNY_PhysicalID"); err == nil {
			if pid, ok := prop.(uint32); ok {
				physicalId = pid
				fmt.Printf("🔧 从连接属性获取PhysicalID: 0x%08X\n", physicalId)
			}
		}
		if physicalId == 0 {
			logger.WithFields(logrus.Fields{
				"connID": conn.GetConnID(),
				"msgID":  msg.GetMsgID(),
			}).Error("无法获取PhysicalID，设备注册失败")
			return
		}
	}

	// 🔧 重要修复：从连接属性获取ICCID，因为ICCID是通过单独的特殊消息发送的
	var iccid string
	if prop, err := conn.GetProperty(constants.PropKeyICCID); err == nil {
		if iccidStr, ok := prop.(string); ok {
			iccid = iccidStr
			fmt.Printf("🔧 从连接属性获取ICCID: %s\n", iccid)
		}
	}
	if iccid == "" {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
		}).Error("无法获取ICCID，设备注册失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":     conn.GetConnID(),
		"physicalID": fmt.Sprintf("0x%08X", physicalId),
		"dataLen":    len(data),
	}).Info("设备注册处理器：处理标准Zinx数据格式")

	// 解析设备注册数据
	registerData := &dny_protocol.DeviceRegisterData{}
	if err := registerData.UnmarshalBinary(data); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"dataLen":    len(data),
			"dataHex":    fmt.Sprintf("%x", data),
			"error":      err.Error(),
		}).Error("设备注册数据解析失败")

		// 🔧 新增：发送错误响应而不是直接返回
		responseData := []byte{dny_protocol.ResponseFailed}
		messageID := uint16(time.Now().Unix() & 0xFFFF)
		if sendErr := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdDeviceRegister), responseData); sendErr != nil {
			logger.WithFields(logrus.Fields{
				"connID":     conn.GetConnID(),
				"physicalId": fmt.Sprintf("0x%08X", physicalId),
				"sendError":  sendErr.Error(),
			}).Error("发送设备注册错误响应失败")
		}
		return
	}

	// 🔧 重要：将解析出的ICCID与连接属性中的ICCID合并
	if registerData.ICCID == "" {
		registerData.ICCID = iccid // 使用从连接属性获取的ICCID
	}

	logger.WithFields(logrus.Fields{
		"connID":          conn.GetConnID(),
		"physicalId":      fmt.Sprintf("0x%08X", physicalId),
		"iccid":           registerData.ICCID,
		"deviceType":      registerData.DeviceType,
		"deviceVersion":   string(registerData.DeviceVersion[:]),
		"heartbeatPeriod": registerData.HeartbeatPeriod,
	}).Info("收到设备注册请求")

	// 将设备ID绑定到连接
	deviceIdStr := fmt.Sprintf("%08X", physicalId)

	// 存储ICCID - 🔧 修复：不要重复声明iccid变量
	conn.SetProperty(constants.PropKeyICCID, iccid)

	// 🔧 重构：支持多设备管理的会话处理
	sessionManager := monitor.GetSessionManager()
	var session *monitor.DeviceSession
	var isReconnect bool

	// 1. 检查该设备是否已有会话（设备重连）
	if existSession, exists := sessionManager.GetSession(deviceIdStr); exists {
		session = existSession
		isReconnect = true

		logger.WithFields(logrus.Fields{
			"deviceID":  deviceIdStr,
			"iccid":     iccid,
			"sessionID": existSession.SessionID,
		}).Info("设备重连，恢复现有会话")

		// 恢复会话
		sessionManager.ResumeSession(deviceIdStr, conn)
	} else {
		// 2. 新设备注册，检查同一ICCID下是否有其他设备
		existingDevices := sessionManager.GetAllSessionsByICCID(iccid)

		if len(existingDevices) > 0 {
			logger.WithFields(logrus.Fields{
				"newDeviceID":     deviceIdStr,
				"iccid":           iccid,
				"existingDevices": len(existingDevices),
			}).Info("同一ICCID下发现其他设备，支持多设备并发")

			// 记录现有设备信息
			for existingDeviceID := range existingDevices {
				logger.WithFields(logrus.Fields{
					"iccid":            iccid,
					"existingDeviceID": existingDeviceID,
					"newDeviceID":      deviceIdStr,
				}).Debug("ICCID下的现有设备")
			}
		}

		// 3. 创建新的设备会话
		session = sessionManager.CreateSession(deviceIdStr, conn)
		isReconnect = false

		logger.WithFields(logrus.Fields{
			"deviceID":  deviceIdStr,
			"iccid":     iccid,
			"sessionID": session.SessionID,
		}).Info("创建新设备会话")
	}

	// 绑定设备ID到连接
	pkg.Monitor.GetGlobalMonitor().BindDeviceIdToConnection(deviceIdStr, conn)

	// 通知业务层设备上线
	deviceService := app.GetServiceManager().DeviceService
	go deviceService.HandleDeviceOnline(deviceIdStr, iccid)

	// 构建响应数据
	responseData := make([]byte, 5)
	responseData[0] = dny_protocol.ResponseSuccess        // 成功
	responseData[1] = uint8(registerData.DeviceType)      // 设备类型
	responseData[2] = uint8(registerData.DeviceType >> 8) // 设备类型高位
	responseData[3] = 0                                   // 预留
	responseData[4] = 0                                   // 预留

	// 发送响应
	// 生成消息ID
	messageID := uint16(time.Now().Unix() & 0xFFFF)
	if err := pkg.Protocol.SendDNYResponse(conn, physicalId, messageID, uint8(dny_protocol.CmdDeviceRegister), responseData); err != nil {
		logger.WithFields(logrus.Fields{
			"connID":     conn.GetConnID(),
			"physicalId": fmt.Sprintf("0x%08X", physicalId),
			"error":      err.Error(),
		}).Error("发送设备注册响应失败")
		return
	}

	logger.WithFields(logrus.Fields{
		"connID":      conn.GetConnID(),
		"physicalId":  fmt.Sprintf("0x%08X", physicalId),
		"deviceId":    deviceIdStr,
		"isReconnect": isReconnect,
		"iccid":       iccid,
	}).Debug("设备注册响应发送成功")

	// 更新心跳时间
	pkg.Monitor.GetGlobalMonitor().UpdateLastHeartbeatTime(conn)
}

// PostHandle 后处理设备注册请求
func (h *DeviceRegisterHandler) PostHandle(request ziface.IRequest) {
	logger.WithFields(logrus.Fields{
		"connID":     request.GetConnection().GetConnID(),
		"remoteAddr": request.GetConnection().RemoteAddr().String(),
	}).Debug("设备注册请求处理完成")
}
